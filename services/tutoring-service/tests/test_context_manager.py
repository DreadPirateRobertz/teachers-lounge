"""Unit tests for app.context_manager — pruning, token estimation, and logging.

All tests are pure (no I/O) except the summarise_history tests which use a
mocked AsyncOpenAI client.
"""

import logging
from unittest.mock import AsyncMock, MagicMock

import pytest

from app.context_manager import (
    estimate_tokens,
    log_token_usage,
    prune_to_window,
    summarise_history,
)

# ── Helpers ──────────────────────────────────────────────────────────────────


def _make_interactions(n: int) -> list[MagicMock]:
    """Return a list of ``n`` mock Interaction objects with a ``role`` attribute."""
    interactions = []
    for i in range(n):
        m = MagicMock()
        m.role = "student" if i % 2 == 0 else "tutor"
        m.content = f"message {i}"
        interactions.append(m)
    return interactions


def _make_messages(n: int) -> list[dict]:
    """Return a list of ``n`` OpenAI-format message dicts."""
    return [
        {"role": "user" if i % 2 == 0 else "assistant", "content": f"message content {i}"}
        for i in range(n)
    ]


# ── estimate_tokens ───────────────────────────────────────────────────────────


class TestEstimateTokens:
    """Tests for the estimate_tokens function."""

    def test_empty_list_returns_zero(self):
        """An empty message list should contribute zero tokens."""
        assert estimate_tokens([]) == 0

    def test_single_message_overhead(self):
        """A message with empty content should still count per-message overhead."""
        result = estimate_tokens([{"role": "system", "content": ""}])
        assert result == 4  # 0 content tokens + 4 overhead

    def test_content_length_contributes(self):
        """Content of 40 chars → 10 tokens + 4 overhead = 14."""
        msg = {"role": "user", "content": "a" * 40}
        assert estimate_tokens([msg]) == 14

    def test_multiple_messages_summed(self):
        """Token estimates across multiple messages are summed."""
        messages = [
            {"role": "system", "content": "a" * 40},  # 10 + 4 = 14
            {"role": "user", "content": "b" * 80},  # 20 + 4 = 24
            {"role": "assistant", "content": "c" * 20},  # 5 + 4 = 9
        ]
        assert estimate_tokens(messages) == 47

    def test_missing_content_key_treated_as_empty(self):
        """Messages without a content key should not raise."""
        assert estimate_tokens([{"role": "user"}]) == 4


# ── prune_to_window ───────────────────────────────────────────────────────────


class TestPruneToWindow:
    """Tests for the sliding-window pruning function."""

    def test_no_pruning_when_within_window(self):
        """When interactions <= max_turns * 2, nothing is pruned."""
        interactions = _make_interactions(10)
        active, older = prune_to_window(interactions, max_turns=5)
        assert active == interactions
        assert older == []

    def test_exact_window_boundary(self):
        """Exactly max_turns * 2 interactions → no pruning."""
        interactions = _make_interactions(20)
        active, older = prune_to_window(interactions, max_turns=10)
        assert active == interactions
        assert older == []

    def test_prunes_older_messages(self):
        """Excess messages are moved to the older list, not discarded."""
        interactions = _make_interactions(30)
        active, older = prune_to_window(interactions, max_turns=10)
        assert len(active) == 20
        assert len(older) == 10
        # active window is the LAST 20 messages
        assert active == interactions[10:]
        # older is the FIRST 10 messages
        assert older == interactions[:10]

    def test_ordering_preserved(self):
        """Active window and older list preserve oldest-first ordering."""
        interactions = _make_interactions(40)
        active, older = prune_to_window(interactions, max_turns=10)
        # Both sublists maintain original relative ordering
        assert active == interactions[20:]
        assert older == interactions[:20]

    def test_odd_count_preserved_in_active(self):
        """Trailing partial turn (odd total) is included in active window."""
        # 21 messages = 10 full turns + 1 trailing student message
        interactions = _make_interactions(21)
        active, older = prune_to_window(interactions, max_turns=10)
        assert len(active) == 20
        assert len(older) == 1

    def test_empty_interactions(self):
        """Empty interaction list returns two empty lists."""
        active, older = prune_to_window([], max_turns=10)
        assert active == []
        assert older == []

    def test_max_turns_one(self):
        """max_turns=1 keeps only the last 2 messages, moving the rest to older."""
        interactions = _make_interactions(6)
        active, older = prune_to_window(interactions, max_turns=1)
        assert len(active) == 2
        assert len(older) == 4
        assert active == interactions[-2:]
        assert older == interactions[:-2]


# ── log_token_usage ───────────────────────────────────────────────────────────


class TestLogTokenUsage:
    """Tests for the token usage logging function."""

    def test_no_log_below_80_percent(self, caplog):
        """No log emitted when utilisation is below 80 %."""
        with caplog.at_level(logging.DEBUG):
            log_token_usage("sess-1", token_count=50_000, model_context_limit=128_000)
        assert caplog.records == []

    def test_warning_at_80_percent(self, caplog):
        """A WARNING is emitted when utilisation reaches 80 %."""
        with caplog.at_level(logging.WARNING):
            log_token_usage("sess-2", token_count=102_400, model_context_limit=128_000)
        assert len(caplog.records) == 1
        assert caplog.records[0].levelno == logging.WARNING
        assert "sess-2" in caplog.records[0].message

    def test_error_at_95_percent(self, caplog):
        """An ERROR is emitted when utilisation reaches 95 %."""
        with caplog.at_level(logging.ERROR):
            log_token_usage("sess-3", token_count=121_600, model_context_limit=128_000)
        assert len(caplog.records) == 1
        assert caplog.records[0].levelno == logging.ERROR

    def test_zero_limit_does_not_raise(self):
        """A model_context_limit of 0 should not raise ZeroDivisionError."""
        log_token_usage("sess-4", token_count=1000, model_context_limit=0)  # must not raise

    def test_exactly_at_80_percent_boundary(self, caplog):
        """Exactly 80 % utilisation triggers a WARNING, not an ERROR."""
        with caplog.at_level(logging.WARNING):
            log_token_usage("sess-5", token_count=80, model_context_limit=100)
        assert len(caplog.records) == 1
        assert caplog.records[0].levelno == logging.WARNING


# ── summarise_history ─────────────────────────────────────────────────────────


class TestSummariseHistory:
    """Tests for the AI-gateway-backed summarisation function."""

    def _make_client(self, summary: str) -> MagicMock:
        """Build a minimal mock AsyncOpenAI client that returns ``summary``."""
        choice = MagicMock()
        choice.message.content = f"  {summary}  "  # intentional whitespace to test strip()

        response = MagicMock()
        response.choices = [choice]

        completions = MagicMock()
        completions.create = AsyncMock(return_value=response)

        client = MagicMock()
        client.chat.completions = completions
        return client

    @pytest.mark.asyncio
    async def test_returns_summary_text(self):
        """summarise_history returns the stripped model output."""
        client = self._make_client("The student discussed algebra.")
        messages = [
            {"role": "user", "content": "What is x + 2 = 5?"},
            {"role": "assistant", "content": "x equals 3."},
        ]
        result = await summarise_history(client, messages, model="tutor-fast")
        assert result == "The student discussed algebra."

    @pytest.mark.asyncio
    async def test_empty_messages_returns_empty_string(self):
        """Empty older_messages list short-circuits without calling the gateway."""
        client = MagicMock()
        result = await summarise_history(client, [], model="tutor-fast")
        assert result == ""
        client.chat.completions.create.assert_not_called()

    @pytest.mark.asyncio
    async def test_uses_fast_model(self):
        """The fast model alias is passed to the completions call."""
        client = self._make_client("Summary here.")
        messages = [{"role": "user", "content": "Hello?"}]
        await summarise_history(client, messages, model="tutor-fast")

        call_kwargs = client.chat.completions.create.call_args.kwargs
        assert call_kwargs["model"] == "tutor-fast"
        assert call_kwargs["stream"] is False

    @pytest.mark.asyncio
    async def test_conversation_text_included_in_prompt(self):
        """The older messages are formatted and sent to the gateway."""
        client = self._make_client("Summary.")
        messages = [
            {"role": "user", "content": "Tell me about mitosis."},
            {"role": "assistant", "content": "Mitosis has 4 phases."},
        ]
        await summarise_history(client, messages, model="tutor-fast")

        call_kwargs = client.chat.completions.create.call_args.kwargs
        user_msg = next(m for m in call_kwargs["messages"] if m["role"] == "user")
        assert "Tell me about mitosis" in user_msg["content"]
        assert "Mitosis has 4 phases" in user_msg["content"]
