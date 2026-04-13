"""Context window management — sliding-window pruning and summarisation fallback.

Provides utilities to keep prompt sizes within model context limits as chat
sessions grow long.  All public functions are pure (no I/O) except
``summarise_history``, which calls the AI gateway.

Usage in the chat pipeline:
  1. Fetch the last ``context_summary_threshold`` messages from the DB.
  2. Call ``prune_to_window`` to get the active sliding window.
  3. If older messages exist (beyond the window), call ``summarise_history``
     and prepend the summary to the system prompt.
  4. Call ``estimate_tokens`` on the assembled messages list.
  5. Call ``log_token_usage`` to emit warnings at 80 %/95 % utilisation.
"""

import logging

logger = logging.getLogger(__name__)

# Rough chars-per-token estimate (GPT-style tokenisers average ~4 chars/token).
_CHARS_PER_TOKEN = 4

# Overhead (in tokens) added per message for role markers and framing.
_TOKENS_PER_MESSAGE_OVERHEAD = 4


def estimate_tokens(messages: list[dict]) -> int:
    """Estimate the token count of an OpenAI-format message list.

    Uses a rough approximation of 4 characters per token, plus a fixed overhead
    of 4 tokens per message to account for role markers and framing bytes.

    Args:
        messages: List of ``{"role": ..., "content": ...}`` dicts.

    Returns:
        Estimated token count (integer, always >= 0).
    """
    return sum(
        len(msg.get("content", "")) // _CHARS_PER_TOKEN + _TOKENS_PER_MESSAGE_OVERHEAD
        for msg in messages
    )


def prune_to_window(interactions: list, max_turns: int) -> tuple[list, list]:
    """Split interactions into active window and older history.

    Keeps the most recent ``max_turns`` full exchanges (student + tutor pairs)
    as the active window.  Older interactions are returned separately so the
    caller can decide whether to summarise them.

    If the total interaction count is not even (trailing partial turn), the
    partial turn is included in the active window.

    Args:
        interactions: Ordered list of Interaction ORM objects (oldest first).
        max_turns: Maximum number of full exchanges to retain in the active
            window.  One turn = one student message + one tutor reply.

    Returns:
        Tuple of ``(active_window, older_interactions)`` where both are lists
        of Interaction ORM objects ordered oldest-first.
    """
    max_messages = max_turns * 2
    if len(interactions) <= max_messages:
        return interactions, []

    active = interactions[-max_messages:]
    older = interactions[:-max_messages]
    logger.debug(
        "Sliding window: kept last %d messages, older=%d messages available for summary",
        len(active),
        len(older),
    )
    return active, older


def log_token_usage(
    session_id: str,
    token_count: int,
    model_context_limit: int,
) -> None:
    """Log a warning when prompt tokens approach the model context limit.

    Emits a WARNING at 80 % utilisation and an ERROR at 95 % to surface
    sessions that are at risk of hitting the model's context ceiling.

    Args:
        session_id: UUID string of the tutoring session (for log correlation).
        token_count: Estimated token count of the current prompt.
        model_context_limit: Maximum token count the model supports.
    """
    if model_context_limit <= 0:
        return
    utilisation = token_count / model_context_limit
    if utilisation >= 0.95:
        logger.error(
            "Context near limit: session=%s tokens=%d limit=%d utilisation=%.0f%%",
            session_id,
            token_count,
            model_context_limit,
            utilisation * 100,
        )
    elif utilisation >= 0.80:
        logger.warning(
            "Context approaching limit: session=%s tokens=%d limit=%d utilisation=%.0f%%",
            session_id,
            token_count,
            model_context_limit,
            utilisation * 100,
        )


async def summarise_history(
    client,
    older_messages: list[dict],
    model: str,
) -> str:
    """Generate a compressed summary of older conversation messages.

    Calls the AI gateway with a summarisation prompt to produce a concise
    3-to-5 sentence digest of the provided messages.  This summary is meant
    to be prepended to the system prompt so the model retains earlier context
    without paying the full token cost.

    Args:
        client: AsyncOpenAI client pointed at the LiteLLM proxy.
        older_messages: OpenAI-format messages representing the older history
            (the portion that was pruned from the active sliding window).
        model: Model alias to use for summarisation (typically the fast model
            to minimise latency and cost).

    Returns:
        A short summary string, or an empty string if ``older_messages`` is
        empty.
    """
    if not older_messages:
        return ""

    conversation_text = "\n".join(
        f"{m['role'].capitalize()}: {m['content']}" for m in older_messages
    )
    summary_prompt = [
        {
            "role": "system",
            "content": (
                "You are a summarisation assistant. "
                "Summarise the following tutoring conversation excerpt in 3-5 sentences, "
                "capturing the key topics covered, concepts explained, and any important "
                "student misunderstandings that were corrected. "
                "Write in third person (e.g. 'The student asked about...')."
            ),
        },
        {"role": "user", "content": conversation_text},
    ]
    response = await client.chat.completions.create(
        model=model,
        messages=summary_prompt,
        stream=False,
        max_tokens=300,
    )
    return response.choices[0].message.content.strip()
