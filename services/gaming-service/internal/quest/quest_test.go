package quest_test

import (
	"testing"

	"github.com/teacherslounge/gaming-service/internal/quest"
)

func TestByID(t *testing.T) {
	for _, def := range quest.Daily {
		got := quest.ByID(def.ID)
		if got == nil {
			t.Errorf("ByID(%q) = nil, want non-nil", def.ID)
			continue
		}
		if got.ID != def.ID {
			t.Errorf("ByID(%q).ID = %q, want %q", def.ID, got.ID, def.ID)
		}
	}

	if quest.ByID("nonexistent") != nil {
		t.Error("ByID(\"nonexistent\") should return nil")
	}
}

func TestForAction(t *testing.T) {
	tests := []struct {
		action   string
		wantIDs  []string
	}{
		{"question_answered", []string{"questions_answered"}},
		{"streak_checkin", []string{"keep_streak_alive"}},
		{"concept_mastered", []string{"master_new_concept"}},
		{"unknown_action", nil},
		{"", nil},
	}

	for _, tt := range tests {
		got := quest.ForAction(tt.action)
		if len(got) != len(tt.wantIDs) {
			t.Errorf("ForAction(%q) = %v, want %v", tt.action, got, tt.wantIDs)
			continue
		}
		for i, id := range tt.wantIDs {
			if got[i] != id {
				t.Errorf("ForAction(%q)[%d] = %q, want %q", tt.action, i, got[i], id)
			}
		}
	}
}

func TestDailyQuestsAreComplete(t *testing.T) {
	if len(quest.Daily) != 3 {
		t.Errorf("expected 3 daily quests, got %d", len(quest.Daily))
	}

	for _, def := range quest.Daily {
		if def.ID == "" {
			t.Error("quest has empty ID")
		}
		if def.Title == "" {
			t.Errorf("quest %q has empty Title", def.ID)
		}
		if def.Target <= 0 {
			t.Errorf("quest %q has non-positive Target: %d", def.ID, def.Target)
		}
		if def.XPReward <= 0 {
			t.Errorf("quest %q has non-positive XPReward: %d", def.ID, def.XPReward)
		}
		if def.GemsReward <= 0 {
			t.Errorf("quest %q has non-positive GemsReward: %d", def.ID, def.GemsReward)
		}
	}
}
