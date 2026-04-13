"""Redis cache — session history snapshots and cross-student insights.

Session history:
  Caches the last N session IDs (by user) and their history payloads
  for session_history_cache_ttl seconds.

Cross-student insights:
  Caches aggregate per-concept insights (e.g. "72% of students struggle here")
  for INSIGHT_TTL seconds (6 h).  These are written by an offline analytics job
  that queries BigQuery; the tutoring service only reads them.  Missing entries
  are silently skipped so the pipeline degrades gracefully.

All cache operations are best-effort: errors are silently swallowed so a Redis
outage never breaks the tutoring flow.
"""
from __future__ import annotations

import json
import logging
from typing import Any
from uuid import UUID

import redis.asyncio as aioredis
from prometheus_client import Counter

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
_INSIGHT_PREFIX = "tutoring:insight:"
# Keep the last 5 session IDs per user in a Redis list
_MAX_CACHED_SESSIONS_PER_USER = 5
# Cross-student insight cache lifetime (6 hours — matches BigQuery job cadence)
_INSIGHT_TTL = 6 * 3600

# ── Cache hit/miss counters (Prometheus) ─────────────────────────────────────

_cache_hits = Counter(
    "tl_cache_hits_total",
    "Total Redis cache hits, labelled by key namespace.",
    labelnames=["namespace"],
)
_cache_misses = Counter(
    "tl_cache_misses_total",
    "Total Redis cache misses (including errors), labelled by key namespace.",
    labelnames=["namespace"],
)


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
    """Return the Redis key for a session's message history.

    Args:
        session_id: UUID of the tutoring session.

    Returns:
        Redis key string under the session-history prefix.
    """
    return f"{_SESSION_HISTORY_PREFIX}{session_id}"


def _user_sessions_key(user_id: UUID) -> str:
    """Return the Redis key for the recent-sessions list of a user.

    Args:
        user_id: UUID of the student.

    Returns:
        Redis key string under the user-sessions prefix.
    """
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
            _cache_misses.labels(namespace="session_history").inc()
            return None
        _cache_hits.labels(namespace="session_history").inc()
        return json.loads(raw)
    except Exception as exc:  # noqa: BLE001
        _cache_misses.labels(namespace="session_history").inc()
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


# ── Cross-student insight cache ───────────────────────────────────────────────

def _insight_key(concept_name: str) -> str:
    """Return the Redis key for cross-student insights about a concept.

    The concept name is lower-cased and whitespace-normalised so that minor
    capitalisation differences do not cause cache misses.

    Args:
        concept_name: Human-readable concept name (e.g. "Mitosis").

    Returns:
        Redis key string under the insight prefix.
    """
    normalised = concept_name.lower().strip()
    return f"{_INSIGHT_PREFIX}{normalised}"


async def get_cross_student_insights(concept_name: str) -> list[str] | None:
    """Return cached cross-student insights for a concept, or None on miss / error.

    Insights are aggregate observations produced offline by a BigQuery analytics
    job and written into Redis by that job.  The tutoring service is a read-only
    consumer: it surfaces insights in the system prompt so Professor Nova can
    mention common struggle points and effective explanation styles.

    Args:
        concept_name: Human-readable name of the concept being discussed.

    Returns:
        List of insight strings if a cached entry exists, otherwise None.
    """
    if _redis is None:
        return None
    try:
        raw = await _redis.get(_insight_key(concept_name))
        if raw is None:
            _cache_misses.labels(namespace="insight").inc()
            return None
        _cache_hits.labels(namespace="insight").inc()
        return json.loads(raw)
    except Exception as exc:  # noqa: BLE001
        _cache_misses.labels(namespace="insight").inc()
        log.debug("cache get_insight miss/error for %s: %s", _log_safe(concept_name), exc)
        return None


async def set_cross_student_insights(concept_name: str, insights: list[str]) -> None:
    """Cache cross-student insights for a concept with a 6-hour TTL.

    Intended to be called by the offline BigQuery analytics job, not by the
    tutoring service request path.  Provided here so tests and scripts can
    seed the cache without coupling to the analytics job internals.

    Args:
        concept_name: Human-readable name of the concept.
        insights: List of insight strings to cache (e.g. aggregated struggle %,
            effective explanation styles, common misconceptions).
    """
    if _redis is None:
        return
    try:
        await _redis.set(
            _insight_key(concept_name),
            json.dumps(insights),
            ex=_INSIGHT_TTL,
        )
    except Exception as exc:  # noqa: BLE001
        log.debug("cache set_insight error for %s: %s", _log_safe(concept_name), exc)
