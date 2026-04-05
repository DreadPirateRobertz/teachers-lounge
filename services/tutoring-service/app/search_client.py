"""HTTP client for the Search Service.

Calls GET /v1/search with the student's question and course_id, returning
grounding chunks for the agentic RAG pipeline. All network errors are caught
and logged — callers receive an empty list, triggering graceful fallback to
non-grounded mode.
"""
import logging
from uuid import UUID

import httpx
from pydantic import BaseModel

from .config import settings

logger = logging.getLogger(__name__)


class SearchResult(BaseModel):
    """A single curriculum chunk returned by the Search Service."""

    chunk_id: str
    material_id: str
    course_id: str
    content: str
    score: float
    chapter: str | None = None
    section: str | None = None
    page: int | None = None
    content_type: str = "text"


class DiagramResult(BaseModel):
    """A single diagram returned by the Search Service diagram endpoint."""

    diagram_id: str
    course_id: str
    gcs_path: str
    caption: str
    figure_type: str = "diagram"
    page: int | None = None
    chapter: str | None = None
    score: float


async def fetch_curriculum_chunks(
    query: str,
    course_id: UUID,
    limit: int = 8,
) -> list[SearchResult]:
    """Retrieve curriculum chunks from the Search Service for the given query.

    Returns an empty list on any error so the tutoring service degrades
    gracefully to non-grounded mode rather than failing outright.

    Args:
        query: The student's question.
        course_id: Scopes results to the student's enrolled course.
        limit: Maximum number of chunks to return.

    Returns:
        Ordered list of SearchResult objects, or [] on any error.
    """
    try:
        async with httpx.AsyncClient(timeout=5.0) as client:
            resp = await client.get(
                f"{settings.search_service_url}/v1/search",
                params={"q": query, "course_id": str(course_id), "limit": limit},
            )
            resp.raise_for_status()
            body = resp.json()
            return [SearchResult(**r) for r in body.get("results", [])]
    except Exception as exc:
        logger.warning("Search Service unavailable — falling back to non-grounded mode: %s", exc)
        return []


async def fetch_diagram_chunks(
    query: str,
    course_id: UUID,
    limit: int = 3,
) -> list[DiagramResult]:
    """Retrieve diagram results from the Search Service CLIP endpoint.

    Calls GET /v1/search/diagrams with the student's question.  Returns an
    empty list on any error so the tutoring service degrades gracefully.

    Args:
        query: The student's question (used as CLIP text query).
        course_id: Scopes results to the student's enrolled course.
        limit: Maximum number of diagrams to return.

    Returns:
        Ordered list of DiagramResult objects, or [] on any error.
    """
    try:
        async with httpx.AsyncClient(timeout=5.0) as client:
            resp = await client.get(
                f"{settings.search_service_url}/v1/search/diagrams",
                params={"q": query, "course_id": str(course_id), "limit": limit},
            )
            resp.raise_for_status()
            body = resp.json()
            return [DiagramResult(**r) for r in body.get("results", [])]
    except Exception as exc:
        logger.warning("Diagram search unavailable — skipping visual context: %s", exc)
        return []
