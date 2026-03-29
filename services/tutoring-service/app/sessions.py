"""Session management endpoints."""
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.ext.asyncio import AsyncSession

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
async def new_session(body: CreateSessionRequest, db: AsyncSession = Depends(get_db)):
    """Create a new tutoring session for a user."""
    session = await create_session(db, user_id=body.user_id, course_id=body.course_id)
    return SessionResponse(
        session_id=session.id,
        user_id=session.user_id,
        course_id=session.course_id,
        created_at=session.created_at,
        message_count=0,
    )


@router.get("/{session_id}", response_model=HistoryResponse)
async def get_session_history(session_id: UUID, db: AsyncSession = Depends(get_db)):
    """Return full conversation history for a session."""
    session = await get_session(db, session_id)
    if session is None:
        raise HTTPException(status_code=404, detail="Session not found")

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
