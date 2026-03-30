import json
import logging
from uuid import UUID

import asyncpg

from app.config import settings
from app.models import ProcessingStatus

logger = logging.getLogger(__name__)

_pool: asyncpg.Pool | None = None


async def get_pool() -> asyncpg.Pool:
    global _pool
    if _pool is None:
        # Strip the SQLAlchemy driver prefix for asyncpg
        dsn = settings.database_url.replace("postgresql+asyncpg://", "postgresql://")
        _pool = await asyncpg.create_pool(dsn, min_size=2, max_size=10)
    return _pool


async def close_pool() -> None:
    global _pool
    if _pool:
        await _pool.close()
        _pool = None


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
    pool = await get_pool()
    await pool.execute(
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
    logger.info("created material %s status=pending", material_id)


async def update_material_status(
    material_id: UUID,
    status: ProcessingStatus,
    chunk_count: int | None = None,
) -> None:
    pool = await get_pool()
    if chunk_count is not None:
        await pool.execute(
            "UPDATE materials SET processing_status=$1, chunk_count=$2 WHERE id=$3",
            status,
            chunk_count,
            material_id,
        )
    else:
        await pool.execute(
            "UPDATE materials SET processing_status=$1 WHERE id=$2",
            status,
            material_id,
        )
    logger.info("material %s → %s", material_id, status)


async def insert_chunks(chunks: list[dict]) -> None:
    """Bulk-insert chunk metadata into the chunks table.

    Each dict must contain: id, material_id, course_id, content,
    chapter, section, page, content_type, metadata.
    """
    if not chunks:
        return
    pool = await get_pool()
    await pool.executemany(
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
    logger.info("inserted %d chunks for material %s", len(chunks), chunks[0]["material_id"])
