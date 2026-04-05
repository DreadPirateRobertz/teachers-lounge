// Package flashcard implements the SM-2 spaced repetition algorithm and
// Anki .apkg export for the TeachersLounge flashcard system.
package flashcard

import (
	"math"
	"time"
)

// ReviewResult holds the updated SM-2 scheduling values after a review.
type ReviewResult struct {
	// EaseFactor is the updated ease factor (minimum 1.3).
	EaseFactor float64
	// IntervalDays is the number of days until the next review.
	IntervalDays int
	// Repetitions is the updated consecutive-correct-review count.
	Repetitions int
	// NextReviewAt is the UTC timestamp when the card should next be reviewed.
	NextReviewAt time.Time
}

// Apply runs one SM-2 review cycle.
//
// quality must be in [0, 5]:
//
//	0 = complete blackout
//	1 = incorrect; serious error
//	2 = incorrect; easy to recall on seeing answer
//	3 = correct with serious difficulty
//	4 = correct with hesitation
//	5 = perfect recall
//
// If quality < 3 the repetition counter resets to 0 and the interval
// returns to 1 day (re-learning mode).
// EaseFactor is clamped to a minimum of 1.3.
func Apply(quality int, easeFactor float64, intervalDays, repetitions int) ReviewResult {
	var newInterval int
	var newRepetitions int

	if quality < 3 {
		// Re-learning: reset repetitions and interval.
		newRepetitions = 0
		newInterval = 1
	} else {
		// Correct recall: advance the interval.
		switch repetitions {
		case 0:
			newInterval = 1
		case 1:
			newInterval = 6
		default:
			newInterval = int(math.Round(float64(intervalDays) * easeFactor))
		}
		newRepetitions = repetitions + 1
	}

	// Update ease factor using SM-2 formula.
	newEF := easeFactor + 0.1 - float64(5-quality)*(0.08+float64(5-quality)*0.02)
	if newEF < 1.3 {
		newEF = 1.3
	}

	return ReviewResult{
		EaseFactor:   newEF,
		IntervalDays: newInterval,
		Repetitions:  newRepetitions,
		NextReviewAt: time.Now().UTC().AddDate(0, 0, newInterval),
	}
}
