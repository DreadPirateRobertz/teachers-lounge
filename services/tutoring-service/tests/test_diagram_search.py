"""Tests for diagram search integration in the chat flow (Phase 6)."""
from unittest.mock import AsyncMock, patch

import pytest

from app.chat import _sse
from app.search_client import DiagramResult, fetch_diagram_chunks


class TestSseHelper:
    def test_diagram_event_includes_diagram_key(self):
        """_sse with event_type='diagram' serialises the diagram dict."""
        import json
        diagram = {"diagram_id": "abc", "gcs_path": "gs://bucket/fig.png", "score": 0.9}
        frame = _sse("diagram", message_id="msg-1", diagram=diagram)
        data = frame.strip().removeprefix("data: ")
        payload = json.loads(data)
        assert payload["type"] == "diagram"
        assert payload["diagram"]["diagram_id"] == "abc"
        assert payload["message_id"] == "msg-1"

    def test_sources_event_has_no_diagram_key(self):
        """_sse with event_type='sources' omits diagram key."""
        import json
        frame = _sse("sources", message_id="m", sources=[])
        payload = json.loads(frame.strip().removeprefix("data: "))
        assert "diagram" not in payload


class TestFetchDiagramChunks:
    async def test_returns_diagram_results_on_success(self):
        """fetch_diagram_chunks parses DiagramResult objects from response."""
        mock_response = {
            "results": [
                {
                    "diagram_id": "d1",
                    "course_id": "00000000-0000-0000-0000-000000000001",
                    "gcs_path": "gs://bucket/fig1.png",
                    "caption": "Benzene ring",
                    "figure_type": "diagram",
                    "page": 10,
                    "chapter": "Ch 6",
                    "score": 0.88,
                }
            ]
        }

        import uuid
        course_id = uuid.UUID("00000000-0000-0000-0000-000000000001")

        class _FakeResp:
            def raise_for_status(self): pass
            def json(self): return mock_response

        class _FakeClient:
            async def __aenter__(self): return self
            async def __aexit__(self, *a): pass
            async def get(self, *a, **kw): return _FakeResp()

        with patch("app.search_client.httpx.AsyncClient", return_value=_FakeClient()):
            results = await fetch_diagram_chunks("benzene ring", course_id)

        assert len(results) == 1
        assert results[0].caption == "Benzene ring"
        assert results[0].score == pytest.approx(0.88)

    async def test_returns_empty_on_http_error(self):
        """fetch_diagram_chunks returns [] when the search service is unavailable."""
        import uuid
        course_id = uuid.UUID("00000000-0000-0000-0000-000000000002")

        class _FailClient:
            async def __aenter__(self): return self
            async def __aexit__(self, *a): pass
            async def get(self, *a, **kw):
                raise ConnectionError("service down")

        with patch("app.search_client.httpx.AsyncClient", return_value=_FailClient()):
            results = await fetch_diagram_chunks("benzene ring", course_id)

        assert results == []
