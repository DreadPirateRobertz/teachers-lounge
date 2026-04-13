"""Tests for query reformulation in ``app.rag``.

Covers:
  * Empty-history short-circuit (no gateway call, message returned verbatim).
  * Pronoun resolution (gateway returns rewritten query, rewrite is used).
  * Gateway exception → fallback to original message.
  * Empty/blank rewrite → fallback to original message.
  * Model + client overrides honoured.
"""

from __future__ import annotations

from types import SimpleNamespace
from unittest.mock import AsyncMock, MagicMock

import pytest

from app.rag import REFORMULATION_SYSTEM_PROMPT, reformulate_query


def _fake_response(content: str | None) -> SimpleNamespace:
    """Build a minimal object shaped like an OpenAI ChatCompletion response."""
    return SimpleNamespace(
        choices=[SimpleNamespace(message=SimpleNamespace(content=content))]
    )


def _make_client_returning(content: str | None) -> MagicMock:
    """Build a MagicMock ``AsyncOpenAI`` client whose completion returns ``content``."""
    client = MagicMock()
    client.chat = MagicMock()
    client.chat.completions = MagicMock()
    client.chat.completions.create = AsyncMock(return_value=_fake_response(content))
    return client


def _make_client_raising(exc: Exception) -> MagicMock:
    """Build a MagicMock client whose completion call raises ``exc``."""
    client = MagicMock()
    client.chat = MagicMock()
    client.chat.completions = MagicMock()
    client.chat.completions.create = AsyncMock(side_effect=exc)
    return client


# ── Behaviour ────────────────────────────────────────────────────────────────


class TestReformulateQuery:
    """Unit tests for ``reformulate_query``."""

    @pytest.mark.asyncio
    async def test_empty_history_returns_message_unchanged(self):
        """With no prior turns, the function must not call the gateway."""
        client = _make_client_returning("should not be used")

        result = await reformulate_query("what is benzene?", [], client=client)

        assert result == "what is benzene?"
        client.chat.completions.create.assert_not_called()

    @pytest.mark.asyncio
    async def test_pronoun_resolved_via_gateway(self):
        """A rewritten query from the gateway is returned verbatim (stripped)."""
        history = [
            {"role": "student", "content": "what are SN2 reactions?"},
            {"role": "tutor", "content": "SN2 is a bimolecular substitution..."},
        ]
        client = _make_client_returning(
            "  What are the stereochemistry outcomes of SN2 reactions?  "
        )

        result = await reformulate_query(
            "what about their stereochemistry?", history, client=client
        )

        assert result == "What are the stereochemistry outcomes of SN2 reactions?"
        client.chat.completions.create.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_gateway_exception_falls_back_to_original(self):
        """Any exception from the gateway results in the original message."""
        history = [{"role": "student", "content": "hi"}]
        client = _make_client_raising(RuntimeError("gateway down"))

        result = await reformulate_query("what about that?", history, client=client)

        assert result == "what about that?"

    @pytest.mark.asyncio
    async def test_empty_rewrite_falls_back_to_original(self):
        """An empty-string completion means "no rewrite needed" — fallback."""
        history = [{"role": "student", "content": "hi"}]
        client = _make_client_returning("   ")

        result = await reformulate_query("already-clear query", history, client=client)

        assert result == "already-clear query"

    @pytest.mark.asyncio
    async def test_none_content_falls_back_to_original(self):
        """A None content field must not crash — fallback to original."""
        history = [{"role": "student", "content": "hi"}]
        client = _make_client_returning(None)

        result = await reformulate_query("still fine", history, client=client)

        assert result == "still fine"

    @pytest.mark.asyncio
    async def test_uses_custom_model_override(self):
        """The ``model`` kwarg is forwarded to the gateway call."""
        history = [{"role": "student", "content": "hi"}]
        client = _make_client_returning("rewritten")

        await reformulate_query("q?", history, client=client, model="custom-fast")

        call = client.chat.completions.create.await_args
        assert call.kwargs["model"] == "custom-fast"

    @pytest.mark.asyncio
    async def test_system_prompt_and_history_passed_to_gateway(self):
        """Verify the system prompt leads the messages and history is mapped correctly."""
        history = [
            {"role": "student", "content": "What are SN1 reactions?"},
            {"role": "tutor", "content": "Unimolecular substitution..."},
        ]
        client = _make_client_returning("rewritten")

        await reformulate_query("why is that?", history, client=client)

        messages = client.chat.completions.create.await_args.kwargs["messages"]
        assert messages[0] == {"role": "system", "content": REFORMULATION_SYSTEM_PROMPT}
        # student → user, tutor → assistant
        assert messages[1] == {"role": "user", "content": "What are SN1 reactions?"}
        assert messages[2] == {
            "role": "assistant",
            "content": "Unimolecular substitution...",
        }
        assert messages[-1]["role"] == "user"
        assert "why is that?" in messages[-1]["content"]

    @pytest.mark.asyncio
    async def test_default_client_fetched_via_gateway(self, monkeypatch):
        """When no client is passed, ``get_gateway_client`` supplies the default."""
        history = [{"role": "student", "content": "hi"}]
        client = _make_client_returning("rewritten")

        import app.gateway as gateway_mod

        monkeypatch.setattr(gateway_mod, "get_gateway_client", lambda: client)

        result = await reformulate_query("q?", history)

        assert result == "rewritten"
        client.chat.completions.create.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_malformed_response_falls_back_to_original(self):
        """A response with an empty ``choices`` list raises IndexError → fallback."""
        history = [{"role": "student", "content": "hi"}]
        client = MagicMock()
        client.chat = MagicMock()
        client.chat.completions = MagicMock()
        client.chat.completions.create = AsyncMock(
            return_value=SimpleNamespace(choices=[])
        )

        result = await reformulate_query("original", history, client=client)

        assert result == "original"

    @pytest.mark.asyncio
    async def test_orm_row_history_is_accepted(self):
        """ORM-like objects with ``role``/``content`` attributes are accepted."""
        row = SimpleNamespace(role="student", content="What is entropy?")
        client = _make_client_returning("rewritten")

        result = await reformulate_query("why does it always increase?", [row], client=client)

        assert result == "rewritten"
        messages = client.chat.completions.create.await_args.kwargs["messages"]
        assert messages[1] == {"role": "user", "content": "What is entropy?"}
