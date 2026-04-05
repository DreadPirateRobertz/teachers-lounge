"""Qdrant writer for diagram/figure vectors.

Provides a single ``upsert_diagram`` coroutine that upsertes a CLIP-encoded
diagram point into the ``diagrams`` Qdrant collection.  All Qdrant errors are
caught and logged rather than propagated so that a Qdrant outage does not
abort PDF processing.
"""
from __future__ import annotations

import asyncio
import logging
from uuid import UUID

from qdrant_client import QdrantClient  # type: ignore[import-untyped]
from qdrant_client.models import (  # type: ignore[import-untyped]
    Distance,
    PointStruct,
    VectorParams,
)

from app.config import settings

logger = logging.getLogger(__name__)


def _upsert_diagram_sync(
    diagram_id: UUID,
    material_id: UUID,
    course_id: UUID,
    page: int,
    caption: str,
    image_b64_thumb: str,
    clip_vector: list[float],
) -> None:
    """Synchronous Qdrant upsert, intended to run in a thread executor.

    Creates the diagrams collection if it does not already exist, then upserts
    a single point with its CLIP vector and metadata payload.

    Args:
        diagram_id: UUID for this diagram point.
        material_id: UUID of the source material.
        course_id: UUID of the course this material belongs to.
        page: 1-indexed page number of the diagram.
        caption: Detected figure caption (empty string if none found).
        image_b64_thumb: Base64-encoded PNG thumbnail (<= 256px).
        clip_vector: 768-dimensional CLIP embedding.
    """
    client = QdrantClient(host=settings.qdrant_host, port=settings.qdrant_port)
    try:
        existing = {c.name for c in client.get_collections().collections}
        if settings.diagrams_collection not in existing:
            client.create_collection(
                collection_name=settings.diagrams_collection,
                vectors_config=VectorParams(size=settings.clip_embedding_dim, distance=Distance.COSINE),
            )
            logger.info("Created Qdrant collection: %s", settings.diagrams_collection)

        point = PointStruct(
            id=str(diagram_id),
            vector=clip_vector,
            payload={
                "diagram_id": str(diagram_id),
                "material_id": str(material_id),
                "course_id": str(course_id),
                "page": page,
                "caption": caption,
                "image_b64_thumb": image_b64_thumb,
            },
        )
        client.upsert(
            collection_name=settings.diagrams_collection,
            points=[point],
        )
        logger.info(
            "Upserted diagram %s (material=%s page=%d) to collection=%s",
            diagram_id,
            material_id,
            page,
            settings.diagrams_collection,
        )
    finally:
        client.close()


async def upsert_diagram(
    diagram_id: UUID,
    material_id: UUID,
    course_id: UUID,
    page: int,
    caption: str,
    image_b64_thumb: str,
    clip_vector: list[float],
) -> None:
    """Upsert a diagram point to the Qdrant diagrams collection.

    Creates the collection if it does not exist (768d cosine vectors).
    Non-fatal: logs and returns on any Qdrant error.

    Args:
        diagram_id: UUID for this diagram point.
        material_id: UUID of the source material.
        course_id: UUID of the course this material belongs to.
        page: 1-indexed page number of the diagram.
        caption: Detected figure caption (empty string if none found).
        image_b64_thumb: Base64-encoded PNG thumbnail (<= 256px).
        clip_vector: 768-dimensional CLIP embedding.
    """
    loop = asyncio.get_running_loop()
    try:
        await loop.run_in_executor(
            None,
            _upsert_diagram_sync,
            diagram_id,
            material_id,
            course_id,
            page,
            caption,
            image_b64_thumb,
            clip_vector,
        )
    except Exception:
        logger.exception(
            "Qdrant upsert failed for diagram %s (material=%s) — continuing",
            diagram_id,
            material_id,
        )
