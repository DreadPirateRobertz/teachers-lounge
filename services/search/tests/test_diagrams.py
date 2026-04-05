"""Tests for GET /v1/search/diagrams (Phase 6 CLIP diagram retrieval)."""
import uuid
from unittest.mock import AsyncMock, patch

import pytest
from httpx import AsyncClient
from httpx._transports.asgi import ASGITransport

from app.main import app
from app.models import DiagramResult


COURSE_ID = uuid.uuid4()

_FAKE_DIAGRAM = DiagramResult(
    diagram_id=str(uuid.uuid4()),
    course_id=COURSE_ID,
    gcs_path="gs://tvtutor-raw-uploads/figures/benzene.png",
    caption="Benzene ring structure",
    figure_type="diagram",
    page=42,
    chapter="Chapter 6: Aromatic Compounds",
    score=0.92,
)


@pytest.fixture()
def stub_clip_embed():
    """Patch embed_text_clip to return a zero vector (avoids model load)."""
    with patch(
        "app.routers.diagrams.embed_text_clip",
        new_callable=AsyncMock,
        return_value=[0.0] * 768,
    ) as m:
        yield m


@pytest.fixture()
def stub_diagram_search_one():
    """Patch diagram_search to return one fake diagram."""
    with patch(
        "app.routers.diagrams.diagram_search",
        new_callable=AsyncMock,
        return_value=[_FAKE_DIAGRAM],
    ) as m:
        yield m


@pytest.fixture()
def stub_diagram_search_empty():
    """Patch diagram_search to return empty (simulates uncollected corpus)."""
    with patch(
        "app.routers.diagrams.diagram_search",
        new_callable=AsyncMock,
        return_value=[],
    ) as m:
        yield m


async def _client():
    return AsyncClient(transport=ASGITransport(app=app), base_url="http://test")


class TestDiagramSearchEndpoint:
    async def test_returns_diagram_results(self, stub_clip_embed, stub_diagram_search_one):
        """Endpoint returns DiagramSearchResponse when diagrams match."""
        async with await _client() as client:
            resp = await client.get(
                "/v1/search/diagrams",
                params={"q": "benzene ring", "course_id": str(COURSE_ID)},
            )
        assert resp.status_code == 200
        body = resp.json()
        assert body["query"] == "benzene ring"
        assert body["total"] == 1
        assert len(body["results"]) == 1
        result = body["results"][0]
        assert result["caption"] == "Benzene ring structure"
        assert result["figure_type"] == "diagram"
        assert result["score"] == pytest.approx(0.92)

    async def test_empty_corpus_returns_empty_list(self, stub_clip_embed, stub_diagram_search_empty):
        """Returns empty results gracefully when no diagrams have been ingested."""
        async with await _client() as client:
            resp = await client.get(
                "/v1/search/diagrams",
                params={"q": "benzene ring", "course_id": str(COURSE_ID)},
            )
        assert resp.status_code == 200
        body = resp.json()
        assert body["total"] == 0
        assert body["results"] == []

    async def test_clip_embed_called_with_query(self, stub_clip_embed, stub_diagram_search_empty):
        """embed_text_clip is invoked with the raw query string."""
        async with await _client() as client:
            await client.get(
                "/v1/search/diagrams",
                params={"q": "molecular orbital", "course_id": str(COURSE_ID)},
            )
        stub_clip_embed.assert_called_once_with("molecular orbital")

    async def test_limit_param_passed_to_search(self, stub_clip_embed, stub_diagram_search_empty):
        """Custom limit is forwarded to diagram_search."""
        async with await _client() as client:
            await client.get(
                "/v1/search/diagrams",
                params={"q": "cell membrane", "course_id": str(COURSE_ID), "limit": 5},
            )
        _, kwargs = stub_diagram_search_empty.call_args
        assert kwargs["limit"] == 5

    async def test_missing_query_returns_422(self, stub_clip_embed, stub_diagram_search_empty):
        """Missing required q param returns HTTP 422."""
        async with await _client() as client:
            resp = await client.get(
                "/v1/search/diagrams",
                params={"course_id": str(COURSE_ID)},
            )
        assert resp.status_code == 422

    async def test_invalid_course_id_returns_422(self, stub_clip_embed, stub_diagram_search_empty):
        """Non-UUID course_id returns HTTP 422."""
        async with await _client() as client:
            resp = await client.get(
                "/v1/search/diagrams",
                params={"q": "benzene", "course_id": "not-a-uuid"},
            )
        assert resp.status_code == 422
