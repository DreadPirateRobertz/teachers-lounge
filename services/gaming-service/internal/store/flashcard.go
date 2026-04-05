package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/teacherslounge/gaming-service/internal/flashcard"
	"github.com/teacherslounge/gaming-service/internal/model"
)

// CreateFlashcard inserts a new flashcard and returns the fully populated row.
func (s *Store) CreateFlashcard(ctx context.Context, card *model.Flashcard) (*model.Flashcard, error) {
	const q = `
		INSERT INTO flashcards
			(user_id, question_id, session_id, front, back, source, topic, course_id,
			 ease_factor, interval_days, repetitions, next_review_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, user_id, question_id, session_id, front, back, source, topic, course_id,
		          ease_factor, interval_days, repetitions,
		          next_review_at, last_reviewed_at, created_at`

	row := s.db.QueryRow(ctx, q,
		card.UserID, card.QuestionID, card.SessionID,
		card.Front, card.Back, card.Source, card.Topic, card.CourseID,
		card.EaseFactor, card.IntervalDays, card.Repetitions, card.NextReviewAt,
	)
	return scanFlashcard(row)
}

// GetFlashcard retrieves one flashcard by its UUID.
// Returns (nil, nil) when no row is found; other errors indicate a database failure.
func (s *Store) GetFlashcard(ctx context.Context, id string) (*model.Flashcard, error) {
	const q = `
		SELECT id, user_id, question_id, session_id, front, back, source, topic, course_id,
		       ease_factor, interval_days, repetitions,
		       next_review_at, last_reviewed_at, created_at
		FROM flashcards
		WHERE id = $1`

	row := s.db.QueryRow(ctx, q, id)
	card, err := scanFlashcard(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return card, nil
}

// ListFlashcards returns all flashcards for a user, newest first.
func (s *Store) ListFlashcards(ctx context.Context, userID string) ([]*model.Flashcard, error) {
	const q = `
		SELECT id, user_id, question_id, session_id, front, back, source, topic, course_id,
		       ease_factor, interval_days, repetitions,
		       next_review_at, last_reviewed_at, created_at
		FROM flashcards
		WHERE user_id = $1
		ORDER BY created_at DESC`

	rows, err := s.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list flashcards user=%s: %w", userID, err)
	}
	defer rows.Close()
	return collectFlashcards(rows)
}

// DueFlashcards returns cards for a user where next_review_at <= now,
// ordered by next_review_at ascending (most overdue first).
func (s *Store) DueFlashcards(ctx context.Context, userID string) ([]*model.Flashcard, error) {
	const q = `
		SELECT id, user_id, question_id, session_id, front, back, source, topic, course_id,
		       ease_factor, interval_days, repetitions,
		       next_review_at, last_reviewed_at, created_at
		FROM flashcards
		WHERE user_id = $1
		  AND next_review_at <= NOW()
		ORDER BY next_review_at ASC`

	rows, err := s.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("due flashcards user=%s: %w", userID, err)
	}
	defer rows.Close()
	return collectFlashcards(rows)
}

// ReviewFlashcard records a review event and updates SM-2 fields on the flashcard
// atomically within a transaction. It applies the SM-2 algorithm and returns
// the updated card.
func (s *Store) ReviewFlashcard(ctx context.Context, cardID, userID string, quality int) (*model.Flashcard, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin review tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock and fetch the current card state.
	const selectQ = `
		SELECT id, user_id, question_id, session_id, front, back, source, topic, course_id,
		       ease_factor, interval_days, repetitions,
		       next_review_at, last_reviewed_at, created_at
		FROM flashcards
		WHERE id = $1
		FOR UPDATE`

	row := tx.QueryRow(ctx, selectQ, cardID)
	card, err := scanFlashcard(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("flashcard %s not found", cardID)
		}
		return nil, fmt.Errorf("get flashcard for review: %w", err)
	}

	// Apply SM-2 algorithm.
	result := flashcard.Apply(quality, card.EaseFactor, card.IntervalDays, card.Repetitions)

	// Insert review history record.
	const insertReviewQ = `
		INSERT INTO flashcard_reviews
			(flashcard_id, user_id, quality,
			 ease_factor_before, interval_before,
			 ease_factor_after, interval_after)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	if _, err := tx.Exec(ctx, insertReviewQ,
		cardID, userID, quality,
		card.EaseFactor, card.IntervalDays,
		result.EaseFactor, result.IntervalDays,
	); err != nil {
		return nil, fmt.Errorf("insert flashcard review: %w", err)
	}

	// Update the flashcard with new SM-2 values.
	const updateQ = `
		UPDATE flashcards SET
			ease_factor      = $2,
			interval_days    = $3,
			repetitions      = $4,
			next_review_at   = $5,
			last_reviewed_at = NOW()
		WHERE id = $1
		RETURNING id, user_id, question_id, session_id, front, back, source, topic, course_id,
		          ease_factor, interval_days, repetitions,
		          next_review_at, last_reviewed_at, created_at`

	updatedRow := tx.QueryRow(ctx, updateQ,
		cardID,
		result.EaseFactor, result.IntervalDays, result.Repetitions, result.NextReviewAt,
	)
	updated, err := scanFlashcard(updatedRow)
	if err != nil {
		return nil, fmt.Errorf("update flashcard sm2: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit review tx: %w", err)
	}
	return updated, nil
}

// FlashcardsForSession returns all flashcards already created for a given session_id.
// Used to avoid duplicating cards when re-generating from the same session.
func (s *Store) FlashcardsForSession(ctx context.Context, sessionID string) ([]*model.Flashcard, error) {
	const q = `
		SELECT id, user_id, question_id, session_id, front, back, source, topic, course_id,
		       ease_factor, interval_days, repetitions,
		       next_review_at, last_reviewed_at, created_at
		FROM flashcards
		WHERE session_id = $1`

	rows, err := s.db.Query(ctx, q, sessionID)
	if err != nil {
		return nil, fmt.Errorf("flashcards for session=%s: %w", sessionID, err)
	}
	defer rows.Close()
	return collectFlashcards(rows)
}

// AllFlashcardsForExport returns all flashcards for a user with no pagination,
// intended for use in Anki export. Ordered by created_at ascending so the
// export is stable across calls.
func (s *Store) AllFlashcardsForExport(ctx context.Context, userID string) ([]*model.Flashcard, error) {
	const q = `
		SELECT id, user_id, question_id, session_id, front, back, source, topic, course_id,
		       ease_factor, interval_days, repetitions,
		       next_review_at, last_reviewed_at, created_at
		FROM flashcards
		WHERE user_id = $1
		ORDER BY created_at ASC`

	rows, err := s.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("all flashcards export user=%s: %w", userID, err)
	}
	defer rows.Close()
	return collectFlashcards(rows)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// scanFlashcard scans a single flashcard row into a model.Flashcard.
func scanFlashcard(row scanner) (*model.Flashcard, error) {
	c := &model.Flashcard{}
	if err := row.Scan(
		&c.ID, &c.UserID, &c.QuestionID, &c.SessionID,
		&c.Front, &c.Back, &c.Source, &c.Topic, &c.CourseID,
		&c.EaseFactor, &c.IntervalDays, &c.Repetitions,
		&c.NextReviewAt, &c.LastReviewedAt, &c.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan flashcard: %w", err)
	}
	return c, nil
}

// collectFlashcards iterates pgx.Rows and builds a slice of *model.Flashcard.
func collectFlashcards(rows pgx.Rows) ([]*model.Flashcard, error) {
	var cards []*model.Flashcard
	for rows.Next() {
		c, err := scanFlashcard(rows)
		if err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}
