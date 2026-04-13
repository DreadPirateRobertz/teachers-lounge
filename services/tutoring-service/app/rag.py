"""Query reformulation for the RAG retrieval step.

``reformulate_query`` rewrites pronoun-heavy, context-dependent student
messages into self-contained retrieval queries before they hit the
Search Service.  Without this step, short follow-ups like "what about
in E1 reactions?" or "why does that work?" produce low-quality
retrieval results because the pronouns lose their antecedents at the
embedding layer.

The rewrite is performed by the cheap ``tutor_fast_model`` through the
shared AI-gateway client.  Failure modes are all non-fatal: an empty
history short-circuits to the original message, a gateway exception
falls back to the original message, and an empty/blank or malformed
response also falls back.  Malformed responses include missing
attributes, empty ``choices`` lists, and ``choices=None``; each is
caught and the original message is returned so callers do not need
defensive try/except around this function for the documented
response shapes.
"""

from __future__ import annotations

import logging
from typing import Any

from openai import AsyncOpenAI

from .config import settings

logger = logging.getLogger(__name__)


REFORMULATION_SYSTEM_PROMPT = (
    "You rewrite follow-up student questions into self-contained search "
    "queries for a textbook retrieval system.  Use the provided "
    "conversation history only to resolve pronouns and implicit "
    "references.  Return ONE line of plain text — the rewritten "
    "query, with no quotes, no prefix, no explanation.  If the "
    "question is already self-contained, return it unchanged."
)


def _history_to_chat_messages(history: list[Any]) -> list[dict[str, str]]:
    """Convert a list of ORM ``Interaction`` rows to OpenAI-style messages.

    Accepts either ORM rows (with ``role``/``content`` attributes) or
    plain dicts — the latter makes tests trivial to write.
    """
    out: list[dict[str, str]] = []
    for item in history:
        role = getattr(item, "role", None) or item.get("role")  # type: ignore[union-attr]
        content = getattr(item, "content", None) or item.get("content")  # type: ignore[union-attr]
        mapped = "user" if role == "student" else "assistant"
        out.append({"role": mapped, "content": content})
    return out


async def reformulate_query(
    message: str,
    history: list[Any],
    client: AsyncOpenAI | None = None,
    model: str | None = None,
) -> str:
    """Rewrite ``message`` into a self-contained retrieval query.

    Args:
        message: The student's raw follow-up question.
        history: Ordered list of prior interactions (most-recent last).
            Only the last few turns are typically needed — the caller
            should slice this (e.g. ``history[-4:]``) before passing it
            in.  An empty list short-circuits to ``message`` unchanged.
        client: Optional ``AsyncOpenAI`` client.  If ``None``, a shared
            gateway client is fetched lazily via ``get_gateway_client``.
            Injected for testing.
        model: Optional model override.  Defaults to
            ``settings.tutor_fast_model``.

    Returns:
        The rewritten, self-contained query on success.  The original
        ``message`` on any failure (empty history, gateway error, empty
        rewrite).
    """
    if not history:
        return message

    if client is None:
        # Local import to avoid a circular dependency at module load time —
        # gateway.py imports from config and openai only, but keeping the
        # import lazy here also makes the function cheap to test in isolation.
        from .gateway import get_gateway_client

        client = get_gateway_client()

    chat_history = _history_to_chat_messages(history)
    prompt_messages: list[dict[str, str]] = [
        {"role": "system", "content": REFORMULATION_SYSTEM_PROMPT},
        *chat_history,
        {
            "role": "user",
            "content": (
                "Rewrite this follow-up as a self-contained search query:\n"
                f"{message}"
            ),
        },
    ]

    try:
        response = await client.chat.completions.create(
            model=model or settings.tutor_fast_model,
            messages=prompt_messages,
            temperature=0.0,
            max_tokens=128,
        )
    except Exception:  # noqa: BLE001  non-fatal — degrade to original
        logger.warning("reformulate_query: gateway error, falling back to original message")
        return message

    try:
        rewritten = (response.choices[0].message.content or "").strip()
    except (AttributeError, IndexError, TypeError):
        # AttributeError: missing .choices / .message / .content
        # IndexError:    empty choices list
        # TypeError:     choices=None or message=None (NoneType not subscriptable)
        logger.warning("reformulate_query: malformed gateway response, falling back")
        return message

    if not rewritten:
        return message
    return rewritten
