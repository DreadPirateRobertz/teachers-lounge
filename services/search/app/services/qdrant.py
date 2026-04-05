import hashlib
import logging
import re
from collections import Counter
from uuid import UUID

from opentelemetry import trace
from qdrant_client import AsyncQdrantClient
from qdrant_client.models import (
    FieldCondition,
    Filter,
    MatchValue,
    NamedSparseVector,
    SparseVector,
)

from app.config import settings
from app.models import ChunkResult, DiagramResult

logger = logging.getLogger(__name__)
_tracer = trace.get_tracer("search-service.qdrant")

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
    """Return the initialized Qdrant client, raising if not yet initialized."""
    if _client is None:
        raise RuntimeError("Qdrant client not initialized — call init_client() at startup")
    return _client


async def close_client() -> None:
    """Close and release the Qdrant client. Safe to call if not initialized."""
    global _client
    if _client:
        await _client.close()
        _client = None


def _build_filter(
    course_id: UUID,
    chapter: str | None = None,
    section: str | None = None,
    content_type: str | None = None,
) -> Filter:
    """Build a Qdrant filter scoped to a course with optional field narrowing.

    Args:
        course_id: Required — all results must belong to this course.
        chapter: Optional chapter name to restrict results.
        section: Optional section name to restrict results.
        content_type: Optional content type (text, table, equation, figure, quiz).

    Returns:
        A Qdrant Filter with all active conditions joined via AND (must).
    """
    conditions = [
        FieldCondition(key="course_id", match=MatchValue(value=str(course_id))),
    ]
    if chapter is not None:
        conditions.append(FieldCondition(key="chapter", match=MatchValue(value=chapter)))
    if section is not None:
        conditions.append(FieldCondition(key="section", match=MatchValue(value=section)))
    if content_type is not None:
        conditions.append(FieldCondition(key="content_type", match=MatchValue(value=content_type)))
    return Filter(must=conditions)


async def dense_search(
    query_vector: list[float],
    course_id: UUID,
    limit: int,
    chapter: str | None = None,
    section: str | None = None,
    content_type: str | None = None,
) -> list[ChunkResult]:
    """Search curriculum collection with dense vector, filtered by course_id and optional fields.

    Args:
        query_vector: Dense embedding of the query text.
        course_id: Scope results to this course.
        limit: Maximum number of results to return.
        chapter: Optional chapter filter.
        section: Optional section filter.
        content_type: Optional content type filter (text, table, equation, figure, quiz).

    Returns:
        Ordered list of ChunkResult objects by descending cosine score.
    """
    client = get_client()

    query_filter = _build_filter(course_id, chapter, section, content_type)

    with _tracer.start_as_current_span("qdrant.search") as span:
        span.set_attribute("search_type", "dense")
        span.set_attribute("course_id", str(course_id))
        span.set_attribute("limit", limit)

        hits = await client.search(
            collection_name=settings.curriculum_collection,
            query_vector=("dense", query_vector),
            query_filter=query_filter,
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
    """Produce a sparse term-frequency vector from *text*.

    Each unique lowercase token is mapped to a deterministic integer index via
    sha1(token) % VOCAB_SIZE.  Values are normalized term frequencies (TF),
    matching the format Qdrant's BM25/BM42 sparse fields expect at query time.

    VOCAB_SIZE=30000 is intentionally larger than typical BM25 vocabularies to
    reduce hash collisions while staying within Qdrant's sparse vector limits.

    Args:
        text: Raw text to tokenize.

    Returns:
        Dict mapping token index → normalized TF weight. Empty if no tokens found.
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
        idx = int(hashlib.sha1(token.encode()).hexdigest()[:8], 16) % _VOCAB_SIZE
        sparse[idx] = sparse.get(idx, 0.0) + count / total
    return sparse


async def sparse_search(
    query: str,
    course_id: UUID,
    limit: int,
    chapter: str | None = None,
    section: str | None = None,
    content_type: str | None = None,
) -> list[ChunkResult]:
    """Search curriculum collection with BM25 sparse vector, filtered by course_id and optional fields.

    Returns an empty list if the collection has no sparse vector field yet
    (i.e. ingestion has not stored BM25 vectors) rather than raising an error.
    The hybrid combiner degrades gracefully to dense-only in that case.

    Args:
        query: Raw query string to tokenize into a sparse TF vector.
        course_id: Scope results to this course.
        limit: Maximum number of results to return.
        chapter: Optional chapter filter.
        section: Optional section filter.
        content_type: Optional content type filter (text, table, equation, figure, quiz).

    Returns:
        Ordered list of ChunkResult objects by descending BM25 score, or [] on failure.
    """
    client = get_client()

    sparse_tf = _tokenize(query)
    if not sparse_tf:
        logger.info("sparse_search: empty query after tokenization — returning []")
        return []

    indices = list(sparse_tf.keys())
    values = [sparse_tf[i] for i in indices]

    query_filter = _build_filter(course_id, chapter, section, content_type)

    with _tracer.start_as_current_span("qdrant.search") as span:
        span.set_attribute("search_type", "sparse")
        span.set_attribute("course_id", str(course_id))
        span.set_attribute("limit", limit)
        try:
            hits = await client.search(
                collection_name=settings.curriculum_collection,
                query_vector=NamedSparseVector(
                    name="sparse",
                    vector=SparseVector(indices=indices, values=values),
                ),
                query_filter=query_filter,
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
        "sparse_search course_id=%s terms=%d limit=%d → %d results",
        course_id,
        len(indices),
        limit,
        len(results),
    )
    return results


async def diagram_search(
    query_vector: list[float],
    course_id: UUID,
    limit: int,
) -> list[DiagramResult]:
    """Search the diagrams collection with a CLIP text vector.

    Args:
        query_vector: 768-d CLIP text embedding of the query.
        course_id: Scopes results to a single course.
        limit: Maximum number of results to return.

    Returns:
        Ordered list of DiagramResult objects, or [] if the collection is empty
        or unavailable.
    """
    client = get_client()
    query_filter = Filter(
        must=[FieldCondition(key="course_id", match=MatchValue(value=str(course_id)))]
    )

    with _tracer.start_as_current_span("qdrant.search") as span:
        span.set_attribute("search_type", "diagram")
        span.set_attribute("course_id", str(course_id))
        span.set_attribute("limit", limit)
        try:
            hits = await client.search(
                collection_name=settings.diagrams_collection,
                query_vector=query_vector,
                query_filter=query_filter,
                limit=limit,
                with_payload=True,
                with_vectors=False,
            )
        except Exception as exc:
            logger.debug("diagram_search unavailable (collection missing?): %s", exc)
            return []

    results = []
    for hit in hits:
        payload = hit.payload or {}
        results.append(
            DiagramResult(
                diagram_id=str(payload.get("diagram_id", hit.id)),
                course_id=course_id,
                gcs_path=payload.get("gcs_path", ""),
                caption=payload.get("caption", ""),
                figure_type=payload.get("figure_type", "diagram"),
                page=payload.get("page"),
                chapter=payload.get("chapter"),
                score=hit.score,
            )
        )

    logger.info("diagram_search course_id=%s limit=%d → %d results", course_id, limit, len(results))
    return results
