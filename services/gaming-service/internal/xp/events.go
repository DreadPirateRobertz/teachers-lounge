package xp

import "math"

// EventType identifies the source of an XP award.
type EventType string

const (
	EventLessonComplete EventType = "lesson_complete"
	EventQuizCorrect    EventType = "quiz_correct"
	EventQuizWrong      EventType = "quiz_wrong"
	EventStreakBonus     EventType = "streak_bonus"
	EventBossVictory    EventType = "boss_victory"
)

// baseXP maps each event type to its base XP reward.
var baseXP = map[EventType]int64{
	EventLessonComplete: 50,
	EventQuizCorrect:    25,
	EventQuizWrong:      5,
	EventStreakBonus:     10,
	EventBossVictory:    100,
}

// DailyCap is the maximum XP a user can earn in a single UTC day.
const DailyCap int64 = 1000

// BaseXPFor returns the base XP for an event type, or 0 if unknown.
func BaseXPFor(e EventType) int64 {
	return baseXP[e]
}

// ValidEvent returns true if the event type is recognized.
func ValidEvent(e EventType) bool {
	_, ok := baseXP[e]
	return ok
}

// StreakMultiplier returns the XP multiplier for the given streak length.
// Base is 1.0, +0.1 per streak day, capped at 2.0x.
func StreakMultiplier(streakDays int) float64 {
	if streakDays <= 0 {
		return 1.0
	}
	m := 1.0 + float64(streakDays)*0.1
	if m > 2.0 {
		return 2.0
	}
	return m
}

// CalculateAward computes the final XP award for an event, applying the streak
// multiplier and enforcing the daily cap.
//
// For streak_bonus events, the base XP is scaled by the current streak length
// (base * streak_days) before the multiplier is applied.
//
// Returns the XP to award (may be 0 if the daily cap is reached) and whether
// the cap was hit.
func CalculateAward(event EventType, streakDays int, dailyXPSoFar int64) (award int64, capped bool) {
	base := BaseXPFor(event)
	if base == 0 {
		return 0, false
	}

	// Streak bonus scales with streak length.
	if event == EventStreakBonus {
		base = base * int64(max(streakDays, 1))
	}

	multiplier := StreakMultiplier(streakDays)
	raw := int64(math.Round(float64(base) * multiplier))

	remaining := DailyCap - dailyXPSoFar
	if remaining <= 0 {
		return 0, true
	}

	if raw > remaining {
		return remaining, true
	}
	return raw, false
}
