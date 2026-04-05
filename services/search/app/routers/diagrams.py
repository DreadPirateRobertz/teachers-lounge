"""Diagram search endpoint — CLIP-based visual retrieval (Phase 6).

GET /v1/search/diagrams?q=benzene+ring&course_id=<uuid>

Encodes the text query into a 768-d CLIP vector and performs nearest-neighbour
search against the ``diagrams`` Qdrant collection, which is populated during
ingestion when figures are extracted from uploaded PDFs.
"""
import uuid
from typing import Annotated

from fastapi import APIRouter, Query

from app.config import settings
from app.models import DiagramSearchResponse
from app.services.clip_embedder import embed_text_clip
from app.services.qdrant import diagram_search

router = APIRouter(prefix="/v1", tags=["diagrams"])


@router.get("/search/diagrams", response_model=DiagramSearchResponse)
async def search_diagrams(
    q: Annotated[str, Query(min_length=1, max_length=1000, description="Visual search query")],
    course_id: Annotated[uuid.UUID, Query(description="Course to search within")],
    limit: Annotated[
        int,
        Query(
            ge=1,
            le=settings.max_diagram_limit,
            description="Max diagrams to return",
        ),
    ] = settings.default_diagram_limit,
) -> DiagramSearchResponse:
    """Search the diagram collection using CLIP text→image similarity.

    Encodes *q* as a CLIP text vector and retrieves the closest diagram
    embeddings (generated from figure images during ingestion).  Returns
    GCS paths and metadata so the tutoring service can embed signed URLs
    in chat responses.

    Falls back to an empty result list when the diagrams collection has
    not been populated yet (e.g. before any course material is ingested).
    """
    query_vector = await embed_text_clip(q)
    results = await diagram_search(
        query_vector=query_vector,
        course_id=course_id,
        limit=limit,
    )
    return DiagramSearchResponse(
        query=q,
        course_id=course_id,
        results=results,
        total=len(results),
    )
