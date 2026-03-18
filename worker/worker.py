import asyncio
import json
import time

import nats
from nats.js.api import ConsumerConfig, RetentionPolicy, StreamConfig
import structlog

from config import Settings
from models import Job, RunEventLog
from db import create_db
from sqlalchemy.orm import sessionmaker, Session

log = structlog.get_logger()


class Worker:
    def __init__(
        self,
        settings: Settings,
        session_factory: sessionmaker[Session],
        scraper,
        ai_client,
    ):
        self.settings = settings
        self.session_factory = session_factory
        self.scraper = scraper
        self.ai_client = ai_client
        self.nc = None
        self.js = None
        self.sub = None
        self._job_owner_cache: dict[int, int] = {}

    async def connect(self):
        self.nc = await nats.connect(self.settings.nats_url)
        self.js = self.nc.jetstream()
        log.info("Connected to NATS JetStream", url=self.settings.nats_url)

    async def _ensure_stream(self, name: str, subjects: list[str]):
        try:
            await self.js.find_stream_name_by_subject(subjects[0])
            log.info("Stream already exists", stream=name)
        except Exception:
            await self.js.add_stream(
                StreamConfig(
                    name=name,
                    subjects=subjects,
                    retention=RetentionPolicy.LIMITS,
                )
            )
            log.info("Stream created", stream=name)

    async def listen(self):
        await self._ensure_stream("SRG_JOBS", ["srg.jobs.>"])
        await self._ensure_stream("SRG_LOGS", ["srg.logs.>"])
        await self._ensure_stream("SRG_WS", ["srg.ws.>"])

        # One-time migration: remove old non-queue consumer
        try:
            await self.js.delete_consumer("SRG_JOBS", "srg-worker-py")
            log.info("Deleted old consumer", consumer="srg-worker-py")
        except Exception:
            pass

        # Queue group config — enables horizontal scaling (N workers, round-robin delivery)
        config = ConsumerConfig(
            ack_wait=600,          # 10 min — longest scrape can take 5-6 min
            max_deliver=3,         # Retry up to 3 times on failure
            max_ack_pending=10,    # Prefetch limit per worker instance
        )

        # Push-based subscription with queue group
        try:
            self.sub = await self.js.subscribe(
                "srg.jobs.>",
                queue="srg-workers",
                config=config,
                manual_ack=True,
            )
        except Exception as e:
            # Consumer config may conflict — delete and recreate
            log.warning("Subscribe failed, recreating consumer", error=str(e))
            try:
                await self.js.delete_consumer("SRG_JOBS", "srg-workers")
            except Exception:
                pass
            self.sub = await self.js.subscribe(
                "srg.jobs.>",
                queue="srg-workers",
                config=config,
                manual_ack=True,
            )

        log.info("Worker listening for jobs on srg.jobs.>")

        async for msg in self.sub.messages:
            subject = msg.subject
            log.info("Received job", subject=subject)
            try:
                await self._handle_job(subject, msg.data)
                await msg.ack()
            except Exception as e:
                log.error("Job handler error", error=str(e), subject=subject)
                await msg.nak()

    async def _handle_job(self, subject: str, data: bytes):
        from handlers.scrape import handle_scrape_task
        from handlers.detect import handle_change_detection
        from handlers.report import handle_report_generation

        if subject.startswith("srg.jobs.scrape"):
            await handle_scrape_task(self, data)
        elif subject.startswith("srg.jobs.detect"):
            await handle_change_detection(self, data)
        elif subject.startswith("srg.jobs.report"):
            await handle_report_generation(self, data)
        else:
            log.warning("No handler for subject", subject=subject)
            raise ValueError(f"unknown job subject: {subject}")

    def get_job_owner(self, job_id: int) -> int:
        if job_id in self._job_owner_cache:
            return self._job_owner_cache[job_id]

        with self.session_factory() as session:
            # Unscoped — ignore soft deletes
            job = session.query(Job.user_id).filter(Job.id == job_id).first()
            if job is None:
                raise ValueError(f"job {job_id} not found")
            user_id = job.user_id

        self._job_owner_cache[job_id] = user_id
        return user_id

    async def publish_event(self, event: dict):
        if event.get("timestamp") is None or event["timestamp"] == 0:
            event["timestamp"] = time.time_ns()

        data = json.dumps(event).encode()

        # Persist to DB
        with self.session_factory() as session:
            log_entry = RunEventLog(
                run_id=event["run_id"],
                job_id=event["job_id"],
                event_type=event["type"],
                data=event,
            )
            session.add(log_entry)
            session.commit()

        # Publish raw event to per-run subject
        subject = f"srg.logs.{event['job_id']}.{event['run_id']}"
        await self.nc.publish(subject, data)

        # Publish wrapped event to per-user WS subject
        try:
            user_id = self.get_job_owner(event["job_id"])
            ws_event = {"type": "logs", "payload": event}
            ws_data = json.dumps(ws_event).encode()
            user_subject = f"srg.ws.{user_id}"
            await self.nc.publish(user_subject, ws_data)
        except Exception as e:
            log.error("Failed to publish WS event", error=str(e))

    async def publish_nats(self, subject: str, data: dict):
        encoded = json.dumps(data).encode()
        await self.js.publish(subject, encoded)

    async def stop(self):
        if self.sub:
            await self.sub.unsubscribe()
        if self.nc:
            await self.nc.close()
        log.info("Worker stopped")
