package xp_test

import (
	"testing"

	"github.com/teacherslounge/gaming-service/internal/xp"
)

func TestLevelFor(t *testing.T) {
	tests := []struct {
		name      string
		totalXP   int64
		wantLevel int
	}{
		{"zero xp is level 1", 0, 1},
		{"just below level 2", 499, 1},
		{"exactly level 2", 500, 2},
		{"just above level 2", 501, 2},
		{"exactly level 3", 1200, 3},
		{"exactly level 4", 2200, 4},
		{"exactly level 5", 3500, 5},
		{"exactly level 10", 16000, 10},
		{"exactly level 20", 110000, 20},
		{"beyond max level stays at 20", 999999, 20},
		{"negative xp is level 1", -100, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := xp.LevelFor(tt.totalXP)
			if got != tt.wantLevel {
				t.Errorf("LevelFor(%d) = %d, want %d", tt.totalXP, got, tt.wantLevel)
			}
		})
	}
}

func TestThresholdFor(t *testing.T) {
	tests := []struct {
		level         int
		wantThreshold int64
	}{
		{1, 0},
		{2, 500},
		{3, 1200},
		{10, 16000},
		{20, 110000},
		{0, 0},   // below 1 → 0
		{21, 110000}, // above max → clamped
	}

	for _, tt := range tests {
		got := xp.ThresholdFor(tt.level)
		if got != tt.wantThreshold {
			t.Errorf("ThresholdFor(%d) = %d, want %d", tt.level, got, tt.wantThreshold)
		}
	}
}

func TestApply(t *testing.T) {
	tests := []struct {
		name         string
		currentXP    int64
		currentLevel int
		amount       int64
		wantXP       int64
		wantLevel    int
		wantLevelUp  bool
	}{
		{
			name:         "gain XP within same level",
			currentXP:    100, currentLevel: 1, amount: 200,
			wantXP: 300, wantLevel: 1, wantLevelUp: false,
		},
		{
			name:         "gain XP triggers level up",
			currentXP:    450, currentLevel: 1, amount: 100,
			wantXP: 550, wantLevel: 2, wantLevelUp: true,
		},
		{
			name:         "gain XP skips multiple levels",
			currentXP:    0, currentLevel: 1, amount: 3500,
			wantXP: 3500, wantLevel: 5, wantLevelUp: true,
		},
		{
			name:         "at max level no further level up",
			currentXP:    110000, currentLevel: 20, amount: 5000,
			wantXP: 115000, wantLevel: 20, wantLevelUp: false,
		},
		{
			name:         "negative amount clamps to zero xp",
			currentXP:    100, currentLevel: 1, amount: -200,
			wantXP: 0, wantLevel: 1, wantLevelUp: false,
		},
		{
			name:         "exact threshold boundary",
			currentXP:    499, currentLevel: 1, amount: 1,
			wantXP: 500, wantLevel: 2, wantLevelUp: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotXP, gotLevel, gotLevelUp := xp.Apply(tt.currentXP, tt.currentLevel, tt.amount)
			if gotXP != tt.wantXP {
				t.Errorf("Apply xp: got %d, want %d", gotXP, tt.wantXP)
			}
			if gotLevel != tt.wantLevel {
				t.Errorf("Apply level: got %d, want %d", gotLevel, tt.wantLevel)
			}
			if gotLevelUp != tt.wantLevelUp {
				t.Errorf("Apply leveledUp: got %v, want %v", gotLevelUp, tt.wantLevelUp)
			}
		})
	}
}
