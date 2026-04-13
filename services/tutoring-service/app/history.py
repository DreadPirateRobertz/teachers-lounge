"""Conversation history — CRUD helpers for sessions and interactions."""
from uuid import UUID, uuid4

from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession

from .orm import Interaction, Session


async def create_session(
    db: AsyncSession,
    user_id: UUID,
    course_id: UUID | None = None,
) -> Session:
    """Create a new tutoring session row and return the persisted object.

    Args:
        db: Async SQLAlchemy session.
        user_id: UUID of the authenticated student.
        course_id: Optional UUID of the course this session is for.

    Returns:
        Newly created and refreshed Session ORM object.
    """
    session = Session(id=uuid4(), user_id=user_id, course_id=course_id)
    db.add(session)
    await db.commit()
    await db.refresh(session)
    return session


async def get_session(db: AsyncSession, session_id: UUID) -> Session | None:
    """Fetch a single tutoring session by primary key.

    Args:
        db: Async SQLAlchemy session.
        session_id: UUID of the tutoring session.

    Returns:
        Session ORM object, or ``None`` if not found.
    """
    result = await db.execute(select(Session).where(Session.id == session_id))
    return result.scalar_one_or_none()


async def count_history(db: AsyncSession, session_id: UUID) -> int:
    """Count the total number of messages in a tutoring session.

    Args:
        db: Async SQLAlchemy session.
        session_id: UUID of the tutoring session.

    Returns:
        Total message count (student + tutor turns combined).
    """
    result = await db.execute(
        select(func.count()).where(Interaction.session_id == session_id)
    )
    return result.scalar_one()


async def get_history(
    db: AsyncSession,
    session_id: UUID,
    limit: int = 20,
) -> list[Interaction]:
    """Load the most recent *limit* messages for a session (sliding window).

    Fetches the last ``limit`` interactions in descending order then reverses
    them so callers receive chronological (oldest-first) order.  Uses the
    composite index on ``(session_id, created_at DESC)`` for efficient lookups
    without a full-table sort.

    Args:
        db: Async SQLAlchemy session.
        session_id: UUID of the tutoring session.
        limit: Maximum messages to return. Defaults to 20 (10 exchanges).

    Returns:
        Up to ``limit`` Interactions ordered oldest-first (ascending created_at).
    """
    result = await db.execute(
        select(Interaction)
        .where(Interaction.session_id == session_id)
        .order_by(Interaction.created_at.desc())
        .limit(limit)
    )
    interactions = list(result.scalars().all())
    interactions.reverse()
    return interactions


async def get_history_slice(
    db: AsyncSession,
    session_id: UUID,
    offset: int,
    limit: int,
) -> list[Interaction]:
    """Load a chronological slice of a session's message history.

    Used by the context summarisation pipeline to retrieve older messages
    that fall outside the active sliding window.

    Args:
        db: Async SQLAlchemy session.
        session_id: UUID of the tutoring session.
        offset: Number of oldest messages to skip.
        limit: Maximum messages to return after the offset.

    Returns:
        Interactions ordered oldest-first (ascending created_at).
    """
    result = await db.execute(
        select(Interaction)
        .where(Interaction.session_id == session_id)
        .order_by(Interaction.created_at)
        .offset(offset)
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
    """Append a message to a tutoring session and return the persisted row.

    Args:
        db: Async SQLAlchemy session.
        session_id: UUID of the tutoring session.
        user_id: UUID of the message author.
        role: Message role — ``"student"`` or ``"tutor"``.
        content: Text content of the message.
        response_time_ms: Optional latency in milliseconds (for tutor messages).

    Returns:
        Newly created and refreshed Interaction ORM object.
    """
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
