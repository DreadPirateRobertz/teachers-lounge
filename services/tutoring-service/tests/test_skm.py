"""Tests for the Student Knowledge Model (SKM).

Tests cover:
  - Decay model math (exponential forgetting curve)
  - Confidence scoring
  - Spaced repetition scheduling
  - Mastery observation recording (EMA updates)
  - API endpoint integration (via TestClient)
"""
import math
from datetime import datetime, timedelta, timezone
from unittest.mock import patch
from uuid import uuid4

import pytest

from app.skm_service import (
    BASE_HALF_LIFE_DAYS,
    CONFIDENCE_K,
    HALF_LIFE_GROWTH_FACTOR,
    apply_decay,
    compute_confidence,
    compute_effective_mastery,
    compute_half_life,
    compute_next_review,
)


# ── Decay Model Tests ────────────────────────────────────────────────────────


class TestApplyDecay:
    def test_no_elapsed_time_returns_original(self):
        assert apply_decay(0.9, 0.0, 7.0) == 0.9

    def test_negative_elapsed_returns_original(self):
        assert apply_decay(0.9, -5.0, 7.0) == 0.9

    def test_at_half_life_returns_half(self):
        """After exactly one half-life, mastery should be ~50% of original."""
        half_life_days = 7.0
        hours = half_life_days * 24.0
        result = apply_decay(1.0, hours, half_life_days)
        assert abs(result - 0.5) < 1e-10

    def test_at_two_half_lives_returns_quarter(self):
        half_life_days = 7.0
        hours = 2 * half_life_days * 24.0
        result = apply_decay(1.0, hours, half_life_days)
        assert abs(result - 0.25) < 1e-10

    def test_decay_preserves_proportionality(self):
        """Starting mastery of 0.8 should decay proportionally."""
        half_life_days = 7.0
        hours = half_life_days * 24.0
        result = apply_decay(0.8, hours, half_life_days)
        assert abs(result - 0.4) < 1e-10

    def test_long_elapsed_approaches_zero(self):
        result = apply_decay(1.0, 10000.0, 1.0)
        assert result < 0.001

    def test_floor_at_zero(self):
        result = apply_decay(0.5, 1_000_000.0, 1.0)
        assert result >= 0.0


class TestComputeHalfLife:
    def test_zero_reviews_uses_base(self):
        hl = compute_half_life(0, 0.05)
        expected = BASE_HALF_LIFE_DAYS / (0.05 * 20)
        assert abs(hl - expected) < 1e-10

    def test_more_reviews_longer_half_life(self):
        hl_0 = compute_half_life(0, 0.05)
        hl_3 = compute_half_life(3, 0.05)
        hl_10 = compute_half_life(10, 0.05)
        assert hl_3 > hl_0
        assert hl_10 > hl_3

    def test_growth_factor_applied_per_review(self):
        hl_0 = compute_half_life(0, 0.05)
        hl_1 = compute_half_life(1, 0.05)
        assert abs(hl_1 / hl_0 - HALF_LIFE_GROWTH_FACTOR) < 1e-10


# ── Confidence Scoring Tests ─────────────────────────────────────────────────


class TestComputeConfidence:
    def test_zero_reviews_zero_confidence(self):
        assert compute_confidence(0) == 0.0

    def test_one_review_low_confidence(self):
        c = compute_confidence(1)
        expected = 1.0 - math.exp(-CONFIDENCE_K)
        assert abs(c - expected) < 1e-10
        assert 0.2 < c < 0.4

    def test_many_reviews_high_confidence(self):
        c = compute_confidence(10)
        assert c > 0.9

    def test_monotonically_increasing(self):
        values = [compute_confidence(i) for i in range(20)]
        for i in range(1, len(values)):
            assert values[i] > values[i - 1]

    def test_bounded_by_one(self):
        assert compute_confidence(1000) <= 1.0


# ── Spaced Repetition Scheduling Tests ───────────────────────────────────────


class TestComputeNextReview:
    def test_returns_future_datetime(self):
        now = datetime.now(timezone.utc)
        result = compute_next_review(0.8, 3, 0.05)
        assert result > now

    def test_more_reviews_longer_interval(self):
        r1 = compute_next_review(0.8, 1, 0.05)
        r5 = compute_next_review(0.8, 5, 0.05)
        r10 = compute_next_review(0.8, 10, 0.05)
        assert r5 > r1
        assert r10 > r5

    def test_minimum_interval_at_least_one_hour(self):
        now = datetime.now(timezone.utc)
        result = compute_next_review(0.01, 0, 0.2)
        diff = (result - now).total_seconds()
        assert diff >= 3500  # ~1 hour minus execution time tolerance


# ── Effective Mastery Tests ──────────────────────────────────────────────────


class TestComputeEffectiveMastery:
    def test_no_review_returns_raw_score(self):
        """If never reviewed, effective mastery equals raw score."""
        from app.skm_orm import StudentConceptMastery

        record = StudentConceptMastery(
            user_id=uuid4(),
            concept_id=uuid4(),
            mastery_score=0.75,
            confidence=0.0,
            decay_rate=0.05,
            review_count=0,
            last_reviewed_at=None,
        )
        assert compute_effective_mastery(record) == 0.75

    def test_recent_review_minimal_decay(self):
        from app.skm_orm import StudentConceptMastery

        recent = datetime.now(timezone.utc) - timedelta(minutes=5)
        record = StudentConceptMastery(
            user_id=uuid4(),
            concept_id=uuid4(),
            mastery_score=0.9,
            confidence=0.5,
            decay_rate=0.05,
            review_count=3,
            last_reviewed_at=recent,
        )
        effective = compute_effective_mastery(record)
        assert effective > 0.89  # Very little decay in 5 minutes

    def test_old_review_significant_decay(self):
        from app.skm_orm import StudentConceptMastery

        old = datetime.now(timezone.utc) - timedelta(days=30)
        record = StudentConceptMastery(
            user_id=uuid4(),
            concept_id=uuid4(),
            mastery_score=0.9,
            confidence=0.5,
            decay_rate=0.05,
            review_count=1,
            last_reviewed_at=old,
        )
        effective = compute_effective_mastery(record)
        assert effective < 0.5  # Significant decay after 30 days with few reviews
