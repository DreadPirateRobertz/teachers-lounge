"""Behavioral learning-style detection for the tutoring service.

Implements the Felder-Silverman Learning Styles Model with four bipolar dimensions:
  active/reflective  — preference for trying things vs. thinking things through
  sensing/intuitive  — preference for concrete facts vs. abstract concepts
  visual/verbal      — preference for diagrams/images vs. words/text
  sequential/global  — preference for step-by-step vs. big-picture thinking

Dials live in [-1.0, 1.0]:
  -1.0 → strong first-pole preference  (active / sensing / visual / sequential)
   0.0 → neutral / unknown
  +1.0 → strong second-pole preference (reflective / intuitive / verbal / global)

Detection is purely regex-based — no LLM call.  Signals are applied to dials
via exponential moving average (EMA) so a single message cannot dominate.

Assessment-based classification is also provided via classify_style(), which
converts explicit A/B questionnaire answers into dimension scores.  Answers are
grouped in blocks of 4, one block per dimension in the order listed above.
Within each block, 'A' votes for the first pole (-1.0 direction) and 'B' votes
for the second pole (+1.0 direction).
"""

import re
from typing import NamedTuple

# ── constants ────────────────────────────────────────────────────────────────

ALPHA: float = 0.15  # EMA smoothing factor — higher = faster adaptation
THRESHOLD: float = 0.2  # |dial| must exceed this to emit style guidance

DIMENSIONS = ("active_reflective", "sensing_intuitive", "visual_verbal", "sequential_global")

DEFAULT_DIALS: dict[str, float] = {d: 0.0 for d in DIMENSIONS}

# Number of A/B questions per dimension in the standard assessment.
QUESTIONS_PER_DIMENSION: int = 4


# ── signal ───────────────────────────────────────────────────────────────────


class StyleSignal(NamedTuple):
    """A single detected style cue extracted from a student message.

    Attributes:
        dimension: One of the four Felder-Silverman dimension names.
        direction: +1.0 moves toward the second pole; -1.0 toward the first.
    """

    dimension: str
    direction: float


class LearningStyle(NamedTuple):
    """Felder-Silverman learning style scores derived from an A/B assessment.

    Each attribute holds a score in [-1.0, 1.0] where:
      -1.0 → strong first-pole preference
       0.0 → neutral / balanced
      +1.0 → strong second-pole preference

    Attributes:
        active_reflective: Negative = active (do/try); positive = reflective (think/why).
        sensing_intuitive: Negative = sensing (concrete); positive = intuitive (abstract).
        visual_verbal: Negative = visual (diagrams); positive = verbal (words).
        sequential_global: Negative = sequential (step-by-step); positive = global (big-picture).
    """

    active_reflective: float
    sensing_intuitive: float
    visual_verbal: float
    sequential_global: float


# ── regex pattern table ───────────────────────────────────────────────────────
# Each row: (compiled_pattern, dimension, direction)
#   active_reflective:  -1 = active (do / try),   +1 = reflective (why / think)
#   sensing_intuitive:  -1 = sensing (example / concrete), +1 = intuitive (theory / concept)
#   visual_verbal:      -1 = visual (diagram / picture),   +1 = verbal (explain / describe)
#   sequential_global:  -1 = sequential (step-by-step),    +1 = global (big picture)

_PATTERNS: list[tuple[re.Pattern, str, float]] = [
    # active/reflective
    (
        re.compile(
            r"\b(let me try|let('s| us) try|i('ll| will) try|hands[- ]on|practice problem)\b", re.I
        ),
        "active_reflective",
        -1.0,
    ),
    (
        re.compile(r"\bwhy (does|is|do|did|would|should|can|are|was|were)\b", re.I),
        "active_reflective",
        +1.0,
    ),
    (
        re.compile(r"\b(think|reflect|consider|ponder|reason|understand the reasoning)\b", re.I),
        "active_reflective",
        +1.0,
    ),
    # sensing/intuitive
    (
        re.compile(
            r"\b(for example|for instance|give me an example|real[- ]world|concrete|specific case)\b",
            re.I,
        ),
        "sensing_intuitive",
        -1.0,
    ),
    (
        re.compile(
            r"\b(in general|theory|theoretical|abstract|concept|principle|underlying|fundamentally)\b",
            re.I,
        ),
        "sensing_intuitive",
        +1.0,
    ),
    # visual/verbal
    (
        re.compile(
            r"\b(diagram|draw|picture|visual|chart|graph|illustrate|show me|sketch|figure)\b", re.I
        ),
        "visual_verbal",
        -1.0,
    ),
    (
        re.compile(
            r"\b(explain|describe|tell me|walk me through|in words|elaborate|clarify)\b", re.I
        ),
        "visual_verbal",
        +1.0,
    ),
    # sequential/global
    (
        re.compile(
            r"\b(step[- ]by[- ]step|first .{0,20} then|in order|one by one|each step|next step)\b",
            re.I,
        ),
        "sequential_global",
        -1.0,
    ),
    (
        re.compile(
            r"\b(big picture|overview|in general|overall|broadly|at a high level|summary|how does .{0,30} fit)\b",
            re.I,
        ),
        "sequential_global",
        +1.0,
    ),
]


# ── public API ────────────────────────────────────────────────────────────────


def classify_style(answers: list[str]) -> LearningStyle:
    """Classify learning style from A/B assessment answers.

    Answers are grouped in blocks of QUESTIONS_PER_DIMENSION (default 4), one
    block per Felder-Silverman dimension in the canonical order:
      active_reflective → sensing_intuitive → visual_verbal → sequential_global

    Within each block, 'A' votes toward the first pole (-1.0) and 'B' votes
    toward the second pole (+1.0).  The dimension score is the mean vote:
      score = (count_B - count_A) / QUESTIONS_PER_DIMENSION

    Args:
        answers: List of answer keys, each 'A' or 'B' (case-insensitive).
            Length must be exactly QUESTIONS_PER_DIMENSION * 4.

    Returns:
        LearningStyle with each dimension score in [-1.0, 1.0].

    Raises:
        ValueError: If answers is empty, contains invalid keys, or has a length
            that is not a multiple of 4 or does not match the expected total.
    """
    expected = QUESTIONS_PER_DIMENSION * len(DIMENSIONS)
    if not answers:
        raise ValueError("answers must not be empty")
    normalised = [a.upper() for a in answers]
    invalid = [a for a in normalised if a not in ("A", "B")]
    if invalid:
        raise ValueError(f"invalid answer keys (must be 'A' or 'B'): {invalid}")
    if len(normalised) != expected:
        raise ValueError(
            f"expected {expected} answers ({QUESTIONS_PER_DIMENSION} per dimension × "
            f"{len(DIMENSIONS)} dimensions), got {len(normalised)}"
        )

    scores: list[float] = []
    for i in range(len(DIMENSIONS)):
        block = normalised[i * QUESTIONS_PER_DIMENSION : (i + 1) * QUESTIONS_PER_DIMENSION]
        count_b = block.count("B")
        count_a = block.count("A")
        scores.append((count_b - count_a) / QUESTIONS_PER_DIMENSION)

    return LearningStyle(
        active_reflective=scores[0],
        sensing_intuitive=scores[1],
        visual_verbal=scores[2],
        sequential_global=scores[3],
    )


def detect_signals(message: str) -> list[StyleSignal]:
    """Scan a student message and return every Felder-Silverman style signal found.

    Runs all regex patterns against the message text.  Multiple signals from the
    same dimension are all returned; callers can choose how to aggregate.

    Args:
        message: Raw student message text.

    Returns:
        List of StyleSignal named-tuples, possibly empty if no patterns match.
        Order follows the pattern table definition.
    """
    signals: list[StyleSignal] = []
    for pattern, dimension, direction in _PATTERNS:
        if pattern.search(message):
            signals.append(StyleSignal(dimension=dimension, direction=direction))
    return signals


def update_dials(
    current: dict[str, float],
    signals: list[StyleSignal],
    alpha: float = ALPHA,
) -> dict[str, float]:
    """Apply detected signals to current dials via exponential moving average.

    Each signal nudges the relevant dial toward the signal direction by alpha.
    When multiple signals fire on the same dimension, each is applied in sequence
    so stronger evidence causes faster movement.

    The returned dict is a new object; *current* is never mutated.

    Args:
        current: Current dial values, keyed by dimension name.  Missing dimensions
            default to 0.0.
        signals: Style signals to apply, as returned by detect_signals().
        alpha: EMA smoothing factor in (0, 1].  Higher values adapt faster.
            Defaults to module-level ALPHA (0.15).

    Returns:
        New dict with updated dial values, all clamped to [-1.0, 1.0].
    """
    updated = dict(current)
    for sig in signals:
        old = updated.get(sig.dimension, 0.0)
        new = old + alpha * (sig.direction - old)
        updated[sig.dimension] = max(-1.0, min(1.0, new))
    return updated


def build_style_prompt_section(dials: dict[str, float]) -> str:
    """Build an adaptive system-prompt addendum based on current Felder-Silverman dials.

    Only dimensions whose absolute value exceeds THRESHOLD contribute guidance.
    Returns an empty string when all dials are near zero (no detectable preference).

    The returned string is intended to be appended directly to the base system prompt
    so Professor Nova adjusts tone and format for this student's detected style.

    Args:
        dials: Current dial values, keyed by dimension name.  Missing dimensions
            are treated as 0.0.

    Returns:
        A multi-line string with style guidance, or '' if all dials are within
        the neutral zone (|dial| <= THRESHOLD).
    """
    lines: list[str] = []

    ar = dials.get("active_reflective", 0.0)
    si = dials.get("sensing_intuitive", 0.0)
    vv = dials.get("visual_verbal", 0.0)
    sg = dials.get("sequential_global", 0.0)

    if abs(ar) > THRESHOLD:
        if ar < 0:
            lines.append(
                "- ACTIVE learner: include a short practice problem or 'try it' prompt at the end of your response."
            )
        else:
            lines.append(
                "- REFLECTIVE learner: explain the reasoning and 'why' behind concepts before giving solutions."
            )

    if abs(si) > THRESHOLD:
        if si < 0:
            lines.append(
                "- SENSING learner: ground every concept in a concrete, real-world example before generalizing."
            )
        else:
            lines.append(
                "- INTUITIVE learner: lead with the abstract principle or theory; examples can follow."
            )

    if abs(vv) > THRESHOLD:
        if vv < 0:
            lines.append(
                "- VISUAL learner: use ASCII diagrams, structured layouts, or spatial analogies wherever possible."
            )
        else:
            lines.append(
                "- VERBAL learner: use precise written explanations; avoid cluttering responses with diagrams."
            )

    if abs(sg) > THRESHOLD:
        if sg < 0:
            lines.append(
                "- SEQUENTIAL learner: structure your response as an ordered series of numbered steps (1, 2, 3…)."
            )
        else:
            lines.append(
                "- GLOBAL learner: open with the big picture / context before drilling into details."
            )

    if not lines:
        return ""

    header = "\nThis student's detected learning style (adapt your response accordingly):"
    return header + "\n" + "\n".join(lines)
