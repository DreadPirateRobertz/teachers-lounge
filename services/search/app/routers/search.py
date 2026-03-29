import uuid
from typing import Annotated

from fastapi import APIRouter, HTTPException, Query, status

from app.config import settings
from app.models import SearchResponse
from app.services.embedder import embed_query
from app.services.hybrid import combine_dense_sparse
from app.services.qdrant import dense_search
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

    Currently performs dense vector search only (random stub embeddings).
    Phase 2 full implementation: real embeddings + BM25 sparse + RRF fusion + re-ranking.
    """
    query_vector = await embed_query(q)

    dense_results = await dense_search(
        query_vector=query_vector,
        course_id=course_id,
        limit=limit,
    )

    # Sparse search not wired yet — pass empty list
    fused = combine_dense_sparse(dense_results, sparse_results=[])

    ranked = rerank(q, fused)

    return SearchResponse(
        query=q,
        course_id=course_id,
        results=ranked[:limit],
        total=len(ranked),
        search_mode="dense",
    )
