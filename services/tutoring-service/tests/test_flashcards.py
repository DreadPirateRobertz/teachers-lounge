"""HTTP-layer tests for the flashcard system (tl-y3v).

Covers all four endpoints:
  POST /v1/flashcards/generate
  GET  /v1/flashcards
  POST /v1/flashcards/{id}/review
  GET  /v1/flashcards/export

Uses FastAPI dependency overrides for auth (require_auth) and database
(get_db) so no real Postgres is needed.  The gateway client is monkey-
patched for generation tests so no real LiteLLM call happens.
"""

from __future__ import annotations

import csv
import io
import json
from datetime import datetime, timedelta, timezone
from types import SimpleNamespace
from unittest.mock import AsyncMock, MagicMock, patch
from uuid import UUID, uuid4

import pytest
from httpx import AsyncClient
from httpx._transports.asgi import ASGITransport

from app.auth import JWTClaims, require_auth
from app.database import get_db
from app.flashcards import _parse_extraction_payload
from app.main import app

STUDENT_ID = uuid4()
OTHER_STUDENT_ID = uuid4()


# ── Shared fixtures ──────────────────────────────────────────────────────────


def _claims(user_id: UUID = STUDENT_ID) -> JWTClaims:
    """Minimal JWTClaims for tests."""
    return JWTClaims(
        user_id=user_id,
        email="student@test.com",
        account_type="standard",
        sub_status="active",
    )


def _override_auth(user_id: UUID = STUDENT_ID) -> JWTClaims:
    """Install a fixed-claims auth override on the FastAPI app."""
    claims = _claims(user_id)
    app.dependency_overrides[require_auth] = lambda: claims
    return claims


def _override_db(session) -> None:
    """Install a DB override that yields ``session`` on every get_db."""

    async def _fake_db():
        yield session

    app.dependency_overrides[get_db] = _fake_db


def _clear_overrides() -> None:
    app.dependency_overrides.pop(require_auth, None)
    app.dependency_overrides.pop(get_db, None)


def _make_card(
    *,
    user_id: UUID = STUDENT_ID,
    due_at: datetime | None = None,
    front: str = "What is a mole?",
    back: str = "6.022e23 particles.",
    concept_id: str | None = "chemistry.stoichiometry.mole",
    card_id: UUID | None = None,
) -> MagicMock:
    """Return a MagicMock mimicking a Flashcard ORM row."""
    card = MagicMock()
    card.id = card_id or uuid4()
    card.user_id = user_id
    card.front = front
    card.back = back
    card.concept_id = concept_id
    card.source_session_id = uuid4()
    card.created_at = datetime.now(timezone.utc)
    card.last_reviewed_at = None
    card.due_at = due_at or datetime.now(timezone.utc)
    card.sm2_interval_days = 1
    card.sm2_ease_factor = 2.5
    card.sm2_repetitions = 0
    return card


def _session_row(user_id: UUID = STUDENT_ID, session_id: UUID | None = None) -> MagicMock:
    """Mimic a chat_sessions row for ownership checks."""
    row = MagicMock()
    row.id = session_id or uuid4()
    row.user_id = user_id
    return row


# ── Pure extraction parser ───────────────────────────────────────────────────


class TestExtractionPayloadParser:
    """Tests for the defensive JSON parser used on gateway output."""

    def test_parses_well_formed_json(self):
        """Well-formed JSON with a cards array returns cleaned card dicts."""
        raw = json.dumps(
            {
                "cards": [
                    {"front": "Q1", "back": "A1", "concept_id": "math.algebra"},
                    {"front": "Q2", "back": "A2", "concept_id": None},
                ]
            }
        )
        out = _parse_extraction_payload(raw)
        assert out == [
            {"front": "Q1", "back": "A1", "concept_id": "math.algebra"},
            {"front": "Q2", "back": "A2", "concept_id": None},
        ]

    def test_extracts_json_from_prose(self):
        """Model output with prose around the JSON still parses successfully."""
        raw = 'Here are the cards:\n{"cards": [{"front": "Q", "back": "A"}]}\nCheers!'
        out = _parse_extraction_payload(raw)
        assert out == [{"front": "Q", "back": "A", "concept_id": None}]

    def test_drops_empty_or_invalid_cards(self):
        """Cards missing front/back are silently discarded."""
        raw = json.dumps(
            {
                "cards": [
                    {"front": "", "back": "A"},  # empty front → drop
                    {"front": "Q", "back": ""},  # empty back → drop
                    "not-a-dict",  # wrong type → drop
                    {"front": "Q", "back": "A"},  # valid
                ]
            }
        )
        out = _parse_extraction_payload(raw)
        assert out == [{"front": "Q", "back": "A", "concept_id": None}]

    def test_malformed_json_returns_empty(self):
        """Malformed model output never raises; it returns an empty list."""
        assert _parse_extraction_payload("not json at all") == []
        assert _parse_extraction_payload("") == []

    def test_cards_missing_returns_empty(self):
        """JSON without a ``cards`` key returns an empty list."""
        assert _parse_extraction_payload(json.dumps({"other": []})) == []


# ── POST /v1/flashcards/generate ─────────────────────────────────────────────


class TestGenerateFlashcards:
    """Tests for POST /v1/flashcards/generate."""

    def setup_method(self):
        _override_auth()

    def teardown_method(self):
        _clear_overrides()

    async def _post(self, body: dict, status_code: int = 200) -> dict:
        """Helper — POST /generate and return the parsed JSON body."""
        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
            resp = await ac.post("/v1/flashcards/generate", json=body)
        assert resp.status_code == status_code, resp.text
        return resp.json()

    @pytest.mark.asyncio
    async def test_404_when_session_missing(self):
        """A session ID that resolves to no row returns 404."""
        db = MagicMock()
        # First execute() — Session lookup: returns no row.
        session_res = MagicMock()
        session_res.scalar_one_or_none.return_value = None
        db.execute = AsyncMock(return_value=session_res)
        _override_db(db)

        body = await self._post({"session_id": str(uuid4())}, status_code=404)
        assert "Session not found" in body["detail"]

    @pytest.mark.asyncio
    async def test_404_when_session_owned_by_other_user(self):
        """A session owned by a different student returns 404 (no data leak)."""
        db = MagicMock()
        session_res = MagicMock()
        session_res.scalar_one_or_none.return_value = _session_row(user_id=OTHER_STUDENT_ID)
        db.execute = AsyncMock(return_value=session_res)
        _override_db(db)

        await self._post({"session_id": str(uuid4())}, status_code=404)

    @pytest.mark.asyncio
    async def test_empty_transcript_returns_zero_cards(self):
        """A session with no messages yields 0 cards but is not an error."""
        session = _session_row(user_id=STUDENT_ID)
        db = MagicMock()
        session_res = MagicMock()
        session_res.scalar_one_or_none.return_value = session
        db.execute = AsyncMock(return_value=session_res)
        db.add = MagicMock()
        db.flush = AsyncMock()
        db.commit = AsyncMock()
        _override_db(db)

        with patch("app.flashcards.get_history", new=AsyncMock(return_value=[])):
            body = await self._post({"session_id": str(session.id)})

        assert body["created_count"] == 0
        assert body["cards"] == []
        db.add.assert_not_called()
        db.commit.assert_not_called()

    @pytest.mark.asyncio
    async def test_gateway_failure_returns_zero_cards(self):
        """Gateway exceptions must not 500 the endpoint — generation is best-effort."""
        session = _session_row(user_id=STUDENT_ID)
        db = MagicMock()
        session_res = MagicMock()
        session_res.scalar_one_or_none.return_value = session
        db.execute = AsyncMock(return_value=session_res)
        db.add = MagicMock()
        db.flush = AsyncMock()
        db.commit = AsyncMock()
        _override_db(db)

        history_row = SimpleNamespace(role="student", content="hello")
        with patch("app.flashcards.get_history", new=AsyncMock(return_value=[history_row])):
            bad_client = MagicMock()
            bad_client.chat.completions.create = AsyncMock(side_effect=RuntimeError("litellm down"))
            with patch("app.flashcards.get_gateway_client", return_value=bad_client):
                body = await self._post({"session_id": str(session.id)})

        assert body["created_count"] == 0
        assert body["cards"] == []

    @pytest.mark.asyncio
    async def test_happy_path_inserts_extracted_cards(self):
        """Gateway returns two cards → both are persisted and echoed back."""
        session = _session_row(user_id=STUDENT_ID)
        db = MagicMock()
        session_res = MagicMock()
        session_res.scalar_one_or_none.return_value = session
        db.execute = AsyncMock(return_value=session_res)
        db.add = MagicMock()
        db.flush = AsyncMock()
        db.commit = AsyncMock()
        _override_db(db)

        transcript = [
            SimpleNamespace(role="student", content="What is a mole?"),
            SimpleNamespace(
                role="tutor",
                content="A mole is 6.022e23 particles — Avogadro's number.",
            ),
        ]

        fake_completion = SimpleNamespace(
            choices=[
                SimpleNamespace(
                    message=SimpleNamespace(
                        content=json.dumps(
                            {
                                "cards": [
                                    {
                                        "front": "What is a mole?",
                                        "back": "6.022e23 particles.",
                                        "concept_id": "chemistry.stoichiometry.mole",
                                    },
                                    {
                                        "front": "Avogadro's number?",
                                        "back": "~6.022e23 per mole.",
                                        "concept_id": None,
                                    },
                                ]
                            }
                        )
                    )
                )
            ]
        )

        good_client = MagicMock()
        good_client.chat.completions.create = AsyncMock(return_value=fake_completion)

        with patch("app.flashcards.get_history", new=AsyncMock(return_value=transcript)):
            with patch("app.flashcards.get_gateway_client", return_value=good_client):
                body = await self._post({"session_id": str(session.id)})

        assert body["created_count"] == 2
        assert len(body["cards"]) == 2
        assert body["cards"][0]["front"] == "What is a mole?"
        assert body["cards"][0]["concept_id"] == "chemistry.stoichiometry.mole"
        # Cards started with default SM-2 state.
        assert body["cards"][0]["sm2_interval_days"] == 1
        assert body["cards"][0]["sm2_ease_factor"] == 2.5
        # Commit happened exactly once because at least one card was added.
        db.commit.assert_awaited_once()
        # db.add called once per extracted card.
        assert db.add.call_count == 2

    @pytest.mark.asyncio
    async def test_max_cards_cap_is_enforced(self):
        """Even if the model returns 25 cards, ``max_cards`` bounds the insert."""
        session = _session_row(user_id=STUDENT_ID)
        db = MagicMock()
        session_res = MagicMock()
        session_res.scalar_one_or_none.return_value = session
        db.execute = AsyncMock(return_value=session_res)
        db.add = MagicMock()
        db.flush = AsyncMock()
        db.commit = AsyncMock()
        _override_db(db)

        payload_cards = [{"front": f"Q{i}", "back": f"A{i}"} for i in range(25)]
        fake_completion = SimpleNamespace(
            choices=[
                SimpleNamespace(
                    message=SimpleNamespace(content=json.dumps({"cards": payload_cards}))
                )
            ]
        )
        good_client = MagicMock()
        good_client.chat.completions.create = AsyncMock(return_value=fake_completion)

        transcript = [SimpleNamespace(role="student", content="x")]
        with patch("app.flashcards.get_history", new=AsyncMock(return_value=transcript)):
            with patch("app.flashcards.get_gateway_client", return_value=good_client):
                body = await self._post({"session_id": str(session.id), "max_cards": 5})

        assert body["created_count"] == 5
        assert db.add.call_count == 5


# ── GET /v1/flashcards ────────────────────────────────────────────────────────


class TestListFlashcards:
    """Tests for GET /v1/flashcards."""

    def setup_method(self):
        _override_auth()

    def teardown_method(self):
        _clear_overrides()

    @pytest.mark.asyncio
    async def test_returns_all_cards_when_due_false(self):
        """Without the ``due=true`` filter every card comes back, future ones included."""
        past = datetime.now(timezone.utc) - timedelta(days=2)
        future = datetime.now(timezone.utc) + timedelta(days=3)
        cards = [_make_card(due_at=past), _make_card(due_at=future)]

        db = MagicMock()
        list_res = MagicMock()
        list_res.scalars.return_value.all.return_value = cards
        count_res = MagicMock()
        count_res.scalar_one.return_value = 2
        db.execute = AsyncMock(side_effect=[list_res, count_res])
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
            resp = await ac.get("/v1/flashcards?due=false")
        assert resp.status_code == 200
        body = resp.json()
        assert body["total"] == 2
        assert len(body["items"]) == 2

    @pytest.mark.asyncio
    async def test_filters_to_due_cards_only(self):
        """``due=true`` returns only cards whose due_at is in the past/now."""
        past = datetime.now(timezone.utc) - timedelta(days=1)
        cards = [_make_card(due_at=past)]

        db = MagicMock()
        list_res = MagicMock()
        list_res.scalars.return_value.all.return_value = cards
        count_res = MagicMock()
        count_res.scalar_one.return_value = 1
        db.execute = AsyncMock(side_effect=[list_res, count_res])
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
            resp = await ac.get("/v1/flashcards?due=true")
        assert resp.status_code == 200
        body = resp.json()
        assert body["total"] == 1
        assert body["items"][0]["front"] == "What is a mole?"

    @pytest.mark.asyncio
    async def test_empty_deck_returns_zero(self):
        """A student with no cards sees an empty deck, not an error."""
        db = MagicMock()
        list_res = MagicMock()
        list_res.scalars.return_value.all.return_value = []
        count_res = MagicMock()
        count_res.scalar_one.return_value = 0
        db.execute = AsyncMock(side_effect=[list_res, count_res])
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
            resp = await ac.get("/v1/flashcards")
        assert resp.status_code == 200
        assert resp.json() == {"items": [], "total": 0}


# ── POST /v1/flashcards/{id}/review ──────────────────────────────────────────


class TestReviewFlashcard:
    """Tests for POST /v1/flashcards/{id}/review."""

    def setup_method(self):
        _override_auth()

    def teardown_method(self):
        _clear_overrides()

    @pytest.mark.asyncio
    async def test_404_when_card_missing(self):
        """An unknown card ID returns 404 without touching SM-2 state."""
        db = MagicMock()
        res = MagicMock()
        res.scalar_one_or_none.return_value = None
        db.execute = AsyncMock(return_value=res)
        db.commit = AsyncMock()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
            resp = await ac.post(f"/v1/flashcards/{uuid4()}/review", json={"quality": 5})
        assert resp.status_code == 404
        db.commit.assert_not_awaited()

    @pytest.mark.asyncio
    async def test_404_when_card_owned_by_other_user(self):
        """A card belonging to a different student returns 404 (no leak)."""
        card = _make_card(user_id=OTHER_STUDENT_ID)
        db = MagicMock()
        res = MagicMock()
        res.scalar_one_or_none.return_value = card
        db.execute = AsyncMock(return_value=res)
        db.commit = AsyncMock()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
            resp = await ac.post(f"/v1/flashcards/{card.id}/review", json={"quality": 5})
        assert resp.status_code == 404
        db.commit.assert_not_awaited()

    @pytest.mark.asyncio
    async def test_quality_5_advances_interval(self):
        """quality=5 on a fresh card → repetitions=1, interval=1d."""
        card = _make_card()
        db = MagicMock()
        res = MagicMock()
        res.scalar_one_or_none.return_value = card
        db.execute = AsyncMock(return_value=res)
        db.commit = AsyncMock()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
            resp = await ac.post(f"/v1/flashcards/{card.id}/review", json={"quality": 5})
        assert resp.status_code == 200
        body = resp.json()
        assert body["quality"] == 5
        # INITIAL_INTERVALS[0] == 1 day after first successful review.
        assert body["sm2_interval_days"] == 1
        assert body["sm2_repetitions"] == 1
        assert body["sm2_ease_factor"] >= 2.5  # quality=5 nudges EF up
        # due_at pushed into the future.
        due = datetime.fromisoformat(body["due_at"])
        assert due > datetime.now(timezone.utc)
        db.commit.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_quality_1_resets_repetitions(self):
        """quality=1 (fail) resets repetitions and schedules for tomorrow."""
        card = _make_card()
        card.sm2_repetitions = 4
        card.sm2_interval_days = 30
        card.sm2_ease_factor = 2.7
        db = MagicMock()
        res = MagicMock()
        res.scalar_one_or_none.return_value = card
        db.execute = AsyncMock(return_value=res)
        db.commit = AsyncMock()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
            resp = await ac.post(f"/v1/flashcards/{card.id}/review", json={"quality": 1})
        assert resp.status_code == 200
        body = resp.json()
        assert body["sm2_repetitions"] == 0
        assert body["sm2_interval_days"] == 1

    @pytest.mark.asyncio
    async def test_quality_out_of_range_422(self):
        """Pydantic rejects quality outside 0..5 before reaching the handler."""
        db = MagicMock()
        db.execute = AsyncMock()
        db.commit = AsyncMock()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
            resp = await ac.post(f"/v1/flashcards/{uuid4()}/review", json={"quality": 9})
        assert resp.status_code == 422
        db.execute.assert_not_awaited()


# ── GET /v1/flashcards/export ────────────────────────────────────────────────


class TestExportFlashcards:
    """Tests for GET /v1/flashcards/export."""

    def setup_method(self):
        _override_auth()

    def teardown_method(self):
        _clear_overrides()

    def _install_rows(self, rows):
        db = MagicMock()
        res = MagicMock()
        res.scalars.return_value.all.return_value = rows
        db.execute = AsyncMock(return_value=res)
        _override_db(db)
        return db

    @pytest.mark.asyncio
    async def test_anki_tsv_has_header_and_rows(self):
        """Anki format emits a tab-separated header plus one row per card."""
        cards = [
            _make_card(front="Q1", back="A1", concept_id="math.algebra"),
            _make_card(front="Q2", back="A2", concept_id=None),
        ]
        self._install_rows(cards)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
            resp = await ac.get("/v1/flashcards/export?format=anki")
        assert resp.status_code == 200
        assert "text/tab-separated-values" in resp.headers["content-type"]
        assert (
            'attachment; filename="teacherslounge-flashcards.tsv"'
            in (resp.headers["content-disposition"])
        )

        lines = resp.text.strip().split("\n")
        assert lines[0] == "front\tback\ttags"
        reader = csv.reader(io.StringIO(resp.text), delimiter="\t")
        rows = list(reader)
        assert rows[1] == ["Q1", "A1", "math.algebra"]
        assert rows[2] == ["Q2", "A2", ""]

    @pytest.mark.asyncio
    async def test_csv_format_has_comma_delimiter(self):
        """``format=csv`` emits the same columns with a comma delimiter + CSV mime."""
        cards = [_make_card(front="Q", back="A", concept_id="x")]
        self._install_rows(cards)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
            resp = await ac.get("/v1/flashcards/export?format=csv")
        assert resp.status_code == 200
        assert "text/csv" in resp.headers["content-type"]
        reader = csv.reader(io.StringIO(resp.text))
        rows = list(reader)
        assert rows[0] == ["front", "back", "tags"]
        assert rows[1] == ["Q", "A", "x"]

    @pytest.mark.asyncio
    async def test_rejects_unknown_format(self):
        """Query validator rejects formats that aren't anki or csv."""
        # No DB touch expected — the validator rejects first.
        db = MagicMock()
        db.execute = AsyncMock()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
            resp = await ac.get("/v1/flashcards/export?format=xml")
        assert resp.status_code == 422
        db.execute.assert_not_awaited()

    @pytest.mark.asyncio
    async def test_empty_deck_exports_header_only(self):
        """A student with no cards still gets a well-formed header row."""
        self._install_rows([])

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
            resp = await ac.get("/v1/flashcards/export")
        assert resp.status_code == 200
        assert resp.text.strip() == "front\tback\ttags"
