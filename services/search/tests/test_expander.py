"""Tests for the short-query expansion module (tl-afb).

Coverage:
    * Happy path — short query + context turns → expanded rewrite returned.
    * Passthrough (long query) — no gateway call, passthrough counter incremented.
    * Passthrough (no context) — no gateway call, passthrough counter incremented.
    * Graceful failure — gateway raises or returns blank → raw query + WARNING
      + ``search_query_expansion_total{outcome="fallback_*"}`` incremented.
    * Clipping — only the most-recent N turns are sent to the gateway.
    * Endpoint wiring — ``/v1/search`` uses expanded query through the pipeline.
"""
from __future__ import annotations

import uuid
from types import SimpleNamespace
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
from fastapi.testclient import TestClient
from prometheus_client import REGISTRY

from app import expander
from app.expander import expand_query
from app.main import app

COURSE_ID = uuid.uuid4()


def _outcome(outcome: str) -> float:
    """Read the current value of search_query_expansion_total for an outcome."""
    v = REGISTRY.get_sample_value(
        "search_query_expansion_total", {"outcome": outcome}
    )
    return 0.0 if v is None else v


def _fake_completion(text: str) -> SimpleNamespace:
    """Build a minimal OpenAI-shape chat.completion response containing *text*."""
    return SimpleNamespace(
        choices=[SimpleNamespace(message=SimpleNamespace(content=text))]
    )


@pytest.fixture(autouse=True)
def _reset_client():
    """Ensure each test re-runs the lazy client init — avoids leaked state."""
    expander._client = None
    yield
    expander._client = None


class TestExpandQuery:
    """Unit tests for :func:`app.expander.expand_query`."""

    @pytest.mark.asyncio
    async def test_happy_path_returns_expanded_rewrite(self) -> None:
        """Short query + context turns should yield the gateway rewrite."""
        before = _outcome("expanded")
        fake = MagicMock()
        fake.chat.completions.create = AsyncMock(
            return_value=_fake_completion("What is the ideal gas law?")
        )
        with patch.object(expander, "_get_client", return_value=fake):
            out = await expand_query(
                "why?",
                context_turns=[
                    "student: what's PV=nRT used for?",
                    "tutor: it relates pressure, volume, and temperature for gases.",
                ],
            )
        assert out == "What is the ideal gas law?"
        assert _outcome("expanded") == before + 1
        # And the gateway actually got called
        fake.chat.completions.create.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_long_query_passthrough_no_gateway_call(self) -> None:
        """Queries at/above the token threshold must skip expansion entirely."""
        before = _outcome("passthrough_long")
        fake = MagicMock()
        fake.chat.completions.create = AsyncMock(
            return_value=_fake_completion("should-not-appear")
        )
        with patch.object(expander, "_get_client", return_value=fake):
            out = await expand_query(
                "how does the ideal gas law relate pressure and volume",
                context_turns=["irrelevant"],
            )
        assert out == "how does the ideal gas law relate pressure and volume"
        assert _outcome("passthrough_long") == before + 1
        fake.chat.completions.create.assert_not_awaited()

    @pytest.mark.asyncio
    async def test_no_context_passthrough(self) -> None:
        """Short query with empty context must passthrough without a gateway call."""
        before = _outcome("passthrough_nocontext")
        fake = MagicMock()
        fake.chat.completions.create = AsyncMock(
            return_value=_fake_completion("should-not-appear")
        )
        with patch.object(expander, "_get_client", return_value=fake):
            out = await expand_query("why?", context_turns=None)
        assert out == "why?"
        assert _outcome("passthrough_nocontext") == before + 1
        fake.chat.completions.create.assert_not_awaited()

    @pytest.mark.asyncio
    async def test_blank_turns_treated_as_no_context(self) -> None:
        """Whitespace-only turns must not trigger a gateway call."""
        before = _outcome("passthrough_nocontext")
        fake = MagicMock()
        fake.chat.completions.create = AsyncMock()
        with patch.object(expander, "_get_client", return_value=fake):
            out = await expand_query("why?", context_turns=["   ", "", "\t\n"])
        assert out == "why?"
        assert _outcome("passthrough_nocontext") == before + 1
        fake.chat.completions.create.assert_not_awaited()

    @pytest.mark.asyncio
    async def test_gateway_failure_falls_back_to_raw_query(self, caplog) -> None:
        """Any exception from the gateway must produce raw query + WARNING."""
        import logging as _logging

        before = _outcome("fallback_error")
        fake = MagicMock()
        fake.chat.completions.create = AsyncMock(side_effect=RuntimeError("boom"))
        with (
            patch.object(expander, "_get_client", return_value=fake),
            caplog.at_level(_logging.WARNING, logger="app.expander"),
        ):
            out = await expand_query("why?", context_turns=["tutor: gas laws"])
        assert out == "why?"
        assert _outcome("fallback_error") == before + 1
        assert any("falling back" in rec.message for rec in caplog.records)

    @pytest.mark.asyncio
    async def test_blank_expansion_falls_back_to_raw_query(self) -> None:
        """Whitespace-only gateway output must fall back, not propagate."""
        before = _outcome("fallback_blank")
        fake = MagicMock()
        fake.chat.completions.create = AsyncMock(return_value=_fake_completion("   "))
        with patch.object(expander, "_get_client", return_value=fake):
            out = await expand_query("why?", context_turns=["tutor: gas laws"])
        assert out == "why?"
        assert _outcome("fallback_blank") == before + 1

    @pytest.mark.asyncio
    async def test_context_is_clipped_to_max(self) -> None:
        """Only the most-recent ``query_expansion_max_context_turns`` are sent."""
        from app.config import settings as _settings

        max_turns = _settings.query_expansion_max_context_turns
        turns = [f"turn-{i}" for i in range(max_turns + 4)]  # 4 extras

        captured: dict = {}

        async def _capture(**kwargs):
            captured.update(kwargs)
            return _fake_completion("expanded")

        fake = MagicMock()
        fake.chat.completions.create = AsyncMock(side_effect=_capture)
        with patch.object(expander, "_get_client", return_value=fake):
            await expand_query("why?", context_turns=turns)

        user_msg = captured["messages"][1]["content"]
        # The oldest turns must have been dropped
        assert "turn-0" not in user_msg
        assert "turn-3" not in user_msg
        # And the newest must be present
        assert f"turn-{max_turns + 3}" in user_msg


class TestSearchEndpointWiring:
    """End-to-end: ``/v1/search`` uses the expander and propagates the rewrite."""

    @pytest.mark.asyncio
    async def test_short_query_with_context_is_expanded_in_pipeline(self) -> None:
        """The effective query downstream (embedder, rerank, response) must be the expansion."""
        captured: dict[str, str] = {}

        async def _capture_embed(q: str) -> list[float]:
            captured["embed"] = q
            return [0.1] * 1536

        async def _capture_rerank(q, results):
            captured["rerank"] = q
            return results

        with (
            patch("app.expander.expand_query", new_callable=AsyncMock, return_value="What is PV=nRT?"),
            patch("app.routers.search.embed_query", new=_capture_embed),
            patch("app.routers.search.dense_search", new_callable=AsyncMock, return_value=[]),
            patch("app.routers.search.sparse_search", new_callable=AsyncMock, return_value=[]),
            patch("app.routers.search.rerank", new=_capture_rerank),
        ):
            with TestClient(app) as client:
                resp = client.get(
                    "/v1/search",
                    params=[
                        ("q", "why?"),
                        ("course_id", str(COURSE_ID)),
                        ("context_turns", "student: PV=nRT?"),
                        ("context_turns", "tutor: it's the ideal gas law."),
                    ],
                )
        assert resp.status_code == 200
        body = resp.json()
        assert body["query"] == "What is PV=nRT?"
        assert captured["embed"] == "What is PV=nRT?"
        assert captured["rerank"] == "What is PV=nRT?"

    @pytest.mark.asyncio
    async def test_endpoint_never_500s_on_expansion_failure(self) -> None:
        """Even if the expander module raises, the endpoint must return 200.

        expand_query is total by contract — but if a future refactor breaks
        that contract, the endpoint should still return results (the router
        itself is not wrapped in try/except, so this test documents that the
        expander contract is what protects the endpoint).
        """
        # Simulate the real expander being called but gateway failing — the
        # module's try/except must swallow and return the raw query.
        fake = MagicMock()
        fake.chat.completions.create = AsyncMock(side_effect=RuntimeError("gw down"))
        with (
            patch.object(expander, "_get_client", return_value=fake),
            patch(
                "app.routers.search.embed_query",
                new_callable=AsyncMock,
                return_value=[0.1] * 1536,
            ),
            patch(
                "app.routers.search.dense_search",
                new_callable=AsyncMock,
                return_value=[],
            ),
            patch(
                "app.routers.search.sparse_search",
                new_callable=AsyncMock,
                return_value=[],
            ),
            patch(
                "app.routers.search.rerank",
                new_callable=AsyncMock,
                side_effect=lambda q, r: r,
            ),
        ):
            with TestClient(app) as client:
                resp = client.get(
                    "/v1/search",
                    params=[
                        ("q", "why?"),
                        ("course_id", str(COURSE_ID)),
                        ("context_turns", "tutor: gas laws"),
                    ],
                )
        assert resp.status_code == 200
        assert resp.json()["query"] == "why?"
