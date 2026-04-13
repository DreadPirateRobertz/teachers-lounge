"""Tests for the Redis session-history cache (app/cache.py).

Uses fakeredis for in-process Redis emulation — no external service required.
"""

import json
from uuid import UUID, uuid4

import fakeredis.aioredis as fakeredis
import pytest
import pytest_asyncio

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


# ── Cross-student insights ────────────────────────────────────────────────────

@pytest.mark.asyncio
async def test_get_insight_miss_returns_none():
    """get_cross_student_insights returns None when no entry exists."""
    result = await cache_module.get_cross_student_insights("Mitosis")
    assert result is None


@pytest.mark.asyncio
async def test_set_then_get_insight_round_trips():
    """set_cross_student_insights stores insights retrievable by get_cross_student_insights."""
    insights = [
        "72% of students confuse mitosis with meiosis at first",
        "Visual cell diagrams improve comprehension by 40%",
    ]
    await cache_module.set_cross_student_insights("Mitosis", insights)
    result = await cache_module.get_cross_student_insights("Mitosis")
    assert result == insights


@pytest.mark.asyncio
async def test_get_insight_normalises_concept_name():
    """get_cross_student_insights is case-insensitive and strips whitespace."""
    insights = ["Common struggle: phase ordering"]
    await cache_module.set_cross_student_insights("mitosis", insights)
    result = await cache_module.get_cross_student_insights("  MITOSIS  ")
    assert result == insights


@pytest.mark.asyncio
async def test_set_insight_sets_ttl(fake_redis):
    """set_cross_student_insights stores entry with a positive TTL."""
    await cache_module.set_cross_student_insights("Entropy", ["Students often conflate entropy and enthalpy"])
    key = cache_module._insight_key("Entropy")
    ttl = await fake_redis.ttl(key)
    assert ttl > 0, f"expected positive TTL for insight key, got {ttl}"


@pytest.mark.asyncio
async def test_set_insight_ttl_matches_constant(fake_redis):
    """TTL is set to _INSIGHT_TTL (6 hours)."""
    await cache_module.set_cross_student_insights("DNA Replication", ["step 3 has high error rate"])
    key = cache_module._insight_key("DNA Replication")
    ttl = await fake_redis.ttl(key)
    # Allow a small delta for test execution time
    assert abs(ttl - cache_module._INSIGHT_TTL) <= 5


@pytest.mark.asyncio
async def test_get_insight_returns_none_when_redis_unavailable(monkeypatch):
    """get_cross_student_insights returns None when Redis is None."""
    monkeypatch.setattr(cache_module, "_redis", None)
    result = await cache_module.get_cross_student_insights("Mitosis")
    assert result is None


@pytest.mark.asyncio
async def test_set_insight_is_noop_when_redis_unavailable(monkeypatch):
    """set_cross_student_insights does not raise when Redis is None."""
    monkeypatch.setattr(cache_module, "_redis", None)
    await cache_module.set_cross_student_insights("Mitosis", ["insight"])  # must not raise
