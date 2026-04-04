"""Tests for the search endpoint: query validation, chapter filtering, and result schema."""
import uuid
from unittest.mock import AsyncMock, patch

import pytest
from fastapi.testclient import TestClient

from app.main import app
from app.models import ChunkResult

COURSE_ID = uuid.uuid4()

# Patch all three I/O dependencies for every endpoint test so requests
# never hit real services.
_SEARCH_PATCHES = {
    "embed": "app.routers.search.embed_query",
    "dense": "app.routers.search.dense_search",
    "sparse": "app.routers.search.sparse_search",
    "rerank": "app.routers.search.rerank",
}


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


def _patch_pipeline(dense_results=None, sparse_results=None, rerank_passthrough=True):
    """Return a context manager that patches embed, dense, sparse, and rerank."""
    import contextlib

    dense_results = dense_results or []
    sparse_results = sparse_results or []

    @contextlib.contextmanager
    def _ctx():
        with (
            patch(_SEARCH_PATCHES["embed"], new_callable=AsyncMock, return_value=[0.1] * 1536),
            patch(_SEARCH_PATCHES["dense"], new_callable=AsyncMock, return_value=dense_results) as mock_dense,
            patch(_SEARCH_PATCHES["sparse"], new_callable=AsyncMock, return_value=sparse_results) as mock_sparse,
            patch(_SEARCH_PATCHES["rerank"], new_callable=AsyncMock, side_effect=lambda q, r: r) as mock_rerank,
        ):
            yield {
                "dense": mock_dense,
                "sparse": mock_sparse,
                "rerank": mock_rerank,
            }

    return _ctx()


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

    def test_valid_request_accepted(self, client):
        with _patch_pipeline():
            resp = client.get(f"/v1/search?q=what+is+entropy&course_id={COURSE_ID}")
            assert resp.status_code == 200

    def test_default_limit_applied(self, client):
        with _patch_pipeline() as mocks:
            client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}")
            _, kwargs = mocks["dense"].call_args
            assert kwargs["limit"] == 10

    def test_custom_limit_passed_through(self, client):
        with _patch_pipeline() as mocks:
            client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}&limit=5")
            _, kwargs = mocks["dense"].call_args
            assert kwargs["limit"] == 5


class TestChapterFiltering:
    def test_chapter_none_by_default(self, client):
        with _patch_pipeline() as mocks:
            client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}")
            _, kwargs = mocks["dense"].call_args
            assert kwargs["chapter"] is None
            _, kwargs = mocks["sparse"].call_args
            assert kwargs["chapter"] is None

    def test_chapter_forwarded_to_searches(self, client):
        with _patch_pipeline() as mocks:
            client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}&chapter=Chapter+3")
            _, dense_kwargs = mocks["dense"].call_args
            assert dense_kwargs["chapter"] == "Chapter 3"
            _, sparse_kwargs = mocks["sparse"].call_args
            assert sparse_kwargs["chapter"] == "Chapter 3"

    def test_chapter_filter_accepted(self, client):
        with _patch_pipeline():
            resp = client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}&chapter=Chapter+1")
            assert resp.status_code == 200


class TestResultSchema:
    def test_response_shape(self, client):
        chunks = [_make_chunk(score=0.95), _make_chunk(score=0.80)]
        with _patch_pipeline(dense_results=chunks):
            resp = client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}")
            assert resp.status_code == 200
            body = resp.json()
            assert body["query"] == "entropy"
            assert str(body["course_id"]) == str(COURSE_ID)
            assert body["total"] == 2
            assert len(body["results"]) == 2
            assert body["search_mode"] == "hybrid"

    def test_result_fields_present(self, client):
        with _patch_pipeline(dense_results=[_make_chunk(score=0.9)]):
            resp = client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}")
            result = resp.json()["results"][0]
            assert "chunk_id" in result
            assert "material_id" in result
            assert "course_id" in result
            assert "content" in result
            assert "score" in result
            assert "content_type" in result

    def test_empty_results_valid(self, client):
        with _patch_pipeline():
            resp = client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}")
            body = resp.json()
            assert body["results"] == []
            assert body["total"] == 0

    def test_limit_caps_results(self, client):
        chunks = [_make_chunk(score=s / 10) for s in range(20, 0, -1)]
        with _patch_pipeline(dense_results=chunks):
            resp = client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}&limit=5")
            assert len(resp.json()["results"]) == 5


class TestCourseIdFiltering:
    def test_course_id_forwarded_to_qdrant(self, client):
        cid = uuid.uuid4()
        with _patch_pipeline() as mocks:
            client.get(f"/v1/search?q=entropy&course_id={cid}")
            _, dense_kwargs = mocks["dense"].call_args
            assert dense_kwargs["course_id"] == cid
            _, sparse_kwargs = mocks["sparse"].call_args
            assert sparse_kwargs["course_id"] == cid
