"""Flashcard system — auto-generated Q/A from tutoring sessions (tl-y3v).

Endpoints (all JWT-protected via :func:`app.auth.require_auth`):

  POST /v1/flashcards/generate
      Body: {session_id, max_cards?}
      Calls the AI gateway to extract concept/definition pairs from the
      session transcript and persists them.  ``user_id`` comes from the JWT,
      never the request body, so students cannot seed cards into another
      student's deck.

  GET  /v1/flashcards?due=true&limit=N
      Returns cards ordered by ``due_at`` ascending.  ``due=true`` filters to
      cards where ``due_at <= now()`` — the review queue.  ``due=false``
      returns every card the student owns.

  POST /v1/flashcards/{id}/review
      Body: {quality: 0-5}
      Applies :func:`app.srs.sm2_update` (shared with concept reviews) and
      persists the new interval / ease factor / repetitions + ``due_at``.

  GET  /v1/flashcards/export?format=anki
      Returns Anki-compatible TSV (``text/tab-separated-values``) with
      columns ``front``, ``back``, ``tags``.  Suitable for Anki's File →
      Import → "Fields separated by: Tab" dialog.  No external ``genanki``
      dependency required; TSV is the format Anki itself recommends when a
      .apkg bundle is not needed.
"""

from __future__ import annotations

import csv
import io
import json
import logging
import re
from datetime import datetime, timezone
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException, Query, status
from fastapi.responses import Response
from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession

from .auth import JWTClaims, require_auth
from .database import get_db
from .gateway import get_gateway_client
from .history import get_history
from .models import (
    FlashcardListResponse,
    FlashcardResponse,
    FlashcardReviewRequest,
    FlashcardReviewResponse,
    GenerateFlashcardsRequest,
    GenerateFlashcardsResponse,
)
from .orm import Flashcard, Session
from .srs import next_review_time, sm2_update

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/flashcards", tags=["flashcards"])


# ── Constants ─────────────────────────────────────────────────────────────────

# Model used for Q/A extraction — small/fast model is enough for deterministic
# JSON output.  Falls back to the configured tutor model when unset.
_EXTRACTION_MODEL = "claude-haiku-4-5-20251001"

_EXTRACTION_SYSTEM_PROMPT = """You extract study flashcards from a tutoring \
session transcript.  Output a JSON object with a single key ``cards`` whose \
value is an array of objects of the form:

  {"front": "<concise question or concept>",
   "back":  "<definition, explanation, or answer>",
   "concept_id": "<lowercase.snake.case tag or null>"}

Rules:
 - Extract only durable learning material — skip chitchat, meta comments,
   apologies, and duplicate restatements.
 - ``front`` must be a short question or concept (< 120 chars).
 - ``back`` is the definition / explanation (< 400 chars).
 - ``concept_id`` is a dot-separated lowercase slug when the transcript
   names a canonical concept (e.g. ``chemistry.organic.alkane``); ``null``
   otherwise.  Never invent a slug.
 - Do not include cards whose ``front`` or ``back`` is empty.
 - Return an empty ``cards`` array when nothing is extractable.
"""


# ── Helpers ───────────────────────────────────────────────────────────────────


def _to_response(card: Flashcard) -> FlashcardResponse:
    """Copy the ORM row into the JSON-safe Pydantic response model."""
    return FlashcardResponse(
        id=card.id,
        front=card.front,
        back=card.back,
        concept_id=card.concept_id,
        source_session_id=card.source_session_id,
        created_at=card.created_at,
        last_reviewed_at=card.last_reviewed_at,
        due_at=card.due_at,
        sm2_interval_days=card.sm2_interval_days,
        sm2_ease_factor=card.sm2_ease_factor,
        sm2_repetitions=card.sm2_repetitions,
    )


def _parse_extraction_payload(raw: str) -> list[dict]:
    """Parse the model's JSON output and return the list of card dicts.

    The gateway is configured for JSON mode but we are defensive in case the
    model wraps the JSON in prose; we locate the first ``{`` … ``}`` block
    and parse that.  Malformed output is logged and treated as "no cards" —
    extraction is best-effort, not authoritative.
    """
    if not raw:
        return []
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError:
        # Locate the first balanced JSON object in the prose.
        match = re.search(r"\{.*\}", raw, re.DOTALL)
        if not match:
            logger.warning("flashcard extraction: no JSON object in model output")
            return []
        try:
            payload = json.loads(match.group(0))
        except json.JSONDecodeError:
            logger.warning("flashcard extraction: malformed JSON in model output")
            return []

    cards = payload.get("cards")
    if not isinstance(cards, list):
        return []
    cleaned: list[dict] = []
    for item in cards:
        if not isinstance(item, dict):
            continue
        front = (item.get("front") or "").strip()
        back = (item.get("back") or "").strip()
        if not front or not back:
            continue
        concept_id = item.get("concept_id")
        if isinstance(concept_id, str):
            concept_id = concept_id.strip() or None
        else:
            concept_id = None
        cleaned.append({"front": front, "back": back, "concept_id": concept_id})
    return cleaned


async def _extract_cards_from_transcript(
    transcript: list[tuple[str, str]],
    max_cards: int,
) -> list[dict]:
    """Call the AI gateway to produce card dicts from a (role, content) list.

    Returns at most ``max_cards`` dicts; the LLM is additionally nudged
    toward that cap in the user prompt.  Returns ``[]`` on any gateway
    failure — generation must never take the endpoint down.
    """
    if not transcript:
        return []

    rendered = "\n".join(f"{role.upper()}: {content}" for role, content in transcript)
    user_prompt = (
        f"Extract up to {max_cards} flashcards from this tutoring session.\n\n---\n{rendered}\n---"
    )

    client = get_gateway_client()
    try:
        completion = await client.chat.completions.create(
            model=_EXTRACTION_MODEL,
            messages=[
                {"role": "system", "content": _EXTRACTION_SYSTEM_PROMPT},
                {"role": "user", "content": user_prompt},
            ],
            response_format={"type": "json_object"},
            max_tokens=1200,
        )
    except Exception:  # noqa: BLE001 — gateway failures must not 500 the endpoint
        logger.exception("flashcard extraction: gateway call failed")
        return []

    content = ""
    if completion.choices:
        content = completion.choices[0].message.content or ""
    cards = _parse_extraction_payload(content)
    return cards[:max_cards]


# ── Endpoints ─────────────────────────────────────────────────────────────────


@router.post("/generate", response_model=GenerateFlashcardsResponse)
async def generate_flashcards(
    body: GenerateFlashcardsRequest,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
) -> GenerateFlashcardsResponse:
    """Extract flashcards from a tutoring session transcript and persist them.

    Returns a 404 if the session does not exist or belongs to a different
    user.  Returns 200 with ``created_count=0`` (and an empty list) when the
    transcript yields no extractable Q/A pairs — an empty session is not an
    error.
    """
    session_result = await db.execute(select(Session).where(Session.id == body.session_id))
    session = session_result.scalar_one_or_none()
    if session is None or session.user_id != user.user_id:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Session not found")

    history = await get_history(db, body.session_id, limit=50)
    transcript = [(row.role, row.content) for row in history]

    extracted = await _extract_cards_from_transcript(transcript, body.max_cards)

    created: list[Flashcard] = []
    for item in extracted:
        card = Flashcard(
            user_id=user.user_id,
            front=item["front"],
            back=item["back"],
            concept_id=item["concept_id"],
            source_session_id=body.session_id,
        )
        db.add(card)
        created.append(card)

    if created:
        await db.flush()
        await db.commit()

    return GenerateFlashcardsResponse(
        session_id=body.session_id,
        created_count=len(created),
        cards=[_to_response(c) for c in created],
    )


@router.get("", response_model=FlashcardListResponse)
async def list_flashcards(
    due: bool = Query(default=False, description="If true, return only cards whose due_at <= now."),
    limit: int = Query(default=50, ge=1, le=500),
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
) -> FlashcardListResponse:
    """Return the student's flashcards, optionally filtered to the review queue.

    Cards are ordered by ``due_at`` ascending so the most overdue cards
    appear first.  ``total`` is the count of cards matching the filter
    (may exceed ``limit``) so clients can paginate.
    """
    now = datetime.now(timezone.utc)
    base = select(Flashcard).where(Flashcard.user_id == user.user_id)
    if due:
        base = base.where(Flashcard.due_at <= now)

    result = await db.execute(base.order_by(Flashcard.due_at.asc()).limit(limit))
    rows = list(result.scalars().all())

    count_stmt = (
        select(func.count()).select_from(Flashcard).where(Flashcard.user_id == user.user_id)
    )
    if due:
        count_stmt = count_stmt.where(Flashcard.due_at <= now)
    total_result = await db.execute(count_stmt)
    total = int(total_result.scalar_one() or 0)

    return FlashcardListResponse(items=[_to_response(r) for r in rows], total=total)


@router.post("/{card_id}/review", response_model=FlashcardReviewResponse)
async def review_flashcard(
    card_id: UUID,
    body: FlashcardReviewRequest,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
) -> FlashcardReviewResponse:
    """Record a review response and advance the card's SM-2 schedule.

    Returns 404 if the card does not exist or is owned by a different user;
    the ownership check lives at the application layer in addition to the
    RLS policy on the table so requests that bypass row-level security
    (e.g. superuser connections) still cannot cross-review.
    """
    result = await db.execute(select(Flashcard).where(Flashcard.id == card_id))
    card = result.scalar_one_or_none()
    if card is None or card.user_id != user.user_id:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Flashcard not found")

    new_interval, new_ef, new_reps = sm2_update(
        quality=body.quality,
        ease_factor=card.sm2_ease_factor,
        interval_days=card.sm2_interval_days,
        repetitions=card.sm2_repetitions,
    )
    now = datetime.now(timezone.utc)
    card.sm2_interval_days = new_interval
    card.sm2_ease_factor = new_ef
    card.sm2_repetitions = new_reps
    card.last_reviewed_at = now
    card.due_at = next_review_time(new_interval, now)

    await db.commit()

    return FlashcardReviewResponse(
        id=card.id,
        quality=body.quality,
        sm2_interval_days=card.sm2_interval_days,
        sm2_ease_factor=card.sm2_ease_factor,
        sm2_repetitions=card.sm2_repetitions,
        last_reviewed_at=card.last_reviewed_at,
        due_at=card.due_at,
    )


@router.get("/export")
async def export_flashcards(
    format: str = Query(default="anki", pattern="^(anki|csv)$"),
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
) -> Response:
    """Export the student's deck for import into Anki.

    Emits TSV (``text/tab-separated-values``) with three columns —
    ``front``, ``back``, ``tags`` — which is the format Anki's File →
    Import dialog consumes when "Fields separated by: Tab" is selected.
    The tags column is a space-separated list (Anki's native convention);
    the card's ``concept_id`` is emitted as the sole tag when set.

    ``format=csv`` emits the same columns as comma-separated values for
    clients that want plain CSV; both formats share the same header row
    so downstream tooling can detect them by content-type.
    """
    result = await db.execute(
        select(Flashcard)
        .where(Flashcard.user_id == user.user_id)
        .order_by(Flashcard.created_at.asc())
    )
    rows = list(result.scalars().all())

    buf = io.StringIO()
    delimiter = "\t" if format == "anki" else ","
    writer = csv.writer(buf, delimiter=delimiter, lineterminator="\n")
    writer.writerow(["front", "back", "tags"])
    for card in rows:
        tags = card.concept_id or ""
        writer.writerow([card.front, card.back, tags])

    if format == "anki":
        media = "text/tab-separated-values; charset=utf-8"
        filename = "teacherslounge-flashcards.tsv"
    else:
        media = "text/csv; charset=utf-8"
        filename = "teacherslounge-flashcards.csv"

    return Response(
        content=buf.getvalue(),
        media_type=media,
        headers={"Content-Disposition": f'attachment; filename="{filename}"'},
    )
