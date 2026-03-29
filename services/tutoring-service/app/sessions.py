"""Session management endpoints — JWT-protected."""
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.ext.asyncio import AsyncSession

from .auth import JWTClaims, require_auth
from .database import get_db
from .history import create_session, get_history, get_session
from .models import (
    CreateSessionRequest,
    HistoryResponse,
    MessageRecord,
    Role,
    SessionResponse,
)

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
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Return conversation history. Returns 403 if the session belongs to another user."""
    session = await get_session(db, session_id)
    if session is None:
        raise HTTPException(status_code=404, detail="Session not found")
    if session.user_id != user.user_id:
        raise HTTPException(status_code=403, detail="Forbidden")

    interactions = await get_history(db, session_id)
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
    return HistoryResponse(session_id=session_id, messages=messages)
