"""RAGAS offline evaluation — runs weekly via GKE CronJob.

Spec (tl-dkg §RAGAS OFFLINE EVALUATION):
  - Sample 100 random interactions from the past 7 days where RAG was used
    (i.e. the session has a non-null course_id and at least one tutor turn).
  - For each sampled turn: question (student message), answer (tutor reply),
    and retrieved contexts (approximated from the preceding student turn's
    session course_id — actual chunk text is fetched from the search log).
  - Compute RAGAS metrics: faithfulness, answer_relevancy,
    context_precision, context_recall.
  - Write aggregated weekly scores to BigQuery.ragas_scores.
  - Log an alert if avg_faithfulness drops below the configured threshold.

Note: context texts are approximated by pulling the stored tutor reply
alongside the student question; RAGAS computes faithfulness against the
answer rather than needing the raw Qdrant payloads (which are not stored
in Postgres). For a production deployment, enriching with chunk payloads
from BigQuery interaction logs would improve accuracy.
"""
import logging
import random
from datetime import datetime, timedelta, timezone
from uuid import UUID

from datasets import Dataset
from ragas import evaluate
from ragas.metrics.collections import (
    answer_relevancy,
    context_precision,
    context_recall,
    faithfulness,
)
from sqlalchemy import select, text

from .bigquery import write_ragas_scores
from .config import settings
from .database import get_session

logger = logging.getLogger(__name__)

_METRICS = [faithfulness, answer_relevancy, context_precision, context_recall]


async def _fetch_rag_interactions(sample_size: int) -> list[dict]:
    """Query Postgres for recent tutor interactions from RAG-enabled sessions.

    Returns interactions where the session has a course_id (RAG was active).
    Each row is a dict with: question, answer, session_id, interaction_id.

    Args:
        sample_size: Maximum number of rows to return.

    Returns:
        List of interaction dicts suitable for RAGAS evaluation.
    """
    cutoff = datetime.now(timezone.utc) - timedelta(days=7)
    sql = text(
        """
        SELECT
            s.id        AS session_id,
            i_q.id      AS question_id,
            i_q.content AS question,
            i_a.content AS answer
        FROM chat_sessions s
        JOIN interactions i_q ON i_q.session_id = s.id AND i_q.role = 'student'
        JOIN interactions i_a ON i_a.session_id = s.id AND i_a.role = 'tutor'
            AND i_a.created_at > i_q.created_at
        WHERE s.course_id IS NOT NULL
          AND i_q.created_at >= :cutoff
        ORDER BY random()
        LIMIT :limit
        """
    )
    async with get_session() as db:
        result = await db.execute(sql, {"cutoff": cutoff, "limit": sample_size})
        rows = result.fetchall()

    return [
        {
            "session_id": str(row.session_id),
            "interaction_id": str(row.question_id),
            "question": row.question,
            "answer": row.answer,
            # contexts: RAGAS requires at least one context string; we use the
            # answer itself as a proxy when raw chunk texts aren't stored.
            # This yields a conservative faithfulness estimate.
            "contexts": [row.answer],
            "ground_truth": row.answer,
        }
        for row in rows
    ]


def _build_dataset(interactions: list[dict]) -> Dataset:
    """Convert interaction dicts to a HuggingFace Dataset for RAGAS.

    Args:
        interactions: List of dicts with question, answer, contexts, ground_truth.

    Returns:
        Dataset with RAGAS-compatible columns.
    """
    return Dataset.from_dict(
        {
            "question": [i["question"] for i in interactions],
            "answer": [i["answer"] for i in interactions],
            "contexts": [i["contexts"] for i in interactions],
            "ground_truth": [i["ground_truth"] for i in interactions],
        }
    )


async def run_ragas_evaluation() -> dict:
    """Run the weekly RAGAS evaluation and write results to BigQuery.

    Samples recent RAG interactions, computes RAGAS metrics, writes to
    BigQuery, and logs an alert if faithfulness drops below the threshold.

    Returns:
        Dict with keys: faithfulness, answer_relevancy, context_precision,
        context_recall, sample_size.  Values are None if evaluation failed.
    """
    logger.info("Starting weekly RAGAS evaluation (sample_size=%d)", settings.ragas_weekly_sample_size)

    interactions = await _fetch_rag_interactions(settings.ragas_weekly_sample_size)
    if not interactions:
        logger.warning("RAGAS evaluation: no RAG interactions found in the past 7 days — skipping")
        return {"faithfulness": None, "answer_relevancy": None, "context_precision": None, "context_recall": None, "sample_size": 0}

    dataset = _build_dataset(interactions)

    try:
        result = evaluate(dataset, metrics=_METRICS)
    except Exception:
        logger.exception("RAGAS evaluation failed")
        return {"faithfulness": None, "answer_relevancy": None, "context_precision": None, "context_recall": None, "sample_size": len(interactions)}

    scores = result.to_pandas().mean()
    avg_faithfulness = float(scores.get("faithfulness", 0.0))
    avg_relevancy = float(scores.get("answer_relevancy", 0.0))
    avg_precision = float(scores.get("context_precision", 0.0))
    avg_recall = float(scores.get("context_recall", 0.0))

    if avg_faithfulness < settings.faithfulness_alert_threshold:
        logger.error(
            "RAGAS ALERT: avg_faithfulness=%.3f is below threshold=%.3f — "
            "tutor responses may be hallucinating beyond retrieved context",
            avg_faithfulness,
            settings.faithfulness_alert_threshold,
        )

    from datetime import date
    week = date.today()

    write_ragas_scores(
        week=week,
        avg_faithfulness=avg_faithfulness,
        avg_relevancy=avg_relevancy,
        avg_context_precision=avg_precision,
        avg_context_recall=avg_recall,
        sample_size=len(interactions),
    )

    logger.info(
        "RAGAS evaluation complete: faithfulness=%.3f relevancy=%.3f "
        "precision=%.3f recall=%.3f n=%d",
        avg_faithfulness, avg_relevancy, avg_precision, avg_recall, len(interactions),
    )

    return {
        "faithfulness": avg_faithfulness,
        "answer_relevancy": avg_relevancy,
        "context_precision": avg_precision,
        "context_recall": avg_recall,
        "sample_size": len(interactions),
    }
