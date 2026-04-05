package model

import "time"

// Flashcard is a single spaced-repetition card owned by a student.
// SM-2 scheduling fields (EaseFactor, IntervalDays, Repetitions, NextReviewAt)
// are updated each time the card is reviewed.
type Flashcard struct {
	// ID is the UUID primary key of the flashcard.
	ID string `json:"id"`
	// UserID is the UUID of the student who owns this flashcard.
	UserID string `json:"user_id"`
	// QuestionID is the optional UUID of the source question in question_bank.
	QuestionID *string `json:"question_id,omitempty"`
	// SessionID is the optional UUID of the quiz session that generated this card.
	SessionID *string `json:"session_id,omitempty"`
	// Front is the question side of the card.
	Front string `json:"front"`
	// Back is the answer side of the card (correct answer + explanation).
	Back string `json:"back"`
	// Source is either "quiz" (auto-generated from a quiz session) or "manual".
	Source string `json:"source"`
	// Topic is an optional subject tag.
	Topic *string `json:"topic,omitempty"`
	// CourseID is an optional course association UUID.
	CourseID *string `json:"course_id,omitempty"`
	// EaseFactor is the SM-2 ease factor (minimum 1.3, default 2.5).
	EaseFactor float64 `json:"ease_factor"`
	// IntervalDays is the number of days until the next review.
	IntervalDays int `json:"interval_days"`
	// Repetitions is the count of consecutive correct reviews.
	Repetitions int `json:"repetitions"`
	// NextReviewAt is the UTC timestamp when the card is next due for review.
	NextReviewAt time.Time `json:"next_review_at"`
	// LastReviewedAt is the UTC timestamp of the most recent review, or nil if never reviewed.
	LastReviewedAt *time.Time `json:"last_reviewed_at,omitempty"`
	// CreatedAt is the UTC timestamp when the card was created.
	CreatedAt time.Time `json:"created_at"`
}

// FlashcardReview is one SM-2 review event recorded when a student reviews a flashcard.
type FlashcardReview struct {
	// ID is the UUID primary key of the review record.
	ID string `json:"id"`
	// FlashcardID is the UUID of the reviewed flashcard.
	FlashcardID string `json:"flashcard_id"`
	// UserID is the UUID of the student who performed the review.
	UserID string `json:"user_id"`
	// Quality is the student's self-reported recall quality (0–5).
	Quality int `json:"quality"`
	// EaseFactorBefore is the ease factor before this review.
	EaseFactorBefore float64 `json:"ease_factor_before"`
	// IntervalBefore is the interval in days before this review.
	IntervalBefore int `json:"interval_before"`
	// EaseFactorAfter is the ease factor after applying SM-2.
	EaseFactorAfter float64 `json:"ease_factor_after"`
	// IntervalAfter is the interval in days after applying SM-2.
	IntervalAfter int `json:"interval_after"`
	// ReviewedAt is the UTC timestamp of the review event.
	ReviewedAt time.Time `json:"reviewed_at"`
}

// GenerateFlashcardsRequest triggers flashcard generation from a completed quiz session.
type GenerateFlashcardsRequest struct {
	// UserID is the UUID of the student requesting generation.
	UserID string `json:"user_id"`
	// SessionID is the UUID of the completed quiz session to generate cards from.
	SessionID string `json:"session_id"`
}

// GenerateFlashcardsResponse is returned after generation.
type GenerateFlashcardsResponse struct {
	// Created is the number of new flashcards created during this call.
	Created int `json:"created"`
	// Cards is the list of newly created flashcards.
	Cards []*Flashcard `json:"cards"`
}

// ListFlashcardsResponse is returned for GET /gaming/flashcards.
type ListFlashcardsResponse struct {
	// Cards is the list of flashcards for the authenticated user.
	Cards []*Flashcard `json:"cards"`
	// DueCount is the number of cards currently due for review.
	DueCount int `json:"due_count"`
	// Total is the total number of flashcards owned by the user.
	Total int `json:"total"`
}

// ReviewFlashcardRequest is the body for POST /gaming/flashcards/{id}/review.
type ReviewFlashcardRequest struct {
	// UserID is the UUID of the student submitting the review.
	UserID string `json:"user_id"`
	// Quality is the student's self-reported recall quality (0–5).
	Quality int `json:"quality"`
}

// ReviewFlashcardResponse is returned after a review.
type ReviewFlashcardResponse struct {
	// Card is the flashcard with updated SM-2 scheduling fields.
	Card *Flashcard `json:"card"`
	// NextReviewAt is the UTC timestamp when the card is next due.
	NextReviewAt time.Time `json:"next_review_at"`
	// IntervalDays is the new interval in days until the next review.
	IntervalDays int `json:"interval_days"`
}
