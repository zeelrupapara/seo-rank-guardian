from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker, Session

from config import Settings


def create_db(settings: Settings) -> sessionmaker[Session]:
    engine = create_engine(
        settings.database_url,
        pool_size=25,
        max_overflow=5,
        pool_pre_ping=True,
    )
    return sessionmaker(bind=engine)
