"""Unit tests for app.knowledge_model.

Covers all public functions using AsyncMock DB sessions — no real Postgres needed.

Functions under test:
  - get_or_create_learning_profile
  - update_learning_profile_dials
  - get_dials
  - log_explanation_preference
  - get_explanation_preferences
  - log_misconception
  - get_active_misconceptions
  - resolve_misconception
  - get_due_review_prompt
"""
from __future__ import annotations

from datetime import datetime, timedelta, timezone
from unittest.mock import AsyncMock, MagicMock, patch
from uuid import UUID, uuid4

import pytest

from app.knowledge_model import (
    get_active_misconceptions,
    get_dials,
    get_due_review_prompt,
    get_explanation_preferences,
    get_or_create_learning_profile,
    log_explanation_preference,
    log_misconception,
    resolve_misconception,
    update_learning_profile_dials,
)
from app.style_detector import DEFAULT_DIALS

# ── Constants ─────────────────────────────────────────────────────────────────

USER_ID = uuid4()
CONCEPT_ID = uuid4()
NOW = datetime(2026, 4, 5, 12, 0, 0, tzinfo=timezone.utc)


# ── Helpers ───────────────────────────────────────────────────────────────────

def _make_profile(
    user_id: UUID | None = None,
    active_reflective: float = 0.0,
    sensing_intuitive: float = 0.0,
    visual_verbal: float = 0.0,
    sequential_global: float = 0.0,
) -> MagicMock:
    """Return a MagicMock mimicking a LearningProfile ORM row."""
    row = MagicMock()
    row.user_id = user_id or USER_ID
    row.active_reflective = active_reflective
    row.sensing_intuitive = sensing_intuitive
    row.visual_verbal = visual_verbal
    row.sequential_global = sequential_global
    row.updated_at = NOW
    return row


def _make_misconception(
    concept_id: UUID | None = None,
    description: str = "Confuses X with Y",
    confidence: float = 1.0,
    recorded_at: datetime | None = None,
    resolved: bool = False,
) -> MagicMock:
    """Return a MagicMock mimicking a Misconception ORM row."""
    row = MagicMock()
    row.id = uuid4()
    row.user_id = USER_ID
    row.concept_id = concept_id or CONCEPT_ID
    row.description = description
    row.confidence = confidence
    row.recorded_at = recorded_at or NOW
    row.last_seen_at = recorded_at or NOW
    row.resolved = resolved
    return row


def _make_mastery_row(
    concept_id: UUID | None = None,
    next_review_at: datetime | None = None,
) -> MagicMock:
    """Return a MagicMock mimicking a StudentConceptMastery ORM row."""
    row = MagicMock()
    row.concept_id = concept_id or CONCEPT_ID
    row.next_review_at = next_review_at
    row.concept = MagicMock()
    row.concept.name = "Integration by Parts"
    return row


# ── get_or_create_learning_profile ───────────────────────────────────────────

class TestGetOrCreateLearningProfile:
    @pytest.mark.asyncio
    async def test_returns_existing_profile(self):
        """Returns the existing row when one is found."""
        db = AsyncMock()
        profile = _make_profile()
        result = MagicMock()
        result.scalar_one_or_none.return_value = profile
        db.execute = AsyncMock(return_value=result)

        out = await get_or_create_learning_profile(db, USER_ID)

        assert out is profile
        db.add.assert_not_called()

    @pytest.mark.asyncio
    async def test_creates_new_profile_when_missing(self):
        """Creates and flushes a new LearningProfile when none exists."""
        db = AsyncMock()
        db.add = MagicMock()
        db.flush = AsyncMock()
        result = MagicMock()
        result.scalar_one_or_none.return_value = None
        db.execute = AsyncMock(return_value=result)

        out = await get_or_create_learning_profile(db, USER_ID)

        db.add.assert_called_once()
        db.flush.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_new_profile_has_zero_dials(self):
        """A freshly created profile has all dials at 0.0."""
        db = AsyncMock()
        db.add = MagicMock()
        db.flush = AsyncMock()
        result = MagicMock()
        result.scalar_one_or_none.return_value = None
        db.execute = AsyncMock(return_value=result)

        # Capture what was passed to db.add
        added = []
        db.add.side_effect = added.append

        await get_or_create_learning_profile(db, USER_ID)

        assert len(added) == 1
        created = added[0]
        assert created.active_reflective == 0.0
        assert created.visual_verbal == 0.0


# ── update_learning_profile_dials ─────────────────────────────────────────────

class TestUpdateLearningProfileDials:
    @pytest.mark.asyncio
    async def test_updates_all_four_dials(self):
        """All four Felder-Silverman dials are written to the ORM row."""
        db = AsyncMock()
        db.commit = AsyncMock()
        profile = _make_profile()
        result = MagicMock()
        result.scalar_one_or_none.return_value = profile
        db.execute = AsyncMock(return_value=result)

        new_dials = {
            "active_reflective": -0.3,
            "sensing_intuitive": 0.5,
            "visual_verbal": -0.7,
            "sequential_global": 0.2,
        }
        out = await update_learning_profile_dials(db, USER_ID, new_dials)

        assert out.active_reflective == -0.3
        assert out.sensing_intuitive == 0.5
        assert out.visual_verbal == -0.7
        assert out.sequential_global == 0.2
        db.commit.assert_not_awaited()  # helper is commit-free; caller owns the transaction

    @pytest.mark.asyncio
    async def test_partial_dials_only_updates_provided_keys(self):
        """Only dials present in the dict are updated; others keep their value."""
        db = AsyncMock()
        db.commit = AsyncMock()
        profile = _make_profile(visual_verbal=0.4)
        result = MagicMock()
        result.scalar_one_or_none.return_value = profile
        db.execute = AsyncMock(return_value=result)

        out = await update_learning_profile_dials(db, USER_ID, {"active_reflective": 0.6})

        assert out.active_reflective == 0.6
        assert out.visual_verbal == 0.4  # unchanged

    @pytest.mark.asyncio
    async def test_creates_profile_if_missing(self):
        """Creates a new profile if none exists, then applies the dials."""
        db = AsyncMock()
        db.add = MagicMock()
        db.flush = AsyncMock()
        db.commit = AsyncMock()
        result = MagicMock()
        result.scalar_one_or_none.return_value = None
        db.execute = AsyncMock(return_value=result)

        added = []
        db.add.side_effect = added.append

        await update_learning_profile_dials(db, USER_ID, {"visual_verbal": -0.5})

        assert len(added) == 1


# ── get_dials ─────────────────────────────────────────────────────────────────

class TestGetDials:
    @pytest.mark.asyncio
    async def test_returns_dict_with_four_dimensions(self):
        """Returns a dict with all four Felder-Silverman dimension keys."""
        db = AsyncMock()
        profile = _make_profile(visual_verbal=-0.5, sequential_global=0.3)
        result = MagicMock()
        result.scalar_one_or_none.return_value = profile
        db.execute = AsyncMock(return_value=result)

        dials = await get_dials(db, USER_ID)

        assert set(dials.keys()) == {
            "active_reflective",
            "sensing_intuitive",
            "visual_verbal",
            "sequential_global",
        }
        assert dials["visual_verbal"] == -0.5
        assert dials["sequential_global"] == 0.3

    @pytest.mark.asyncio
    async def test_returns_default_dials_when_no_profile(self):
        """Returns DEFAULT_DIALS (all zero) when no profile row exists."""
        db = AsyncMock()
        result = MagicMock()
        result.scalar_one_or_none.return_value = None
        db.execute = AsyncMock(return_value=result)

        dials = await get_dials(db, USER_ID)

        assert dials == dict(DEFAULT_DIALS)


# ── log_explanation_preference ────────────────────────────────────────────────

class TestLogExplanationPreference:
    @pytest.mark.asyncio
    async def test_adds_row_without_commit(self):
        """Adds an ExplanationPreference row but does NOT commit (caller's responsibility)."""
        db = AsyncMock()
        db.add = MagicMock()
        db.commit = AsyncMock()

        added = []
        db.add.side_effect = added.append

        result = await log_explanation_preference(
            db, USER_ID, CONCEPT_ID, "visual", helpful=True
        )

        db.add.assert_called_once()
        db.commit.assert_not_awaited()
        pref = added[0]
        assert pref.user_id == USER_ID
        assert pref.concept_id == CONCEPT_ID
        assert pref.explanation_type == "visual"
        assert pref.helpful is True

    @pytest.mark.asyncio
    async def test_logs_unhelpful_preference(self):
        """Can log unhelpful explanation preferences (helpful=False)."""
        db = AsyncMock()
        db.add = MagicMock()
        db.commit = AsyncMock()

        added = []
        db.add.side_effect = added.append

        await log_explanation_preference(
            db, USER_ID, CONCEPT_ID, "derivation", helpful=False
        )

        assert added[0].helpful is False


# ── get_explanation_preferences ───────────────────────────────────────────────

class TestGetExplanationPreferences:
    @pytest.mark.asyncio
    async def test_returns_list_of_preference_dicts(self):
        """Returns a list of preference dicts for the student+concept."""
        db = AsyncMock()
        pref = MagicMock()
        pref.explanation_type = "visual"
        pref.helpful = True
        pref.recorded_at = NOW
        result = MagicMock()
        result.scalars.return_value.all.return_value = [pref]
        db.execute = AsyncMock(return_value=result)

        prefs = await get_explanation_preferences(db, USER_ID, CONCEPT_ID)

        assert len(prefs) == 1
        assert prefs[0]["explanation_type"] == "visual"
        assert prefs[0]["helpful"] is True

    @pytest.mark.asyncio
    async def test_returns_empty_list_when_none(self):
        """Returns empty list when no preferences exist."""
        db = AsyncMock()
        result = MagicMock()
        result.scalars.return_value.all.return_value = []
        db.execute = AsyncMock(return_value=result)

        prefs = await get_explanation_preferences(db, USER_ID, CONCEPT_ID)

        assert prefs == []


# ── log_misconception ─────────────────────────────────────────────────────────

class TestLogMisconception:
    def _build_db_no_existing(self) -> AsyncMock:
        """DB mock where no existing misconception is found (insert path)."""
        db = AsyncMock()
        db.add = MagicMock()
        db.commit = AsyncMock()
        no_existing = MagicMock()
        no_existing.scalar_one_or_none.return_value = None
        db.execute = AsyncMock(return_value=no_existing)
        return db

    @pytest.mark.asyncio
    async def test_adds_new_misconception_row_without_commit(self):
        """Creates a Misconception ORM row with correct fields; does NOT commit."""
        db = self._build_db_no_existing()

        added = []
        db.add.side_effect = added.append

        await log_misconception(db, USER_ID, CONCEPT_ID, "Confuses div with grad")

        db.add.assert_called_once()
        db.commit.assert_not_awaited()
        m = added[0]
        assert m.user_id == USER_ID
        assert m.concept_id == CONCEPT_ID
        assert m.description == "Confuses div with grad"
        assert m.resolved is False

    @pytest.mark.asyncio
    async def test_new_misconception_has_full_confidence(self):
        """A freshly logged misconception starts with confidence=1.0."""
        db = self._build_db_no_existing()

        added = []
        db.add.side_effect = added.append

        await log_misconception(db, USER_ID, CONCEPT_ID, "wrong sign")

        assert added[0].confidence == 1.0

    @pytest.mark.asyncio
    async def test_upserts_existing_misconception(self):
        """When the same description already exists unresolved, refreshes last_seen_at."""
        db = AsyncMock()
        db.add = MagicMock()
        db.commit = AsyncMock()
        existing = _make_misconception(description="wrong sign")
        existing.resolved = False
        found_result = MagicMock()
        found_result.scalar_one_or_none.return_value = existing
        db.execute = AsyncMock(return_value=found_result)

        m = await log_misconception(db, USER_ID, CONCEPT_ID, "wrong sign")

        # Should update existing row, NOT add a new one
        db.add.assert_not_called()
        assert m is existing
        assert m.confidence == 1.0


# ── get_active_misconceptions ─────────────────────────────────────────────────

class TestGetActiveMisconceptions:
    @pytest.mark.asyncio
    async def test_returns_list_with_recency_weight(self):
        """Each misconception entry includes a recency_weight field."""
        db = AsyncMock()
        m = _make_misconception(recorded_at=NOW - timedelta(days=5))
        result = MagicMock()
        result.scalars.return_value.all.return_value = [m]
        db.execute = AsyncMock(return_value=result)

        entries = await get_active_misconceptions(db, USER_ID)

        assert len(entries) == 1
        assert "recency_weight" in entries[0]
        assert 0.0 < entries[0]["recency_weight"] <= 1.0

    @pytest.mark.asyncio
    async def test_recent_misconception_has_higher_weight_than_old(self):
        """A misconception from today has higher recency_weight than one from 30 days ago."""
        db = AsyncMock()
        recent = _make_misconception(recorded_at=NOW - timedelta(days=1))
        old = _make_misconception(recorded_at=NOW - timedelta(days=30))
        result = MagicMock()
        result.scalars.return_value.all.return_value = [recent, old]
        db.execute = AsyncMock(return_value=result)

        entries = await get_active_misconceptions(db, USER_ID)

        weights = {e["description"]: e["recency_weight"] for e in entries}
        assert weights[recent.description] > weights[old.description]

    @pytest.mark.asyncio
    async def test_empty_list_when_no_misconceptions(self):
        """Returns empty list when the student has no active misconceptions."""
        db = AsyncMock()
        result = MagicMock()
        result.scalars.return_value.all.return_value = []
        db.execute = AsyncMock(return_value=result)

        entries = await get_active_misconceptions(db, USER_ID)

        assert entries == []

    @pytest.mark.asyncio
    async def test_resolved_misconceptions_excluded(self):
        """A resolved misconception is filtered out even if the mock returns it.

        get_active_misconceptions applies a Python-level resolved guard in
        addition to the SQL WHERE clause, so this verifies the belt-and-suspenders
        filter directly.
        """
        db = AsyncMock()
        resolved_m = _make_misconception(recorded_at=NOW)
        resolved_m.resolved = True  # mark as resolved
        result = MagicMock()
        result.scalars.return_value.all.return_value = [resolved_m]
        db.execute = AsyncMock(return_value=result)

        entries = await get_active_misconceptions(db, USER_ID)

        # Python-level guard must exclude the resolved misconception
        assert entries == []


# ── resolve_misconception ─────────────────────────────────────────────────────

class TestResolveMisconception:
    @pytest.mark.asyncio
    async def test_sets_resolved_true_without_commit(self):
        """Marks the misconception resolved; does NOT commit (caller's responsibility)."""
        db = AsyncMock()
        db.commit = AsyncMock()
        misc_id = uuid4()
        m = _make_misconception()
        m.id = misc_id
        result = MagicMock()
        result.scalar_one_or_none.return_value = m
        db.execute = AsyncMock(return_value=result)

        ok = await resolve_misconception(db, misc_id, USER_ID)

        assert ok is True
        assert m.resolved is True
        db.commit.assert_not_awaited()

    @pytest.mark.asyncio
    async def test_returns_false_when_not_found(self):
        """Returns False if the misconception doesn't exist."""
        db = AsyncMock()
        result = MagicMock()
        result.scalar_one_or_none.return_value = None
        db.execute = AsyncMock(return_value=result)

        ok = await resolve_misconception(db, uuid4(), USER_ID)

        assert ok is False
        db.commit.assert_not_awaited()


# ── get_due_review_prompt ─────────────────────────────────────────────────────

class TestGetDueReviewPrompt:
    @pytest.mark.asyncio
    async def test_returns_none_when_nothing_due(self):
        """Returns None when no concepts are due for review."""
        db = AsyncMock()
        result = MagicMock()
        result.scalars.return_value.all.return_value = []
        db.execute = AsyncMock(return_value=result)

        prompt = await get_due_review_prompt(db, USER_ID)

        assert prompt is None

    @pytest.mark.asyncio
    async def test_returns_string_with_concept_name_when_due(self):
        """Returns a non-empty string mentioning the overdue concept."""
        db = AsyncMock()
        overdue_row = _make_mastery_row(next_review_at=NOW - timedelta(days=2))
        result = MagicMock()
        result.scalars.return_value.all.return_value = [overdue_row]
        db.execute = AsyncMock(return_value=result)

        prompt = await get_due_review_prompt(db, USER_ID)

        assert prompt is not None
        assert isinstance(prompt, str)
        assert len(prompt) > 0
        assert "Integration by Parts" in prompt

    @pytest.mark.asyncio
    async def test_respects_limit_parameter(self):
        """The prompt mentions at most `limit` concepts.

        The mock returns all 5 rows (ignoring SQL LIMIT), so this test verifies
        the Python-level [:limit] slice in get_due_review_prompt.
        """
        db = AsyncMock()
        rows = [
            _make_mastery_row(concept_id=uuid4(), next_review_at=NOW - timedelta(days=i + 1))
            for i in range(5)
        ]
        for i, r in enumerate(rows):
            r.concept.name = f"Concept {i}"

        result = MagicMock()
        result.scalars.return_value.all.return_value = rows
        db.execute = AsyncMock(return_value=result)

        prompt = await get_due_review_prompt(db, USER_ID, limit=2)

        assert prompt is not None
        # Python [:2] slice means exactly 2 concept names appear in the prompt
        mentioned = [f"Concept {i}" in prompt for i in range(5)]
        assert sum(mentioned) == 2
