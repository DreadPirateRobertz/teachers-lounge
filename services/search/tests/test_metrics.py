"""Tests for Prometheus metrics: histogram registration, labels, and /metrics endpoint."""
import uuid
from unittest.mock import AsyncMock, patch

import pytest
from fastapi.testclient import TestClient
from prometheus_client import REGISTRY

from app.main import app
from app.metrics import search_query_duration_seconds

COURSE_ID = uuid.uuid4()

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


class TestHistogramRegistration:
    def test_histogram_registered_in_registry(self):
        names = [m.name for m in REGISTRY.collect()]
        assert "search_query_duration_seconds" in names

    def test_histogram_has_query_type_label(self):
        metric = next(
            m for m in REGISTRY.collect() if m.name == "search_query_duration_seconds"
        )
        label_names = metric.samples[0].labels.keys() if metric.samples else set()
        # Confirm the metric object has query_type in its label config
        assert "query_type" in search_query_duration_seconds._labelnames

    def test_histogram_has_status_label(self):
        assert "status" in search_query_duration_seconds._labelnames

    def test_valid_label_combinations_do_not_raise(self):
        for query_type in ("keyword", "semantic", "hybrid"):
            for status in ("success", "error"):
                search_query_duration_seconds.labels(
                    query_type=query_type, status=status
                ).observe(0.01)


class TestMetricsEndpoint:
    def test_metrics_endpoint_returns_200(self, client):
        resp = client.get("/metrics")
        assert resp.status_code == 200

    def test_metrics_endpoint_contains_histogram_name(self, client):
        resp = client.get("/metrics")
        assert b"search_query_duration_seconds" in resp.content

    def test_metrics_content_type_is_text(self, client):
        resp = client.get("/metrics")
        assert "text/plain" in resp.headers["content-type"]


class TestHistogramRecordedOnSearch:
    def test_success_observation_recorded(self, client):
        with (
            patch(_SEARCH_PATCHES["embed"], new_callable=AsyncMock, return_value=[0.1] * 1536),
            patch(_SEARCH_PATCHES["dense"], new_callable=AsyncMock, return_value=[]),
            patch(_SEARCH_PATCHES["sparse"], new_callable=AsyncMock, return_value=[]),
            patch(_SEARCH_PATCHES["rerank"], new_callable=AsyncMock, side_effect=lambda q, r: r),
        ):
            resp = client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}")
        assert resp.status_code == 200

        metrics_resp = client.get("/metrics")
        assert b"search_query_duration_seconds" in metrics_resp.content

    def test_semantic_label_when_sparse_empty(self, client):
        with (
            patch(_SEARCH_PATCHES["embed"], new_callable=AsyncMock, return_value=[0.1] * 1536),
            patch(_SEARCH_PATCHES["dense"], new_callable=AsyncMock, return_value=[]),
            patch(_SEARCH_PATCHES["sparse"], new_callable=AsyncMock, return_value=[]),
            patch(_SEARCH_PATCHES["rerank"], new_callable=AsyncMock, side_effect=lambda q, r: r),
        ):
            resp = client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}")
        assert resp.status_code == 200

        metrics_text = client.get("/metrics").text
        assert 'query_type="semantic"' in metrics_text

    def test_hybrid_label_when_sparse_returns_results(self, client):
        from app.models import ChunkResult

        chunk = ChunkResult(
            chunk_id=uuid.uuid4(),
            material_id=uuid.uuid4(),
            course_id=COURSE_ID,
            content="test",
            score=0.9,
            chapter=None,
            section=None,
            page=None,
            content_type="text",
        )
        with (
            patch(_SEARCH_PATCHES["embed"], new_callable=AsyncMock, return_value=[0.1] * 1536),
            patch(_SEARCH_PATCHES["dense"], new_callable=AsyncMock, return_value=[chunk]),
            patch(_SEARCH_PATCHES["sparse"], new_callable=AsyncMock, return_value=[chunk]),
            patch(_SEARCH_PATCHES["rerank"], new_callable=AsyncMock, side_effect=lambda q, r: r),
        ):
            resp = client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}")
        assert resp.status_code == 200

        metrics_text = client.get("/metrics").text
        assert 'query_type="hybrid"' in metrics_text

    def test_error_label_on_qdrant_failure(self):
        error_client = TestClient(app, raise_server_exceptions=False)
        with (
            patch(_SEARCH_PATCHES["embed"], new_callable=AsyncMock, return_value=[0.1] * 1536),
            patch(
                _SEARCH_PATCHES["dense"],
                new_callable=AsyncMock,
                side_effect=RuntimeError("qdrant down"),
            ),
            patch(_SEARCH_PATCHES["sparse"], new_callable=AsyncMock, return_value=[]),
        ):
            resp = error_client.get(f"/v1/search?q=entropy&course_id={COURSE_ID}")
        assert resp.status_code == 500

        metrics_text = error_client.get("/metrics").text
        assert 'status="error"' in metrics_text
