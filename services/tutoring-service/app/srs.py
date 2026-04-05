"""Spaced repetition engine — SM-2 algorithm + forgetting curve model.

Pure functions only; no DB calls. The review scheduling and persistence
logic lives in reviews.py.

SM-2 Reference: Wozniak (1990), SuperMemo 2 algorithm.
"""
from __future__ import annotations

import math
from datetime import datetime, timedelta, timezone

# ── Constants ─────────────────────────────────────────────────────────────────

MIN_EASE_FACTOR = 1.3
DEFAULT_EASE_FACTOR = 2.5
PASS_THRESHOLD = 3          # quality >= 3 counts as "recalled"
INITIAL_INTERVALS = [1, 6]  # days for first two successful reviews


# ── Core SM-2 scheduler ───────────────────────────────────────────────────────

def sm2_update(
    quality: int,
    ease_factor: float,
    interval_days: int,
    repetitions: int,
) -> tuple[int, float, int]:
    """Apply one SM-2 review response and return updated scheduling state.

    Args:
        quality:     Review quality 0-5 (0=blackout, 5=perfect recall).
        ease_factor: Current ease factor (EF), starts at 2.5.
        interval_days: Current interval in days.
        repetitions: Number of consecutive successful reviews so far.

    Returns:
        (new_interval_days, new_ease_factor, new_repetitions)
    """
    if not 0 <= quality <= 5:
        raise ValueError(f"quality must be 0-5, got {quality}")

    if quality < PASS_THRESHOLD:
        # Failed recall — reset repetitions, restart from day 1
        return 1, ease_factor, 0

    # Successful recall — advance interval
    if repetitions == 0:
        new_interval = INITIAL_INTERVALS[0]
    elif repetitions == 1:
        new_interval = INITIAL_INTERVALS[1]
    else:
        new_interval = max(1, round(interval_days * ease_factor))

    # Update ease factor (EF' = EF + 0.1 - (5-q)(0.08 + (5-q)*0.02))
    delta = 0.1 - (5 - quality) * (0.08 + (5 - quality) * 0.02)
    new_ef = max(MIN_EASE_FACTOR, ease_factor + delta)

    return new_interval, new_ef, repetitions + 1


def next_review_time(interval_days: int, now: datetime | None = None) -> datetime:
    """Return the UTC datetime when the next review is due."""
    base = now or datetime.now(timezone.utc)
    return base + timedelta(days=interval_days)


# ── Forgetting curve ──────────────────────────────────────────────────────────

def retention(
    elapsed_days: float,
    stability: float,
) -> float:
    """Ebbinghaus retention estimate R = e^(-elapsed / stability).

    Args:
        elapsed_days: Days since last review.
        stability:    Memory stability parameter (larger → slower forgetting).
                      Approximated from the current interval_days.

    Returns:
        Retention probability in [0, 1].
    """
    if elapsed_days < 0:
        return 1.0
    return math.exp(-elapsed_days / max(stability, 0.01))


def mastery_from_retention(
    base_mastery: float,
    elapsed_days: float,
    decay_rate: float = 0.1,
) -> float:
    """Decay current mastery score using the forgetting curve.

    mastery_now = base_mastery * e^(-decay_rate * elapsed_days)

    Args:
        base_mastery: Mastery score at time of last review (0-1).
        elapsed_days: Days elapsed since last review.
        decay_rate:   Per-day decay rate (default 0.1 ≈ ~10-day half-life).

    Returns:
        Current estimated mastery in [0, 1].
    """
    decayed = base_mastery * math.exp(-decay_rate * max(elapsed_days, 0))
    return max(0.0, min(1.0, decayed))


def mastery_after_review(current_mastery: float, quality: int) -> float:
    """Update mastery score after a review response.

    quality 5 → mastery → 1.0
    quality 3 → mastery increases modestly
    quality < 3 → mastery decreases

    Returns updated mastery in [0, 1].
    """
    # Map quality to a mastery delta
    delta_map = {5: 0.20, 4: 0.12, 3: 0.05, 2: -0.10, 1: -0.20, 0: -0.30}
    delta = delta_map.get(quality, 0.0)
    return max(0.0, min(1.0, current_mastery + delta))
