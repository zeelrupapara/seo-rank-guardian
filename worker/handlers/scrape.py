import json
import time

import structlog

from models import JobRun, SearchPair, SearchResult

log = structlog.get_logger()


async def handle_scrape_task(worker, data: bytes):
    msg = json.loads(data)
    pair_id = msg["pair_id"]
    run_id = msg["run_id"]
    job_id = msg["job_id"]
    search_query = msg["search_query"]
    keyword = msg["keyword"]
    state = msg["state"]
    country = msg.get("country", "us")
    domain = msg["domain"]
    competitors = msg.get("competitors", [])

    log.info("Processing scrape pair", pair_id=pair_id, query=search_query)

    # Idempotency check
    with worker.session_factory() as session:
        pair = session.query(SearchPair).filter(SearchPair.id == pair_id).first()
        if pair and pair.status in ("completed", "failed"):
            log.info("Pair already processed, skipping", pair_id=pair_id, status=pair.status)
            return

    # Emit scrape_started
    await worker.publish_event({
        "type": "scrape_started",
        "run_id": run_id,
        "job_id": job_id,
        "payload": {
            "pair_id": pair_id,
            "keyword": keyword,
            "state": state,
            "message": "Scraping started",
        },
    })

    # Mark pair as running
    now = time.time_ns()
    with worker.session_factory() as session:
        session.query(SearchPair).filter(SearchPair.id == pair_id).update(
            {"status": "running", "started_at": now}
        )
        session.commit()

    # Callback to stream retry/fallback events to LiveMonitor
    async def search_event_cb(event_kind: str, details: dict):
        if event_kind == "attempt_failed":
            await worker.publish_event({
                "type": "scrape_retry",
                "run_id": run_id,
                "job_id": job_id,
                "payload": {
                    "pair_id": pair_id,
                    "keyword": keyword,
                    "state": state,
                    "message": f"Attempt {details['attempt']}/{details['max_retries']} failed: {details['error']}",
                    "attempt": details["attempt"],
                    "max_retries": details["max_retries"],
                    "method": details["method"],
                    "is_retryable": details.get("is_retryable", False),
                    "retry_delay": details.get("retry_delay", 0),
                },
            })
        elif event_kind in ("fallback", "fallback_success", "fallback_failed"):
            await worker.publish_event({
                "type": "scrape_fallback",
                "run_id": run_id,
                "job_id": job_id,
                "payload": {
                    "pair_id": pair_id,
                    "keyword": keyword,
                    "state": state,
                    "message": f"Falling back to {details['method']}" if event_kind == "fallback"
                        else f"Serper API {'succeeded' if event_kind == 'fallback_success' else 'failed'}"
                        + (f": {details.get('error', '')}" if event_kind == "fallback_failed" else ""),
                    "method": details.get("method", ""),
                    "reason": details.get("reason", ""),
                },
            })

    # Scrape Google
    try:
        results, search_method = await worker.scraper.search_async(
            query=search_query,
            region=country.lower(),
            language="en",
            result_limit=worker.settings.scrape_result_limit,
            on_event=search_event_cb,
        )
    except Exception as e:
        search_method = "all_failed"
        log.error("Scrape failed", pair_id=pair_id, error=str(e))
        finished_at = time.time_ns()
        with worker.session_factory() as session:
            session.query(SearchPair).filter(SearchPair.id == pair_id).update(
                {"status": "failed", "error_msg": str(e), "finished_at": finished_at}
            )
            session.query(JobRun).filter(JobRun.id == run_id).update(
                {"failed_pairs": JobRun.failed_pairs + 1}
            )
            session.commit()

        await worker.publish_event({
            "type": "scrape_failed",
            "run_id": run_id,
            "job_id": job_id,
            "payload": {
                "pair_id": pair_id,
                "keyword": keyword,
                "state": state,
                "message": "Scrape failed",
                "error": str(e),
                "search_method": search_method,
            },
        })

        await _check_run_completion(worker, run_id, job_id)
        return

    # Clean target domain for matching
    target_domain = domain.removeprefix("https://").removeprefix("http://").removeprefix("www.").lower()

    # Store results
    target_position = 0
    with worker.session_factory() as session:
        for r in results:
            clean_domain = r.domain.removeprefix("www.").lower()

            is_target = clean_domain == target_domain
            is_competitor = False
            for comp in competitors:
                clean_comp = comp.removeprefix("https://").removeprefix("http://").removeprefix("www.").lower()
                if clean_domain == clean_comp:
                    is_competitor = True
                    break

            result = SearchResult(
                pair_id=pair_id,
                run_id=run_id,
                job_id=job_id,
                domain=r.domain,
                position=r.position,
                url=r.url,
                title=r.title,
                snippet=r.snippet,
                is_target=is_target,
                is_competitor=is_competitor,
                keyword=keyword,
                state=state,
            )
            session.add(result)

            if is_target and target_position == 0:
                target_position = r.position

        # Mark pair completed
        finished_at = time.time_ns()
        session.query(SearchPair).filter(SearchPair.id == pair_id).update(
            {"status": "completed", "finished_at": finished_at}
        )
        session.query(JobRun).filter(JobRun.id == run_id).update(
            {"completed_pairs": JobRun.completed_pairs + 1}
        )
        session.commit()

    # Emit scrape_complete
    await worker.publish_event({
        "type": "scrape_complete",
        "run_id": run_id,
        "job_id": job_id,
        "payload": {
            "pair_id": pair_id,
            "keyword": keyword,
            "state": state,
            "message": "Scrape complete",
            "position": target_position,
            "result_count": len(results),
            "domain": domain,
            "search_method": search_method,
        },
    })

    # Emit run_progress
    with worker.session_factory() as session:
        run = session.query(JobRun).filter(JobRun.id == run_id).first()
        if run:
            await worker.publish_event({
                "type": "run_progress",
                "run_id": run_id,
                "job_id": job_id,
                "payload": {
                    "message": "Progress",
                    "completed_pairs": run.completed_pairs,
                    "total_pairs": run.total_pairs,
                },
            })

    log.info("Completed scrape pair", pair_id=pair_id, results=len(results), search_method=search_method, keyword=keyword)
    await _check_run_completion(worker, run_id, job_id)


async def _check_run_completion(worker, run_id: int, job_id: int):
    with worker.session_factory() as session:
        # Use FOR UPDATE to prevent two workers from finalizing the same run
        run = session.query(JobRun).filter(JobRun.id == run_id).with_for_update().first()
        if not run:
            log.error("Failed to load run for completion check", run_id=run_id)
            return

        if run.completed_pairs + run.failed_pairs < run.total_pairs:
            return

        # Already finalized by another worker
        if run.status in ("completed", "partial", "failed"):
            return

        # Capture values before session closes
        completed_pairs = run.completed_pairs
        failed_pairs = run.failed_pairs
        total_pairs = run.total_pairs

        now = time.time_ns()
        if failed_pairs > 0 and completed_pairs == 0:
            status = "failed"
        elif failed_pairs > 0:
            status = "partial"
        else:
            status = "completed"

        session.query(JobRun).filter(JobRun.id == run_id).update(
            {"status": status, "completed_at": now}
        )
        session.commit()

    log.info(
        "Run finished",
        run_id=run_id,
        status=status,
        completed=completed_pairs,
        failed=failed_pairs,
    )

    if status == "failed":
        await worker.publish_event({
            "type": "run_failed",
            "run_id": run_id,
            "job_id": job_id,
            "payload": {
                "message": "Run failed",
                "total_pairs": total_pairs,
            },
        })
        return

    await worker.publish_event({
        "type": "run_complete",
        "run_id": run_id,
        "job_id": job_id,
        "payload": {
            "message": f"Run completed with status: {status}",
            "total_pairs": total_pairs,
        },
    })

    # Trigger change detection
    await worker.publish_nats("srg.jobs.detect", {"run_id": run_id, "job_id": job_id})
