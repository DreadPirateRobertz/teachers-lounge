import logging
import re
from collections import Counter
from uuid import UUID

from qdrant_client import AsyncQdrantClient
from qdrant_client.models import (
    Filter,
    FieldCondition,
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


# ---------------------------------------------------------------------------
# Sparse / BM25 search
# ---------------------------------------------------------------------------
# Vocabulary size must match what the ingestion service uses when building BM25
# sparse vectors at index time. Both sides use the same simple hash mapping
# until Phase 4 standardises on fastembed BM25 (tracked in tl-zxk).
_SPARSE_VOCAB_SIZE = 30522  # BERT vocab size — default for fastembed BM25


def _bm25_query_vector(query: str) -> tuple[list[int], list[float]]:
    """
    Compute a lightweight BM25-style sparse query vector.

    Tokenises query → term frequencies → hash-based vocab indices.
    Phase 4 will replace this with fastembed SparseTextEmbedding so that
    query and ingestion vocabularies are guaranteed identical.
    """
    tokens = re.findall(r"\b[a-z0-9]+\b", query.lower())
    if not tokens:
        return [], []
    counts = Counter(tokens)
    indices = [abs(hash(t)) % _SPARSE_VOCAB_SIZE for t in counts]
    values = [float(v) for v in counts.values()]
    return indices, values


async def sparse_search(
    query: str,
    course_id: UUID,
    limit: int,
) -> list[ChunkResult]:
    """
    BM25 keyword search over the curriculum collection using sparse vectors.

    Returns an empty list if the collection has no sparse vector field yet
    (i.e. ingestion has not stored BM25 vectors) rather than raising an error.
    The hybrid combiner degrades gracefully to dense-only in that case.
    """
    indices, values = _bm25_query_vector(query)
    if not indices:
        return []

    client = get_client()
    course_filter = Filter(
        must=[
            FieldCondition(key="course_id", match=MatchValue(value=str(course_id)))
        ]
    )

    try:
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
    except Exception as exc:
        # Collection may not have sparse vectors indexed yet (pre-ingestion).
        logger.debug("sparse_search skipped (no sparse index?): %s", exc)
        return []

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
        "sparse_search course_id=%s limit=%d → %d results", course_id, limit, len(results)
    )
    return results
