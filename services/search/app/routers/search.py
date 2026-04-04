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
    chapter: Annotated[
        str | None,
        Query(max_length=200, description="Filter results to a specific chapter"),
    ] = None,
    limit: Annotated[
        int,
        Query(ge=1, le=settings.max_search_limit, description="Max results to return"),
    ] = settings.default_search_limit,
) -> SearchResponse:
    """
    Hybrid search over the curriculum collection for a given course.

    Runs dense (semantic) and sparse (BM25) searches in parallel, then fuses
    them with Reciprocal Rank Fusion (RRF) before returning ranked results.
    """
    query_vector, dense_results, sparse_results = await _run_search(q, course_id, limit, chapter)

    fused = combine_dense_sparse(dense_results, sparse_results)
    ranked = await rerank(q, fused)

    search_mode = "hybrid" if sparse_results else "dense"
    return SearchResponse(
        query=q,
        course_id=course_id,
        results=ranked[:limit],
        total=len(ranked),
        search_mode="hybrid",
    )


async def _run_search(
    q: str,
    course_id: uuid.UUID,
    limit: int,
    chapter: str | None = None,
) -> tuple[list[float], list, list]:
    """Run embedding, dense search, and sparse search concurrently."""
    import asyncio

    query_vector = await embed_query(q)

    dense_task = asyncio.create_task(
        dense_search(query_vector=query_vector, course_id=course_id, limit=limit, chapter=chapter)
    )
    sparse_task = asyncio.create_task(
        sparse_search(query=q, course_id=course_id, limit=limit, chapter=chapter)
    )

    dense_results, sparse_results = await asyncio.gather(dense_task, sparse_task)
    return query_vector, dense_results, sparse_results
