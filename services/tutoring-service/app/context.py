"""Context window management — sliding window pruning, summarisation, token counting.

Implements three layers of protection against runaway context growth in long
tutoring sessions:

1. Sliding window  — only the last *window_size* messages are sent to the AI
   gateway (enforced in ``history.get_history`` via DESC LIMIT + reverse).

2. Summarisation fallback  — when total message count exceeds
   *summarise_threshold*, the messages outside the window are condensed into a
   single paragraph prepended to the system prompt.  Uses the fast tutor model
   to keep latency low.

3. Token accounting  — before every gateway call the estimated prompt token
   count is logged at WARNING level when it approaches the model context limit.
"""
from __future__ import annotations

import logging
from uuid import UUID

from sqlalchemy.ext.asyncio import AsyncSession

from .history import count_history, get_history, get_history_slice

logger = logging.getLogger(__name__)

# Characters-per-token approximation.  GPT/Claude-family models average ~4
# chars/token for English prose.  This is intentionally loose — precision is
# not worth a tokeniser dependency.
_CHARS_PER_TOKEN = 4


def count_tokens(messages: list[dict]) -> int:
    """Estimate the total token count for a list of OpenAI-style messages.

    Uses a simple characters-per-token heuristic accurate to ±20 % for English
    prose — sufficient for threshold-based monitoring without a tokeniser dep.

    Each message carries ~4 tokens of overhead (role + formatting).

    Args:
        messages: List of ``{"role": ..., "content": ...}`` dicts.

    Returns:
        Estimated token count (rounded down).
    """
    total_chars = sum(len(m.get("content", "") or "") for m in messages)
    return total_chars // _CHARS_PER_TOKEN + len(messages) * 4


def log_token_usage(
    messages: list[dict],
    model_context_limit: int,
    warn_ratio: float,
    session_id: UUID,
) -> int:
    """Emit a WARNING when estimated token count nears the model context limit.

    Args:
        messages: Fully assembled prompt messages (including system prompt).
        model_context_limit: Maximum tokens the target model accepts.
        warn_ratio: Fraction of the limit at which to emit a WARNING (e.g. 0.8).
        session_id: Included in the log line for traceability.

    Returns:
        Estimated token count (returned so callers can log or record it).
    """
    tokens = count_tokens(messages)
    threshold = int(model_context_limit * warn_ratio)
    if tokens >= threshold:
        pct = 100.0 * tokens / model_context_limit
        # Sanitize session_id to prevent log injection — strip newlines before
        # interpolating user-correlated data into the log message.
        safe_sid = str(session_id).replace("\n", "_").replace("\r", "_")
        logger.warning(
            "Session %s approaching context limit: ~%d tokens (%.0f%% of %d limit)",
            safe_sid,
            tokens,
            pct,
            model_context_limit,
        )
    return tokens


async def summarise_older_context(
    client,
    model: str,
    older_messages: list[dict],
) -> str:
    """Generate a concise summary of older conversation messages via the AI gateway.

    Called only when total history exceeds *summarise_threshold*.  Uses the
    fast model to keep the extra round-trip latency minimal.

    Args:
        client: AsyncOpenAI-compatible client pointed at the LiteLLM proxy.
        model: Model alias for the summarisation call (prefer fast model).
        older_messages: OpenAI-style message dicts for the older portion of the
            conversation (the messages being replaced by the summary).

    Returns:
        Plain-text summary paragraph, or an empty string on any failure.
    """
    if not older_messages:
        return ""

    prompt = [
        {
            "role": "system",
            "content": (
                "You are summarising an educational tutoring conversation. "
                "Produce a concise 3-5 sentence summary capturing: the student's "
                "main questions, key concepts covered, any misconceptions resolved, "
                "and the current difficulty level being practised."
            ),
        },
        *older_messages,
        {"role": "user", "content": "Summarise the above tutoring exchange concisely."},
    ]

    try:
        response = await client.chat.completions.create(
            model=model,
            messages=prompt,
            max_tokens=256,
            stream=False,
        )
        return response.choices[0].message.content or ""
    except Exception:
        logger.warning(
            "Context summarisation failed — continuing without summary",
            exc_info=True,
        )
        return ""


async def build_pruned_history(
    db: AsyncSession,
    client,
    session_id: UUID,
    window_size: int,
    summarise_threshold: int,
    fast_model: str,
) -> tuple[list, str | None]:
    """Return the windowed history and an optional summary of older context.

    Decision matrix:

    =====================  ===========  ===========
    Condition              Window       Summary
    =====================  ===========  ===========
    total <= window_size   all msgs     none
    window < total <= thr  last N msgs  none
    total > threshold      last N msgs  summarised
    =====================  ===========  ===========

    Args:
        db: Async SQLAlchemy session.
        client: Gateway client (used only when summarisation triggers).
        session_id: UUID of the tutoring session.
        window_size: Number of most-recent messages to keep in the window.
        summarise_threshold: Total message count above which summarisation runs.
        fast_model: Model alias used for the summarisation call.

    Returns:
        ``(recent_interactions, summary_text_or_None)``
    """
    total = await count_history(db, session_id)
    recent = await get_history(db, session_id, limit=window_size)

    if total <= summarise_threshold:
        return recent, None

    older_count = total - window_size
    if older_count <= 0:
        return recent, None

    older = await get_history_slice(db, session_id, offset=0, limit=older_count)
    older_messages = [
        {
            "role": "user" if i.role == "student" else "assistant",
            "content": i.content,
        }
        for i in older
    ]
    summary = await summarise_older_context(client, fast_model, older_messages)
    return recent, summary or None
