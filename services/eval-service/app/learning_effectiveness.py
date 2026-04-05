"""Custom learning effectiveness metric — runs weekly via GKE CronJob.

Spec (tl-dkg §CUSTOM LEARNING EFFECTIVENESS):
  North star metric: per topic, compare student quiz scores BEFORE vs AFTER
  tutoring sessions on that topic.

  effectiveness_score = (post_correct_rate - pre_correct_rate) / pre_correct_rate

  Minimum sample of 10 quiz attempts per topic (for k-anonymity).
  Results written to BigQuery.learning_effectiveness.
  Topics below the configured threshold are flagged for human review.
"""
import logging
from datetime import date, datetime, timedelta, timezone
from typing import Any

from sqlalchemy import text

from .bigquery import write_learning_effectiveness
from .config import settings
from .database import get_session

logger = logging.getLogger(__name__)

_MIN_SAMPLE = 10  # k-anonymity minimum per topic


async def _fetch_quiz_effectiveness() -> list[dict[str, Any]]:
    """Compute per-topic learning effectiveness from Postgres quiz_results.

    For each topic (concept name), find students who:
      1. Took quiz questions on that topic BEFORE a tutoring session.
      2. Took quiz questions on that topic AFTER a tutoring session.

    Returns a list of dicts with keys: topic, pre_rate, post_rate,
    effectiveness_score, sample_size.  Only topics meeting the minimum
    sample threshold are included.

    Returns:
        List of effectiveness metric dicts, one per qualifying topic.
    """
    cutoff = datetime.now(timezone.utc) - timedelta(days=7)

    # This query joins quiz_results with chat_sessions to find the boundary
    # (first tutoring session timestamp for each student+concept).  We then
    # compare quiz correctness before vs after that boundary.
    sql = text(
        """
        WITH tutoring_sessions AS (
            SELECT DISTINCT
                s.user_id,
                c.name AS topic,
                MIN(s.created_at) AS first_tutored_at
            FROM chat_sessions s
            JOIN interactions i ON i.session_id = s.id AND i.role = 'tutor'
            JOIN concepts c ON c.course_id = s.course_id
            WHERE s.course_id IS NOT NULL
              AND s.created_at >= :cutoff
            GROUP BY s.user_id, c.name
        ),
        quiz_stats AS (
            SELECT
                ts.topic,
                COUNT(*) FILTER (WHERE qr.answered_at < ts.first_tutored_at AND qr.correct) AS pre_correct,
                COUNT(*) FILTER (WHERE qr.answered_at < ts.first_tutored_at)                 AS pre_total,
                COUNT(*) FILTER (WHERE qr.answered_at >= ts.first_tutored_at AND qr.correct) AS post_correct,
                COUNT(*) FILTER (WHERE qr.answered_at >= ts.first_tutored_at)                AS post_total
            FROM tutoring_sessions ts
            JOIN quiz_results qr ON qr.user_id = ts.user_id
                AND LOWER(qr.topic) = LOWER(ts.topic)
            GROUP BY ts.topic
        )
        SELECT
            topic,
            NULLIF(pre_total, 0)  AS pre_total,
            NULLIF(post_total, 0) AS post_total,
            pre_correct,
            post_correct
        FROM quiz_stats
        WHERE pre_total >= :min_sample AND post_total >= :min_sample
        """
    )
    async with get_session() as db:
        result = await db.execute(sql, {"cutoff": cutoff, "min_sample": _MIN_SAMPLE})
        rows = result.fetchall()

    metrics = []
    for row in rows:
        pre_rate = row.pre_correct / row.pre_total
        post_rate = row.post_correct / row.post_total
        if pre_rate == 0:
            logger.debug("Topic %r has pre_rate=0 — skipping effectiveness calc", row.topic)
            continue
        effectiveness = (post_rate - pre_rate) / pre_rate
        metrics.append(
            {
                "topic": row.topic,
                "effectiveness_score": effectiveness,
                "sample_size": row.post_total,
                "pre_rate": pre_rate,
                "post_rate": post_rate,
            }
        )

    return metrics


async def run_learning_effectiveness() -> list[dict[str, Any]]:
    """Compute and persist the weekly learning effectiveness metric.

    Flags topics below the configured threshold for human review.

    Returns:
        List of effectiveness dicts written to BigQuery.
    """
    logger.info("Starting weekly learning effectiveness computation")

    metrics = await _fetch_quiz_effectiveness()
    if not metrics:
        logger.warning("Learning effectiveness: no qualifying topics found — skipping BigQuery write")
        return []

    week = date.today()
    write_learning_effectiveness(week=week, rows=metrics)

    for m in metrics:
        if m["effectiveness_score"] < settings.effectiveness_flag_threshold:
            logger.warning(
                "EFFECTIVENESS FLAG: topic=%r score=%.3f (pre=%.2f post=%.2f) — "
                "explanation strategy may not be working, flagged for human review",
                m["topic"],
                m["effectiveness_score"],
                m["pre_rate"],
                m["post_rate"],
            )
        else:
            logger.info(
                "topic=%r effectiveness=%.3f pre=%.2f post=%.2f",
                m["topic"],
                m["effectiveness_score"],
                m["pre_rate"],
                m["post_rate"],
            )

    return metrics
