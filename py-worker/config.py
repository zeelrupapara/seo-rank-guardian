from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    # PostgreSQL
    postgres_host: str = "localhost"
    postgres_port: str = "5433"
    postgres_user: str = "srg"
    postgres_password: str = "srg_secret"
    postgres_db: str = "srg_db"
    postgres_sslmode: str = "disable"

    # NATS
    nats_url: str = "nats://localhost:4223"

    # Scraper
    scrape_result_limit: int = 10

    # Proxy (residential proxy URL, e.g. http://user:pass@host:port)
    proxy_url: str = ""

    # Serper.dev API fallback (free 2500 queries/month)
    serper_api_key: str = ""

    # AI
    ai_provider: str = "gemini"
    ai_api_key: str = ""
    ai_model: str = "gemini-2.0-flash"
    ai_search_grounding: bool = True
    # Logger
    log_level: str = "debug"

    @property
    def database_url(self) -> str:
        return (
            f"postgresql://{self.postgres_user}:{self.postgres_password}"
            f"@{self.postgres_host}:{self.postgres_port}/{self.postgres_db}"
        )

    model_config = {"env_file": "../.env", "env_file_encoding": "utf-8", "extra": "ignore"}
