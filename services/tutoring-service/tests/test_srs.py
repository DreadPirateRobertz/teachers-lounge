"""Unit tests for the SM-2 spaced repetition engine (pure functions)."""
import math
from datetime import datetime, timezone

import pytest

from app.srs import (
    DEFAULT_EASE_FACTOR,
    MIN_EASE_FACTOR,
    mastery_after_review,
    mastery_from_retention,
    next_review_time,
    retention,
    sm2_update,
)


# ── sm2_update ────────────────────────────────────────────────────────────────

class TestSm2Update:
    def test_first_successful_review_interval_1(self):
        interval, ef, reps = sm2_update(quality=5, ease_factor=2.5, interval_days=1, repetitions=0)
        assert interval == 1
        assert reps == 1
        assert ef > MIN_EASE_FACTOR

    def test_second_successful_review_interval_6(self):
        interval, ef, reps = sm2_update(quality=5, ease_factor=2.5, interval_days=1, repetitions=1)
        assert interval == 6
        assert reps == 2

    def test_third_review_multiplies_by_ef(self):
        interval, ef, reps = sm2_update(quality=5, ease_factor=2.5, interval_days=6, repetitions=2)
        assert interval == round(6 * 2.5)
        assert reps == 3

    def test_failed_review_resets_repetitions(self):
        interval, ef, reps = sm2_update(quality=2, ease_factor=2.5, interval_days=10, repetitions=3)
        assert interval == 1
        assert reps == 0

    def test_quality_3_minimum_pass(self):
        interval, ef, reps = sm2_update(quality=3, ease_factor=2.5, interval_days=1, repetitions=0)
        assert interval == 1
        assert reps == 1

    def test_ef_decreases_with_lower_quality(self):
        _, ef_high, _ = sm2_update(quality=5, ease_factor=2.5, interval_days=1, repetitions=0)
        _, ef_low, _ = sm2_update(quality=3, ease_factor=2.5, interval_days=1, repetitions=0)
        assert ef_high > ef_low

    def test_ef_never_below_minimum(self):
        # Repeatedly score 3 should not drop EF below MIN
        ef = DEFAULT_EASE_FACTOR
        interval, reps = 1, 0
        for _ in range(20):
            interval, ef, reps = sm2_update(quality=3, ease_factor=ef, interval_days=interval, repetitions=reps)
        assert ef >= MIN_EASE_FACTOR

    def test_invalid_quality_raises(self):
        with pytest.raises(ValueError):
            sm2_update(quality=6, ease_factor=2.5, interval_days=1, repetitions=0)
        with pytest.raises(ValueError):
            sm2_update(quality=-1, ease_factor=2.5, interval_days=1, repetitions=0)

    def test_perfect_score_increases_ef(self):
        _, ef, _ = sm2_update(quality=5, ease_factor=2.5, interval_days=1, repetitions=0)
        assert ef > 2.5

    def test_failed_review_preserves_ef(self):
        _, ef, _ = sm2_update(quality=0, ease_factor=2.5, interval_days=10, repetitions=5)
        assert ef == 2.5  # EF unchanged on failure


# ── next_review_time ──────────────────────────────────────────────────────────

class TestNextReviewTime:
    def test_returns_future_datetime(self):
        now = datetime(2026, 1, 1, 0, 0, 0, tzinfo=timezone.utc)
        result = next_review_time(interval_days=3, now=now)
        assert result.year == 2026
        assert (result - now).days == 3

    def test_interval_1_is_tomorrow(self):
        now = datetime(2026, 6, 15, 12, 0, 0, tzinfo=timezone.utc)
        result = next_review_time(interval_days=1, now=now)
        assert (result - now).total_seconds() == 86400


# ── retention ────────────────────────────────────────────────────────────────

class TestRetention:
    def test_zero_elapsed_is_full_retention(self):
        assert retention(elapsed_days=0, stability=10) == 1.0

    def test_negative_elapsed_is_full_retention(self):
        assert retention(elapsed_days=-1, stability=10) == 1.0

    def test_retention_decays_over_time(self):
        r1 = retention(elapsed_days=1, stability=10)
        r2 = retention(elapsed_days=5, stability=10)
        assert r1 > r2

    def test_higher_stability_means_slower_decay(self):
        r_low = retention(elapsed_days=5, stability=5)
        r_high = retention(elapsed_days=5, stability=20)
        assert r_high > r_low

    def test_returns_float_between_0_and_1(self):
        r = retention(elapsed_days=100, stability=1)
        assert 0.0 <= r <= 1.0


# ── mastery_from_retention ───────────────────────────────────────────────────

class TestMasteryFromRetention:
    def test_no_elapsed_returns_base(self):
        result = mastery_from_retention(base_mastery=0.8, elapsed_days=0)
        assert result == pytest.approx(0.8)

    def test_decay_reduces_mastery(self):
        result = mastery_from_retention(base_mastery=1.0, elapsed_days=10, decay_rate=0.1)
        assert result < 1.0
        assert result == pytest.approx(math.exp(-1.0), rel=1e-5)

    def test_clamped_to_zero(self):
        result = mastery_from_retention(base_mastery=0.1, elapsed_days=1000, decay_rate=1.0)
        assert result == 0.0


# ── mastery_after_review ─────────────────────────────────────────────────────

class TestMasteryAfterReview:
    def test_perfect_score_increases_mastery(self):
        assert mastery_after_review(0.5, quality=5) > 0.5

    def test_failure_decreases_mastery(self):
        assert mastery_after_review(0.5, quality=0) < 0.5

    def test_mastery_clamped_to_1(self):
        assert mastery_after_review(0.95, quality=5) <= 1.0

    def test_mastery_clamped_to_0(self):
        assert mastery_after_review(0.05, quality=0) >= 0.0

    @pytest.mark.parametrize("q", [0, 1, 2, 3, 4, 5])
    def test_all_quality_values_return_valid_range(self, q):
        result = mastery_after_review(0.5, quality=q)
        assert 0.0 <= result <= 1.0
