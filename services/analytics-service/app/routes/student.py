"""Student analytics endpoints.

Provides three read-only endpoints that aggregate a student's learning data
from the gaming_profiles, quiz_results, and interactions tables.  All
endpoints require a valid JWT and enforce that callers may only read their own
data (self-access only).
"""
from datetime import date, timedelta

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession

from ..auth import require_auth
from ..database import get_db

router = APIRouter(prefix="/v1/analytics/student", tags=["analytics"])


# ── Response models ──────────────────────────────────────────────────────────

class Overview(BaseModel):
    """Aggregate snapshot of a student's progress and gaming state.

    Attributes:
        user_id: UUID of the student.
        level: Current gamification level.
        xp: Total experience points earned.
        current_streak: Active daily study streak in days.
        longest_streak: All-time best streak in days.
        total_questions: Lifetime questions attempted.
        correct_answers: Lifetime correct answers.
        accuracy_pct: Correct / total * 100, rounded to one decimal.
        bosses_defeated: Number of boss battles won.
        gems: Current gem balance.
        total_sessions: Distinct study sessions from the interactions table.
        total_messages: Student messages sent across all sessions.
    """

    user_id: str
    level: int
    xp: int
    current_streak: int
    longest_streak: int
    total_questions: int
    correct_answers: int
    accuracy_pct: float
    bosses_defeated: int
    gems: int
    total_sessions: int
    total_messages: int


class TopicStat(BaseModel):
    """Per-topic quiz performance summary.

    Attributes:
        topic: Topic label as stored in quiz_results.
        total: Total questions attempted for this topic.
        correct: Correct answers for this topic.
        accuracy_pct: Correct / total * 100, rounded to one decimal.
    """

    topic: str
    total: int
    correct: int
    accuracy_pct: float


class QuizBreakdown(BaseModel):
    """Collection of per-topic quiz stats, ordered by attempt volume desc.

    Attributes:
        topics: Up to 20 topics with accuracy metrics.
    """

    topics: list[TopicStat]


class DayActivity(BaseModel):
    """Student message count for a single calendar day.

    Attributes:
        date: ISO-8601 date string (YYYY-MM-DD).
        messages: Number of student-role messages sent that day.
    """

    date: str
    messages: int


class ActivityHistory(BaseModel):
    """30-day rolling activity history.

    Attributes:
        days: Exactly 30 entries, one per day, newest entry last.
            Days with no activity have messages=0.
    """

    days: list[DayActivity]


# ── Helpers ──────────────────────────────────────────────────────────────────

def _check_self_or_raise(requesting_user_id: str, target_user_id: str) -> None:
    """Raise HTTP 403 if the caller is not the resource owner.

    Args:
        requesting_user_id: Subject claim from the validated JWT.
        target_user_id: Path parameter identifying the requested student.

    Raises:
        HTTPException: 403 Forbidden when IDs do not match.
    """
    if requesting_user_id != target_user_id:
        raise HTTPException(status_code=403, detail="Forbidden")


# ── Endpoints ────────────────────────────────────────────────────────────────

@router.get("/{user_id}/overview", response_model=Overview)
async def get_overview(
    user_id: str,
    caller: str = Depends(require_auth),
    db: AsyncSession = Depends(get_db),
) -> Overview:
    """Return a student's aggregate progress snapshot.

    Combines gaming_profiles (XP, level, streaks) with interaction counts
    (sessions, messages).  If no gaming profile exists yet a zeroed default
    is returned so the dashboard renders cleanly for brand-new accounts.

    Args:
        user_id: Target student UUID (path parameter).
        caller: Authenticated user ID from JWT (injected by require_auth).
        db: Async SQLAlchemy session (injected by get_db).

    Returns:
        Overview with current level, XP, streaks, accuracy, and session counts.

    Raises:
        HTTPException: 401 if the JWT is missing or invalid.
        HTTPException: 403 if caller != user_id.
    """
    _check_self_or_raise(caller, user_id)

    gaming_row = await db.execute(
        text("""
            SELECT level, xp, current_streak, longest_streak,
                   total_questions, correct_answers, bosses_defeated, gems
            FROM gaming_profiles
            WHERE user_id = :uid
        """),
        {"uid": user_id},
    )
    gp = gaming_row.mappings().first()

    if gp is None:
        # Return zeroed profile — user exists but hasn't started gaming yet
        gp = {
            "level": 1, "xp": 0, "current_streak": 0, "longest_streak": 0,
            "total_questions": 0, "correct_answers": 0,
            "bosses_defeated": 0, "gems": 0,
        }

    session_row = await db.execute(
        text("""
            SELECT
                COUNT(DISTINCT session_id) AS total_sessions,
                COUNT(*) FILTER (WHERE role = 'student') AS total_messages
            FROM interactions
            WHERE user_id = :uid
        """),
        {"uid": user_id},
    )
    sr = session_row.mappings().first() or {"total_sessions": 0, "total_messages": 0}

    total_q = int(gp["total_questions"])
    correct = int(gp["correct_answers"])
    accuracy = round(correct / total_q * 100, 1) if total_q > 0 else 0.0

    return Overview(
        user_id=user_id,
        level=int(gp["level"]),
        xp=int(gp["xp"]),
        current_streak=int(gp["current_streak"]),
        longest_streak=int(gp["longest_streak"]),
        total_questions=total_q,
        correct_answers=correct,
        accuracy_pct=accuracy,
        bosses_defeated=int(gp["bosses_defeated"]),
        gems=int(gp["gems"]),
        total_sessions=int(sr["total_sessions"]),
        total_messages=int(sr["total_messages"]),
    )


@router.get("/{user_id}/quiz-breakdown", response_model=QuizBreakdown)
async def get_quiz_breakdown(
    user_id: str,
    caller: str = Depends(require_auth),
    db: AsyncSession = Depends(get_db),
) -> QuizBreakdown:
    """Return per-topic quiz accuracy for a student.

    Aggregates quiz_results grouped by topic, ordered by attempt volume
    descending, capped at 20 topics.

    Args:
        user_id: Target student UUID (path parameter).
        caller: Authenticated user ID from JWT (injected by require_auth).
        db: Async SQLAlchemy session (injected by get_db).

    Returns:
        QuizBreakdown with a list of TopicStat entries.  Empty list when the
        student has no quiz_results rows yet.

    Raises:
        HTTPException: 401 if the JWT is missing or invalid.
        HTTPException: 403 if caller != user_id.
    """
    _check_self_or_raise(caller, user_id)

    result = await db.execute(
        text("""
            SELECT
                topic,
                COUNT(*) AS total,
                COUNT(*) FILTER (WHERE is_correct) AS correct
            FROM quiz_results
            WHERE user_id = :uid
            GROUP BY topic
            ORDER BY total DESC
            LIMIT 20
        """),
        {"uid": user_id},
    )
    rows = result.mappings().all()

    topics = [
        TopicStat(
            topic=row["topic"],
            total=int(row["total"]),
            correct=int(row["correct"]),
            accuracy_pct=round(int(row["correct"]) / int(row["total"]) * 100, 1),
        )
        for row in rows
    ]
    return QuizBreakdown(topics=topics)


@router.get("/{user_id}/activity", response_model=ActivityHistory)
async def get_activity(
    user_id: str,
    caller: str = Depends(require_auth),
    db: AsyncSession = Depends(get_db),
) -> ActivityHistory:
    """Return a 30-day rolling activity history for a student.

    Counts student-role messages per calendar day for the past 30 days.
    Days with no activity are included with messages=0 so the frontend
    heatmap always receives exactly 30 data points.

    Args:
        user_id: Target student UUID (path parameter).
        caller: Authenticated user ID from JWT (injected by require_auth).
        db: Async SQLAlchemy session (injected by get_db).

    Returns:
        ActivityHistory with exactly 30 DayActivity entries ordered oldest
        to newest.

    Raises:
        HTTPException: 401 if the JWT is missing or invalid.
        HTTPException: 403 if caller != user_id.
    """
    _check_self_or_raise(caller, user_id)

    since = date.today() - timedelta(days=29)

    result = await db.execute(
        text("""
            SELECT
                created_at::date AS day,
                COUNT(*) FILTER (WHERE role = 'student') AS messages
            FROM interactions
            WHERE user_id = :uid
              AND created_at >= :since
            GROUP BY created_at::date
            ORDER BY day
        """),
        {"uid": user_id, "since": since},
    )
    rows = {str(r["day"]): int(r["messages"]) for r in result.mappings().all()}

    # Fill all 30 days (zero for missing dates)
    days = []
    for i in range(30):
        d = str(since + timedelta(days=i))
        days.append(DayActivity(date=d, messages=rows.get(d, 0)))

    return ActivityHistory(days=days)
