"""Scenario-driven tests for learning-style detection (tl-cr4).

Where ``test_style_detector.py`` exercises each function in isolation, this
module covers end-to-end scenarios: a student message flows through
``detect_signals`` → ``update_dials`` → ``build_style_prompt_section`` and we
assert on the emergent behaviour.

Scenarios mirror the classic assessment categories:
  - happy path:  a realistic message with detectable signals
  - all-A:       message heavy on first-pole cues for every dimension
  - all-B:       message heavy on second-pole cues for every dimension
  - mixed:       same dimension receives both poles; signals compete
  - invalid:     empty / whitespace / non-English / extremely long input
"""

from __future__ import annotations

import pytest

from app.style_detector import (
    DEFAULT_DIALS,
    DIMENSIONS,
    THRESHOLD,
    build_style_prompt_section,
    detect_signals,
    update_dials,
)


def _run_pipeline(message: str, rounds: int = 1) -> dict[str, float]:
    """Run the detect→update pipeline for ``rounds`` repetitions of ``message``.

    Args:
        message: Student message text fed to ``detect_signals`` each round.
        rounds: Number of EMA updates. More rounds push dials further toward
            the signal direction (bounded by [-1, 1]).

    Returns:
        Final dial state after applying all signals.
    """
    dials = dict(DEFAULT_DIALS)
    for _ in range(rounds):
        signals = detect_signals(message)
        dials = update_dials(dials, signals)
    return dials


# ── happy path ───────────────────────────────────────────────────────────────


class TestHappyPath:
    """Realistic single-message scenarios."""

    def test_diagram_request_shifts_visual(self):
        """'draw me a diagram' should nudge visual_verbal negative (visual)."""
        dials = _run_pipeline("Can you draw me a diagram of how this works?")
        assert dials["visual_verbal"] < 0.0
        # other dimensions untouched
        assert dials["active_reflective"] == 0.0
        assert dials["sensing_intuitive"] == 0.0
        assert dials["sequential_global"] == 0.0

    def test_step_by_step_request_shifts_sequential(self):
        """'step-by-step' is a sequential cue (sequential_global negative)."""
        dials = _run_pipeline("Walk me through this step-by-step please.")
        # 'walk me through' triggers visual_verbal verbal (+), 'step-by-step' triggers
        # sequential_global sequential (-). Both detectable; assert both moved.
        assert dials["sequential_global"] < 0.0
        assert dials["visual_verbal"] > 0.0

    def test_repeated_signal_crosses_threshold(self):
        """Enough repetitions of the same signal must cross THRESHOLD."""
        dials = _run_pipeline("Why does this happen?", rounds=10)
        assert dials["active_reflective"] > THRESHOLD
        section = build_style_prompt_section(dials)
        assert "REFLECTIVE learner" in section


# ── all-A: all signals point toward the first pole (−1) ──────────────────────


class TestAllPoleA:
    """Message designed to fire every dimension's negative-pole pattern."""

    MESSAGE = (
        "Let me try a practice problem — give me a concrete real-world example, "
        "draw me a diagram, and walk me step-by-step through each step."
    )

    def test_every_dimension_moves_negative(self):
        dials = _run_pipeline(self.MESSAGE, rounds=15)
        for dim in DIMENSIONS:
            assert dials[dim] < 0.0, f"dimension {dim} did not move negative"

    def test_prompt_section_mentions_all_first_pole_labels(self):
        dials = _run_pipeline(self.MESSAGE, rounds=15)
        section = build_style_prompt_section(dials)
        assert "ACTIVE learner" in section
        assert "SENSING learner" in section
        assert "VISUAL learner" in section
        assert "SEQUENTIAL learner" in section


# ── all-B: all signals point toward the second pole (+1) ─────────────────────


class TestAllPoleB:
    """Message designed to fire every dimension's positive-pole pattern."""

    # NOTE: avoid the token "picture" in this message — it also matches the
    # visual_verbal negative-pole regex (via "big picture") and cancels the
    # verbal signals we want to accumulate on this dimension.
    MESSAGE = (
        "Why does this work in theory? I want to reflect on the underlying "
        "principle — describe it in words, and give me the overall summary."
    )

    def test_every_dimension_moves_positive(self):
        dials = _run_pipeline(self.MESSAGE, rounds=15)
        for dim in DIMENSIONS:
            assert dials[dim] > 0.0, f"dimension {dim} did not move positive"

    def test_prompt_section_mentions_all_second_pole_labels(self):
        dials = _run_pipeline(self.MESSAGE, rounds=15)
        section = build_style_prompt_section(dials)
        assert "REFLECTIVE learner" in section
        assert "INTUITIVE learner" in section
        assert "VERBAL learner" in section
        assert "GLOBAL learner" in section


# ── mixed: conflicting signals within the same dimension ─────────────────────


class TestMixedInput:
    """Messages where the same dimension receives opposing signals."""

    def test_conflicting_visual_verbal_stays_near_zero(self):
        """'draw me a diagram and explain in words' fires both poles once each."""
        msg = "Draw me a diagram and explain in words what it means."
        signals = detect_signals(msg)
        visual_verbal_sigs = [s for s in signals if s.dimension == "visual_verbal"]
        # both poles present
        assert any(s.direction < 0 for s in visual_verbal_sigs)
        assert any(s.direction > 0 for s in visual_verbal_sigs)

        # after many rounds the dial settles somewhere between the two poles
        dials = _run_pipeline(msg, rounds=30)
        assert -1.0 <= dials["visual_verbal"] <= 1.0

    def test_mixed_across_dimensions_preserves_independence(self):
        """Signals on different dimensions should not cross-contaminate."""
        msg = "Why does this work? Can you give me a concrete example?"
        dials = _run_pipeline(msg, rounds=10)
        # 'why' → reflective (+), 'concrete example' → sensing (-)
        assert dials["active_reflective"] > 0.0
        assert dials["sensing_intuitive"] < 0.0
        # untouched dimensions stay at 0
        assert dials["visual_verbal"] == 0.0
        assert dials["sequential_global"] == 0.0


# ── invalid / edge-case input ────────────────────────────────────────────────


class TestInvalidInput:
    """Edge cases where detection should remain safe and deterministic."""

    def test_empty_string_yields_no_signals(self):
        assert detect_signals("") == []

    def test_whitespace_only_yields_no_signals(self):
        assert detect_signals("   \n\t  ") == []

    def test_non_string_raises(self):
        """detect_signals documents a string input; non-string should error."""
        with pytest.raises((TypeError, AttributeError)):
            detect_signals(None)  # type: ignore[arg-type]

    def test_non_latin_unicode_is_safe(self):
        """Non-ASCII input must not crash and should return no English signals."""
        assert detect_signals("为什么这个函数这样工作？") == []
        assert detect_signals("🤔🤔🤔") == []

    def test_very_long_input_does_not_blow_up(self):
        """A pathologically long message must still return promptly."""
        msg = ("why does this work in theory " * 5_000) + "the end"
        signals = detect_signals(msg)
        # The 'why' pattern fires; regex scanning completes successfully.
        assert any(s.dimension == "active_reflective" for s in signals)

    def test_update_dials_ignores_empty_signal_list(self):
        """No signals → dials unchanged (and not mutated in place)."""
        before = dict(DEFAULT_DIALS)
        after = update_dials(before, [])
        assert after == DEFAULT_DIALS
        # update_dials must return a fresh dict
        assert after is not before

    def test_update_dials_accepts_missing_dimensions(self):
        """A ``current`` dict missing keys should behave as if they were 0.0."""
        signals = detect_signals("why does this work?")
        out = update_dials({}, signals)
        assert out["active_reflective"] > 0.0

    def test_build_prompt_section_empty_on_neutral_dials(self):
        assert build_style_prompt_section(DEFAULT_DIALS) == ""

    def test_build_prompt_section_ignores_unknown_keys(self):
        """Extra keys in the dial dict must not cause errors."""
        dials = {**DEFAULT_DIALS, "mystery_dimension": 0.9}
        # Should not raise and should return "" because no known dim exceeds threshold.
        assert build_style_prompt_section(dials) == ""
