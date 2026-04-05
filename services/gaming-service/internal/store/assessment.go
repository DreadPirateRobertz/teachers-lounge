package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/teacherslounge/gaming-service/internal/assessment"
	"github.com/teacherslounge/gaming-service/internal/model"
)

// CreateAssessmentSession inserts a new assessment session for the user and
// returns the newly created session.
func (s *Store) CreateAssessmentSession(ctx context.Context, userID string) (*model.AssessmentSession, error) {
	const q = `
		INSERT INTO assessment_sessions (user_id, total_questions)
		VALUES ($1, $2)
		RETURNING id, user_id, status, current_index, total_questions,
		          xp_earned, results, started_at, completed_at`

	row := s.db.QueryRow(ctx, q, userID, assessment.TotalCount)
	return scanAssessmentSession(row)
}

// GetAssessmentSession retrieves a session by ID.
func (s *Store) GetAssessmentSession(ctx context.Context, sessionID string) (*model.AssessmentSession, error) {
	const q = `
		SELECT id, user_id, status, current_index, total_questions,
		       xp_earned, results, started_at, completed_at
		FROM assessment_sessions
		WHERE id = $1`

	row := s.db.QueryRow(ctx, q, sessionID)
	return scanAssessmentSession(row)
}

// RecordAssessmentAnswer persists one answer and advances the session index.
// When the last question is answered it marks the session completed, computes
// the Felder-Silverman dials, stores them in results, and awards XP.
// It returns the updated session.
func (s *Store) RecordAssessmentAnswer(ctx context.Context, sessionID, userID, questionID, chosenKey string) (*model.AssessmentSession, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Insert answer row.
	const insertQ = `
		INSERT INTO assessment_answers (session_id, user_id, question_id, chosen_key)
		VALUES ($1, $2, $3, $4)`
	if _, err := tx.Exec(ctx, insertQ, sessionID, userID, questionID, chosenKey); err != nil {
		return nil, fmt.Errorf("insert assessment answer: %w", err)
	}

	// Fetch current session state (within tx to lock the row).
	const selectQ = `
		SELECT id, user_id, status, current_index, total_questions,
		       xp_earned, results, started_at, completed_at
		FROM assessment_sessions
		WHERE id = $1
		FOR UPDATE`

	row := tx.QueryRow(ctx, selectQ, sessionID)
	sess, err := scanAssessmentSession(row)
	if err != nil {
		return nil, err
	}

	newIndex := sess.CurrentIndex + 1
	isLast := newIndex >= sess.TotalQuestions

	if isLast {
		// Collect all answers for this session to compute dials.
		const answersQ = `SELECT question_id, chosen_key FROM assessment_answers WHERE session_id = $1`
		rows, err := tx.Query(ctx, answersQ, sessionID)
		if err != nil {
			return nil, fmt.Errorf("fetch answers for dials: %w", err)
		}
		answers := make(map[string]string, sess.TotalQuestions)
		for rows.Next() {
			var qid, key string
			if err := rows.Scan(&qid, &key); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan answer row: %w", err)
			}
			answers[qid] = key
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate answer rows: %w", err)
		}

		dials := assessment.ComputeDials(answers)
		dialsJSON, err := json.Marshal(dials)
		if err != nil {
			return nil, fmt.Errorf("marshal dials: %w", err)
		}

		const xpReward = 100 // XP awarded for completing the assessment

		const updateQ = `
			UPDATE assessment_sessions SET
				current_index = $2,
				status        = 'completed',
				xp_earned     = $3,
				results       = $4,
				completed_at  = NOW()
			WHERE id = $1
			RETURNING id, user_id, status, current_index, total_questions,
			          xp_earned, results, started_at, completed_at`

		row := tx.QueryRow(ctx, updateQ, sessionID, newIndex, xpReward, dialsJSON)
		sess, err = scanAssessmentSession(row)
		if err != nil {
			return nil, err
		}
	} else {
		const updateQ = `
			UPDATE assessment_sessions SET current_index = $2
			WHERE id = $1
			RETURNING id, user_id, status, current_index, total_questions,
			          xp_earned, results, started_at, completed_at`

		row := tx.QueryRow(ctx, updateQ, sessionID, newIndex)
		sess, err = scanAssessmentSession(row)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit assessment answer tx: %w", err)
	}
	return sess, nil
}

// ─── helpers ────────────────────────────────────────────────────────────────

func scanAssessmentSession(row scanner) (*model.AssessmentSession, error) {
	s := &model.AssessmentSession{}
	var resultsRaw []byte

	if err := row.Scan(
		&s.ID, &s.UserID, &s.Status, &s.CurrentIndex, &s.TotalQuestions,
		&s.XPEarned, &resultsRaw, &s.StartedAt, &s.CompletedAt,
	); err != nil {
		return nil, fmt.Errorf("scan assessment session: %w", err)
	}

	if resultsRaw != nil {
		if err := json.Unmarshal(resultsRaw, &s.Results); err != nil {
			return nil, fmt.Errorf("unmarshal assessment results: %w", err)
		}
	}
	return s, nil
}
