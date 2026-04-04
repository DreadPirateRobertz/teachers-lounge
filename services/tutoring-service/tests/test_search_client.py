"""Tests for services/tutoring-service/app/search_client.py.

Covers fetch_curriculum_chunks: successful retrieval, HTTP error fallback,
network error fallback, and correct query-parameter construction.
"""
import uuid
from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from app.search_client import SearchResult, fetch_curriculum_chunks


def _make_result(**kwargs) -> dict:
    """Return a minimal raw search-result dict suitable for SearchResult(**r).

    Args:
        **kwargs: Override any default field values.

    Returns:
        Dict matching the SearchResult schema.
    """
    defaults = dict(
        chunk_id=str(uuid.uuid4()),
        material_id=str(uuid.uuid4()),
        course_id=str(uuid.uuid4()),
        content="Photosynthesis converts light energy into chemical energy.",
        score=0.88,
        chapter="Chapter 3",
        section="3.2",
        page=45,
        content_type="text",
    )
    defaults.update(kwargs)
    return defaults


class TestFetchCurriculumChunks:
    """Unit tests for fetch_curriculum_chunks."""

    @pytest.mark.asyncio
    async def test_returns_search_results_on_200(self):
        """A 200 response with results is parsed into a list of SearchResult objects."""
        course_id = uuid.uuid4()
        raw = [_make_result(course_id=str(course_id))]

        mock_response = MagicMock()
        mock_response.raise_for_status = MagicMock()
        mock_response.json.return_value = {"results": raw}

        mock_client = AsyncMock()
        mock_client.get = AsyncMock(return_value=mock_response)

        with patch("app.search_client.httpx.AsyncClient") as mock_cls:
            mock_cls.return_value.__aenter__ = AsyncMock(return_value=mock_client)
            mock_cls.return_value.__aexit__ = AsyncMock(return_value=False)

            results = await fetch_curriculum_chunks("What is photosynthesis?", course_id, limit=4)

        assert len(results) == 1
        assert isinstance(results[0], SearchResult)
        assert results[0].content == raw[0]["content"]
        assert results[0].score == raw[0]["score"]

    @pytest.mark.asyncio
    async def test_returns_empty_list_on_http_error(self):
        """An HTTP error response is caught and an empty list is returned."""
        course_id = uuid.uuid4()

        mock_response = MagicMock()
        mock_response.raise_for_status.side_effect = httpx.HTTPStatusError(
            "404 Not Found",
            request=MagicMock(),
            response=MagicMock(),
        )

        mock_client = AsyncMock()
        mock_client.get = AsyncMock(return_value=mock_response)

        with patch("app.search_client.httpx.AsyncClient") as mock_cls:
            mock_cls.return_value.__aenter__ = AsyncMock(return_value=mock_client)
            mock_cls.return_value.__aexit__ = AsyncMock(return_value=False)

            results = await fetch_curriculum_chunks("What is osmosis?", course_id)

        assert results == []

    @pytest.mark.asyncio
    async def test_returns_empty_list_on_network_error(self):
        """A network-level error (ConnectError) is caught and an empty list is returned."""
        course_id = uuid.uuid4()

        mock_client = AsyncMock()
        mock_client.get = AsyncMock(
            side_effect=httpx.ConnectError("Connection refused")
        )

        with patch("app.search_client.httpx.AsyncClient") as mock_cls:
            mock_cls.return_value.__aenter__ = AsyncMock(return_value=mock_client)
            mock_cls.return_value.__aexit__ = AsyncMock(return_value=False)

            results = await fetch_curriculum_chunks("What is entropy?", course_id)

        assert results == []

    @pytest.mark.asyncio
    async def test_passes_correct_url_params(self):
        """GET is called with q, course_id, and limit query parameters."""
        course_id = uuid.uuid4()
        query = "Explain Newton's second law"
        limit = 6

        mock_response = MagicMock()
        mock_response.raise_for_status = MagicMock()
        mock_response.json.return_value = {"results": []}

        mock_client = AsyncMock()
        mock_client.get = AsyncMock(return_value=mock_response)

        with patch("app.search_client.httpx.AsyncClient") as mock_cls:
            mock_cls.return_value.__aenter__ = AsyncMock(return_value=mock_client)
            mock_cls.return_value.__aexit__ = AsyncMock(return_value=False)

            await fetch_curriculum_chunks(query, course_id, limit=limit)

        mock_client.get.assert_called_once()
        call_kwargs = mock_client.get.call_args

        # Verify the params keyword argument contains the expected values.
        params = call_kwargs.kwargs.get("params") or call_kwargs[1].get("params", {})
        assert params["q"] == query
        assert params["course_id"] == str(course_id)
        assert params["limit"] == limit
