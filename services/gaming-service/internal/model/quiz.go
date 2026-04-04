package model

import "time"

// QuizOption is one answer choice within a question.
type QuizOption struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

// Question is a row from question_bank.
// CorrectKey and Hints are omitted when the question is served to a student
// (populated only internally for scoring).
type Question struct {
	ID          string       `json:"id"`
	CourseID    *string      `json:"course_id,omitempty"`
	Topic       string       `json:"topic"`
	Difficulty  int          `json:"difficulty"`
	Question    string       `json:"question"`
	Options     []QuizOption `json:"options"`
	CorrectKey  string       `json:"-"` // never sent to client
	Hints       []string     `json:"-"` // revealed one at a time via hint endpoint
	Explanation string       `json:"explanation,omitempty"` // sent only after answering
	XPReward    int          `json:"xp_reward"`
}

// QuizSession is a row from quiz_sessions.
type QuizSession struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	CourseID       *string    `json:"course_id,omitempty"`
	Topic          *string    `json:"topic,omitempty"`
	Status         string     `json:"status"`
	QuestionIDs    []string   `json:"question_ids"`
	CurrentIndex   int        `json:"current_index"`
	TotalQuestions int        `json:"total_questions"`
	CorrectCount   int        `json:"correct_count"`
	TotalXPEarned  int        `json:"total_xp_earned"`
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

// StartQuizRequest is the body for POST /gaming/quiz/start.
type StartQuizRequest struct {
	UserID        string  `json:"user_id"`
	Topic         string  `json:"topic"`
	QuestionCount int     `json:"question_count"`
	CourseID      *string `json:"course_id,omitempty"`
}

// StartQuizResponse is the response for POST /gaming/quiz/start.
type StartQuizResponse struct {
	Session  *QuizSession `json:"session"`
	Question *Question    `json:"question"` // first question (no answer/hints)
}

// QuizSessionResponse is the response for GET /gaming/quiz/sessions/{id}.
type QuizSessionResponse struct {
	Session  *QuizSession `json:"session"`
	Question *Question    `json:"question,omitempty"` // nil when session is completed
}

// SubmitAnswerRequest is the body for POST /gaming/quiz/sessions/{id}/answer.
type SubmitAnswerRequest struct {
	UserID      string `json:"user_id"`
	QuestionID  string `json:"question_id"`
	ChosenKey   string `json:"chosen_key"`
	TimeMs      *int   `json:"time_taken_ms,omitempty"`
}

// SubmitAnswerResponse is the response for POST /gaming/quiz/sessions/{id}/answer.
type SubmitAnswerResponse struct {
	Correct     bool         `json:"correct"`
	CorrectKey  string       `json:"correct_key"`
	Explanation string       `json:"explanation"`
	XPEarned    int          `json:"xp_earned"`
	Session     *QuizSession `json:"session"`
	NextQuestion *Question   `json:"next_question,omitempty"` // nil when session ends
}

// HintResponse is the response for GET /gaming/quiz/sessions/{id}/hint.
type HintResponse struct {
	HintIndex  int    `json:"hint_index"`  // 0-based index of this hint
	Hint       string `json:"hint"`
	GemsSpent  int    `json:"gems_spent"`
	GemsRemaining int `json:"gems_remaining"`
	HasMore    bool   `json:"has_more"`
}
