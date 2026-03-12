import json

import structlog

from models import JobRun, RankDiff, SearchResult

log = structlog.get_logger()


async def handle_change_detection(worker, data: bytes):
    msg = json.loads(data)
    run_id = msg["run_id"]
    job_id = msg["job_id"]

    log.info("Running change detection", run_id=run_id, job_id=job_id)

    # Idempotency: skip if diffs already exist
    with worker.session_factory() as session:
        existing = session.query(RankDiff).filter(RankDiff.run_id == run_id).count()
        if existing > 0:
            log.info("Diffs already exist, skipping", run_id=run_id)
            await _trigger_report(worker, run_id, job_id)
            return

    # Emit detect_started
    await worker.publish_event({
        "type": "detect_started",
        "run_id": run_id,
        "job_id": job_id,
        "payload": {"message": "Change detection started"},
    })

    with worker.session_factory() as session:
        # Find previous successful run
        prev_run = (
            session.query(JobRun)
            .filter(
                JobRun.job_id == job_id,
                JobRun.id < run_id,
                JobRun.status.in_(["completed", "partial"]),
                JobRun.deleted_at == 0,
            )
            .order_by(JobRun.id.desc())
            .first()
        )

        if prev_run is None:
            log.info("No previous run found, skipping detection", job_id=job_id)
            await _trigger_report(worker, run_id, job_id)
            return

        # Get current results (target + competitor only)
        current_results = (
            session.query(SearchResult)
            .filter(
                SearchResult.run_id == run_id,
                (SearchResult.is_target == True) | (SearchResult.is_competitor == True),
                SearchResult.deleted_at == 0,
            )
            .all()
        )

        # Get previous results
        prev_results = (
            session.query(SearchResult)
            .filter(
                SearchResult.run_id == prev_run.id,
                (SearchResult.is_target == True) | (SearchResult.is_competitor == True),
                SearchResult.deleted_at == 0,
            )
            .all()
        )

        # Build lookup maps: "keyword|state|domain" -> position
        prev_positions = {}
        for r in prev_results:
            key = f"{r.keyword}|{r.state}|{r.domain}"
            prev_positions[key] = r.position

        curr_positions = {}
        for r in current_results:
            key = f"{r.keyword}|{r.state}|{r.domain}"
            curr_positions[key] = r.position

        improved_count = 0
        dropped_count = 0
        new_count = 0
        lost_count = 0

        # Detect changes for current results
        for r in current_results:
            key = f"{r.keyword}|{r.state}|{r.domain}"
            prev_pos = prev_positions.get(key, 0)
            curr_pos = r.position

            if prev_pos == 0:
                change_type = "new"
                delta = 0
                new_count += 1
            else:
                delta = curr_pos - prev_pos
                if delta < 0:
                    change_type = "improved"
                    improved_count += 1
                elif delta > 0:
                    change_type = "dropped"
                    dropped_count += 1
                else:
                    continue  # no change

            diff = RankDiff(
                job_id=job_id,
                run_id=run_id,
                prev_run_id=prev_run.id,
                domain=r.domain,
                prev_position=prev_pos,
                curr_position=curr_pos,
                delta=delta,
                change_type=change_type,
                keyword=r.keyword,
                state=r.state,
            )
            session.add(diff)

        # Detect "lost" — in previous but not in current
        for r in prev_results:
            key = f"{r.keyword}|{r.state}|{r.domain}"
            if key not in curr_positions:
                lost_count += 1
                diff = RankDiff(
                    job_id=job_id,
                    run_id=run_id,
                    prev_run_id=prev_run.id,
                    domain=r.domain,
                    prev_position=r.position,
                    curr_position=0,
                    delta=r.position,
                    change_type="lost",
                    keyword=r.keyword,
                    state=r.state,
                )
                session.add(diff)

        session.commit()

    log.info("Change detection completed", run_id=run_id)

    await worker.publish_event({
        "type": "detect_complete",
        "run_id": run_id,
        "job_id": job_id,
        "payload": {
            "message": "Change detection complete",
            "improved": improved_count,
            "dropped": dropped_count,
            "new": new_count,
            "lost": lost_count,
        },
    })

    await _trigger_report(worker, run_id, job_id)


async def _trigger_report(worker, run_id: int, job_id: int):
    await worker.publish_nats("srg.jobs.report", {"run_id": run_id, "job_id": job_id})
