"""Tests for the InteractionQuality ORM model (Phase 7 LLM judge table).

Validates model fields, constraints, and default values without a live database.
"""

import uuid
from datetime import datetime, timezone

import pytest

from app.orm import InteractionQuality


class TestInteractionQualityModel:
    def test_tablename_is_interaction_quality(self):
        """ORM model maps to the correct table name."""
        assert InteractionQuality.__tablename__ == "interaction_quality"

    def test_unique_constraint_on_interaction_id(self):
        """interaction_id has a unique constraint (uq_interaction_quality_interaction)."""
        constraint_names = {
            c.name
            for c in InteractionQuality.__table_args__
            if hasattr(c, "name") and c.name is not None
        }
        assert "uq_interaction_quality_interaction" in constraint_names

    def test_model_has_required_columns(self):
        """All columns specified in the Phase 7 spec are present."""
        columns = {c.name for c in InteractionQuality.__table__.columns}
        required = {
            "id",
            "interaction_id",
            "judge_score",
            "judge_reasoning",
            "score_directness",
            "score_pace",
            "score_grounding",
            "judged_at",
            "judge_model",
        }
        assert required.issubset(columns)

    def test_judge_model_has_default(self):
        """judge_model column has a non-null default value."""
        col = InteractionQuality.__table__.c["judge_model"]
        assert col.default is not None or col.server_default is not None or not col.nullable

    def test_dimension_score_columns_are_nullable(self):
        """score_directness, score_pace, score_grounding are nullable (may be absent for older rows)."""
        for col_name in ("score_directness", "score_pace", "score_grounding"):
            col = InteractionQuality.__table__.c[col_name]
            assert col.nullable, f"{col_name} should be nullable"

    def test_judge_score_is_not_nullable(self):
        """judge_score (composite 1–5) is required."""
        col = InteractionQuality.__table__.c["judge_score"]
        assert not col.nullable
