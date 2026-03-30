import logging
from uuid import UUID

from qdrant_client import AsyncQdrantClient
from qdrant_client.models import PointStruct, NamedVector

from app.config import settings

logger = logging.getLogger(__name__)

_client: AsyncQdrantClient | None = None


def init_client() -> None:
    """Initialize the Qdrant client. Call from FastAPI lifespan startup."""
    global _client
    kwargs: dict = dict(host=settings.qdrant_host, port=settings.qdrant_port)
    if settings.qdrant_api_key is not None:
        kwargs["api_key"] = settings.qdrant_api_key
    _client = AsyncQdrantClient(**kwargs)
    logger.info("qdrant client initialized → %s:%d", settings.qdrant_host, settings.qdrant_port)


def get_client() -> AsyncQdrantClient:
    if _client is None:
        raise RuntimeError("Qdrant client not initialized — call init_client() at startup")
    return _client


async def close_client() -> None:
    global _client
    if _client:
        await _client.close()
        _client = None


async def upsert_chunks(
    chunk_ids: list[UUID],
    vectors: list[list[float]],
    payloads: list[dict],
) -> None:
    """Upsert chunk vectors into the curriculum collection.

    Each payload should contain: chunk_id, material_id, course_id,
    content, chapter, section, page, content_type.
    """
    client = get_client()
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
    for i in range(0, len(points), batch_size):
        batch = points[i:i + batch_size]
        await client.upsert(
            collection_name=settings.curriculum_collection,
            points=batch,
        )

    logger.info("upserted %d points to collection=%s",
                len(points), settings.curriculum_collection)
