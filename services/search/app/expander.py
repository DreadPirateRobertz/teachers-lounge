"""Query expansion for short search queries (tl-afb).

A search query of fewer than :data:`app.config.Settings.query_expansion_short_threshold`
tokens is often an underspecified follow-up like "why?" or "what about it" —
running it through Qdrant directly retrieves low-relevance chunks. When the
caller supplies recent conversation turns, we ask the AI gateway
(``tutor_fast_model``) to rewrite the query into a standalone, self-contained
question before it hits the retrieval pipeline.

**Design constraints (per tl-afb spec):**

* Stateless — this module owns no session state; the caller supplies
  ``context_turns`` on every request.
* Graceful — *any* expansion failure (gateway error, blank output, network
  timeout) must fall back to the raw query, log a WARNING, and increment the
  ``search_query_expansion_total`` counter. The search endpoint never 500s on
  expansion alone.
* Cheap — expansion is a single non-streaming completion with a small
  ``max_tokens`` budget.
"""
from __future__ import annotations

import logging
from typing import Iterable

from app.config import settings
from app.metrics import QUERY_EXPANSION_OUTCOMES

logger = logging.getLogger(__name__)

_client = None  # lazy AsyncOpenAI pointed at the AI gateway


def _get_client():
    """Return a lazily-initialised AsyncOpenAI client aimed at the AI gateway."""
    global _client
    if _client is None:
        from openai import AsyncOpenAI  # imported here so tests can patch it

        _client = AsyncOpenAI(
            base_url=settings.ai_gateway_url,
            api_key=settings.openai_api_key or "ai-gateway-local",
        )
    return _client


def _is_short(query: str) -> bool:
    """True if the whitespace-token count is below the configured threshold."""
    return len(query.split()) < settings.query_expansion_short_threshold


def _clip_turns(turns: Iterable[str]) -> list[str]:
    """Keep the most recent N non-empty turns, bounded by config."""
    cleaned = [t.strip() for t in turns if t and t.strip()]
    max_turns = settings.query_expansion_max_context_turns
    return cleaned[-max_turns:]


async def expand_query(query: str, context_turns: list[str] | None) -> str:
    """Rewrite a short follow-up query into a standalone question.

    Args:
        query: Raw user query as received by ``/v1/search``.
        context_turns: Recent conversation turns (caller-supplied, most-recent
            last). ``None`` or empty means the caller has no context and
            expansion is skipped.

    Returns:
        The expanded, self-contained query string — or the original ``query``
        whenever expansion is skipped or fails. This function is total: it
        never raises; callers can treat its output as a drop-in replacement.
    """
    if not _is_short(query):
        QUERY_EXPANSION_OUTCOMES.labels(outcome="passthrough_long").inc()
        return query

    clipped = _clip_turns(context_turns or [])
    if not clipped:
        QUERY_EXPANSION_OUTCOMES.labels(outcome="passthrough_nocontext").inc()
        return query

    prompt = [
        {
            "role": "system",
            "content": (
                "You rewrite a student's short follow-up question into a "
                "fully-specified standalone search query, using the prior "
                "conversation turns for context. Output ONLY the rewritten "
                "query on a single line — no preamble, no quotation marks, "
                "no explanations."
            ),
        },
        {
            "role": "user",
            "content": (
                "Prior conversation turns (oldest → newest):\n"
                + "\n".join(f"- {t}" for t in clipped)
                + f"\n\nShort follow-up query: {query}\n\nStandalone query:"
            ),
        },
    ]

    try:
        client = _get_client()
        response = await client.chat.completions.create(
            model=settings.tutor_fast_model,
            messages=prompt,
            max_tokens=settings.query_expansion_max_tokens,
            stream=False,
        )
        expanded = (response.choices[0].message.content or "").strip()
    except Exception:
        QUERY_EXPANSION_OUTCOMES.labels(outcome="fallback_error").inc()
        logger.warning(
            "query expansion failed — falling back to raw query",
            exc_info=True,
        )
        return query

    if not expanded:
        QUERY_EXPANSION_OUTCOMES.labels(outcome="fallback_blank").inc()
        logger.warning("query expansion returned blank — falling back to raw query")
        return query

    QUERY_EXPANSION_OUTCOMES.labels(outcome="expanded").inc()
    return expanded
