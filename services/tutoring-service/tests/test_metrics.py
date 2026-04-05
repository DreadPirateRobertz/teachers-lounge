"""Tests for Prometheus metrics and middleware in tutoring-service."""

import pytest
from httpx import ASGITransport, AsyncClient
from prometheus_client import REGISTRY


@pytest.mark.asyncio
async def test_metrics_endpoint_reachable():
    """The /metrics endpoint returns 200 with prometheus text format.

    Verifies that make_asgi_app() is correctly mounted.
    Starlette sub-app mounts redirect /metrics → /metrics/ so we follow.
    """
    from app.main import app

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://test",
        follow_redirects=True,
    ) as client:
        response = await client.get("/metrics")

    assert response.status_code == 200
    assert "text/plain" in response.headers["content-type"]


@pytest.mark.asyncio
async def test_health_endpoint_records_http_metrics():
    """Calling /health causes http_requests_total to increment."""
    from app.main import app
    from app.metrics import http_requests_total

    collector = http_requests_total.labels(method="GET", path="/health", status="200")
    before = collector._value.get()

    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        response = await client.get("/health")

    assert response.status_code == 200
    after = collector._value.get()
    assert after > before


def test_rag_latency_histogram_registered():
    """tl_rag_latency_seconds is registered in the default Prometheus registry."""
    metric_names = [m.name for m in REGISTRY.collect()]
    assert "tl_rag_latency_seconds" in metric_names


def test_stream_ttfb_histogram_registered():
    """tl_stream_ttfb_seconds is registered in the default Prometheus registry."""
    metric_names = [m.name for m in REGISTRY.collect()]
    assert "tl_stream_ttfb_seconds" in metric_names


def test_tokens_used_counter_registered():
    """tl_tokens_used_total is registered in the default Prometheus registry.

    prometheus_client strips the _total suffix from Counter names internally;
    the full tl_tokens_used_total name appears in the exposition output, but
    REGISTRY.collect() returns the base name tl_tokens_used.
    """
    metric_names = [m.name for m in REGISTRY.collect()]
    # prometheus_client stores Counter base names without the _total suffix.
    assert "tl_tokens_used" in metric_names


def test_rag_latency_observe():
    """Observing a RAG latency step increments the histogram sample count."""
    from app.metrics import rag_latency_seconds

    before = rag_latency_seconds.labels(step="retrieval")._sum.get()
    rag_latency_seconds.labels(step="retrieval").observe(0.123)
    after = rag_latency_seconds.labels(step="retrieval")._sum.get()

    assert after > before
