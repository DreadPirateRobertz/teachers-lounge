"""Session management endpoints — JWT-protected."""

from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException, Request
from sqlalchemy.ext.asyncio import AsyncSession

from .audit import ACTION_READ_INTERACTIONS, write_audit_log
from .auth import JWTClaims, require_auth
from .cache import get_cached_history, set_cached_history
from .database import get_db
from .history import create_session, get_history, get_session
from .models import (
    CreateSessionRequest,
    HistoryResponse,
    MessageRecord,
    Role,
    SessionResponse,
)
from .summary import SessionSummary, compute_session_summary

router = APIRouter(prefix="/sessions", tags=["sessions"])


@router.post("", response_model=SessionResponse, status_code=201)
async def new_session(
    body: CreateSessionRequest,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Create a new tutoring session. user_id is taken from the JWT — not the request body."""
    session = await create_session(db, user_id=user.user_id, course_id=body.course_id)
    return SessionResponse(
        session_id=session.id,
        user_id=session.user_id,
        course_id=session.course_id,
        created_at=session.created_at,
        message_count=0,
    )


@router.get("/{session_id}", response_model=HistoryResponse)
async def get_session_history(
    session_id: UUID,
    request: Request,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Return conversation history for a session.

    Checks a Redis cache first (5-minute TTL) before hitting Postgres.
    Returns 403 if the session belongs to another user.
    """
    # Authorisation check always hits Postgres to prevent cache poisoning
    session = await get_session(db, session_id)
    if session is None:
        raise HTTPException(status_code=404, detail="Session not found")
    if session.user_id != user.user_id:
        raise HTTPException(status_code=403, detail="Forbidden")

    # Try cache
    cached = await get_cached_history(session_id)
    if cached is not None:
        return HistoryResponse(
            session_id=session_id,
            messages=[MessageRecord(**m) for m in cached],
        )

    # Cache miss — load from Postgres
    interactions = await get_history(db, session_id)

    # FERPA: log every interaction history read
    await write_audit_log(
        db,
        accessor_id=user.user_id,
        student_id=user.user_id,
        action=ACTION_READ_INTERACTIONS,
        data_accessed=f"chat_session:{session_id}",
        purpose="user_request",
        ip_address=request.client.host if request.client else "",
    )

    messages = [
        MessageRecord(
            id=i.id,
            session_id=i.session_id,
            role=Role(i.role),
            content=i.content,
            created_at=i.created_at,
        )
        for i in interactions
    ]

    # Populate cache for next request (fire-and-forget — errors are swallowed in cache.py)
    await set_cached_history(
        user_id=user.user_id,
        session_id=session_id,
        messages=[m.model_dump(mode="json") for m in messages],
    )

    return HistoryResponse(session_id=session_id, messages=messages)


@router.get("/{session_id}/summary", response_model=SessionSummary)
async def get_session_summary(
    session_id: UUID,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
) -> SessionSummary:
    """Return aggregated statistics for a tutoring session.

    Returns 404 if the session does not exist, 403 if it belongs to
    another user. Owners receive the session's message count, average
    tutor response latency, total duration, and the list of concept
    topics touched during the session window.
    """
    session = await get_session(db, session_id)
    if session is None:
        raise HTTPException(status_code=404, detail="Session not found")
    if session.user_id != user.user_id:
        raise HTTPException(status_code=403, detail="Forbidden")

    return await compute_session_summary(db, session)
