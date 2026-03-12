import asyncio
import signal

import structlog
from dotenv import load_dotenv

from config import Settings
from db import create_db
from scraper.google_search import GoogleScraper
from ai.client import create_ai_client
from worker import Worker


def setup_logging(level: str):
    structlog.configure(
        wrapper_class=structlog.make_filtering_bound_logger(
            {"debug": 10, "info": 20, "warning": 30, "error": 40}.get(level.lower(), 20)
        ),
        processors=[
            structlog.processors.add_log_level,
            structlog.processors.TimeStamper(fmt="iso"),
            structlog.dev.ConsoleRenderer(),
        ],
    )


async def main():
    # Load .env from parent directory (same .env as Go backend)
    load_dotenv("../.env")

    settings = Settings()
    setup_logging(settings.log_level)

    log = structlog.get_logger()
    log.info("Starting Python worker", ai_provider=settings.ai_provider)

    # Database
    session_factory = create_db(settings)
    log.info("Connected to PostgreSQL", host=settings.postgres_host, port=settings.postgres_port)

    # Google scraper
    scraper = GoogleScraper(
        proxy_url=settings.proxy_url,
        serper_api_key=settings.serper_api_key,
    )
    engines = ["patchright"]
    if settings.proxy_url:
        engines.append("proxy")
    if settings.serper_api_key:
        engines.append("serper.dev")
    log.info("Google scraper initialized", engines=engines)

    # AI client
    ai_client = create_ai_client(
        provider=settings.ai_provider,
        api_key=settings.ai_api_key,
        model=settings.ai_model,
        search_grounding=settings.ai_search_grounding,
    )
    if ai_client:
        log.info("AI client initialized", provider=settings.ai_provider)
    else:
        log.warning("AI client not configured (missing API key)")

    # Worker
    worker = Worker(
        settings=settings,
        session_factory=session_factory,
        scraper=scraper,
        ai_client=ai_client,
    )

    await worker.connect()

    # Graceful shutdown
    loop = asyncio.get_running_loop()
    shutdown_event = asyncio.Event()

    def handle_signal():
        log.info("Shutdown signal received")
        shutdown_event.set()

    for sig in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(sig, handle_signal)

    # Start listening in background
    listen_task = asyncio.create_task(worker.listen())

    # Wait for either shutdown signal or listen task failure
    shutdown_task = asyncio.create_task(shutdown_event.wait())
    done, pending = await asyncio.wait(
        [listen_task, shutdown_task],
        return_when=asyncio.FIRST_COMPLETED,
    )

    # If listen_task finished first, it crashed
    if listen_task in done:
        try:
            listen_task.result()  # raises the exception
        except Exception as e:
            log.error("Worker listen() failed", error=str(e))
        shutdown_task.cancel()
    else:
        # Shutdown signal received
        log.info("Shutting down worker...")
        listen_task.cancel()
        try:
            await listen_task
        except asyncio.CancelledError:
            pass

    await worker.stop()
    await scraper.close()
    log.info("Worker shutdown complete")


if __name__ == "__main__":
    asyncio.run(main())
