package store_test

// Extra coverage tests targeting the highest-gap functions:
// RecordAssessmentAnswer (happy path), ReviewFlashcard (happy path),
// GetHintIndex (key found), RecordAnswer (commit error).

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/teacherslounge/gaming-service/internal/store"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// newStoreWithRedis creates a Store backed by a miniredis instance and returns
// both so callers can seed Redis state before the function under test is called.
func newStoreWithRedis(t *testing.T, db store.DB) (*store.Store, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return store.New(db, rdb), mr
}

// txWithRowQueue extends mockTx with a FIFO queue of QueryRow responses.
// Each call to QueryRow pops the next row; once exhausted returns errFuncRow.
type txWithRowQueue struct {
	*mockTx
	rows []pgx.Row
	idx  int
}

func (t *txWithRowQueue) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if t.idx >= len(t.rows) {
		return errFuncRow(errors.New("txWithRowQueue: no more rows"))
	}
	r := t.rows[t.idx]
	t.idx++
	return r
}

// ── RecordAssessmentAnswer — happy path (non-last question) ───────────────────

// TestRecordAssessmentAnswer_NotLastQuestion_HappyPath covers the else branch:
// the session has more questions remaining, so only current_index is advanced.
func TestRecordAssessmentAnswer_NotLastQuestion_HappyPath(t *testing.T) {
	// Both QueryRow calls (SELECT FOR UPDATE and UPDATE current_index RETURNING)
	// return the same assessment session row, which is fine for this test.
	tx := &txWithRowQueue{
		mockTx: &mockTx{},
		rows:   []pgx.Row{assessmentSessionScanRow(), assessmentSessionScanRow()},
	}
	s := newMasteryStore(t, &txDB{tx: tx})
	sess, err := s.RecordAssessmentAnswer(context.Background(), "asess-1", "user-1", "q-1", "A")
	if err != nil {
		t.Fatalf("RecordAssessmentAnswer: %v", err)
	}
	if sess.UserID != "user-1" {
		t.Errorf("UserID: got %q, want user-1", sess.UserID)
	}
}

// TestRecordAssessmentAnswer_CommitError_Propagated covers the Commit failure path.
func TestRecordAssessmentAnswer_CommitError_Propagated(t *testing.T) {
	commitErr := errors.New("commit failed")
	tx := &txWithRowQueue{
		mockTx: &mockTx{commitErr: commitErr},
		rows:   []pgx.Row{assessmentSessionScanRow(), assessmentSessionScanRow()},
	}
	s := newMasteryStore(t, &txDB{tx: tx})
	_, err := s.RecordAssessmentAnswer(context.Background(), "asess-1", "user-1", "q-1", "A")
	if !errors.Is(err, commitErr) {
		t.Errorf("expected commit error, got %v", err)
	}
}

// ── ReviewFlashcard — happy path ──────────────────────────────────────────────

// TestReviewFlashcard_HappyPath covers the SELECT → INSERT review → UPDATE path.
func TestReviewFlashcard_HappyPath(t *testing.T) {
	// QueryRow is called twice: SELECT FOR UPDATE and UPDATE RETURNING.
	tx := &txWithRowQueue{
		mockTx: &mockTx{},
		rows:   []pgx.Row{flashcardScanRow(), flashcardScanRow()},
	}
	s := newMasteryStore(t, &txDB{tx: tx})
	card, err := s.ReviewFlashcard(context.Background(), "card-1", "user-1", 4)
	if err != nil {
		t.Fatalf("ReviewFlashcard: %v", err)
	}
	if card.ID != "card-1" {
		t.Errorf("card.ID: got %q, want card-1", card.ID)
	}
}

// TestReviewFlashcard_UpdateError_Propagated covers scan failure on the UPDATE row.
func TestReviewFlashcard_UpdateError_Propagated(t *testing.T) {
	updateErr := errors.New("update failed")
	tx := &txWithRowQueue{
		mockTx: &mockTx{},
		rows:   []pgx.Row{flashcardScanRow(), errFuncRow(updateErr)},
	}
	s := newMasteryStore(t, &txDB{tx: tx})
	_, err := s.ReviewFlashcard(context.Background(), "card-1", "user-1", 4)
	if !errors.Is(err, updateErr) {
		t.Errorf("expected update error, got %v", err)
	}
}

// TestReviewFlashcard_CommitError_Propagated covers Commit failure.
func TestReviewFlashcard_CommitError_Propagated(t *testing.T) {
	commitErr := errors.New("commit failed")
	tx := &txWithRowQueue{
		mockTx: &mockTx{commitErr: commitErr},
		rows:   []pgx.Row{flashcardScanRow(), flashcardScanRow()},
	}
	s := newMasteryStore(t, &txDB{tx: tx})
	_, err := s.ReviewFlashcard(context.Background(), "card-1", "user-1", 4)
	if !errors.Is(err, commitErr) {
		t.Errorf("expected commit error, got %v", err)
	}
}

// ── GetHintIndex — key exists ─────────────────────────────────────────────────

// TestGetHintIndex_ExistingKey_ReturnsParsedCount verifies that an existing key
// in Redis returns the stored integer value.
func TestGetHintIndex_ExistingKey_ReturnsParsedCount(t *testing.T) {
	s, mr := newStoreWithRedis(t, &execDB{})
	// Seed Redis directly via miniredis.
	_ = mr.Set("quiz:hints:sess-1:q-1", "3")
	n, err := s.GetHintIndex(context.Background(), "sess-1", "q-1")
	if err != nil {
		t.Fatalf("GetHintIndex: %v", err)
	}
	if n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
}

// ── RecordAnswer — commit error ───────────────────────────────────────────────

// TestRecordAnswer_CommitError_Propagated covers the Commit failure path in
// RecordAnswer (the quiz session answer transaction).
func TestRecordAnswer_CommitError_Propagated(t *testing.T) {
	commitErr := errors.New("commit quiz answer failed")
	tx := &txWithRowQueue{
		mockTx: &mockTx{commitErr: commitErr},
		rows:   []pgx.Row{quizSessionScanRow()},
	}
	s := newMasteryStore(t, &txDB{tx: tx})
	_, err := s.RecordAnswer(context.Background(), "sess-1", "user-1", "q-1", "A", true, 0, 10, nil)
	if !errors.Is(err, commitErr) {
		t.Errorf("expected commit error, got %v", err)
	}
}
