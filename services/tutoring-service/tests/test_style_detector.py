"""
Unit tests for app.style_detector.

Covers:
  - detect_signals: pattern matching across all four dimensions, both poles
  - update_dials: EMA math, bounds clamping, multiple signals, no-op on empty
  - build_style_prompt_section: threshold gating, correct labels, empty return
"""
import pytest

from app.style_detector import (
    DEFAULT_DIALS,
    ALPHA,
    THRESHOLD,
    StyleSignal,
    build_style_prompt_section,
    detect_signals,
    update_dials,
)


# ── detect_signals ────────────────────────────────────────────────────────────

class TestDetectSignals:
    def test_returns_empty_for_neutral_message(self):
        assert detect_signals("What is 2 + 2?") == []

    # active / reflective
    def test_active_signal_try(self):
        sigs = detect_signals("let me try this problem")
        assert any(s.dimension == "active_reflective" and s.direction == -1.0 for s in sigs)

    def test_active_signal_hands_on(self):
        sigs = detect_signals("I prefer hands-on practice")
        assert any(s.dimension == "active_reflective" and s.direction == -1.0 for s in sigs)

    def test_reflective_signal_why(self):
        sigs = detect_signals("Why does the derivative work that way?")
        assert any(s.dimension == "active_reflective" and s.direction == +1.0 for s in sigs)

    def test_reflective_signal_think(self):
        sigs = detect_signals("I need to think about this more carefully")
        assert any(s.dimension == "active_reflective" and s.direction == +1.0 for s in sigs)

    # sensing / intuitive
    def test_sensing_signal_example(self):
        sigs = detect_signals("Can you give me an example?")
        assert any(s.dimension == "sensing_intuitive" and s.direction == -1.0 for s in sigs)

    def test_sensing_signal_real_world(self):
        sigs = detect_signals("Show me a real-world application")
        assert any(s.dimension == "sensing_intuitive" and s.direction == -1.0 for s in sigs)

    def test_intuitive_signal_theory(self):
        sigs = detect_signals("What is the underlying theory here?")
        assert any(s.dimension == "sensing_intuitive" and s.direction == +1.0 for s in sigs)

    def test_intuitive_signal_abstract(self):
        sigs = detect_signals("I want to understand the abstract concept")
        assert any(s.dimension == "sensing_intuitive" and s.direction == +1.0 for s in sigs)

    # visual / verbal
    def test_visual_signal_diagram(self):
        sigs = detect_signals("Can you draw a diagram?")
        assert any(s.dimension == "visual_verbal" and s.direction == -1.0 for s in sigs)

    def test_visual_signal_chart(self):
        sigs = detect_signals("Show me a chart of this data")
        assert any(s.dimension == "visual_verbal" and s.direction == -1.0 for s in sigs)

    def test_verbal_signal_explain(self):
        sigs = detect_signals("Please explain this to me")
        assert any(s.dimension == "visual_verbal" and s.direction == +1.0 for s in sigs)

    def test_verbal_signal_walk_me_through(self):
        sigs = detect_signals("Can you walk me through the proof?")
        assert any(s.dimension == "visual_verbal" and s.direction == +1.0 for s in sigs)

    # sequential / global
    def test_sequential_signal_step_by_step(self):
        sigs = detect_signals("Explain this step-by-step please")
        assert any(s.dimension == "sequential_global" and s.direction == -1.0 for s in sigs)

    def test_sequential_signal_in_order(self):
        sigs = detect_signals("Give me the steps in order")
        assert any(s.dimension == "sequential_global" and s.direction == -1.0 for s in sigs)

    def test_global_signal_big_picture(self):
        sigs = detect_signals("Give me the big picture first")
        assert any(s.dimension == "sequential_global" and s.direction == +1.0 for s in sigs)

    def test_global_signal_overview(self):
        sigs = detect_signals("I want a high-level overview")
        assert any(s.dimension == "sequential_global" and s.direction == +1.0 for s in sigs)

    def test_case_insensitive(self):
        sigs = detect_signals("WHY DOES THIS WORK")
        assert any(s.dimension == "active_reflective" for s in sigs)

    def test_multiple_signals_returned(self):
        """A message triggering multiple patterns returns all signals."""
        sigs = detect_signals("Can you explain this step-by-step with a diagram?")
        dims = {s.dimension for s in sigs}
        assert "visual_verbal" in dims
        assert "sequential_global" in dims


# ── update_dials ──────────────────────────────────────────────────────────────

class TestUpdateDials:
    def test_no_signals_returns_unchanged_copy(self):
        dials = {"active_reflective": 0.3}
        result = update_dials(dials, [])
        assert result == dials
        assert result is not dials   # must be a new dict

    def test_ema_math_positive_direction(self):
        dials = {"active_reflective": 0.0}
        sigs = [StyleSignal("active_reflective", +1.0)]
        result = update_dials(dials, sigs, alpha=0.1)
        expected = 0.0 + 0.1 * (1.0 - 0.0)
        assert abs(result["active_reflective"] - expected) < 1e-9

    def test_ema_math_negative_direction(self):
        dials = {"visual_verbal": 0.0}
        sigs = [StyleSignal("visual_verbal", -1.0)]
        result = update_dials(dials, sigs, alpha=0.15)
        expected = 0.0 + 0.15 * (-1.0 - 0.0)
        assert abs(result["visual_verbal"] - expected) < 1e-9

    def test_ema_moves_toward_signal_from_nonzero(self):
        dials = {"sensing_intuitive": 0.5}
        sigs = [StyleSignal("sensing_intuitive", -1.0)]
        result = update_dials(dials, sigs)
        assert result["sensing_intuitive"] < 0.5

    def test_upper_bound_clamp(self):
        dials = {"active_reflective": 0.99}
        sigs = [StyleSignal("active_reflective", +1.0)]
        result = update_dials(dials, sigs, alpha=1.0)
        assert result["active_reflective"] <= 1.0

    def test_lower_bound_clamp(self):
        dials = {"active_reflective": -0.99}
        sigs = [StyleSignal("active_reflective", -1.0)]
        result = update_dials(dials, sigs, alpha=1.0)
        assert result["active_reflective"] >= -1.0

    def test_multiple_signals_same_dimension(self):
        """Two signals on the same dimension compound their effect."""
        dials = {"visual_verbal": 0.0}
        sigs = [StyleSignal("visual_verbal", -1.0), StyleSignal("visual_verbal", -1.0)]
        result = update_dials(dials, sigs)
        single_result = update_dials(dials, [StyleSignal("visual_verbal", -1.0)])
        assert result["visual_verbal"] < single_result["visual_verbal"]

    def test_unrelated_dimension_unchanged(self):
        dials = {"active_reflective": 0.3, "visual_verbal": 0.5}
        sigs = [StyleSignal("active_reflective", +1.0)]
        result = update_dials(dials, sigs)
        assert result["visual_verbal"] == 0.5

    def test_missing_dimension_defaults_to_zero(self):
        result = update_dials({}, [StyleSignal("sensing_intuitive", +1.0)])
        expected = ALPHA * 1.0
        assert abs(result["sensing_intuitive"] - expected) < 1e-9

    def test_original_dict_not_mutated(self):
        dials = {"sequential_global": 0.4}
        original_val = dials["sequential_global"]
        update_dials(dials, [StyleSignal("sequential_global", -1.0)])
        assert dials["sequential_global"] == original_val


# ── build_style_prompt_section ────────────────────────────────────────────────

class TestBuildStylePromptSection:
    def test_all_zero_dials_returns_empty_string(self):
        assert build_style_prompt_section(DEFAULT_DIALS) == ""

    def test_empty_dict_returns_empty_string(self):
        assert build_style_prompt_section({}) == ""

    def test_below_threshold_returns_empty_string(self):
        dials = {"visual_verbal": THRESHOLD - 0.01}
        assert build_style_prompt_section(dials) == ""

    def test_at_threshold_boundary_returns_empty_string(self):
        """Exactly at threshold is NOT enough — must exceed it."""
        dials = {"visual_verbal": THRESHOLD}
        assert build_style_prompt_section(dials) == ""

    def test_just_above_threshold_returns_section(self):
        dials = {"visual_verbal": THRESHOLD + 0.01}
        result = build_style_prompt_section(dials)
        assert result != ""

    def test_visual_learner_label(self):
        dials = {"visual_verbal": -0.5}
        result = build_style_prompt_section(dials)
        assert "VISUAL" in result

    def test_verbal_learner_label(self):
        dials = {"visual_verbal": +0.5}
        result = build_style_prompt_section(dials)
        assert "VERBAL" in result

    def test_active_learner_label(self):
        dials = {"active_reflective": -0.5}
        result = build_style_prompt_section(dials)
        assert "ACTIVE" in result

    def test_reflective_learner_label(self):
        dials = {"active_reflective": +0.5}
        result = build_style_prompt_section(dials)
        assert "REFLECTIVE" in result

    def test_sensing_learner_label(self):
        dials = {"sensing_intuitive": -0.5}
        result = build_style_prompt_section(dials)
        assert "SENSING" in result

    def test_intuitive_learner_label(self):
        dials = {"sensing_intuitive": +0.5}
        result = build_style_prompt_section(dials)
        assert "INTUITIVE" in result

    def test_sequential_learner_label(self):
        dials = {"sequential_global": -0.5}
        result = build_style_prompt_section(dials)
        assert "SEQUENTIAL" in result

    def test_global_learner_label(self):
        dials = {"sequential_global": +0.5}
        result = build_style_prompt_section(dials)
        assert "GLOBAL" in result

    def test_multiple_active_dimensions_all_present(self):
        dials = {"visual_verbal": -0.5, "sequential_global": -0.5}
        result = build_style_prompt_section(dials)
        assert "VISUAL" in result
        assert "SEQUENTIAL" in result

    def test_section_starts_with_newline(self):
        """Style section appends cleanly to any base prompt."""
        dials = {"visual_verbal": -0.5}
        result = build_style_prompt_section(dials)
        assert result.startswith("\n")
