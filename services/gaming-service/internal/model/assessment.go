package model

import "time"

// AssessmentSession is a row from assessment_sessions.
type AssessmentSession struct {
	ID             string             `json:"id"`
	UserID         string             `json:"user_id"`
	Status         string             `json:"status"`
	CurrentIndex   int                `json:"current_index"`
	TotalQuestions int                `json:"total_questions"`
	XPEarned       int                `json:"xp_earned"`
	Results        map[string]float64 `json:"results,omitempty"` // nil until completed
	StartedAt      time.Time          `json:"started_at"`
	CompletedAt    *time.Time         `json:"completed_at,omitempty"`
}

// StartAssessmentRequest is the body for POST /gaming/assessment/start.
type StartAssessmentRequest struct {
	UserID string `json:"user_id"`
}

// AssessmentQuestion is the client-safe view of a question (no weights).
type AssessmentQuestion struct {
	ID        string                   `json:"id"`
	Index     int                      `json:"index"`   // 0-based position in the sequence
	Total     int                      `json:"total"`   // total question count
	Dimension string                   `json:"dimension"`
	Stem      string                   `json:"stem"`
	Options   []AssessmentQuestionOpt  `json:"options"`
}

// AssessmentQuestionOpt is a client-safe option (key + text only, no weight).
type AssessmentQuestionOpt struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

// StartAssessmentResponse is the response for POST /gaming/assessment/start.
type StartAssessmentResponse struct {
	Session  *AssessmentSession  `json:"session"`
	Question *AssessmentQuestion `json:"question"`
}

// AssessmentSessionResponse is the response for GET /gaming/assessment/sessions/{id}.
type AssessmentSessionResponse struct {
	Session  *AssessmentSession  `json:"session"`
	Question *AssessmentQuestion `json:"question,omitempty"` // nil when completed
}

// SubmitAssessmentAnswerRequest is the body for POST /gaming/assessment/sessions/{id}/answer.
type SubmitAssessmentAnswerRequest struct {
	UserID     string `json:"user_id"`
	QuestionID string `json:"question_id"`
	ChosenKey  string `json:"chosen_key"`
}

// SubmitAssessmentAnswerResponse is the response for POST /gaming/assessment/sessions/{id}/answer.
type SubmitAssessmentAnswerResponse struct {
	Session      *AssessmentSession  `json:"session"`
	NextQuestion *AssessmentQuestion `json:"next_question,omitempty"` // nil when assessment ends
	XPEarned     int                 `json:"xp_earned,omitempty"`     // >0 only on final answer
}
