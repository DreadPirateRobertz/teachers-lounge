"""Redis cache — session history snapshots.

Caches the last N session IDs (by user) and their history payloads
for session_history_cache_ttl seconds.  Cache is best-effort: all errors
are silently swallowed so a Redis outage never breaks the tutoring flow.
"""
from __future__ import annotations

import json
import logging
from typing import Any
from uuid import UUID

import redis.asyncio as aioredis

from .config import settings

log = logging.getLogger(__name__)


def _log_safe(val: object) -> str:
    """Sanitize a value for log output to prevent log injection.

    Args:
        val: Any value to sanitize.

    Returns:
        String with newlines escaped.
    """
    return str(val).replace("\n", "\\n").replace("\r", "\\r")


# Injected at startup via init_cache() — None if Redis is unavailable.
_redis: aioredis.Redis | None = None

# Key schema
_SESSION_HISTORY_PREFIX = "tutoring:session_history:"
_USER_SESSIONS_PREFIX = "tutoring:user_sessions:"
# Keep the last 5 session IDs per user in a Redis list
_MAX_CACHED_SESSIONS_PER_USER = 5


async def init_cache() -> None:
    """Initialise the Redis connection pool.  Called once at application startup."""
    global _redis
    try:
        pool = aioredis.ConnectionPool.from_url(
            settings.redis_url,
            encoding="utf-8",
            decode_responses=True,
            max_connections=20,
        )
        _redis = aioredis.Redis(connection_pool=pool)
        await _redis.ping()
        log.info("Redis cache connected: %s", settings.redis_url)
    except Exception as exc:  # noqa: BLE001
        log.warning("Redis cache unavailable — running without cache: %s", exc)
        _redis = None


async def close_cache() -> None:
    """Close the Redis connection pool.  Called at application shutdown."""
    global _redis
    if _redis is not None:
        await _redis.aclose()
        _redis = None


# ── Session history cache ─────────────────────────────────────────────────────

def _history_key(session_id: UUID) -> str:
    return f"{_SESSION_HISTORY_PREFIX}{session_id}"


def _user_sessions_key(user_id: UUID) -> str:
    return f"{_USER_SESSIONS_PREFIX}{user_id}"


async def get_cached_history(session_id: UUID) -> list[dict[str, Any]] | None:
    """Return cached history for session_id, or None on miss / error.

    Args:
        session_id: UUID of the tutoring session.

    Returns:
        List of message dicts (as originally serialised) or None.
    """
    if _redis is None:
        return None
    try:
        raw = await _redis.get(_history_key(session_id))
        if raw is None:
            return None
        return json.loads(raw)
    except Exception as exc:  # noqa: BLE001
        log.debug("cache get_history miss/error for %s: %s", _log_safe(session_id), exc)
        return None


async def set_cached_history(
    user_id: UUID,
    session_id: UUID,
    messages: list[dict[str, Any]],
) -> None:
    """Cache history for session_id, updating the per-user recent-sessions list.

    Args:
        user_id: Owner of the session (used to maintain the per-user list).
        session_id: UUID of the tutoring session.
        messages: Serialisable list of message dicts.
    """
    if _redis is None:
        return
    try:
        pipe = _redis.pipeline()
        key = _history_key(session_id)
        pipe.set(key, json.dumps(messages), ex=settings.session_history_cache_ttl)

        # Track the most recent sessions for this user (capped list)
        user_key = _user_sessions_key(user_id)
        sid_str = str(session_id)
        pipe.lrem(user_key, 0, sid_str)  # remove duplicate if present
        pipe.lpush(user_key, sid_str)
        pipe.ltrim(user_key, 0, _MAX_CACHED_SESSIONS_PER_USER - 1)
        pipe.expire(user_key, settings.session_history_cache_ttl)

        await pipe.execute()
    except Exception as exc:  # noqa: BLE001
        log.debug("cache set_history error for %s: %s", _log_safe(session_id), exc)


async def invalidate_session_history(session_id: UUID) -> None:
    """Evict the cached history for session_id (called after new messages are appended).

    Args:
        session_id: UUID of the tutoring session to invalidate.
    """
    if _redis is None:
        return
    try:
        await _redis.delete(_history_key(session_id))
    except Exception as exc:  # noqa: BLE001
        log.debug("cache invalidate error for %s: %s", _log_safe(session_id), exc)
