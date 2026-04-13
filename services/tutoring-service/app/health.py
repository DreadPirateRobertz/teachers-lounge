"""Kubernetes health probe endpoints for the tutoring service.

Provides:
- GET /healthz  — liveness probe: checks DB and Redis connectivity.
- GET /readyz   — readiness probe: additionally checks AI gateway reachability.

Kubernetes wires these paths into its liveness/readiness probe config so that
unhealthy pods are restarted (liveness) or removed from load-balancing rotation
(readiness) automatically.
"""

from __future__ import annotations

import logging

import httpx
from fastapi import APIRouter
from fastapi.responses import JSONResponse
from sqlalchemy import text

from .config import settings
from .database import engine

log = logging.getLogger(__name__)

router = APIRouter(tags=["health"])


async def _ping_db() -> bool:
    """Check database connectivity by executing ``SELECT 1``.

    Returns:
        True if the database responds, False on any error.
    """
    try:
        async with engine.connect() as conn:
            await conn.execute(text("SELECT 1"))
        return True
    except Exception as exc:  # noqa: BLE001
        log.warning("health: DB ping failed: %s", exc)
        return False


async def _ping_redis() -> bool:
    """Check Redis connectivity by opening a fresh connection and sending PING.

    A fresh connection is used so that a stale pool at startup doesn't mask
    a recovered Redis instance (or vice-versa).

    Returns:
        True if Redis responds, False on any error.
    """
    import redis.asyncio as aioredis

    try:
        client = aioredis.from_url(
            settings.redis_url,
            encoding="utf-8",
            decode_responses=True,
        )
        await client.ping()
        await client.aclose()
        return True
    except Exception as exc:  # noqa: BLE001
        log.warning("health: Redis ping failed: %s", exc)
        return False


async def _ping_gateway() -> bool:
    """Check AI gateway reachability via HTTP GET.

    Attempts ``GET {ai_gateway_url}/health``; on non-2xx falls back to
    ``GET {ai_gateway_url}/`` before declaring the gateway unreachable.

    Returns:
        True if any probe request returns a 2xx response, False otherwise.
    """
    base = settings.ai_gateway_url.rstrip("/")
    async with httpx.AsyncClient(timeout=5.0) as client:
        for path in ("/health", "/"):
            try:
                resp = await client.get(f"{base}{path}")
                if resp.is_success:
                    return True
            except Exception as exc:  # noqa: BLE001
                log.warning("health: gateway probe %s failed: %s", path, exc)
    return False


@router.get("/healthz", summary="Liveness probe")
async def healthz() -> JSONResponse:
    """Kubernetes liveness probe.

    Checks that the service process is alive and its core dependencies (DB and
    Redis) are reachable.  A 503 response tells Kubernetes to restart the pod.

    Returns:
        200 JSON ``{"status": "ok"}`` when healthy.
        503 JSON ``{"status": "error", "detail": <failed checks>}`` when unhealthy.
    """
    failed: list[str] = []

    if not await _ping_db():
        failed.append("db")
    if not await _ping_redis():
        failed.append("redis")

    if failed:
        return JSONResponse(
            status_code=503,
            content={"status": "error", "detail": f"unhealthy: {', '.join(failed)}"},
        )
    return JSONResponse(status_code=200, content={"status": "ok"})


@router.get("/readyz", summary="Readiness probe")
async def readyz() -> JSONResponse:
    """Kubernetes readiness probe.

    Checks all liveness conditions plus AI gateway reachability.  A 503 response
    tells Kubernetes to stop sending traffic to this pod until it recovers.

    Returns:
        200 JSON ``{"status": "ready"}`` when ready to serve traffic.
        503 JSON ``{"status": "error", "detail": <failed checks>}`` when not ready.
    """
    failed: list[str] = []

    if not await _ping_db():
        failed.append("db")
    if not await _ping_redis():
        failed.append("redis")
    if not await _ping_gateway():
        failed.append("gateway")

    if failed:
        return JSONResponse(
            status_code=503,
            content={"status": "error", "detail": f"unhealthy: {', '.join(failed)}"},
        )
    return JSONResponse(status_code=200, content={"status": "ready"})
