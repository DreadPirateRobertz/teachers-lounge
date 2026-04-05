package testutil_test

import (
	"encoding/json"
	"testing"

	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/testutil"
)

// ── MakeProfile ───────────────────────────────────────────────────────────────

func TestMakeProfile_Defaults(t *testing.T) {
	p := testutil.MakeProfile()
	if p.UserID == "" {
		t.Error("MakeProfile: UserID must not be empty")
	}
	if p.Level < 1 {
		t.Errorf("MakeProfile: Level must be >= 1, got %d", p.Level)
	}
	if p.PowerUps == nil {
		t.Error("MakeProfile: PowerUps must not be nil")
	}
	var v any
	if err := json.Unmarshal(p.PowerUps, &v); err != nil {
		t.Errorf("MakeProfile: PowerUps is not valid JSON: %v", err)
	}
}

func TestMakeProfile_OptionOverridesLevel(t *testing.T) {
	p := testutil.MakeProfile(func(p *model.Profile) {
		p.Level = 7
		p.XP = 3000
	})
	if p.Level != 7 {
		t.Errorf("want Level=7, got %d", p.Level)
	}
	if p.XP != 3000 {
		t.Errorf("want XP=3000, got %d", p.XP)
	}
	// Unmodified defaults must still be present.
	if p.UserID == "" {
		t.Error("UserID must still be set after override")
	}
}

func TestMakeProfile_MultipleOptions(t *testing.T) {
	p := testutil.MakeProfile(
		func(p *model.Profile) { p.UserID = "custom-user" },
		func(p *model.Profile) { p.BossesDefeated = 3 },
	)
	if p.UserID != "custom-user" {
		t.Errorf("want UserID=custom-user, got %q", p.UserID)
	}
	if p.BossesDefeated != 3 {
		t.Errorf("want BossesDefeated=3, got %d", p.BossesDefeated)
	}
}

// ── MakeQuizSession ───────────────────────────────────────────────────────────

func TestMakeQuizSession_Defaults(t *testing.T) {
	s := testutil.MakeQuizSession()
	if s.ID == "" {
		t.Error("MakeQuizSession: ID must not be empty")
	}
	if s.UserID == "" {
		t.Error("MakeQuizSession: UserID must not be empty")
	}
	if s.Status == "" {
		t.Error("MakeQuizSession: Status must not be empty")
	}
	if len(s.QuestionIDs) == 0 {
		t.Error("MakeQuizSession: QuestionIDs must not be empty")
	}
	if s.TotalQuestions != len(s.QuestionIDs) {
		t.Errorf("MakeQuizSession: TotalQuestions (%d) must equal len(QuestionIDs) (%d)",
			s.TotalQuestions, len(s.QuestionIDs))
	}
	if s.StartedAt.IsZero() {
		t.Error("MakeQuizSession: StartedAt must not be zero")
	}
}

func TestMakeQuizSession_OptionOverridesStatus(t *testing.T) {
	s := testutil.MakeQuizSession(func(s *model.QuizSession) {
		s.Status = "completed"
		s.CorrectCount = 2
		s.TotalXPEarned = 40
	})
	if s.Status != "completed" {
		t.Errorf("want Status=completed, got %q", s.Status)
	}
	if s.CorrectCount != 2 {
		t.Errorf("want CorrectCount=2, got %d", s.CorrectCount)
	}
}

// ── MakeQuestion ──────────────────────────────────────────────────────────────

func TestMakeQuestion_Defaults(t *testing.T) {
	q := testutil.MakeQuestion()
	if q.ID == "" {
		t.Error("MakeQuestion: ID must not be empty")
	}
	if q.Topic == "" {
		t.Error("MakeQuestion: Topic must not be empty")
	}
	if len(q.Options) < 2 {
		t.Errorf("MakeQuestion: want >= 2 options, got %d", len(q.Options))
	}
	if q.CorrectKey == "" {
		t.Error("MakeQuestion: CorrectKey must not be empty")
	}
	if len(q.Hints) == 0 {
		t.Error("MakeQuestion: Hints must not be empty (needed for stripping tests)")
	}
	if q.XPReward <= 0 {
		t.Errorf("MakeQuestion: XPReward must be > 0, got %d", q.XPReward)
	}
}

func TestMakeQuestion_OptionOverridesTopic(t *testing.T) {
	q := testutil.MakeQuestion(func(q *model.Question) {
		q.Topic = "algebra"
		q.Difficulty = 3
	})
	if q.Topic != "algebra" {
		t.Errorf("want Topic=algebra, got %q", q.Topic)
	}
	if q.Difficulty != 3 {
		t.Errorf("want Difficulty=3, got %d", q.Difficulty)
	}
	// CorrectKey default still present.
	if q.CorrectKey != "B" {
		t.Errorf("want default CorrectKey=B, got %q", q.CorrectKey)
	}
}

func TestMakeQuestion_CorrectKeyMatchesOption(t *testing.T) {
	q := testutil.MakeQuestion()
	found := false
	for _, o := range q.Options {
		if o.Key == q.CorrectKey {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("MakeQuestion: CorrectKey %q not present in Options", q.CorrectKey)
	}
}

// TestMakeQuestion_SecretFieldsNotInJSON verifies that the json:"-" tags on
// CorrectKey and Hints are respected — the factories produce structs that behave
// correctly when marshalled.
func TestMakeQuestion_SecretFieldsNotInJSON(t *testing.T) {
	q := testutil.MakeQuestion()
	raw, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if _, ok := m["correct_key"]; ok {
		t.Error("json output must not contain correct_key")
	}
	if _, ok := m["hints"]; ok {
		t.Error("json output must not contain hints")
	}
}
