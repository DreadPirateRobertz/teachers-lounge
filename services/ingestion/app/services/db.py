import json
import logging
from uuid import UUID

import asyncpg

from app.config import settings
from app.models import ProcessingStatus

logger = logging.getLogger(__name__)


def _get_dsn() -> str:
    """Strip the SQLAlchemy driver prefix for asyncpg."""
    return settings.database_url.replace("postgresql+asyncpg://", "postgresql://")


async def _connect() -> asyncpg.Connection:
    """Open a single connection to the database.

    The Pub/Sub subscriber thread runs each processor via asyncio.run()
    which creates a new event loop. asyncpg connections are bound to the
    loop they're created in, so we create fresh connections per call
    rather than maintaining a persistent pool.
    """
    return await asyncpg.connect(_get_dsn())


async def close_pool() -> None:
    """No-op — connections are created per-call."""
    pass


async def create_material(
    *,
    material_id: UUID,
    course_id: UUID,
    user_id: UUID,
    filename: str,
    gcs_path: str,
    file_type: str,
) -> None:
    """Insert a new row into the materials table with status=pending."""
    conn = await _connect()
    try:
        await conn.execute(
            """
            INSERT INTO materials
                (id, course_id, user_id, filename, gcs_path, file_type, processing_status, chunk_count, created_at)
            VALUES
                ($1, $2, $3, $4, $5, $6, $7, 0, NOW())
            """,
            material_id,
            course_id,
            user_id,
            filename,
            gcs_path,
            file_type,
            ProcessingStatus.PENDING,
        )
    finally:
        await conn.close()
    logger.info("created material %s status=pending", material_id)


async def update_material_status(
    material_id: UUID,
    status: ProcessingStatus,
    chunk_count: int | None = None,
) -> None:
    conn = await _connect()
    try:
        if chunk_count is not None:
            await conn.execute(
                "UPDATE materials SET processing_status=$1, chunk_count=$2 WHERE id=$3",
                status,
                chunk_count,
                material_id,
            )
        else:
            await conn.execute(
                "UPDATE materials SET processing_status=$1 WHERE id=$2",
                status,
                material_id,
            )
    finally:
        await conn.close()
    logger.info("material %s → %s", material_id, status)


async def insert_chunks(chunks: list[dict]) -> None:
    """Bulk-insert chunk metadata into the chunks table.

    Each dict must contain: id, material_id, course_id, content,
    chapter, section, page, content_type, metadata.
    """
    if not chunks:
        return
    conn = await _connect()
    try:
        await conn.executemany(
            """
            INSERT INTO chunks
                (id, material_id, course_id, content, chapter, section,
                 page, content_type, figure_gcs_path, metadata, created_at)
            VALUES
                ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
            """,
            [
                (
                    c["id"],
                    c["material_id"],
                    c["course_id"],
                    c["content"],
                    c.get("chapter"),
                    c.get("section"),
                    c.get("page"),
                    c.get("content_type", "text"),
                    c.get("figure_gcs_path"),
                    json.dumps(c.get("metadata", {})),
                )
                for c in chunks
            ],
        )
    finally:
        await conn.close()
    logger.info("inserted %d chunks for material %s", len(chunks), chunks[0]["material_id"])
