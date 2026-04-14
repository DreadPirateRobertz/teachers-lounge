package store_test

// Additional coverage tests to close the gap to ≥90% total coverage.
// Focuses on: scanAssessmentSession with results, battle Redis errors,
// GetFlashcard non-ErrNoRows error, StreakCheckin reset path.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ── scanAssessmentSession — results JSON path ─────────────────────────────────

// assessmentSessionWithResultsScan returns a row where resultsRaw is a valid
// JSON blob, exercising the json.Unmarshal branch in scanAssessmentSession.
func assessmentSessionWithResultsScan() pgx.Row {
	now := time.Now()
	// Use a simple map as results placeholder.
	resultsJSON, _ := json.Marshal(map[string]float64{"visual": 0.5, "verbal": -0.3})
	return &funcRow{fn: func(dest ...any) error {
		*(dest[0].(*string))   = "asess-2"
		*(dest[1].(*string))   = "user-2"
		*(dest[2].(*string))   = "completed"
		*(dest[3].(*int))      = 4
		*(dest[4].(*int))      = 4
		*(dest[5].(*int))      = 100
		*(dest[6].(*[]byte))   = resultsJSON
		*(dest[7].(*time.Time)) = now
		*(dest[8].(**time.Time)) = nil
		return nil
	}}
}

// TestGetAssessmentSession_WithResults_UnmarshalsOK verifies that a completed
// session row with a non-null results blob is correctly unmarshalled.
func TestGetAssessmentSession_WithResults_UnmarshalsOK(t *testing.T) {
	db := &rowQueueDB{rows: []pgx.Row{assessmentSessionWithResultsScan()}}
	s := newMasteryStore(t, db)
	sess, err := s.GetAssessmentSession(context.Background(), "asess-2")
	if err != nil {
		t.Fatalf("GetAssessmentSession: %v", err)
	}
	if sess.ID != "asess-2" {
		t.Errorf("ID: got %q, want asess-2", sess.ID)
	}
}

// ── GetFlashcard — non-ErrNoRows error ───────────────────────────────────────

// TestGetFlashcard_ScanError_Propagated verifies that scan errors other than
// ErrNoRows are returned as errors (not silently swallowed).
func TestGetFlashcard_ScanError_Propagated(t *testing.T) {
	scanErr := errors.New("column type mismatch")
	db := &rowQueueDB{rows: []pgx.Row{errFuncRow(scanErr)}}
	s := newMasteryStore(t, db)
	_, err := s.GetFlashcard(context.Background(), "card-1")
	if !errors.Is(err, scanErr) {
		t.Errorf("expected scan error, got %v", err)
	}
}

// ── Battle Redis error paths ──────────────────────────────────────────────────

// TestSaveBattleSession_RedisError_Propagated verifies that a Redis Set failure
// is wrapped and returned.
func TestSaveBattleSession_RedisError_Propagated(t *testing.T) {
	s, mr := newStoreWithRedis(t, &execDB{})
	mr.Close() // force connection failure
	sess := battleSession("s-err")
	sess.ExpiresAt = time.Now().Add(60 * time.Second)
	err := s.SaveBattleSession(context.Background(), sess)
	if err == nil {
		t.Fatal("expected Redis error from SaveBattleSession, got nil")
	}
}

// TestGetBattleSession_RedisError_Propagated verifies that a non-nil Redis
// error (not redis.Nil) from Get is wrapped and returned.
func TestGetBattleSession_RedisError_Propagated(t *testing.T) {
	s, mr := newStoreWithRedis(t, &execDB{})
	mr.Close()
	_, err := s.GetBattleSession(context.Background(), "s-err")
	if err == nil {
		t.Fatal("expected Redis error from GetBattleSession, got nil")
	}
}

// TestDeleteBattleSession_RedisError_Propagated verifies that a Redis Del
// failure is wrapped and returned.
func TestDeleteBattleSession_RedisError_Propagated(t *testing.T) {
	s, mr := newStoreWithRedis(t, &execDB{})
	mr.Close()
	err := s.DeleteBattleSession(context.Background(), "s-err")
	if err == nil {
		t.Fatal("expected Redis error from DeleteBattleSession, got nil")
	}
}

// ── StreakCheckin — reset path ────────────────────────────────────────────────

// TestStreakCheckin_ResetPath_GapExceedsWindow verifies that when the stored
// last_ts is more than 24 hours ago, the streak resets to 1 with reset=true.
func TestStreakCheckin_ResetPath_GapExceedsWindow(t *testing.T) {
	s, mr := newStoreWithRedis(t, &rowQueueDB{rows: []pgx.Row{&battleIntRow{value: 1}}})

	// Seed Redis with a last_ts that is 26 hours in the past.
	oldTS := time.Now().Add(-26 * time.Hour).Unix()
	mr.HSet("streak:user-reset", "count", "5", "last_ts", fmt.Sprintf("%d", oldTS))

	current, _, reset, err := s.StreakCheckin(context.Background(), "user-reset")
	if err != nil {
		t.Fatalf("StreakCheckin: %v", err)
	}
	if !reset {
		t.Error("expected reset=true for gap > 24h")
	}
	if current != 1 {
		t.Errorf("current: got %d, want 1 after reset", current)
	}
}

// ── FlashcardsForSession / AllFlashcardsForExport — query error paths ─────────

// TestFlashcardsForSession_QueryError_Propagated verifies db.Query failures surface.
func TestFlashcardsForSession_QueryError_Propagated(t *testing.T) {
	queryErr := errors.New("session query failed")
	db := &flashcardQueryDB{queryErr: queryErr}
	s := newMasteryStore(t, db)
	_, err := s.FlashcardsForSession(context.Background(), "sess-1")
	if !errors.Is(err, queryErr) {
		t.Errorf("expected query error, got %v", err)
	}
}

// TestAllFlashcardsForExport_QueryError_Propagated verifies db.Query failures surface.
func TestAllFlashcardsForExport_QueryError_Propagated(t *testing.T) {
	queryErr := errors.New("export query failed")
	db := &flashcardQueryDB{queryErr: queryErr}
	s := newMasteryStore(t, db)
	_, err := s.AllFlashcardsForExport(context.Background(), "user-1")
	if !errors.Is(err, queryErr) {
		t.Errorf("expected query error, got %v", err)
	}
}

// ── LeaderboardUpdateCourse — Redis error path ────────────────────────────────

// TestLeaderboardUpdateCourse_RedisError_Propagated verifies that a ZAdd failure
// is wrapped and returned.
func TestLeaderboardUpdateCourse_RedisError_Propagated(t *testing.T) {
	s, mr := newStoreWithRedis(t, &execDB{})
	mr.Close()
	err := s.LeaderboardUpdateCourse(context.Background(), "user-1", "course-1", 50)
	if err == nil {
		t.Fatal("expected Redis error from LeaderboardUpdateCourse, got nil")
	}
}

// ── scanQuestion — options unmarshal error ────────────────────────────────────

// questionWithBadOptionsRow returns a scan row where optionsRaw is malformed JSON,
// triggering the json.Unmarshal error path in scanQuestion.
func questionWithBadOptionsRow() pgx.Row {
	courseID := "course-1"
	return &funcRow{fn: func(dest ...any) error {
		*(dest[0].(*string))  = "q-bad"
		*(dest[1].(**string)) = &courseID
		*(dest[2].(*string))  = "math"
		*(dest[3].(*int))     = 1
		*(dest[4].(*string))  = "Bad question"
		*(dest[5].(*[]byte))  = []byte("not valid json") // bad options
		*(dest[6].(*string))  = "A"
		*(dest[7].(*[]byte))  = []byte(`["hint"]`)
		*(dest[8].(*string))  = "explanation"
		*(dest[9].(*int))     = 5
		return nil
	}}
}

// TestGetQuestion_BadOptionsJSON_ReturnsError verifies that malformed options JSON
// is detected and returned as an error rather than silently corrupting data.
func TestGetQuestion_BadOptionsJSON_ReturnsError(t *testing.T) {
	db := &quizQueryDB{qrow: questionWithBadOptionsRow()}
	s := newMasteryStore(t, db)
	_, err := s.GetQuestion(context.Background(), "q-bad")
	if err == nil {
		t.Fatal("expected unmarshal error for bad options JSON, got nil")
	}
}

// ── collectFlashcards — scan error path ───────────────────────────────────────

// flashcardErrRows implements pgx.Rows for a single row whose Scan always errors.
type flashcardErrRows struct {
	used    bool
	scanErr error
}

func (r *flashcardErrRows) Close()                                         {}
func (r *flashcardErrRows) Err() error                                     { return nil }
func (r *flashcardErrRows) CommandTag() pgconn.CommandTag                  { return pgconn.CommandTag{} }
func (r *flashcardErrRows) FieldDescriptions() []pgconn.FieldDescription   { return nil }
func (r *flashcardErrRows) Conn() *pgx.Conn                                { return nil }
func (r *flashcardErrRows) Next() bool {
	if r.used {
		return false
	}
	r.used = true
	return true
}
func (r *flashcardErrRows) Scan(_ ...any) error    { return r.scanErr }
func (r *flashcardErrRows) Values() ([]any, error) { return nil, nil }
func (r *flashcardErrRows) RawValues() [][]byte    { return nil }

// TestFlashcardsForSession_ScanError_Propagated verifies that a scan error
// inside collectFlashcards is surfaced rather than silently dropped.
func TestFlashcardsForSession_ScanError_Propagated(t *testing.T) {
	scanErr := errors.New("flashcard scan failed")
	rows := &flashcardErrRows{scanErr: scanErr}
	db := &flashcardQueryDB{qrows: rows}
	s := newMasteryStore(t, db)
	_, err := s.FlashcardsForSession(context.Background(), "sess-1")
	if err == nil {
		t.Fatal("expected scan error from FlashcardsForSession, got nil")
	}
}
