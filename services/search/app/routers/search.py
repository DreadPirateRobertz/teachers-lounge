import uuid
from typing import Annotated

from fastapi import APIRouter, Query

from app.config import settings
from app.models import SearchResponse
from app.services.embedder import embed_query
from app.services.hybrid import combine_dense_sparse
from app.services.qdrant import dense_search, sparse_search
from app.services.reranker import rerank

router = APIRouter(prefix="/v1", tags=["search"])


@router.get("/search", response_model=SearchResponse)
async def search(
    q: Annotated[str, Query(min_length=1, max_length=1000, description="Search query")],
    # FERPA: course_id scopes results to a single student's course. This endpoint
    # currently has no auth — any caller can query any course_id. Auth middleware
    # must enforce that the requesting user owns course_id before external exposure.
    # Tracked in tl-sui (auth integration milestone).
    course_id: Annotated[uuid.UUID, Query(description="Course to search within")],
    limit: Annotated[
        int,
        Query(ge=1, le=settings.max_search_limit, description="Max results to return"),
    ] = settings.default_search_limit,
) -> SearchResponse:
    """
    Hybrid search over the curriculum collection for a given course.

    Performs dense semantic search (OpenAI text-embedding-3-large) and, when
    BM25 sparse vectors are indexed, adds keyword search with RRF fusion.
    Falls back gracefully to dense-only when sparse vectors are not available.
    """
    fetch_limit = max(limit, settings.sparse_rerank_limit)

    query_vector, sparse_results = await _embed_and_sparse(q, course_id, fetch_limit)

    dense_results = await dense_search(
        query_vector=query_vector,
        course_id=course_id,
        limit=fetch_limit,
    )

    fused = combine_dense_sparse(dense_results, sparse_results)
    ranked = rerank(q, fused)
    final = ranked[:limit]

    search_mode = "hybrid" if sparse_results else "dense"
    return SearchResponse(
        query=q,
        course_id=course_id,
        results=final,
        total=len(final),
        search_mode=search_mode,
    )


async def _embed_and_sparse(
    query: str,
    course_id: uuid.UUID,
    limit: int,
) -> tuple[list[float], list]:
    """Run embedding and sparse search concurrently."""
    import asyncio
    query_vector, sparse_results = await asyncio.gather(
        embed_query(query),
        sparse_search(query=query, course_id=course_id, limit=limit),
    )
    return query_vector, sparse_results
