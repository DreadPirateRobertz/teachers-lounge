package xp_test

import (
	"testing"

	"github.com/teacherslounge/gaming-service/internal/xp"
)

func TestValidEvent(t *testing.T) {
	valid := []xp.EventType{
		xp.EventLessonComplete,
		xp.EventQuizCorrect,
		xp.EventQuizWrong,
		xp.EventStreakBonus,
		xp.EventBossVictory,
	}
	for _, e := range valid {
		if !xp.ValidEvent(e) {
			t.Errorf("ValidEvent(%q) = false, want true", e)
		}
	}
	if xp.ValidEvent("nonexistent") {
		t.Error("ValidEvent(nonexistent) = true, want false")
	}
}

func TestBaseXPFor(t *testing.T) {
	tests := []struct {
		event xp.EventType
		want  int64
	}{
		{xp.EventLessonComplete, 50},
		{xp.EventQuizCorrect, 25},
		{xp.EventQuizWrong, 5},
		{xp.EventStreakBonus, 10},
		{xp.EventBossVictory, 100},
		{"unknown", 0},
	}
	for _, tt := range tests {
		got := xp.BaseXPFor(tt.event)
		if got != tt.want {
			t.Errorf("BaseXPFor(%q) = %d, want %d", tt.event, got, tt.want)
		}
	}
}

func TestStreakMultiplier(t *testing.T) {
	tests := []struct {
		streak int
		want   float64
	}{
		{0, 1.0},
		{-1, 1.0},
		{1, 1.1},
		{5, 1.5},
		{10, 2.0},
		{15, 2.0}, // capped
		{100, 2.0},
	}
	for _, tt := range tests {
		got := xp.StreakMultiplier(tt.streak)
		if got != tt.want {
			t.Errorf("StreakMultiplier(%d) = %f, want %f", tt.streak, got, tt.want)
		}
	}
}

func TestCalculateAward(t *testing.T) {
	tests := []struct {
		name       string
		event      xp.EventType
		streak     int
		dailySoFar int64
		wantAward  int64
		wantCapped bool
	}{
		{
			name:       "lesson complete no streak",
			event:      xp.EventLessonComplete,
			streak:     0,
			dailySoFar: 0,
			wantAward:  50, // 50 * 1.0
			wantCapped: false,
		},
		{
			name:       "lesson complete with streak 5",
			event:      xp.EventLessonComplete,
			streak:     5,
			dailySoFar: 0,
			wantAward:  75, // 50 * 1.5
			wantCapped: false,
		},
		{
			name:       "quiz correct with max streak multiplier",
			event:      xp.EventQuizCorrect,
			streak:     15,
			dailySoFar: 0,
			wantAward:  50, // 25 * 2.0
			wantCapped: false,
		},
		{
			name:       "streak bonus scales with streak days",
			event:      xp.EventStreakBonus,
			streak:     5,
			dailySoFar: 0,
			wantAward:  75, // base 10 * 5 streak = 50, * 1.5 multiplier = 75
			wantCapped: false,
		},
		{
			name:       "daily cap already reached",
			event:      xp.EventLessonComplete,
			streak:     0,
			dailySoFar: 1000,
			wantAward:  0,
			wantCapped: true,
		},
		{
			name:       "daily cap partially remaining",
			event:      xp.EventLessonComplete,
			streak:     0,
			dailySoFar: 980,
			wantAward:  20, // only 20 remaining of 1000 cap
			wantCapped: true,
		},
		{
			name:       "unknown event returns zero",
			event:      "fake",
			streak:     5,
			dailySoFar: 0,
			wantAward:  0,
			wantCapped: false,
		},
		{
			name:       "boss victory no streak",
			event:      xp.EventBossVictory,
			streak:     0,
			dailySoFar: 0,
			wantAward:  100, // 100 * 1.0
			wantCapped: false,
		},
		{
			name:       "boss victory with streak 3",
			event:      xp.EventBossVictory,
			streak:     3,
			dailySoFar: 0,
			wantAward:  130, // 100 * 1.3
			wantCapped: false,
		},
		{
			name:       "quiz wrong minimal xp",
			event:      xp.EventQuizWrong,
			streak:     0,
			dailySoFar: 0,
			wantAward:  5, // 5 * 1.0
			wantCapped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			award, capped := xp.CalculateAward(tt.event, tt.streak, tt.dailySoFar)
			if award != tt.wantAward {
				t.Errorf("award: got %d, want %d", award, tt.wantAward)
			}
			if capped != tt.wantCapped {
				t.Errorf("capped: got %v, want %v", capped, tt.wantCapped)
			}
		})
	}
}
