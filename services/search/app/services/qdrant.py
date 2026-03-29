import logging
from uuid import UUID

from qdrant_client import AsyncQdrantClient
from qdrant_client.models import Filter, FieldCondition, MatchValue, SearchRequest

from app.config import settings
from app.models import ChunkResult

logger = logging.getLogger(__name__)

_client: AsyncQdrantClient | None = None


def init_client() -> None:
    """Eagerly initialize the Qdrant client. Call from FastAPI lifespan startup."""
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


async def dense_search(
    query_vector: list[float],
    course_id: UUID,
    limit: int,
) -> list[ChunkResult]:
    """Search curriculum collection with dense vector, filtered by course_id."""
    client = get_client()

    course_filter = Filter(
        must=[
            FieldCondition(
                key="course_id",
                match=MatchValue(value=str(course_id)),
            )
        ]
    )

    hits = await client.search(
        collection_name=settings.curriculum_collection,
        query_vector=("dense", query_vector),
        query_filter=course_filter,
        limit=limit,
        with_payload=True,
        with_vectors=False,
    )

    results = []
    for hit in hits:
        payload = hit.payload or {}
        results.append(
            ChunkResult(
                chunk_id=payload.get("chunk_id", hit.id),
                material_id=payload.get("material_id", "00000000-0000-0000-0000-000000000000"),
                course_id=course_id,
                content=payload.get("content", ""),
                score=hit.score,
                chapter=payload.get("chapter"),
                section=payload.get("section"),
                page=payload.get("page"),
                content_type=payload.get("content_type", "text"),
            )
        )

    logger.info(
        "dense_search course_id=%s limit=%d → %d results", course_id, limit, len(results)
    )
    return results
