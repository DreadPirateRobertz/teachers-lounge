package handler

import (
	"encoding/json"
	"testing"

	"github.com/teacherslounge/gaming-service/internal/model"
)

func TestXPForAnswer(t *testing.T) {
	tests := []struct {
		name      string
		baseXP    int
		hintsUsed int
		correct   bool
		wantXP    int
	}{
		{"correct no hints", 100, 0, true, 100},
		{"correct 1 hint 75%", 100, 1, true, 75},
		{"correct 2 hints 50%", 100, 2, true, 50},
		{"correct 3 hints 25%", 100, 3, true, 25},
		{"correct many hints still 25%", 100, 10, true, 25},
		{"wrong no hints", 100, 0, false, 0},
		{"wrong with hints", 100, 2, false, 0},
		{"small base xp 1 hint", 10, 1, true, 7},
		{"small base xp 2 hints", 10, 2, true, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := xpForAnswer(tt.baseXP, tt.hintsUsed, tt.correct)
			if got != tt.wantXP {
				t.Errorf("xpForAnswer(%d, %d, %v) = %d, want %d",
					tt.baseXP, tt.hintsUsed, tt.correct, got, tt.wantXP)
			}
		})
	}
}

func TestStripAnswer_HidesSecret(t *testing.T) {
	q := &model.Question{
		ID:          "q1",
		Topic:       "math",
		Difficulty:  2,
		Question:    "What is 2+2?",
		Options:     []model.QuizOption{{Key: "A", Text: "3"}, {Key: "B", Text: "4"}},
		CorrectKey:  "B",
		Hints:       []string{"think addition", "it's more than 3"},
		Explanation: "Because 2+2=4",
		XPReward:    20,
	}

	stripped := stripAnswer(q)

	// Correct key and hints must not be present.
	if stripped.CorrectKey != "" {
		t.Errorf("stripAnswer: CorrectKey should be empty, got %q", stripped.CorrectKey)
	}
	if len(stripped.Hints) != 0 {
		t.Errorf("stripAnswer: Hints should be empty, got %v", stripped.Hints)
	}
	if stripped.Explanation != "" {
		t.Errorf("stripAnswer: Explanation should be empty, got %q", stripped.Explanation)
	}

	// Public fields must be preserved.
	if stripped.ID != q.ID {
		t.Errorf("stripAnswer: ID mismatch: got %q, want %q", stripped.ID, q.ID)
	}
	if stripped.XPReward != q.XPReward {
		t.Errorf("stripAnswer: XPReward mismatch: got %d, want %d", stripped.XPReward, q.XPReward)
	}
	if len(stripped.Options) != len(q.Options) {
		t.Errorf("stripAnswer: Options count mismatch: got %d, want %d", len(stripped.Options), len(q.Options))
	}

	// Verify json:"-" tags mean CorrectKey never appears in marshalled output.
	raw, _ := json.Marshal(q)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if _, ok := m["correct_key"]; ok {
		t.Error("json.Marshal(Question) should not include correct_key field")
	}
	if _, ok := m["hints"]; ok {
		t.Error("json.Marshal(Question) should not include hints field")
	}
}
