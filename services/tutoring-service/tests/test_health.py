"""Tests for /healthz and /readyz Kubernetes probe endpoints.

Each test mocks the three dependency checks (_ping_db, _ping_redis,
_ping_gateway) so no real infrastructure is needed.
"""

from __future__ import annotations

from unittest.mock import AsyncMock, patch

import pytest
from httpx import ASGITransport, AsyncClient

from app.main import app

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_PROBE_PATCHES = {
    "app.health._ping_db": None,
    "app.health._ping_redis": None,
    "app.health._ping_gateway": None,
}


def _mock_all(db: bool = True, redis: bool = True, gateway: bool = True):
    """Return a context-manager stack that patches all three probe helpers."""
    import contextlib

    @contextlib.asynccontextmanager
    async def _ctx():
        with (
            patch("app.health._ping_db", new=AsyncMock(return_value=db)),
            patch("app.health._ping_redis", new=AsyncMock(return_value=redis)),
            patch("app.health._ping_gateway", new=AsyncMock(return_value=gateway)),
        ):
            yield

    return _ctx()


# ---------------------------------------------------------------------------
# /healthz — liveness
# ---------------------------------------------------------------------------


@pytest.mark.anyio
async def test_healthz_all_ok():
    """/healthz returns 200 when DB and Redis are both reachable."""
    async with _mock_all(db=True, redis=True):
        async with AsyncClient(
            transport=ASGITransport(app=app), base_url="http://test"
        ) as client:
            resp = await client.get("/healthz")

    assert resp.status_code == 200
    assert resp.json() == {"status": "ok"}


@pytest.mark.anyio
async def test_healthz_db_down():
    """/healthz returns 503 when DB ping fails."""
    async with _mock_all(db=False, redis=True):
        async with AsyncClient(
            transport=ASGITransport(app=app), base_url="http://test"
        ) as client:
            resp = await client.get("/healthz")

    assert resp.status_code == 503
    body = resp.json()
    assert body["status"] == "error"
    assert "db" in body["detail"]


@pytest.mark.anyio
async def test_healthz_redis_down():
    """/healthz returns 503 when Redis ping fails."""
    async with _mock_all(db=True, redis=False):
        async with AsyncClient(
            transport=ASGITransport(app=app), base_url="http://test"
        ) as client:
            resp = await client.get("/healthz")

    assert resp.status_code == 503
    body = resp.json()
    assert body["status"] == "error"
    assert "redis" in body["detail"]


@pytest.mark.anyio
async def test_healthz_both_down():
    """/healthz returns 503 and lists both failures when DB and Redis are both down."""
    async with _mock_all(db=False, redis=False):
        async with AsyncClient(
            transport=ASGITransport(app=app), base_url="http://test"
        ) as client:
            resp = await client.get("/healthz")

    assert resp.status_code == 503
    body = resp.json()
    assert "db" in body["detail"]
    assert "redis" in body["detail"]


# ---------------------------------------------------------------------------
# /readyz — readiness
# ---------------------------------------------------------------------------


@pytest.mark.anyio
async def test_readyz_all_ok():
    """/readyz returns 200 when DB, Redis, and gateway are all reachable."""
    async with _mock_all(db=True, redis=True, gateway=True):
        async with AsyncClient(
            transport=ASGITransport(app=app), base_url="http://test"
        ) as client:
            resp = await client.get("/readyz")

    assert resp.status_code == 200
    assert resp.json() == {"status": "ready"}


@pytest.mark.anyio
async def test_readyz_db_down():
    """/readyz returns 503 when DB ping fails."""
    async with _mock_all(db=False, redis=True, gateway=True):
        async with AsyncClient(
            transport=ASGITransport(app=app), base_url="http://test"
        ) as client:
            resp = await client.get("/readyz")

    assert resp.status_code == 503
    assert "db" in resp.json()["detail"]


@pytest.mark.anyio
async def test_readyz_redis_down():
    """/readyz returns 503 when Redis ping fails."""
    async with _mock_all(db=True, redis=False, gateway=True):
        async with AsyncClient(
            transport=ASGITransport(app=app), base_url="http://test"
        ) as client:
            resp = await client.get("/readyz")

    assert resp.status_code == 503
    assert "redis" in resp.json()["detail"]


@pytest.mark.anyio
async def test_readyz_gateway_down():
    """/readyz returns 503 when the AI gateway is unreachable."""
    async with _mock_all(db=True, redis=True, gateway=False):
        async with AsyncClient(
            transport=ASGITransport(app=app), base_url="http://test"
        ) as client:
            resp = await client.get("/readyz")

    assert resp.status_code == 503
    assert "gateway" in resp.json()["detail"]


@pytest.mark.anyio
async def test_readyz_all_down():
    """/readyz returns 503 and lists all three failures."""
    async with _mock_all(db=False, redis=False, gateway=False):
        async with AsyncClient(
            transport=ASGITransport(app=app), base_url="http://test"
        ) as client:
            resp = await client.get("/readyz")

    assert resp.status_code == 503
    body = resp.json()
    for component in ("db", "redis", "gateway"):
        assert component in body["detail"]


# ---------------------------------------------------------------------------
# _ping_gateway unit tests (no HTTP server needed)
# ---------------------------------------------------------------------------


@pytest.mark.anyio
async def test_ping_gateway_success_on_health_path(respx_mock):
    """`_ping_gateway` returns True when /health returns 200."""
    import respx

    from app.health import _ping_gateway

    respx_mock.get("http://ai-gateway.teachers-lounge.svc.cluster.local:4000/health").mock(
        return_value=respx.MockResponse(200)
    )
    result = await _ping_gateway()
    assert result is True


@pytest.mark.anyio
async def test_ping_gateway_fallback_to_root(respx_mock):
    """`_ping_gateway` falls back to / when /health is not 2xx."""
    import respx

    from app.health import _ping_gateway

    respx_mock.get("http://ai-gateway.teachers-lounge.svc.cluster.local:4000/health").mock(
        return_value=respx.MockResponse(404)
    )
    respx_mock.get("http://ai-gateway.teachers-lounge.svc.cluster.local:4000/").mock(
        return_value=respx.MockResponse(200)
    )
    result = await _ping_gateway()
    assert result is True


@pytest.mark.anyio
async def test_ping_gateway_all_fail(respx_mock):
    """`_ping_gateway` returns False when both /health and / fail."""
    import respx

    from app.health import _ping_gateway

    respx_mock.get("http://ai-gateway.teachers-lounge.svc.cluster.local:4000/health").mock(
        return_value=respx.MockResponse(503)
    )
    respx_mock.get("http://ai-gateway.teachers-lounge.svc.cluster.local:4000/").mock(
        return_value=respx.MockResponse(503)
    )
    result = await _ping_gateway()
    assert result is False
