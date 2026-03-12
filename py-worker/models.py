from __future__ import annotations

import time
from typing import Optional, List

from sqlalchemy import BigInteger, Boolean, Integer, String, Text
from sqlalchemy.dialects.postgresql import JSONB
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column


class Base(DeclarativeBase):
    pass


class CommonMixin:
    """Mirrors Go's CommonModel with nanosecond timestamps."""

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    created_at: Mapped[int] = mapped_column(BigInteger, default=time.time_ns)
    updated_at: Mapped[int] = mapped_column(BigInteger, default=time.time_ns, onupdate=time.time_ns)
    deleted_at: Mapped[int] = mapped_column(BigInteger, default=0)
    created_by: Mapped[int] = mapped_column(Integer, default=0)
    updated_by: Mapped[int] = mapped_column(Integer, default=0)


class Job(CommonMixin, Base):
    __tablename__ = "srg_jobs"

    user_id: Mapped[int] = mapped_column(Integer, nullable=False, index=True)
    name: Mapped[str] = mapped_column(String(255), nullable=False)
    domain: Mapped[str] = mapped_column(String(255), nullable=False)
    is_active: Mapped[bool] = mapped_column(Boolean, default=True)
    schedule_time: Mapped[Optional[str]] = mapped_column(String(10))
    competitors: Mapped[Optional[dict]] = mapped_column(JSONB)
    regions: Mapped[Optional[dict]] = mapped_column(JSONB)

    def get_competitors(self) -> List[str]:
        if self.competitors is None:
            return []
        if isinstance(self.competitors, list):
            return self.competitors
        return []


class JobKeyword(Base):
    __tablename__ = "srg_job_keywords"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    created_at: Mapped[int] = mapped_column(BigInteger, default=time.time_ns)
    job_id: Mapped[int] = mapped_column(Integer, nullable=False, index=True)
    keyword: Mapped[str] = mapped_column(String(500), nullable=False)


class JobRun(CommonMixin, Base):
    __tablename__ = "srg_job_runs"

    job_id: Mapped[int] = mapped_column(Integer, nullable=False, index=True)
    status: Mapped[str] = mapped_column(String(50), default="pending")
    total_pairs: Mapped[int] = mapped_column(Integer, default=0)
    completed_pairs: Mapped[int] = mapped_column(Integer, default=0)
    failed_pairs: Mapped[int] = mapped_column(Integer, default=0)
    started_at: Mapped[Optional[int]] = mapped_column(BigInteger, nullable=True)
    completed_at: Mapped[Optional[int]] = mapped_column(BigInteger, nullable=True)
    triggered_by: Mapped[Optional[str]] = mapped_column(String(50))


class SearchPair(CommonMixin, Base):
    __tablename__ = "srg_search_pairs"

    run_id: Mapped[int] = mapped_column(Integer, nullable=False, index=True)
    job_id: Mapped[int] = mapped_column(Integer, nullable=False, index=True)
    keyword_id: Mapped[Optional[int]] = mapped_column(Integer, index=True)
    status: Mapped[str] = mapped_column(String(50), default="pending")
    error_msg: Mapped[Optional[str]] = mapped_column(Text)
    started_at: Mapped[Optional[int]] = mapped_column(BigInteger, nullable=True)
    finished_at: Mapped[Optional[int]] = mapped_column(BigInteger, nullable=True)
    keyword: Mapped[Optional[str]] = mapped_column(String(500))
    state: Mapped[Optional[str]] = mapped_column(String(255))
    country: Mapped[Optional[str]] = mapped_column(String(100))
    search_query: Mapped[Optional[str]] = mapped_column(String(1000))


class SearchResult(CommonMixin, Base):
    __tablename__ = "srg_search_results"

    pair_id: Mapped[int] = mapped_column(Integer, nullable=False, index=True)
    run_id: Mapped[int] = mapped_column(Integer, nullable=False, index=True)
    job_id: Mapped[Optional[int]] = mapped_column(Integer, index=True)
    domain: Mapped[str] = mapped_column(String(255), nullable=False)
    position: Mapped[int] = mapped_column(Integer, default=0)
    url: Mapped[Optional[str]] = mapped_column(String(2048))
    title: Mapped[Optional[str]] = mapped_column(String(500))
    snippet: Mapped[Optional[str]] = mapped_column(Text)
    is_target: Mapped[bool] = mapped_column(Boolean, default=False)
    is_competitor: Mapped[bool] = mapped_column(Boolean, default=False)
    keyword: Mapped[Optional[str]] = mapped_column(String(500))
    state: Mapped[Optional[str]] = mapped_column(String(255))


class RankDiff(CommonMixin, Base):
    __tablename__ = "srg_rank_diffs"

    job_id: Mapped[int] = mapped_column(Integer, nullable=False, index=True)
    run_id: Mapped[int] = mapped_column(Integer, nullable=False, index=True)
    prev_run_id: Mapped[int] = mapped_column(Integer, default=0)
    domain: Mapped[Optional[str]] = mapped_column(String(255))
    prev_position: Mapped[int] = mapped_column(Integer, default=0)
    curr_position: Mapped[int] = mapped_column(Integer, default=0)
    delta: Mapped[int] = mapped_column(Integer, default=0)
    change_type: Mapped[Optional[str]] = mapped_column(String(50))
    keyword: Mapped[Optional[str]] = mapped_column(String(500))
    state: Mapped[Optional[str]] = mapped_column(String(255))


class Report(CommonMixin, Base):
    __tablename__ = "srg_reports"

    job_id: Mapped[int] = mapped_column(Integer, nullable=False, index=True)
    run_id: Mapped[int] = mapped_column(Integer, nullable=False, index=True)
    provider: Mapped[Optional[str]] = mapped_column(String(50))
    model: Mapped[Optional[str]] = mapped_column(String(100))
    prompt: Mapped[Optional[str]] = mapped_column(Text)
    result: Mapped[Optional[dict]] = mapped_column(JSONB)
    grounding_meta: Mapped[Optional[dict]] = mapped_column(JSONB)
    status: Mapped[str] = mapped_column(String(50), default="pending")


class RunEventLog(Base):
    __tablename__ = "srg_run_events"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True, autoincrement=True)
    created_at: Mapped[int] = mapped_column(BigInteger, default=time.time_ns)
    run_id: Mapped[int] = mapped_column(Integer, nullable=False, index=True)
    job_id: Mapped[int] = mapped_column(Integer, nullable=False, index=True)
    event_type: Mapped[str] = mapped_column(String(50), nullable=False, index=True)
    data: Mapped[dict] = mapped_column(JSONB, nullable=False)
