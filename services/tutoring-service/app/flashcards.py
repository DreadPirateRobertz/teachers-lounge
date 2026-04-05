"""Flashcard system — CRUD endpoints, session-based auto-generation, and Anki .apkg export.

Flashcard decks are owned by a student and contain question/answer cards.
Cards can be created manually or auto-generated from a chat session by
extracting key Q&A pairs from the tutor's responses.

Anki .apkg format
-----------------
An .apkg file is a ZIP archive containing:
  - ``collection.anki2``  — SQLite database with the Anki2 schema
  - ``media``             — JSON mapping of media filenames (``{}``) for text-only decks

The Anki2 SQLite schema is minimal for text cards:
  - ``col``   — one row of collection metadata (JSON fields for decks/models)
  - ``notes`` — one row per card note (front/back in ``flds``, tab-separated)
  - ``cards`` — one row per card with scheduling defaults
  - ``revlog``/``graves`` — empty tables required by the schema

References:
  https://github.com/ankidroid/Anki-Android/wiki/Database-Structure
"""
from __future__ import annotations

import hashlib
import io
import json
import random
import sqlite3
import time
import zipfile
from typing import Sequence
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException
from fastapi.responses import StreamingResponse
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from .auth import JWTClaims, require_auth
from .database import get_db
from .models import (
    DeckCreate,
    DeckResponse,
    DeckWithCards,
    FlashcardCreate,
    FlashcardResponse,
    GenerateFlashcardsRequest,
    GenerateFlashcardsResponse,
)
from .orm import Flashcard, FlashcardDeck, Interaction, Session

router = APIRouter(prefix="/flashcards", tags=["flashcards"])


# ── Anki .apkg builder ────────────────────────────────────────────────────────

# Static Anki model ID (Basic note type) — consistent across exports
_ANKI_MODEL_ID = 1702900000000  # arbitrary stable epoch-like int
_ANKI_MODEL_NAME = "TeachersLounge Basic"
_ANKI_COL_ID = 1  # collection always has id=1


def _note_checksum(front: str) -> int:
    """Compute Anki's sfld checksum: first 8 hex chars of SHA1 of the sort field.

    Args:
        front: The card front (sort field) text.

    Returns:
        Integer checksum used by Anki for duplicate detection.
    """
    return int(hashlib.sha1(front.encode()).hexdigest()[:8], 16)


def _build_anki_collection(
    deck_id: int,
    deck_name: str,
    cards: Sequence[tuple[str, str]],
) -> bytes:
    """Build a minimal Anki ``collection.anki2`` SQLite database in memory.

    Creates the required tables (col, notes, cards, revlog, graves) and
    populates them with the supplied card data.  Only Basic (front/back)
    note type is supported — media attachments are not included.

    Args:
        deck_id:   Integer Anki deck ID (must be unique per export).
        deck_name: Human-readable name for the Anki deck.
        cards:     Sequence of (front, back) string tuples.

    Returns:
        Raw bytes of the populated SQLite database.
    """
    db_bytes = io.BytesIO()
    # sqlite3 cannot write to a BytesIO directly — use :memory: then serialize
    conn = sqlite3.connect(":memory:")
    cur = conn.cursor()

    # ── Schema ──────────────────────────────────────────────────────────────
    cur.executescript("""
        CREATE TABLE col (
            id      INTEGER PRIMARY KEY,
            crt     INTEGER NOT NULL,
            mod     INTEGER NOT NULL,
            scm     INTEGER NOT NULL,
            ver     INTEGER NOT NULL,
            dty     INTEGER NOT NULL,
            usn     INTEGER NOT NULL,
            ls      INTEGER NOT NULL,
            conf    TEXT NOT NULL,
            models  TEXT NOT NULL,
            decks   TEXT NOT NULL,
            dconf   TEXT NOT NULL,
            tags    TEXT NOT NULL
        );
        CREATE TABLE notes (
            id      INTEGER PRIMARY KEY,
            guid    TEXT NOT NULL,
            mid     INTEGER NOT NULL,
            mod     INTEGER NOT NULL,
            usn     INTEGER NOT NULL,
            tags    TEXT NOT NULL,
            flds    TEXT NOT NULL,
            sfld    TEXT NOT NULL,
            csum    INTEGER NOT NULL,
            flags   INTEGER NOT NULL,
            data    TEXT NOT NULL
        );
        CREATE TABLE cards (
            id      INTEGER PRIMARY KEY,
            nid     INTEGER NOT NULL,
            did     INTEGER NOT NULL,
            ord     INTEGER NOT NULL,
            mod     INTEGER NOT NULL,
            usn     INTEGER NOT NULL,
            type    INTEGER NOT NULL,
            queue   INTEGER NOT NULL,
            due     INTEGER NOT NULL,
            ivl     INTEGER NOT NULL,
            factor  INTEGER NOT NULL,
            reps    INTEGER NOT NULL,
            lapses  INTEGER NOT NULL,
            left    INTEGER NOT NULL,
            odue    INTEGER NOT NULL,
            odid    INTEGER NOT NULL,
            flags   INTEGER NOT NULL,
            data    TEXT NOT NULL
        );
        CREATE TABLE revlog (
            id      INTEGER PRIMARY KEY,
            cid     INTEGER NOT NULL,
            usn     INTEGER NOT NULL,
            ease    INTEGER NOT NULL,
            ivl     INTEGER NOT NULL,
            lastIvl INTEGER NOT NULL,
            factor  INTEGER NOT NULL,
            time    INTEGER NOT NULL,
            type    INTEGER NOT NULL
        );
        CREATE TABLE graves (
            usn     INTEGER NOT NULL,
            oid     INTEGER NOT NULL,
            type    INTEGER NOT NULL
        );
    """)

    now_sec = int(time.time())

    # ── Model definition (Basic note type) ──────────────────────────────────
    models_json = json.dumps({
        str(_ANKI_MODEL_ID): {
            "id": _ANKI_MODEL_ID,
            "name": _ANKI_MODEL_NAME,
            "type": 0,
            "mod": now_sec,
            "usn": -1,
            "sortf": 0,
            "did": deck_id,
            "tmpls": [{
                "name": "Card 1",
                "ord": 0,
                "qfmt": "{{Front}}",
                "afmt": "{{FrontSide}}<hr id=answer>{{Back}}",
                "bqfmt": "",
                "bafmt": "",
                "did": None,
                "bfont": "",
                "bsize": 0,
            }],
            "flds": [
                {"name": "Front", "ord": 0, "sticky": False, "rtl": False,
                 "font": "Arial", "size": 20, "media": []},
                {"name": "Back",  "ord": 1, "sticky": False, "rtl": False,
                 "font": "Arial", "size": 20, "media": []},
            ],
            "css": ".card { font-family: arial; font-size: 20px; }",
            "latexPre": "",
            "latexPost": "",
            "tags": [],
            "vers": [],
        }
    })

    # ── Deck definition ──────────────────────────────────────────────────────
    decks_json = json.dumps({
        "1": {
            "id": 1,
            "name": "Default",
            "extendRev": 50,
            "usn": 0,
            "collapsed": False,
            "newToday": [0, 0],
            "timeToday": [0, 0],
            "dyn": 0,
            "extendNew": 10,
            "conf": 1,
            "revToday": [0, 0],
            "lrnToday": [0, 0],
            "mod": now_sec,
            "desc": "",
        },
        str(deck_id): {
            "id": deck_id,
            "name": deck_name,
            "extendRev": 50,
            "usn": -1,
            "collapsed": False,
            "newToday": [0, 0],
            "timeToday": [0, 0],
            "dyn": 0,
            "extendNew": 10,
            "conf": 1,
            "revToday": [0, 0],
            "lrnToday": [0, 0],
            "mod": now_sec,
            "desc": "",
        },
    })

    cur.execute(
        "INSERT INTO col VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
        (
            _ANKI_COL_ID,
            now_sec,                 # crt
            now_sec,                 # mod
            now_sec * 1000,          # scm (schema mod time in ms)
            11,                      # ver (Anki 2.x format version)
            0,                       # dty (dirty flag)
            -1,                      # usn
            0,                       # ls (last sync)
            "{}",                    # conf
            models_json,
            decks_json,
            '{"1":{"id":1,"mod":0,"name":"Default","usn":0,"maxTake":-1,"new":{"delays":[1,10],"ints":[1,4,7],"initialFactor":2500,"separate":true,"order":1,"perDay":20,"bury":false},"lapse":{"delays":[10],"leechAction":0,"leechFails":8,"minInt":1,"mult":0},"rev":{"bury":false,"ease4":1.3,"fuzz":0.05,"ivlFct":1,"maxIvl":36500,"minSpace":1,"perDay":200},"timer":0,"autoplay":true,"replayq":true}}',
            "{}",                    # tags
        ),
    )

    # ── Notes + Cards ────────────────────────────────────────────────────────
    for i, (front, back) in enumerate(cards):
        note_id = now_sec * 1000 + i           # ms-level unique id
        card_id = note_id + 1
        guid = hashlib.md5(f"{deck_name}{front}{i}".encode()).hexdigest()[:10]
        flds = f"{front}\x1f{back}"            # \x1f is Anki's field separator

        cur.execute(
            "INSERT INTO notes VALUES (?,?,?,?,?,?,?,?,?,?,?)",
            (note_id, guid, _ANKI_MODEL_ID, now_sec, -1, "", flds, front,
             _note_checksum(front), 0, ""),
        )
        cur.execute(
            "INSERT INTO cards VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
            (card_id, note_id, deck_id, 0, now_sec, -1,
             0, 0, i, 0, 0, 0, 0, 0, 0, 0, 0, ""),
        )

    conn.commit()

    # conn.serialize() is available in Python 3.11+ (CPython extension).
    # It returns the raw bytes of the in-memory SQLite database.
    try:
        raw_bytes: bytes = conn.serialize()  # type: ignore[attr-defined]
    except AttributeError:
        # Fallback for environments without serialize(): write to a temp file
        import os
        import tempfile
        with tempfile.NamedTemporaryFile(suffix=".db", delete=False) as tmp:
            tmp_path = tmp.name
        target = sqlite3.connect(tmp_path)
        conn.backup(target)
        target.close()
        with open(tmp_path, "rb") as f:
            raw_bytes = f.read()
        os.unlink(tmp_path)
    finally:
        conn.close()

    return raw_bytes


def build_apkg(deck_name: str, cards: Sequence[tuple[str, str]]) -> bytes:
    """Package a list of (front, back) card pairs into an Anki .apkg ZIP archive.

    The returned bytes can be saved as ``<name>.apkg`` and imported directly
    into Anki desktop or AnkiDroid.

    Args:
        deck_name: Human-readable name for the Anki deck.
        cards:     Sequence of (front, back) string tuples.  Empty sequences
                   produce a valid but empty deck.

    Returns:
        Raw bytes of the .apkg ZIP archive.
    """
    # Anki deck IDs must be integers in a specific range
    deck_id = abs(hash(deck_name)) % (10**13) + 1

    collection_bytes = _build_anki_collection(deck_id, deck_name, cards)

    apkg_buf = io.BytesIO()
    with zipfile.ZipFile(apkg_buf, "w", compression=zipfile.ZIP_DEFLATED) as zf:
        zf.writestr("collection.anki2", collection_bytes)
        zf.writestr("media", "{}")   # no media attachments

    return apkg_buf.getvalue()


# ── Auto-generation from session ──────────────────────────────────────────────

# Regex is deliberately avoided — the LLM already produced well-structured
# tutor responses.  We extract definitions / key facts by scanning for lines
# that follow common "term — definition" or "Q: … A: …" patterns.
import re

_DEFINITION_PATTERN = re.compile(
    r"^\s*[\*\-]?\s*\*?\*?([^:*\n]{3,80})\*?\*?\s*[:\-—]+\s*(.{10,})",
    re.MULTILINE,
)
_QA_PATTERN = re.compile(
    r"Q[:\.]?\s*(.{5,200}?)\s*\n+A[:\.]?\s*(.{5,})",
    re.IGNORECASE | re.DOTALL,
)

_MAX_FRONT_LEN = 300
_MAX_BACK_LEN = 800
_MAX_AUTO_CARDS = 20


def extract_flashcards_from_text(text: str) -> list[tuple[str, str]]:
    """Heuristically extract (front, back) flashcard pairs from tutor response text.

    Scans for two common patterns:
    1. ``**Term**: definition`` or ``Term — definition`` (bullet-list style)
    2. ``Q: question\nA: answer`` blocks

    Duplicates (same front text) are removed; results are capped at
    ``_MAX_AUTO_CARDS`` to avoid overwhelming a single deck.

    Args:
        text: Raw tutor response text to scan for flashcard material.

    Returns:
        List of (front, back) tuples, truncated to fit field length limits.
    """
    seen_fronts: set[str] = set()
    pairs: list[tuple[str, str]] = []

    def _add(front: str, back: str) -> None:
        front = front.strip()[:_MAX_FRONT_LEN]
        back = back.strip()[:_MAX_BACK_LEN]
        if front and back and front not in seen_fronts:
            seen_fronts.add(front)
            pairs.append((front, back))

    for m in _DEFINITION_PATTERN.finditer(text):
        _add(m.group(1), m.group(2))

    for m in _QA_PATTERN.finditer(text):
        _add(m.group(1), m.group(2))

    return pairs[:_MAX_AUTO_CARDS]


# ── Endpoints ─────────────────────────────────────────────────────────────────

@router.get("/decks", response_model=list[DeckResponse])
async def list_decks(
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """List all flashcard decks owned by the authenticated student."""
    result = await db.execute(
        select(FlashcardDeck)
        .where(FlashcardDeck.user_id == user.user_id)
        .order_by(FlashcardDeck.created_at.desc())
    )
    decks = result.scalars().all()
    return [
        DeckResponse(
            id=d.id,
            name=d.name,
            description=d.description,
            session_id=d.session_id,
            card_count=len(d.cards),
            created_at=d.created_at,
        )
        for d in decks
    ]


@router.post("/decks", response_model=DeckResponse, status_code=201)
async def create_deck(
    body: DeckCreate,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Create a new (empty) flashcard deck."""
    deck = FlashcardDeck(
        user_id=user.user_id,
        name=body.name,
        description=body.description,
        session_id=body.session_id,
    )
    db.add(deck)
    await db.commit()
    await db.refresh(deck)
    return DeckResponse(
        id=deck.id,
        name=deck.name,
        description=deck.description,
        session_id=deck.session_id,
        card_count=0,
        created_at=deck.created_at,
    )


@router.get("/decks/{deck_id}", response_model=DeckWithCards)
async def get_deck(
    deck_id: UUID,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Get a deck with all its cards."""
    deck = await _get_owned_deck(db, deck_id, user.user_id)
    return _deck_with_cards(deck)


@router.delete("/decks/{deck_id}", status_code=204)
async def delete_deck(
    deck_id: UUID,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Delete a deck and all its cards."""
    deck = await _get_owned_deck(db, deck_id, user.user_id)
    await db.delete(deck)
    await db.commit()


@router.post("/decks/{deck_id}/cards", response_model=FlashcardResponse, status_code=201)
async def add_card(
    deck_id: UUID,
    body: FlashcardCreate,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Add a single card to a deck."""
    deck = await _get_owned_deck(db, deck_id, user.user_id)
    card = Flashcard(
        deck_id=deck.id,
        user_id=user.user_id,
        front=body.front,
        back=body.back,
        source_interaction_id=body.source_interaction_id,
    )
    db.add(card)
    await db.commit()
    await db.refresh(card)
    return _card_response(card)


@router.delete("/decks/{deck_id}/cards/{card_id}", status_code=204)
async def delete_card(
    deck_id: UUID,
    card_id: UUID,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Remove a single card from a deck."""
    deck = await _get_owned_deck(db, deck_id, user.user_id)
    result = await db.execute(
        select(Flashcard).where(
            Flashcard.id == card_id,
            Flashcard.deck_id == deck.id,
        )
    )
    card = result.scalar_one_or_none()
    if card is None:
        raise HTTPException(status_code=404, detail="Card not found")
    await db.delete(card)
    await db.commit()


@router.get("/decks/{deck_id}/export/anki")
async def export_anki(
    deck_id: UUID,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Export a deck as an Anki .apkg file for import into Anki desktop/mobile.

    Returns a streaming binary response with Content-Disposition: attachment.
    """
    deck = await _get_owned_deck(db, deck_id, user.user_id)
    if not deck.cards:
        raise HTTPException(status_code=422, detail="Deck has no cards to export")

    card_pairs = [(c.front, c.back) for c in deck.cards]
    apkg_bytes = build_apkg(deck.name, card_pairs)

    safe_name = re.sub(r"[^\w\-.]", "_", deck.name)
    filename = f"{safe_name}.apkg"

    return StreamingResponse(
        iter([apkg_bytes]),
        media_type="application/octet-stream",
        headers={"Content-Disposition": f'attachment; filename="{filename}"'},
    )


@router.post("/sessions/{session_id}/generate", response_model=GenerateFlashcardsResponse, status_code=201)
async def generate_from_session(
    session_id: UUID,
    body: GenerateFlashcardsRequest,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Auto-generate flashcards from a chat session's tutor responses.

    Scans all tutor interactions in the session for definition-style and
    Q&A-style patterns, deduplicates, and creates a new deck (or appends
    to an existing deck if ``deck_id`` is provided in the request body).

    Args:
        session_id: The chat session to extract cards from.
        body:       Optional deck_id and deck_name for the new deck.

    Returns:
        The deck ID, name, and list of generated cards.
    """
    # Verify session ownership
    sess_result = await db.execute(
        select(Session).where(Session.id == session_id)
    )
    session = sess_result.scalar_one_or_none()
    if session is None:
        raise HTTPException(status_code=404, detail="Session not found")
    if session.user_id != user.user_id:
        raise HTTPException(status_code=403, detail="Forbidden")

    # Collect tutor messages
    interactions_result = await db.execute(
        select(Interaction)
        .where(Interaction.session_id == session_id, Interaction.role == "tutor")
        .order_by(Interaction.created_at.asc())
    )
    tutor_interactions = interactions_result.scalars().all()

    if not tutor_interactions:
        raise HTTPException(status_code=422, detail="Session has no tutor responses to extract from")

    # Extract cards from each tutor message
    seen_fronts: set[str] = set()
    raw_pairs: list[tuple[str, UUID]] = []  # (front, interaction_id)

    for interaction in tutor_interactions:
        pairs = extract_flashcards_from_text(interaction.content)
        for front, back in pairs:
            if front not in seen_fronts:
                seen_fronts.add(front)
                raw_pairs.append((front, back, interaction.id))  # type: ignore[arg-type]

    if not raw_pairs:
        raise HTTPException(
            status_code=422,
            detail="No flashcard-worthy content detected in this session's tutor responses",
        )

    # Create or retrieve deck
    if body.deck_id is not None:
        deck = await _get_owned_deck(db, body.deck_id, user.user_id)
    else:
        deck_name = body.deck_name or f"Session {str(session_id)[:8]}"
        deck = FlashcardDeck(
            user_id=user.user_id,
            name=deck_name,
            description=f"Auto-generated from chat session {session_id}",
            session_id=session_id,
        )
        db.add(deck)
        await db.flush()  # obtain deck.id before adding cards

    created_cards: list[Flashcard] = []
    for front, back, interaction_id in raw_pairs:
        card = Flashcard(
            deck_id=deck.id,
            user_id=user.user_id,
            front=front,
            back=back,
            source_interaction_id=interaction_id,
        )
        db.add(card)
        created_cards.append(card)

    await db.commit()
    await db.refresh(deck)

    return GenerateFlashcardsResponse(
        deck_id=deck.id,
        deck_name=deck.name,
        generated_count=len(created_cards),
        cards=[_card_response(c) for c in created_cards],
    )


# ── Private helpers ───────────────────────────────────────────────────────────

async def _get_owned_deck(
    db: AsyncSession,
    deck_id: UUID,
    user_id: UUID,
) -> FlashcardDeck:
    """Fetch a FlashcardDeck and verify ownership; raise 404/403 on failure.

    Args:
        db:      Active async DB session.
        deck_id: UUID of the deck to fetch.
        user_id: UUID of the authenticated user.

    Returns:
        The owned FlashcardDeck ORM instance.

    Raises:
        HTTPException 404: Deck not found.
        HTTPException 403: Deck belongs to a different user.
    """
    result = await db.execute(select(FlashcardDeck).where(FlashcardDeck.id == deck_id))
    deck = result.scalar_one_or_none()
    if deck is None:
        raise HTTPException(status_code=404, detail="Deck not found")
    if deck.user_id != user_id:
        raise HTTPException(status_code=403, detail="Forbidden")
    return deck


def _card_response(card: Flashcard) -> FlashcardResponse:
    """Convert a Flashcard ORM row to a response DTO.

    Args:
        card: Flashcard ORM instance.

    Returns:
        FlashcardResponse Pydantic model.
    """
    return FlashcardResponse(
        id=card.id,
        deck_id=card.deck_id,
        front=card.front,
        back=card.back,
        source_interaction_id=card.source_interaction_id,
        created_at=card.created_at,
    )


def _deck_with_cards(deck: FlashcardDeck) -> DeckWithCards:
    """Convert a FlashcardDeck ORM row (with loaded cards) to a response DTO.

    Args:
        deck: FlashcardDeck ORM instance with ``cards`` relationship loaded.

    Returns:
        DeckWithCards Pydantic model including all card data.
    """
    return DeckWithCards(
        id=deck.id,
        name=deck.name,
        description=deck.description,
        session_id=deck.session_id,
        card_count=len(deck.cards),
        created_at=deck.created_at,
        cards=[_card_response(c) for c in deck.cards],
    )
