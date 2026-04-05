"""Async Postgres connection for the eval service.

Reads from Cloud SQL (interactions, quiz_results, chat_sessions tables).
The eval service is read-only from Postgres; BigQuery receives the computed metrics.
"""
import logging
from contextlib import asynccontextmanager
from typing import AsyncGenerator

from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from .config import settings

logger = logging.getLogger(__name__)

_engine = create_async_engine(
    settings.database_url,
    pool_size=2,
    max_overflow=0,
    echo=False,
)

_session_factory = async_sessionmaker(_engine, expire_on_commit=False, class_=AsyncSession)


@asynccontextmanager
async def get_session() -> AsyncGenerator[AsyncSession, None]:
    """Yield an async SQLAlchemy session, closing it on exit.

    Yields:
        AsyncSession connected to Cloud SQL.
    """
    async with _session_factory() as session:
        yield session


async def close_engine() -> None:
    """Dispose the connection pool on shutdown."""
    await _engine.dispose()
    logger.info("Database engine disposed")
