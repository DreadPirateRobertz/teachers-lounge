"""Unit tests for classify_style in app.style_detector.

Covers:
  - Happy path: balanced mixed answers produce expected dimension scores
  - All-A answers: every dimension scores -1.0 (first-pole)
  - All-B answers: every dimension scores +1.0 (second-pole)
  - Mixed per dimension: fine-grained score verification
  - Case-insensitivity: lowercase 'a'/'b' accepted
  - Invalid input: empty list, wrong length, invalid keys
"""

import pytest

from app.style_detector import (
    DIMENSIONS,
    QUESTIONS_PER_DIMENSION,
    LearningStyle,
    classify_style,
)

# ── helpers ───────────────────────────────────────────────────────────────────

TOTAL = QUESTIONS_PER_DIMENSION * len(DIMENSIONS)  # 16 for default config


def _all(key: str) -> list[str]:
    """Return a full answer list where every answer is *key*."""
    return [key] * TOTAL


def _block(answers_per_dim: list[list[str]]) -> list[str]:
    """Flatten a list of per-dimension answer blocks into a single list."""
    result: list[str] = []
    for block in answers_per_dim:
        result.extend(block)
    return result


# ── all-A / all-B edge cases ──────────────────────────────────────────────────


class TestAllSamePole:
    def test_all_a_returns_negative_one_on_every_dimension(self):
        style = classify_style(_all("A"))
        assert style.active_reflective == -1.0
        assert style.sensing_intuitive == -1.0
        assert style.visual_verbal == -1.0
        assert style.sequential_global == -1.0

    def test_all_b_returns_positive_one_on_every_dimension(self):
        style = classify_style(_all("B"))
        assert style.active_reflective == 1.0
        assert style.sensing_intuitive == 1.0
        assert style.visual_verbal == 1.0
        assert style.sequential_global == 1.0

    def test_all_a_returns_learning_style_instance(self):
        assert isinstance(classify_style(_all("A")), LearningStyle)

    def test_all_b_returns_learning_style_instance(self):
        assert isinstance(classify_style(_all("B")), LearningStyle)


# ── happy path: mixed answers ─────────────────────────────────────────────────


class TestHappyPath:
    def test_balanced_answers_score_zero(self):
        """Equal A and B answers per dimension → score 0.0 for that dimension."""
        # 2 A + 2 B per dimension (assuming QUESTIONS_PER_DIMENSION=4)
        if QUESTIONS_PER_DIMENSION != 4:
            pytest.skip("test assumes 4 questions per dimension")
        answers = _block([["A", "B", "A", "B"]] * len(DIMENSIONS))
        style = classify_style(answers)
        assert style.active_reflective == 0.0
        assert style.sensing_intuitive == 0.0
        assert style.visual_verbal == 0.0
        assert style.sequential_global == 0.0

    def test_three_b_one_a_per_dimension_scores_half(self):
        """3 B + 1 A per dimension → (3-1)/4 = 0.5."""
        if QUESTIONS_PER_DIMENSION != 4:
            pytest.skip("test assumes 4 questions per dimension")
        answers = _block([["B", "B", "B", "A"]] * len(DIMENSIONS))
        style = classify_style(answers)
        assert style.active_reflective == pytest.approx(0.5)
        assert style.sensing_intuitive == pytest.approx(0.5)
        assert style.visual_verbal == pytest.approx(0.5)
        assert style.sequential_global == pytest.approx(0.5)

    def test_three_a_one_b_per_dimension_scores_negative_half(self):
        """3 A + 1 B per dimension → (1-3)/4 = -0.5."""
        if QUESTIONS_PER_DIMENSION != 4:
            pytest.skip("test assumes 4 questions per dimension")
        answers = _block([["A", "A", "A", "B"]] * len(DIMENSIONS))
        style = classify_style(answers)
        assert style.active_reflective == pytest.approx(-0.5)
        assert style.sensing_intuitive == pytest.approx(-0.5)
        assert style.visual_verbal == pytest.approx(-0.5)
        assert style.sequential_global == pytest.approx(-0.5)

    def test_independent_dimensions_scored_separately(self):
        """Each dimension block is evaluated independently."""
        if QUESTIONS_PER_DIMENSION != 4:
            pytest.skip("test assumes 4 questions per dimension")
        answers = _block(
            [
                ["A", "A", "A", "A"],  # active_reflective → -1.0
                ["B", "B", "B", "B"],  # sensing_intuitive → +1.0
                ["A", "B", "A", "B"],  # visual_verbal     →  0.0
                ["B", "B", "A", "A"],  # sequential_global →  0.0
            ]
        )
        style = classify_style(answers)
        assert style.active_reflective == pytest.approx(-1.0)
        assert style.sensing_intuitive == pytest.approx(1.0)
        assert style.visual_verbal == pytest.approx(0.0)
        assert style.sequential_global == pytest.approx(0.0)

    def test_scores_in_range(self):
        """All dimension scores lie within [-1.0, 1.0]."""
        import random

        rng = random.Random(42)
        for _ in range(20):
            answers = [rng.choice(["A", "B"]) for _ in range(TOTAL)]
            style = classify_style(answers)
            for dim in DIMENSIONS:
                score = getattr(style, dim)
                assert -1.0 <= score <= 1.0, f"{dim} out of range: {score}"


# ── case-insensitivity ────────────────────────────────────────────────────────


class TestCaseInsensitivity:
    def test_lowercase_a_accepted(self):
        style = classify_style(["a"] * TOTAL)
        assert style.active_reflective == -1.0

    def test_lowercase_b_accepted(self):
        style = classify_style(["b"] * TOTAL)
        assert style.active_reflective == 1.0

    def test_mixed_case_accepted(self):
        if QUESTIONS_PER_DIMENSION != 4:
            pytest.skip("test assumes 4 questions per dimension")
        answers = _block([["A", "a", "B", "b"]] * len(DIMENSIONS))
        style = classify_style(answers)
        assert style.active_reflective == pytest.approx(0.0)


# ── invalid input ─────────────────────────────────────────────────────────────


class TestInvalidInput:
    def test_empty_list_raises(self):
        with pytest.raises(ValueError, match="empty"):
            classify_style([])

    def test_wrong_length_too_short_raises(self):
        with pytest.raises(ValueError, match="expected"):
            classify_style(["A"] * (TOTAL - 1))

    def test_wrong_length_too_long_raises(self):
        with pytest.raises(ValueError, match="expected"):
            classify_style(["A"] * (TOTAL + 1))

    def test_invalid_key_raises(self):
        bad = ["A"] * TOTAL
        bad[0] = "C"
        with pytest.raises(ValueError, match="invalid"):
            classify_style(bad)

    def test_numeric_key_raises(self):
        bad = ["A"] * TOTAL
        bad[3] = "1"
        with pytest.raises(ValueError, match="invalid"):
            classify_style(bad)

    def test_empty_string_key_raises(self):
        bad = ["A"] * TOTAL
        bad[0] = ""
        with pytest.raises(ValueError, match="invalid"):
            classify_style(bad)
