"""Nightly LLM judge — evaluates sampled tutor interactions via Claude Haiku.

Spec (tl-dkg §CUSTOM LLM JUDGE):
  - Sample 20 interactions per night (tutor role only).
  - Call Claude Haiku with a structured prompt rating three dimensions 1–5:
      1. Directness: did the response directly address the student's question?
      2. Pace: was it appropriately paced for the student's level?
      3. Grounding: was it grounded in the source material?
  - Store scores in interaction_quality table (Postgres).
  - Skip interactions that have already been judged (upsert-safe).
"""
import json
import logging
from datetime import datetime, timedelta, timezone

import anthropic
from sqlalchemy import text

from .config import settings
from .database import get_session

logger = logging.getLogger(__name__)

_JUDGE_PROMPT_TEMPLATE = """\
You are evaluating an AI tutor's response to a student question.

Student question:
{question}

Tutor response:
{answer}

Rate the tutor response on three dimensions, each on a scale of 1 (very poor) to 5 (excellent):

1. **Directness** (1–5): Did the response directly address the student's actual question?
2. **Pace** (1–5): Was the response appropriately paced for a student's level — neither too simple nor overwhelming?
3. **Grounding** (1–5): Was the response grounded in the subject material (not vague or off-topic)?

Respond with valid JSON only, in this exact format:
{{"directness": <int>, "pace": <int>, "grounding": <int>, "reasoning": "<one sentence explaining your rating>"}}
"""


async def _sample_interactions(n: int) -> list[dict]:
    """Sample n recent tutor interactions not yet judged.

    Args:
        n: Maximum number of interactions to sample.

    Returns:
        List of dicts with: interaction_id, question (preceding student turn), answer.
    """
    cutoff = datetime.now(timezone.utc) - timedelta(days=1)
    sql = text(
        """
        SELECT
            i_a.id          AS interaction_id,
            i_q.content     AS question,
            i_a.content     AS answer
        FROM interactions i_a
        JOIN interactions i_q
          ON i_q.session_id = i_a.session_id
         AND i_q.role = 'student'
         AND i_q.created_at < i_a.created_at
        LEFT JOIN interaction_quality iq ON iq.interaction_id = i_a.id
        WHERE i_a.role = 'tutor'
          AND i_a.created_at >= :cutoff
          AND iq.id IS NULL
        ORDER BY random()
        LIMIT :limit
        """
    )
    async with get_session() as db:
        result = await db.execute(sql, {"cutoff": cutoff, "limit": n})
        rows = result.fetchall()

    return [
        {"interaction_id": str(r.interaction_id), "question": r.question, "answer": r.answer}
        for r in rows
    ]


def _call_judge(question: str, answer: str) -> dict | None:
    """Call Claude Haiku to rate one tutor interaction.

    Args:
        question: The student's question (context for rating).
        answer: The tutor's response to evaluate.

    Returns:
        Dict with directness, pace, grounding, reasoning; or None on failure.
    """
    client = anthropic.Anthropic(api_key=settings.anthropic_api_key)
    prompt = _JUDGE_PROMPT_TEMPLATE.format(question=question, answer=answer)

    try:
        message = client.messages.create(
            model=settings.judge_model,
            max_tokens=256,
            messages=[{"role": "user", "content": prompt}],
        )
        raw = message.content[0].text.strip()
        return json.loads(raw)
    except json.JSONDecodeError as exc:
        logger.warning("LLM judge returned non-JSON response: %s (raw=%r)", exc, raw[:200])
        return None
    except Exception as exc:
        logger.error("LLM judge API call failed: %s", exc)
        return None


async def _persist_judge_result(interaction_id: str, scores: dict) -> None:
    """Write a judge result row to interaction_quality, skipping duplicates.

    Args:
        interaction_id: UUID string of the judged tutor interaction.
        scores: Dict from _call_judge with directness, pace, grounding, reasoning.
    """
    directness = int(scores.get("directness", 0))
    pace = int(scores.get("pace", 0))
    grounding = int(scores.get("grounding", 0))
    composite = round((directness + pace + grounding) / 3)

    insert_sql = text(
        """
        INSERT INTO interaction_quality
            (id, interaction_id, judge_score, judge_reasoning,
             score_directness, score_pace, score_grounding,
             judged_at, judge_model)
        VALUES
            (gen_random_uuid(), :interaction_id::uuid, :judge_score, :reasoning,
             :directness, :pace, :grounding,
             now(), :model)
        ON CONFLICT (interaction_id) DO NOTHING
        """
    )
    async with get_session() as db:
        await db.execute(
            insert_sql,
            {
                "interaction_id": interaction_id,
                "judge_score": composite,
                "reasoning": scores.get("reasoning", ""),
                "directness": directness,
                "pace": pace,
                "grounding": grounding,
                "model": settings.judge_model,
            },
        )
        await db.commit()


async def run_llm_judge() -> int:
    """Run the nightly LLM judge over sampled interactions.

    Samples recent unjudged tutor interactions, calls Claude Haiku on each,
    and persists scores to the interaction_quality table.

    Returns:
        Number of interactions successfully judged.
    """
    logger.info("Starting nightly LLM judge (sample=%d)", settings.llm_judge_nightly_sample)

    if not settings.anthropic_api_key:
        logger.error("ANTHROPIC_API_KEY not set — LLM judge cannot run")
        return 0

    interactions = await _sample_interactions(settings.llm_judge_nightly_sample)
    if not interactions:
        logger.info("LLM judge: no unjudged interactions in the past 24h — nothing to do")
        return 0

    judged = 0
    for item in interactions:
        scores = _call_judge(item["question"], item["answer"])
        if scores is None:
            continue
        await _persist_judge_result(item["interaction_id"], scores)
        logger.info(
            "judged interaction=%s composite=%d directness=%s pace=%s grounding=%s",
            item["interaction_id"],
            round((scores.get("directness", 0) + scores.get("pace", 0) + scores.get("grounding", 0)) / 3),
            scores.get("directness"),
            scores.get("pace"),
            scores.get("grounding"),
        )
        judged += 1

    logger.info("LLM judge complete: %d/%d interactions judged", judged, len(interactions))
    return judged
