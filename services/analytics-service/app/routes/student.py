"""Student analytics endpoints."""
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
    topic: str
    total: int
    correct: int
    accuracy_pct: float


class QuizBreakdown(BaseModel):
    topics: list[TopicStat]


class DayActivity(BaseModel):
    date: str   # ISO date string
    messages: int


class ActivityHistory(BaseModel):
    days: list[DayActivity]


# ── Helpers ──────────────────────────────────────────────────────────────────

def _check_self_or_raise(requesting_user_id: str, target_user_id: str) -> None:
    """Users may only access their own analytics."""
    if requesting_user_id != target_user_id:
        raise HTTPException(status_code=403, detail="Forbidden")


# ── Endpoints ────────────────────────────────────────────────────────────────

@router.get("/{user_id}/overview", response_model=Overview)
async def get_overview(
    user_id: str,
    caller: str = Depends(require_auth),
    db: AsyncSession = Depends(get_db),
) -> Overview:
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
