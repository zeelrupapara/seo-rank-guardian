import json

import structlog

from models import Job, RankDiff, Report, SearchResult
from ai.client import AnalyzeOptions
from ai.gemini_api import extract_json
from ai.prompt import SEO_SYSTEM_INSTRUCTION, REPORT_SCHEMA, build_report_content

log = structlog.get_logger()

MAX_AI_RETRIES = 2


async def handle_report_generation(worker, data: bytes):
    msg = json.loads(data)
    run_id = msg["run_id"]
    job_id = msg["job_id"]

    log.info("Generating report", run_id=run_id, job_id=job_id)

    # Emit report_started
    await worker.publish_event({
        "type": "report_started",
        "run_id": run_id,
        "job_id": job_id,
        "payload": {"message": "Report generation started"},
    })

    with worker.session_factory() as session:
        # Get job (unscoped — may be soft-deleted)
        job = session.query(Job).filter(Job.id == job_id).first()
        if job is None:
            log.warning("Job not found, skipping report", job_id=job_id)
            return

        # Get all results for this run
        results = (
            session.query(SearchResult)
            .filter(SearchResult.run_id == run_id, SearchResult.deleted_at == 0)
            .order_by(SearchResult.keyword, SearchResult.state, SearchResult.position)
            .all()
        )

        # Get diffs for this run
        diffs = (
            session.query(RankDiff)
            .filter(RankDiff.run_id == run_id, RankDiff.deleted_at == 0)
            .all()
        )

        # Check AI client
        if worker.ai_client is None:
            log.warning("AI client not configured, skipping report")
            await worker.publish_event({
                "type": "report_failed",
                "run_id": run_id,
                "job_id": job_id,
                "payload": {
                    "message": "Report generation skipped",
                    "error": "AI client not configured",
                },
            })
            return

        # Idempotency: skip if report exists
        existing = session.query(Report).filter(Report.run_id == run_id, Report.deleted_at == 0).first()
        if existing is not None:
            log.info("Report already exists", run_id=run_id, status=existing.status)
            return

        # Build prompt
        report_results = [
            {
                "keyword": r.keyword,
                "state": r.state,
                "position": r.position,
                "domain": r.domain,
                "url": r.url,
                "title": r.title,
                "snippet": r.snippet,
                "is_target": r.is_target,
                "is_competitor": r.is_competitor,
            }
            for r in results
        ]
        report_diffs = [
            {
                "domain": d.domain,
                "keyword": d.keyword,
                "state": d.state,
                "change_type": d.change_type,
                "prev_position": d.prev_position,
                "curr_position": d.curr_position,
                "delta": d.delta,
            }
            for d in diffs
        ]

        user_content = build_report_content(
            job.domain, job.get_competitors(), report_results, report_diffs
        )

        # Create report record
        report = Report(
            job_id=job_id,
            run_id=run_id,
            provider=worker.settings.ai_provider,
            model=worker.settings.ai_model,
            prompt=user_content,
            status="generating",
        )
        session.add(report)
        session.commit()
        report_id = report.id

    # Call AI with retry on invalid JSON
    result_json = None
    last_error = None

    for attempt in range(1, MAX_AI_RETRIES + 1):
        try:
            ai_result = await worker.ai_client.analyze_structured(AnalyzeOptions(
                system_instruction=SEO_SYSTEM_INSTRUCTION,
                user_content=user_content,
                response_schema=REPORT_SCHEMA,
                enable_search=worker.settings.ai_search_grounding,
            ))
        except Exception as e:
            log.error("AI analysis failed", run_id=run_id, attempt=attempt, error=str(e))
            last_error = e
            continue

        # Extract JSON from response (handles code fences, surrounding text, etc.)
        try:
            result_json = extract_json(ai_result.content)
            break
        except ValueError as e:
            log.warning(
                "AI returned invalid JSON, retrying",
                run_id=run_id,
                attempt=attempt,
                content_preview=ai_result.content[:300],
            )
            last_error = e

    if result_json is None:
        log.error("AI report failed after retries", run_id=run_id, error=str(last_error))
        with worker.session_factory() as session:
            session.query(Report).filter(Report.id == report_id).update({"status": "failed"})
            session.commit()

        await worker.publish_event({
            "type": "report_failed",
            "run_id": run_id,
            "job_id": job_id,
            "payload": {"message": "Report generation failed", "error": str(last_error)},
        })
        return

    # Update report
    with worker.session_factory() as session:
        updates = {
            "result": result_json,
            "status": "generated",
        }
        grounding = getattr(ai_result, "grounding_meta", None)
        if grounding is not None:
            updates["grounding_meta"] = grounding
        session.query(Report).filter(Report.id == report_id).update(updates)
        session.commit()

    await worker.publish_event({
        "type": "report_complete",
        "run_id": run_id,
        "job_id": job_id,
        "payload": {"message": "AI report generated"},
    })

    log.info("Report generated", run_id=run_id)
