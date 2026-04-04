"""
HTTP client for the Search Service.

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
    chunk_id: str
    material_id: str
    course_id: str
    content: str
    score: float
    chapter: str | None = None
    section: str | None = None
    page: int | None = None
    content_type: str = "text"


async def fetch_curriculum_chunks(
    query: str,
    course_id: UUID,
    limit: int = 8,
) -> list[SearchResult]:
    """
    Retrieve curriculum chunks from the Search Service for the given query.

    Returns an empty list on any error so the tutoring service degrades
    gracefully to non-grounded mode rather than failing outright.
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
