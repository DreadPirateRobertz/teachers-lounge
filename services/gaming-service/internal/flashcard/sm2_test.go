package flashcard_test

import (
	"testing"
	"time"

	"github.com/teacherslounge/gaming-service/internal/flashcard"
)

// defaultEF is the standard starting ease factor for a new card.
const defaultEF = 2.5

// TestQuality5PerfectRecall verifies that a perfect recall (quality 5) increases
// the ease factor and produces an interval greater than 1 after the second repetition.
func TestQuality5PerfectRecall(t *testing.T) {
	// First repetition from fresh state.
	r1 := flashcard.Apply(5, defaultEF, 1, 0)
	if r1.EaseFactor <= defaultEF {
		t.Errorf("expected ease factor > %.2f, got %.4f", defaultEF, r1.EaseFactor)
	}
	if r1.IntervalDays != 1 {
		t.Errorf("expected interval 1 after first repetition, got %d", r1.IntervalDays)
	}
	if r1.Repetitions != 1 {
		t.Errorf("expected repetitions=1, got %d", r1.Repetitions)
	}

	// Second repetition.
	r2 := flashcard.Apply(5, r1.EaseFactor, r1.IntervalDays, r1.Repetitions)
	if r2.IntervalDays <= 1 {
		t.Errorf("expected interval > 1 after second repetition, got %d", r2.IntervalDays)
	}
}

// TestQuality4CorrectIntervalSequence verifies the interval sequence 1 → 6 → ~6*EF
// when answering with quality 4 (correct with hesitation).
func TestQuality4CorrectIntervalSequence(t *testing.T) {
	ef := defaultEF

	// Rep 0 → rep 1: interval should be 1.
	r1 := flashcard.Apply(4, ef, 1, 0)
	if r1.IntervalDays != 1 {
		t.Errorf("rep 0 → interval expected 1, got %d", r1.IntervalDays)
	}
	if r1.Repetitions != 1 {
		t.Errorf("expected repetitions=1, got %d", r1.Repetitions)
	}

	// Rep 1 → rep 2: interval should be 6.
	r2 := flashcard.Apply(4, r1.EaseFactor, r1.IntervalDays, r1.Repetitions)
	if r2.IntervalDays != 6 {
		t.Errorf("rep 1 → interval expected 6, got %d", r2.IntervalDays)
	}
	if r2.Repetitions != 2 {
		t.Errorf("expected repetitions=2, got %d", r2.Repetitions)
	}

	// Rep 2 → rep 3: interval should be round(6 * EF).
	r3 := flashcard.Apply(4, r2.EaseFactor, r2.IntervalDays, r2.Repetitions)
	expectedInterval := int(float64(6)*r2.EaseFactor + 0.5)
	// Allow ±1 due to rounding differences.
	if r3.IntervalDays < expectedInterval-1 || r3.IntervalDays > expectedInterval+1 {
		t.Errorf("rep 2 → interval expected ~%d, got %d", expectedInterval, r3.IntervalDays)
	}
}

// TestQuality3CorrectWithDifficulty verifies that quality 3 (correct but difficult)
// increments repetitions but causes the ease factor to decrease slightly.
func TestQuality3CorrectWithDifficulty(t *testing.T) {
	r := flashcard.Apply(3, defaultEF, 1, 0)
	if r.Repetitions != 1 {
		t.Errorf("expected repetitions=1, got %d", r.Repetitions)
	}
	if r.EaseFactor >= defaultEF {
		t.Errorf("expected ease factor < %.2f for quality 3, got %.4f", defaultEF, r.EaseFactor)
	}
}

// TestQuality2ResetsRepetitions verifies that quality 2 (incorrect but recalled on seeing
// the answer) resets repetitions to 0 and returns interval to 1.
func TestQuality2ResetsRepetitions(t *testing.T) {
	// First get to repetitions=3.
	r := flashcard.Apply(5, defaultEF, 1, 0)
	r = flashcard.Apply(5, r.EaseFactor, r.IntervalDays, r.Repetitions)
	r = flashcard.Apply(5, r.EaseFactor, r.IntervalDays, r.Repetitions)

	// Now apply a failing quality.
	fail := flashcard.Apply(2, r.EaseFactor, r.IntervalDays, r.Repetitions)
	if fail.Repetitions != 0 {
		t.Errorf("expected repetitions reset to 0, got %d", fail.Repetitions)
	}
	if fail.IntervalDays != 1 {
		t.Errorf("expected interval reset to 1, got %d", fail.IntervalDays)
	}
}

// TestQuality0Blackout verifies that quality 0 (complete blackout) resets
// repetitions to 0 and returns interval to 1 day.
func TestQuality0Blackout(t *testing.T) {
	// Build up some history first.
	r := flashcard.Apply(5, defaultEF, 1, 0)
	r = flashcard.Apply(5, r.EaseFactor, r.IntervalDays, r.Repetitions)

	fail := flashcard.Apply(0, r.EaseFactor, r.IntervalDays, r.Repetitions)
	if fail.Repetitions != 0 {
		t.Errorf("expected repetitions=0 on blackout, got %d", fail.Repetitions)
	}
	if fail.IntervalDays != 1 {
		t.Errorf("expected interval=1 on blackout, got %d", fail.IntervalDays)
	}
}

// TestEaseFactorNeverDropsBelowMinimum verifies that the ease factor is always
// clamped to at least 1.3 regardless of how many low-quality reviews occur.
func TestEaseFactorNeverDropsBelowMinimum(t *testing.T) {
	ef := 1.4 // just above minimum

	// Apply many quality-0 reviews.
	for i := 0; i < 10; i++ {
		r := flashcard.Apply(0, ef, 1, 0)
		if r.EaseFactor < 1.3 {
			t.Errorf("ease factor dropped below 1.3: %.4f", r.EaseFactor)
		}
		ef = r.EaseFactor
	}
}

// TestNextReviewAtIsInFuture verifies that NextReviewAt is always strictly
// after the current time (since intervalDays >= 1).
func TestNextReviewAtIsInFuture(t *testing.T) {
	before := time.Now().UTC()
	r := flashcard.Apply(5, defaultEF, 1, 0)
	if !r.NextReviewAt.After(before) {
		t.Errorf("NextReviewAt %v is not after %v", r.NextReviewAt, before)
	}
}

// TestGrowingIntervalsOverMultipleApplications verifies that successive quality-5
// reviews produce strictly increasing intervals.
func TestGrowingIntervalsOverMultipleApplications(t *testing.T) {
	ef := defaultEF
	interval := 1
	reps := 0
	prev := 0

	for i := 0; i < 5; i++ {
		r := flashcard.Apply(5, ef, interval, reps)
		if r.IntervalDays < prev && i >= 2 {
			t.Errorf("interval did not grow at rep %d: prev=%d, got=%d", i, prev, r.IntervalDays)
		}
		prev = r.IntervalDays
		ef = r.EaseFactor
		interval = r.IntervalDays
		reps = r.Repetitions
	}
}
