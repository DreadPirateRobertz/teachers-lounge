"""Tests for search-service Prometheus metrics (tl-he3).

Verifies the ``search_query_duration_seconds`` histogram is registered with
the expected labels/buckets, gets incremented via :func:`observe_query` on
success and error paths, and is wired into Qdrant call sites plus the
``/metrics`` endpoint.
"""
from __future__ import annotations

import uuid
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
from fastapi.testclient import TestClient

from app.main import app
from app.metrics import (
    SEARCH_QUERY_DURATION,
    SEARCH_QUERY_DURATION_BUCKETS,
    observe_query,
)

COURSE_ID = uuid.uuid4()


def _sum(query_type: str, status: str) -> float:
    """Read the current ``_sum`` counter for a label pair from the registry."""
    return (
        SEARCH_QUERY_DURATION.labels(query_type=query_type, status=status)
        ._sum.get()
    )


def _count(query_type: str, status: str) -> float:
    """Read the current ``_count`` counter for a label pair."""
    return (
        SEARCH_QUERY_DURATION.labels(query_type=query_type, status=status)
        ._count.get()
    )


class TestHistogramDefinition:
    """The histogram must match the tl-he3 spec exactly."""

    def test_buckets_match_spec(self) -> None:
        """Spec requires buckets [.01, .05, .1, .25, .5, 1.0, 2.5]."""
        assert SEARCH_QUERY_DURATION_BUCKETS == (0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5)

    def test_label_names(self) -> None:
        """Labels must be query_type and status in that order."""
        assert SEARCH_QUERY_DURATION._labelnames == ("query_type", "status")

    def test_metric_name(self) -> None:
        """Metric name must be search_query_duration_seconds."""
        assert SEARCH_QUERY_DURATION._name == "search_query_duration_seconds"


class TestObserveQuery:
    """``observe_query`` must record once per call, with correct status label."""

    @pytest.mark.asyncio
    async def test_success_records_success_label(self) -> None:
        """Happy path yields status=success."""
        before = _count("semantic", "success")
        async with observe_query("semantic"):
            pass
        assert _count("semantic", "success") == before + 1

    @pytest.mark.asyncio
    async def test_error_records_error_label_and_reraises(self) -> None:
        """Exception path records status=error and propagates."""
        before = _count("semantic", "error")
        with pytest.raises(RuntimeError, match="boom"):
            async with observe_query("semantic"):
                raise RuntimeError("boom")
        assert _count("semantic", "error") == before + 1

    @pytest.mark.asyncio
    async def test_duration_is_positive(self) -> None:
        """The recorded observation must be > 0 (perf_counter monotonic)."""
        before = _sum("keyword", "success")
        async with observe_query("keyword"):
            pass
        assert _sum("keyword", "success") > before


class TestQdrantCallSitesInstrumented:
    """Each Qdrant search call site must emit an observation."""

    @pytest.mark.asyncio
    async def test_dense_search_records_semantic(self) -> None:
        """dense_search wraps its Qdrant call with observe_query('semantic')."""
        from app.services import qdrant as q

        before = _count("semantic", "success")
        fake = MagicMock()
        fake.search = AsyncMock(return_value=[])
        with patch.object(q, "get_client", return_value=fake):
            await q.dense_search([0.0] * 1536, COURSE_ID, limit=3)
        assert _count("semantic", "success") == before + 1

    @pytest.mark.asyncio
    async def test_sparse_search_records_keyword(self) -> None:
        """sparse_search wraps its Qdrant call with observe_query('keyword')."""
        from app.services import qdrant as q

        before = _count("keyword", "success")
        fake = MagicMock()
        fake.search = AsyncMock(return_value=[])
        with patch.object(q, "get_client", return_value=fake):
            await q.sparse_search("algebra intro", COURSE_ID, limit=3)
        assert _count("keyword", "success") == before + 1

    @pytest.mark.asyncio
    async def test_sparse_search_error_records_keyword_error(self) -> None:
        """Qdrant failures on the sparse path still produce an observation."""
        from app.services import qdrant as q

        before = _count("keyword", "error")
        fake = MagicMock()
        fake.search = AsyncMock(side_effect=RuntimeError("no sparse index"))
        with patch.object(q, "get_client", return_value=fake):
            # sparse_search catches the exception and returns [], but the
            # observation should still fire with status=error because
            # observe_query is inside the try.
            result = await q.sparse_search("x", COURSE_ID, limit=3)
        assert result == []
        assert _count("keyword", "error") == before + 1

    @pytest.mark.asyncio
    async def test_diagram_search_records_semantic(self) -> None:
        """diagram_search wraps its Qdrant call with observe_query('semantic')."""
        from app.services import qdrant as q

        before = _count("semantic", "success")
        fake = MagicMock()
        fake.search = AsyncMock(return_value=[])
        with patch.object(q, "get_client", return_value=fake):
            await q.diagram_search([0.0] * 768, COURSE_ID, limit=3)
        assert _count("semantic", "success") == before + 1


class TestHybridEndpointInstrumented:
    """The /v1/search endpoint must record a 'hybrid' observation per call."""

    @pytest.mark.asyncio
    async def test_hybrid_endpoint_records(self) -> None:
        """End-to-end: a hybrid search increments hybrid/success."""
        before = _count("hybrid", "success")
        with (
            patch(
                "app.routers.search.embed_query",
                new_callable=AsyncMock,
                return_value=[0.1] * 1536,
            ),
            patch(
                "app.routers.search.dense_search",
                new_callable=AsyncMock,
                return_value=[],
            ),
            patch(
                "app.routers.search.sparse_search",
                new_callable=AsyncMock,
                return_value=[],
            ),
            patch(
                "app.routers.search.rerank",
                new_callable=AsyncMock,
                side_effect=lambda q, r: r,
            ),
        ):
            with TestClient(app) as client:
                resp = client.get(f"/v1/search?q=hello&course_id={COURSE_ID}")
                assert resp.status_code == 200
        assert _count("hybrid", "success") == before + 1


class TestMetricsEndpoint:
    """/metrics must expose the histogram in Prometheus exposition format."""

    def test_metrics_endpoint_exposes_histogram(self) -> None:
        """The /metrics endpoint must render the histogram for scraping."""
        with TestClient(app) as client:
            resp = client.get("/metrics")
        assert resp.status_code == 200
        body = resp.text
        assert "search_query_duration_seconds" in body
        # Standard histogram sub-series present
        assert "search_query_duration_seconds_bucket" in body
        assert "search_query_duration_seconds_count" in body
