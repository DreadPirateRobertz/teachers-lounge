"""Tests for the search endpoint: query validation, result schema, and hybrid mode."""
import uuid
from unittest.mock import AsyncMock, patch

import pytest
from fastapi.testclient import TestClient

from app.main import app
from app.models import ChunkResult

COURSE_ID = uuid.uuid4()


@pytest.fixture
def client():
    with TestClient(app) as c:
        yield c


def _make_chunk(**kwargs) -> ChunkResult:
    defaults = dict(
        chunk_id=uuid.uuid4(),
        material_id=uuid.uuid4(),
        course_id=COURSE_ID,
        content="Some curriculum content.",
        score=0.9,
        chapter="Chapter 1",
        section="1.1",
        page=5,
        content_type="text",
    )
    defaults.update(kwargs)
    return ChunkResult(**defaults)


# Shared mock for sparse_search — returns empty by default (dense-only mode)
_no_sparse = patch("app.routers.search.sparse_search", new_callable=AsyncMock, return_value=[])


class TestQueryValidation:
    def test_empty_query_rejected(self, client):
        resp = client.get(f"/v1/search?q=&course_id={COURSE_ID}")
        assert resp.status_code == 422

    def test_missing_query_rejected(self, client):
        resp = client.get(f"/v1/search?course_id={COURSE_ID}")
        assert resp.status_code == 422

    def test_missing_course_id_rejected(self, client):
        resp = client.get("/v1/search?q=what+is+entropy")
        assert resp.status_code == 422

    def test_invalid_course_id_rejected(self, client):
        resp = client.get("/v1/search?q=entropy&course_id=not-a-uuid")
        assert resp.status_code == 422

    def test_limit_zero_rejected(self, client):
        resp = client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}&limit=0")
        assert resp.status_code == 422

    def test_limit_over_max_rejected(self, client):
        resp = client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}&limit=51")
        assert resp.status_code == 422

    @patch("app.routers.search.dense_search", new_callable=AsyncMock, return_value=[])
    @_no_sparse
    def test_valid_request_accepted(self, mock_sparse, mock_dense, client):
        resp = client.get(f"/v1/search?q=what+is+entropy&course_id={COURSE_ID}")
        assert resp.status_code == 200

    @patch("app.routers.search.dense_search", new_callable=AsyncMock, return_value=[])
    @_no_sparse
    def test_course_id_forwarded_to_dense_search(self, mock_sparse, mock_dense, client):
        cid = uuid.uuid4()
        client.get(f"/v1/search?q=entropy&course_id={cid}")
        _, kwargs = mock_dense.call_args
        assert kwargs["course_id"] == cid


class TestResultSchema:
    @patch(
        "app.routers.search.dense_search",
        new_callable=AsyncMock,
        return_value=[
            _make_chunk(score=0.95),
            _make_chunk(score=0.80),
        ],
    )
    @_no_sparse
    def test_response_shape(self, mock_sparse, mock_dense, client):
        resp = client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}")
        assert resp.status_code == 200
        body = resp.json()
        assert body["query"] == "entropy"
        assert str(body["course_id"]) == str(COURSE_ID)
        assert body["total"] == 2
        assert len(body["results"]) == 2
        # sparse is empty → dense-only mode
        assert body["search_mode"] == "dense"

    @patch(
        "app.routers.search.dense_search",
        new_callable=AsyncMock,
        return_value=[_make_chunk(score=0.9)],
    )
    @_no_sparse
    def test_result_fields_present(self, mock_sparse, mock_dense, client):
        resp = client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}")
        result = resp.json()["results"][0]
        assert "chunk_id" in result
        assert "material_id" in result
        assert "course_id" in result
        assert "content" in result
        assert "score" in result
        assert "content_type" in result

    @patch("app.routers.search.dense_search", new_callable=AsyncMock, return_value=[])
    @_no_sparse
    def test_empty_results_valid(self, mock_sparse, mock_dense, client):
        resp = client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}")
        body = resp.json()
        assert body["results"] == []
        assert body["total"] == 0

    @patch(
        "app.routers.search.dense_search",
        new_callable=AsyncMock,
        return_value=[_make_chunk(score=s / 10) for s in range(20, 0, -1)],
    )
    @_no_sparse
    def test_limit_caps_results(self, mock_sparse, mock_dense, client):
        resp = client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}&limit=5")
        assert len(resp.json()["results"]) == 5


class TestHybridSearchMode:
    @patch("app.routers.search.dense_search", new_callable=AsyncMock)
    @patch("app.routers.search.sparse_search", new_callable=AsyncMock)
    def test_hybrid_mode_when_sparse_returns_results(
        self, mock_sparse, mock_dense, client
    ):
        """search_mode is 'hybrid' when sparse search returns results."""
        chunk = _make_chunk(score=0.9)
        mock_dense.return_value = [chunk]
        mock_sparse.return_value = [chunk]

        resp = client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}")
        assert resp.json()["search_mode"] == "hybrid"

    @patch("app.routers.search.dense_search", new_callable=AsyncMock, return_value=[])
    @_no_sparse
    def test_dense_mode_when_sparse_empty(self, mock_sparse, mock_dense, client):
        """search_mode is 'dense' when sparse returns nothing (not yet indexed)."""
        resp = client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}")
        assert resp.json()["search_mode"] == "dense"


class TestCourseIdFiltering:
    @patch("app.routers.search.dense_search", new_callable=AsyncMock, return_value=[])
    @_no_sparse
    def test_course_id_forwarded_to_qdrant(self, mock_sparse, mock_dense, client):
        cid = uuid.uuid4()
        client.get(f"/v1/search?q=entropy&course_id={cid}")
        _, kwargs = mock_dense.call_args
        assert kwargs["course_id"] == cid
