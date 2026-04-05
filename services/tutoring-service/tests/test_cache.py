"""Tests for the Redis session-history cache (app/cache.py).

Uses fakeredis for in-process Redis emulation — no external service required.
"""
import json
from uuid import UUID, uuid4

import pytest
import pytest_asyncio
import fakeredis.aioredis as fakeredis

import app.cache as cache_module


@pytest_asyncio.fixture(autouse=True)
async def fake_redis(monkeypatch):
    """Replace the module-level Redis client with a fakeredis instance."""
    fake = fakeredis.FakeRedis(decode_responses=True)
    monkeypatch.setattr(cache_module, "_redis", fake)
    yield fake
    await fake.aclose()


@pytest.fixture
def session_id() -> UUID:
    return uuid4()


@pytest.fixture
def user_id() -> UUID:
    return uuid4()


@pytest.fixture
def messages() -> list[dict]:
    return [
        {"id": str(uuid4()), "role": "student", "content": "What is ATP?"},
        {"id": str(uuid4()), "role": "tutor", "content": "ATP is adenosine triphosphate…"},
    ]


# ── get / set round-trip ───────────────────���──────────────────────────────────

@pytest.mark.asyncio
async def test_cache_miss_returns_none(session_id):
    """get_cached_history returns None when nothing is cached."""
    result = await cache_module.get_cached_history(session_id)
    assert result is None


@pytest.mark.asyncio
async def test_set_then_get_returns_messages(user_id, session_id, messages):
    """Cached history is returned verbatim after set_cached_history."""
    await cache_module.set_cached_history(user_id, session_id, messages)
    result = await cache_module.get_cached_history(session_id)
    assert result == messages


@pytest.mark.asyncio
async def test_set_cached_history_sets_ttl(fake_redis, user_id, session_id, messages):
    """History key has a positive TTL set by set_cached_history."""
    await cache_module.set_cached_history(user_id, session_id, messages)
    key = cache_module._history_key(session_id)
    ttl = await fake_redis.ttl(key)
    assert ttl > 0, f"expected positive TTL, got {ttl}"


# ── invalidation ─────────────────────────��──────────────────────────��────────

@pytest.mark.asyncio
async def test_invalidate_clears_cache(user_id, session_id, messages):
    """invalidate_session_history evicts the cached entry."""
    await cache_module.set_cached_history(user_id, session_id, messages)
    await cache_module.invalidate_session_history(session_id)
    result = await cache_module.get_cached_history(session_id)
    assert result is None


# ── per-user session list ─────────────────────────────────────────────────────

@pytest.mark.asyncio
async def test_user_sessions_list_capped(fake_redis, user_id, messages):
    """Per-user session list is capped at _MAX_CACHED_SESSIONS_PER_USER entries."""
    limit = cache_module._MAX_CACHED_SESSIONS_PER_USER
    for _ in range(limit + 3):
        await cache_module.set_cached_history(user_id, uuid4(), messages)

    user_key = cache_module._user_sessions_key(user_id)
    stored = await fake_redis.lrange(user_key, 0, -1)
    assert len(stored) == limit


@pytest.mark.asyncio
async def test_user_sessions_list_no_duplicates(fake_redis, user_id, session_id, messages):
    """Setting the same session twice does not duplicate its entry in the list."""
    await cache_module.set_cached_history(user_id, session_id, messages)
    await cache_module.set_cached_history(user_id, session_id, messages)

    user_key = cache_module._user_sessions_key(user_id)
    stored = await fake_redis.lrange(user_key, 0, -1)
    count = stored.count(str(session_id))
    assert count == 1, f"expected 1 occurrence of session_id, got {count}"


# ── resilience (Redis unavailable) ─────────────────────────────���─────────────

@pytest.mark.asyncio
async def test_get_returns_none_when_redis_unavailable(monkeypatch, session_id):
    """get_cached_history returns None gracefully when Redis is None."""
    monkeypatch.setattr(cache_module, "_redis", None)
    result = await cache_module.get_cached_history(session_id)
    assert result is None


@pytest.mark.asyncio
async def test_set_is_noop_when_redis_unavailable(monkeypatch, user_id, session_id, messages):
    """set_cached_history does not raise when Redis is None."""
    monkeypatch.setattr(cache_module, "_redis", None)
    await cache_module.set_cached_history(user_id, session_id, messages)  # must not raise


@pytest.mark.asyncio
async def test_invalidate_is_noop_when_redis_unavailable(monkeypatch, session_id):
    """invalidate_session_history does not raise when Redis is None."""
    monkeypatch.setattr(cache_module, "_redis", None)
    await cache_module.invalidate_session_history(session_id)  # must not raise
