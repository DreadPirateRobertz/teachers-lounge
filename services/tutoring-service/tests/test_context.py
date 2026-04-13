"""Unit tests for app/context.py — pruning logic, token counting, summarisation."""
from __future__ import annotations

import uuid
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from app.context import (
    build_pruned_history,
    count_tokens,
    log_token_usage,
    summarise_older_context,
)

# ---------------------------------------------------------------------------
# count_tokens
# ---------------------------------------------------------------------------


def test_count_tokens_empty():
    assert count_tokens([]) == 0


def test_count_tokens_single_message():
    # 40 chars / 4 = 10 content tokens + 4 overhead = 14
    msg = [{"role": "user", "content": "a" * 40}]
    assert count_tokens(msg) == 14


def test_count_tokens_multiple_messages():
    msgs = [
        {"role": "system", "content": "a" * 400},
        {"role": "user", "content": "b" * 80},
        {"role": "assistant", "content": "c" * 120},
    ]
    # (400 + 80 + 120) // 4 + 3 * 4 = 150 + 12 = 162
    assert count_tokens(msgs) == 162


def test_count_tokens_none_content():
    # content=None should not crash
    msgs = [{"role": "user", "content": None}]
    assert count_tokens(msgs) == 4  # 0 chars + 1*4 overhead


def test_count_tokens_missing_content():
    msgs = [{"role": "user"}]
    assert count_tokens(msgs) == 4


# ---------------------------------------------------------------------------
# log_token_usage
# ---------------------------------------------------------------------------


def test_log_token_usage_below_threshold_no_warning(caplog):
    msgs = [{"role": "user", "content": "a" * 100}]
    import logging

    with caplog.at_level(logging.WARNING, logger="app.context"):
        result = log_token_usage(msgs, model_context_limit=10000, warn_ratio=0.8, session_id=uuid.uuid4())
    assert result > 0
    assert "approaching context limit" not in caplog.text


def test_log_token_usage_at_threshold_emits_warning(caplog):
    # Make a message large enough to exceed 80% of 100-token limit
    msgs = [{"role": "user", "content": "a" * 400}]  # ~104 tokens > 80
    import logging

    with caplog.at_level(logging.WARNING, logger="app.context"):
        log_token_usage(msgs, model_context_limit=100, warn_ratio=0.8, session_id=uuid.uuid4())
    assert "approaching context limit" in caplog.text


# ---------------------------------------------------------------------------
# summarise_older_context
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_summarise_older_context_empty_messages():
    """Empty input returns empty string without calling the client."""
    client = MagicMock()
    result = await summarise_older_context(client, "tutor-fast", [])
    assert result == ""
    client.chat.completions.create.assert_not_called()


@pytest.mark.asyncio
async def test_summarise_older_context_returns_content():
    response = MagicMock()
    response.choices[0].message.content = "Student studied acids and bases."
    client = AsyncMock()
    client.chat.completions.create = AsyncMock(return_value=response)

    result = await summarise_older_context(
        client, "tutor-fast", [{"role": "user", "content": "What is pH?"}]
    )
    assert result == "Student studied acids and bases."


@pytest.mark.asyncio
async def test_summarise_older_context_handles_gateway_error():
    client = AsyncMock()
    client.chat.completions.create = AsyncMock(side_effect=RuntimeError("gateway down"))

    result = await summarise_older_context(
        client, "tutor-fast", [{"role": "user", "content": "Some message"}]
    )
    assert result == ""


# ---------------------------------------------------------------------------
# build_pruned_history
# ---------------------------------------------------------------------------


def _make_interactions(n: int):
    """Build n fake Interaction-like objects."""
    interactions = []
    for i in range(n):
        m = MagicMock()
        m.role = "student" if i % 2 == 0 else "tutor"
        m.content = f"message {i}"
        interactions.append(m)
    return interactions


@pytest.mark.asyncio
async def test_build_pruned_history_short_session_no_pruning():
    """Under window_size messages: return all, no summarisation."""
    session_id = uuid.uuid4()
    interactions = _make_interactions(10)

    with (
        patch("app.context.count_history", new_callable=AsyncMock, return_value=10),
        patch("app.context.get_history", new_callable=AsyncMock, return_value=interactions),
    ):
        recent, summary = await build_pruned_history(
            db=MagicMock(),
            client=MagicMock(),
            session_id=session_id,
            window_size=20,
            summarise_threshold=40,
            fast_model="tutor-fast",
        )

    assert recent == interactions
    assert summary is None


@pytest.mark.asyncio
async def test_build_pruned_history_window_exceeded_no_summary():
    """total > window_size but <= summarise_threshold: sliding window, no summary."""
    session_id = uuid.uuid4()
    window = _make_interactions(20)

    with (
        patch("app.context.count_history", new_callable=AsyncMock, return_value=30),
        patch("app.context.get_history", new_callable=AsyncMock, return_value=window),
    ):
        recent, summary = await build_pruned_history(
            db=MagicMock(),
            client=MagicMock(),
            session_id=session_id,
            window_size=20,
            summarise_threshold=40,
            fast_model="tutor-fast",
        )

    assert recent == window
    assert summary is None


@pytest.mark.asyncio
async def test_build_pruned_history_summarisation_triggered():
    """total > summarise_threshold: sliding window + summarisation called."""
    session_id = uuid.uuid4()
    window = _make_interactions(20)
    older = _make_interactions(30)

    response = MagicMock()
    response.choices[0].message.content = "Earlier context summary here."
    gateway = AsyncMock()
    gateway.chat.completions.create = AsyncMock(return_value=response)

    with (
        patch("app.context.count_history", new_callable=AsyncMock, return_value=50),
        patch("app.context.get_history", new_callable=AsyncMock, return_value=window),
        patch("app.context.get_history_slice", new_callable=AsyncMock, return_value=older),
    ):
        recent, summary = await build_pruned_history(
            db=MagicMock(),
            client=gateway,
            session_id=session_id,
            window_size=20,
            summarise_threshold=40,
            fast_model="tutor-fast",
        )

    assert recent == window
    assert summary == "Earlier context summary here."
    gateway.chat.completions.create.assert_called_once()


@pytest.mark.asyncio
async def test_build_pruned_history_summarisation_failure_returns_none():
    """If summarisation fails, returns None (not a crash)."""
    session_id = uuid.uuid4()
    window = _make_interactions(20)
    older = _make_interactions(30)

    gateway = AsyncMock()
    gateway.chat.completions.create = AsyncMock(side_effect=RuntimeError("down"))

    with (
        patch("app.context.count_history", new_callable=AsyncMock, return_value=50),
        patch("app.context.get_history", new_callable=AsyncMock, return_value=window),
        patch("app.context.get_history_slice", new_callable=AsyncMock, return_value=older),
    ):
        recent, summary = await build_pruned_history(
            db=MagicMock(),
            client=gateway,
            session_id=session_id,
            window_size=20,
            summarise_threshold=40,
            fast_model="tutor-fast",
        )

    assert recent == window
    assert summary is None
