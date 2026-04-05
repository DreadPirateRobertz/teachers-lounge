package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/teacherslounge/gaming-service/internal/model"
)

const hintKeyFmt = "quiz:hints:%s:%s" // quiz:hints:{sessionID}:{questionID}

// ErrNoGems is returned by IncrHintIndex when the user has no gems to spend.
var ErrNoGems = errors.New("insufficient gems")

// GetRandomQuestions returns up to n questions for a topic, ordered randomly.
// If fewer than n questions exist for the topic, it returns all of them.
func (s *Store) GetRandomQuestions(ctx context.Context, topic string, n int) ([]*model.Question, error) {
	const q = `
		SELECT id, course_id, topic, difficulty, question,
		       options, correct_key, hints, explanation, xp_reward
		FROM question_bank
		WHERE topic = $1
		ORDER BY RANDOM()
		LIMIT $2`

	rows, err := s.db.Query(ctx, q, topic, n)
	if err != nil {
		return nil, fmt.Errorf("get random questions topic=%s: %w", topic, err)
	}
	defer rows.Close()

	var questions []*model.Question
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, rows.Err()
}

// GetQuestion returns a single question by ID.
func (s *Store) GetQuestion(ctx context.Context, questionID string) (*model.Question, error) {
	const q = `
		SELECT id, course_id, topic, difficulty, question,
		       options, correct_key, hints, explanation, xp_reward
		FROM question_bank
		WHERE id = $1`

	row := s.db.QueryRow(ctx, q, questionID)
	return scanQuestion(row)
}

// CreateQuizSession inserts a new quiz session and returns it.
func (s *Store) CreateQuizSession(ctx context.Context, userID string, topic *string, courseID *string, questionIDs []string) (*model.QuizSession, error) {
	pgIDs := questionIDs // pgx handles []string → uuid[] natively

	const q = `
		INSERT INTO quiz_sessions
			(user_id, topic, course_id, question_ids, total_questions)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, course_id, topic, status,
		          question_ids, current_index, total_questions,
		          correct_count, total_xp_earned, started_at, completed_at`

	row := s.db.QueryRow(ctx, q, userID, topic, courseID, pgIDs, len(questionIDs))
	return scanSession(row)
}

// GetQuizSession retrieves a quiz session by ID.
func (s *Store) GetQuizSession(ctx context.Context, sessionID string) (*model.QuizSession, error) {
	const q = `
		SELECT id, user_id, course_id, topic, status,
		       question_ids, current_index, total_questions,
		       correct_count, total_xp_earned, started_at, completed_at
		FROM quiz_sessions
		WHERE id = $1`

	row := s.db.QueryRow(ctx, q, sessionID)
	return scanSession(row)
}

// RecordAnswer inserts a quiz_answers row and advances the session.
// It returns the updated session after applying correctness + XP.
func (s *Store) RecordAnswer(ctx context.Context, sessionID, userID, questionID, chosenKey string, isCorrect bool, hintsUsed, xpEarned int, timeMs *int) (*model.QuizSession, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Insert answer row.
	const insertQ = `
		INSERT INTO quiz_answers
			(session_id, user_id, question_id, chosen_key, is_correct, hints_used, xp_earned, time_taken_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	if _, err := tx.Exec(ctx, insertQ, sessionID, userID, questionID, chosenKey, isCorrect, hintsUsed, xpEarned, timeMs); err != nil {
		return nil, fmt.Errorf("insert quiz answer: %w", err)
	}

	// Advance session: increment current_index, correct_count, total_xp_earned.
	// If current_index reaches total_questions, mark completed.
	const updateQ = `
		UPDATE quiz_sessions SET
			current_index   = current_index + 1,
			correct_count   = correct_count + $2,
			total_xp_earned = total_xp_earned + $3,
			status = CASE
				WHEN current_index + 1 >= total_questions THEN 'completed'::quiz_status
				ELSE status
			END,
			completed_at = CASE
				WHEN current_index + 1 >= total_questions THEN NOW()
				ELSE completed_at
			END
		WHERE id = $1
		RETURNING id, user_id, course_id, topic, status,
		          question_ids, current_index, total_questions,
		          correct_count, total_xp_earned, started_at, completed_at`

	correctInt := 0
	if isCorrect {
		correctInt = 1
	}
	row := tx.QueryRow(ctx, updateQ, sessionID, correctInt, xpEarned)
	session, err := scanSession(row)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit answer tx: %w", err)
	}
	return session, nil
}

// AbandonQuizSession marks a session as abandoned.
func (s *Store) AbandonQuizSession(ctx context.Context, sessionID string) error {
	const q = `UPDATE quiz_sessions SET status = 'abandoned', completed_at = NOW() WHERE id = $1`
	_, err := s.db.Exec(ctx, q, sessionID)
	return err
}

// GetHintIndex returns how many hints the user has already consumed for this
// question in this session (stored in Redis as an ephemeral counter).
func (s *Store) GetHintIndex(ctx context.Context, sessionID, questionID string) (int, error) {
	key := fmt.Sprintf(hintKeyFmt, sessionID, questionID)
	val, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get hint index: %w", err)
	}
	n, _ := strconv.Atoi(val)
	return n, nil
}

// IncrHintIndex atomically increments the hint counter for a question and
// deducts 1 gem from the user's gaming_profile. Returns the new index and
// the user's remaining gems. Returns an error if the user has no gems.
func (s *Store) IncrHintIndex(ctx context.Context, sessionID, questionID, userID string) (newIndex, gemsRemaining int, err error) {
	// Deduct gem in Postgres (atomic: only succeeds if gems > 0).
	const deductQ = `
		UPDATE gaming_profiles
		SET gems = gems - 1
		WHERE user_id = $1 AND gems > 0
		RETURNING gems`

	if err = s.db.QueryRow(ctx, deductQ, userID).Scan(&gemsRemaining); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, ErrNoGems
		}
		return 0, 0, fmt.Errorf("deduct gem for hint: %w", err)
	}

	// Increment Redis counter (no TTL — session lifetime is short).
	key := fmt.Sprintf(hintKeyFmt, sessionID, questionID)
	newVal, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, 0, fmt.Errorf("incr hint index: %w", err)
	}
	return int(newVal) - 1, gemsRemaining, nil // return 0-based index
}

// --- helpers ---

type scanner interface {
	Scan(dest ...any) error
}

func scanQuestion(row scanner) (*model.Question, error) {
	q := &model.Question{}
	var optionsRaw, hintsRaw []byte
	var courseID *string

	if err := row.Scan(
		&q.ID, &courseID, &q.Topic, &q.Difficulty, &q.Question,
		&optionsRaw, &q.CorrectKey, &hintsRaw, &q.Explanation, &q.XPReward,
	); err != nil {
		return nil, fmt.Errorf("scan question: %w", err)
	}
	q.CourseID = courseID

	if err := json.Unmarshal(optionsRaw, &q.Options); err != nil {
		return nil, fmt.Errorf("unmarshal options: %w", err)
	}
	if err := json.Unmarshal(hintsRaw, &q.Hints); err != nil {
		return nil, fmt.Errorf("unmarshal hints: %w", err)
	}
	return q, nil
}

func scanSession(row scanner) (*model.QuizSession, error) {
	s := &model.QuizSession{}
	var pgIDs []string // pgx scans uuid[] into []string

	if err := row.Scan(
		&s.ID, &s.UserID, &s.CourseID, &s.Topic, &s.Status,
		&pgIDs, &s.CurrentIndex, &s.TotalQuestions,
		&s.CorrectCount, &s.TotalXPEarned, &s.StartedAt, &s.CompletedAt,
	); err != nil {
		return nil, fmt.Errorf("scan quiz session: %w", err)
	}
	s.QuestionIDs = pgIDs
	return s, nil
}
