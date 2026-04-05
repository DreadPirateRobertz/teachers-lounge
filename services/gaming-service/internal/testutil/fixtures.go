// Package testutil provides shared test fixture factories for the gaming service.
//
// Using these factories instead of inline struct literals prevents mock drift:
// when a model type gains a new required field, updating the factory's defaults
// in one place is enough — individual tests remain unaffected unless they care
// about the new field.
//
// Usage:
//
//	p := testutil.MakeProfile()                          // sane defaults
//	p := testutil.MakeProfile(func(p *model.Profile) {   // override one field
//	    p.Level = 5
//	})
package testutil

import (
	"encoding/json"
	"time"

	"github.com/teacherslounge/gaming-service/internal/model"
)

// ProfileOpt is a functional option that modifies a Profile after construction.
type ProfileOpt func(*model.Profile)

// MakeProfile returns a Profile with sane defaults. Pass ProfileOpt functions
// to override individual fields without touching unrelated ones.
func MakeProfile(opts ...ProfileOpt) *model.Profile {
	p := &model.Profile{
		UserID:         "user-test-1",
		Level:          1,
		XP:             0,
		CurrentStreak:  0,
		LongestStreak:  0,
		BossesDefeated: 0,
		Gems:           10,
		PowerUps:       json.RawMessage("{}"),
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// QuizSessionOpt is a functional option that modifies a QuizSession after construction.
type QuizSessionOpt func(*model.QuizSession)

// MakeQuizSession returns a QuizSession with sane defaults. Pass QuizSessionOpt
// functions to override individual fields.
func MakeQuizSession(opts ...QuizSessionOpt) *model.QuizSession {
	s := &model.QuizSession{
		ID:             "session-test-1",
		UserID:         "user-test-1",
		Status:         "active",
		QuestionIDs:    []string{"q-1", "q-2", "q-3"},
		CurrentIndex:   0,
		TotalQuestions: 3,
		CorrectCount:   0,
		TotalXPEarned:  0,
		StartedAt:      time.Now(),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// QuestionOpt is a functional option that modifies a Question after construction.
type QuestionOpt func(*model.Question)

// MakeQuestion returns a Question with sane defaults. Pass QuestionOpt functions
// to override individual fields.
//
// CorrectKey defaults to "B" and Hints default to a single hint string so callers
// that test stripping behaviour have something to assert against.
func MakeQuestion(opts ...QuestionOpt) *model.Question {
	q := &model.Question{
		ID:         "q-test-1",
		Topic:      "general",
		Difficulty: 1,
		Question:   "Which is correct?",
		Options: []model.QuizOption{
			{Key: "A", Text: "Wrong answer"},
			{Key: "B", Text: "Correct answer"},
		},
		CorrectKey:  "B",
		Hints:       []string{"think carefully"},
		Explanation: "B is the correct answer.",
		XPReward:    10,
	}
	for _, o := range opts {
		o(q)
	}
	return q
}
