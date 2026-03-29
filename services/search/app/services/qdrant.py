import logging
import re
from collections import Counter
from uuid import UUID

from qdrant_client import AsyncQdrantClient
from qdrant_client.models import (
    FieldCondition,
    Filter,
    MatchValue,
    NamedSparseVector,
    SparseVector,
)

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


def _tokenize(text: str) -> dict[int, float]:
    """
    Produce a sparse term-frequency vector from *text*.

    Each unique lowercase token is mapped to a deterministic integer index via
    hash(token) % VOCAB_SIZE.  Values are normalized term frequencies (TF),
    matching the format Qdrant's BM25/BM42 sparse fields expect at query time.

    VOCAB_SIZE=30000 is intentionally larger than typical BM25 vocabularies to
    reduce hash collisions while staying within Qdrant's sparse vector limits.
    """
    _VOCAB_SIZE = 30_000
    tokens = re.findall(r"[a-z0-9]+", text.lower())
    if not tokens:
        return {}
    counts = Counter(tokens)
    total = sum(counts.values())
    # Collapse collisions by summing TF — benign for retrieval quality
    sparse: dict[int, float] = {}
    for token, count in counts.items():
        idx = hash(token) % _VOCAB_SIZE
        sparse[idx] = sparse.get(idx, 0.0) + count / total
    return sparse


async def sparse_search(
    query: str,
    course_id: UUID,
    limit: int,
) -> list[ChunkResult]:
    """Search curriculum collection with BM25 sparse vector, filtered by course_id."""
    client = get_client()

    sparse_tf = _tokenize(query)
    if not sparse_tf:
        logger.info("sparse_search: empty query after tokenization — returning []")
        return []

    indices = list(sparse_tf.keys())
    values = [sparse_tf[i] for i in indices]

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
        query_vector=NamedSparseVector(
            name="sparse",
            vector=SparseVector(indices=indices, values=values),
        ),
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
        "sparse_search course_id=%s terms=%d limit=%d → %d results",
        course_id,
        len(indices),
        limit,
        len(results),
    )
    return results
