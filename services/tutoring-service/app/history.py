"""Conversation history — CRUD helpers for sessions and interactions."""
from datetime import datetime, timezone
from uuid import UUID, uuid4

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from .orm import Interaction, Session


async def create_session(
    db: AsyncSession,
    user_id: UUID,
    course_id: UUID | None = None,
) -> Session:
    session = Session(id=uuid4(), user_id=user_id, course_id=course_id)
    db.add(session)
    await db.commit()
    await db.refresh(session)
    return session


async def get_session(db: AsyncSession, session_id: UUID) -> Session | None:
    result = await db.execute(select(Session).where(Session.id == session_id))
    return result.scalar_one_or_none()


async def get_history(
    db: AsyncSession,
    session_id: UUID,
    limit: int = 50,
) -> list[Interaction]:
    result = await db.execute(
        select(Interaction)
        .where(Interaction.session_id == session_id)
        .order_by(Interaction.created_at)
        .limit(limit)
    )
    return list(result.scalars().all())


async def append_message(
    db: AsyncSession,
    session_id: UUID,
    user_id: UUID,
    role: str,
    content: str,
    response_time_ms: int | None = None,
) -> Interaction:
    msg = Interaction(
        id=uuid4(),
        session_id=session_id,
        user_id=user_id,
        role=role,
        content=content,
        response_time_ms=response_time_ms,
    )
    db.add(msg)
    await db.commit()
    await db.refresh(msg)
    return msg
