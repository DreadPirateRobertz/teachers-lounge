import logging
from uuid import UUID

from qdrant_client import AsyncQdrantClient
from qdrant_client.models import PointStruct

from app.config import settings

logger = logging.getLogger(__name__)

# Connection params stored at init; client created per event loop since
# AsyncQdrantClient is bound to the loop it's created in. The Pub/Sub
# subscriber thread calls asyncio.run() which creates a new loop each time.
_client_kwargs: dict | None = None


def init_client() -> None:
    """Store connection params. Actual client created lazily per-loop."""
    global _client_kwargs
    _client_kwargs = dict(host=settings.qdrant_host, port=settings.qdrant_port)
    if settings.qdrant_api_key is not None:
        _client_kwargs["api_key"] = settings.qdrant_api_key
    logger.info("qdrant config stored → %s:%d", settings.qdrant_host, settings.qdrant_port)


def _make_client() -> AsyncQdrantClient:
    if _client_kwargs is None:
        raise RuntimeError("Qdrant not configured — call init_client() at startup")
    return AsyncQdrantClient(**_client_kwargs)


async def close_client() -> None:
    """No-op — clients are created and closed per upsert call."""
    pass


async def upsert_chunks(
    chunk_ids: list[UUID],
    vectors: list[list[float]],
    payloads: list[dict],
) -> None:
    """Upsert chunk vectors into the curriculum collection.

    Each payload should contain: chunk_id, material_id, course_id,
    content, chapter, section, page, content_type.
    """
    client = _make_client()
    points = [
        PointStruct(
            id=str(chunk_id),
            vector={"dense": vector},
            payload=payload,
        )
        for chunk_id, vector, payload in zip(chunk_ids, vectors, payloads)
    ]

    # Upsert in batches of 100 to avoid payload size limits
    batch_size = 100
    try:
        for i in range(0, len(points), batch_size):
            batch = points[i:i + batch_size]
            await client.upsert(
                collection_name=settings.curriculum_collection,
                points=batch,
            )
    finally:
        await client.close()

    logger.info("upserted %d points to collection=%s",
                len(points), settings.curriculum_collection)


async def upsert_diagrams(
    diagram_ids: list,
    vectors: list[list[float]],
    payloads: list[dict],
) -> None:
    """Upsert diagram CLIP vectors into the diagrams collection.

    Each payload should contain: diagram_id, course_id, gcs_path,
    caption, figure_type, chapter, page.

    Args:
        diagram_ids: List of diagram IDs (UUID or str).
        vectors: Corresponding 768-d CLIP image vectors.
        payloads: Metadata dicts for each diagram.
    """
    if not diagram_ids:
        return

    client = _make_client()
    points = [
        PointStruct(
            id=str(did),
            vector=vector,
            payload=payload,
        )
        for did, vector, payload in zip(diagram_ids, vectors, payloads)
    ]

    batch_size = 100
    try:
        for i in range(0, len(points), batch_size):
            batch = points[i:i + batch_size]
            await client.upsert(
                collection_name=settings.diagrams_collection,
                points=batch,
            )
    finally:
        await client.close()

    logger.info("upserted %d diagrams to collection=%s",
                len(points), settings.diagrams_collection)
