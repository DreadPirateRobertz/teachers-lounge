package store_test

// Tests for quiz.go — GetRandomQuestions, GetQuestion, CreateQuizSession,
// GetQuizSession, AbandonQuizSession, GetHintIndex, IncrHintIndex, RecordAnswer.

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/teacherslounge/gaming-service/internal/store"
)

// ── row/rows helpers ──────────────────────────────────────────────────────────

// questionScanRow returns a funcRow that scans 10 fields for a Question.
func questionScanRow() pgx.Row {
	optJSON, _ := json.Marshal([]map[string]string{{"key": "A", "text": "yes"}, {"key": "B", "text": "no"}})
	hintsJSON, _ := json.Marshal([]string{"hint1"})
	courseID := "course-1"
	return &funcRow{fn: func(dest ...any) error {
		*(dest[0].(*string))  = "q-1"
		*(dest[1].(**string)) = &courseID
		*(dest[2].(*string))  = "algebra"
		*(dest[3].(*int))     = 2
		*(dest[4].(*string))  = "What is 1+1?"
		*(dest[5].(*[]byte))  = optJSON
		*(dest[6].(*string))  = "A"
		*(dest[7].(*[]byte))  = hintsJSON
		*(dest[8].(*string))  = "Basic math"
		*(dest[9].(*int))     = 10
		return nil
	}}
}

// quizSessionScanRow returns a funcRow that scans 12 fields for a QuizSession.
func quizSessionScanRow() pgx.Row {
	now := time.Now()
	return &funcRow{fn: func(dest ...any) error {
		*(dest[0].(*string))    = "sess-1"
		*(dest[1].(*string))    = "user-1"
		*(dest[2].(**string))   = nil
		*(dest[3].(**string))   = nil
		*(dest[4].(*string))    = "in_progress"
		*(dest[5].(*[]string))  = []string{"q-1", "q-2"}
		*(dest[6].(*int))       = 0
		*(dest[7].(*int))       = 2
		*(dest[8].(*int))       = 0
		*(dest[9].(*int))       = 0
		*(dest[10].(*time.Time))  = now
		*(dest[11].(**time.Time)) = nil
		return nil
	}}
}

// questionRowsStub implements pgx.Rows for a single question row.
type questionRowsStub struct {
	row    pgx.Row
	used   bool
	rowErr error
}

func (r *questionRowsStub) Close()                                          {}
func (r *questionRowsStub) Err() error                                      { return r.rowErr }
func (r *questionRowsStub) CommandTag() pgconn.CommandTag                   { return pgconn.CommandTag{} }
func (r *questionRowsStub) FieldDescriptions() []pgconn.FieldDescription    { return nil }
func (r *questionRowsStub) Next() bool {
	if r.used {
		return false
	}
	r.used = true
	return true
}
func (r *questionRowsStub) Scan(dest ...any) error    { return r.row.Scan(dest...) }
func (r *questionRowsStub) Values() ([]any, error)    { return nil, nil }
func (r *questionRowsStub) RawValues() [][]byte       { return nil }
func (r *questionRowsStub) Conn() *pgx.Conn           { return nil }

// quizQueryDB routes calls to configurable stubs.
type quizQueryDB struct {
	qrows   pgx.Rows
	qrow    pgx.Row
	execErr error
	tx      pgx.Tx
	txErr   error
}

func (d *quizQueryDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if d.qrow != nil {
		return d.qrow
	}
	return errFuncRow(errors.New("no qrow configured"))
}
func (d *quizQueryDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if d.qrows != nil {
		return d.qrows, nil
	}
	return nil, errors.New("query error")
}
func (d *quizQueryDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, d.execErr
}
func (d *quizQueryDB) Begin(_ context.Context) (pgx.Tx, error) {
	if d.txErr != nil {
		return nil, d.txErr
	}
	return d.tx, nil
}

// ── GetRandomQuestions ────────────────────────────────────────────────────────

// TestGetRandomQuestions_HappyPath verifies a single question row is returned.
func TestGetRandomQuestions_HappyPath(t *testing.T) {
	stub := &questionRowsStub{row: questionScanRow()}
	db := &quizQueryDB{qrows: stub}
	s := newMasteryStore(t, db)
	qs, err := s.GetRandomQuestions(context.Background(), "algebra", 1)
	if err != nil {
		t.Fatalf("GetRandomQuestions: %v", err)
	}
	if len(qs) != 1 {
		t.Fatalf("expected 1 question, got %d", len(qs))
	}
	if qs[0].ID != "q-1" {
		t.Errorf("ID: got %q, want q-1", qs[0].ID)
	}
}

// TestGetRandomQuestions_QueryError_Propagated verifies DB errors are returned.
func TestGetRandomQuestions_QueryError_Propagated(t *testing.T) {
	db := &quizQueryDB{} // nil qrows → error path
	s := newMasteryStore(t, db)
	_, err := s.GetRandomQuestions(context.Background(), "algebra", 1)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── GetQuestion ───────────────────────────────────────────────────────────────

// TestGetQuestion_HappyPath verifies a question is returned by ID.
func TestGetQuestion_HappyPath(t *testing.T) {
	db := &quizQueryDB{qrow: questionScanRow()}
	s := newMasteryStore(t, db)
	q, err := s.GetQuestion(context.Background(), "q-1")
	if err != nil {
		t.Fatalf("GetQuestion: %v", err)
	}
	if q.ID != "q-1" {
		t.Errorf("ID: got %q, want q-1", q.ID)
	}
	if q.XPReward != 10 {
		t.Errorf("XPReward: got %d, want 10", q.XPReward)
	}
}

// TestGetQuestion_NotFound_ReturnsError verifies scan errors are propagated.
func TestGetQuestion_NotFound_ReturnsError(t *testing.T) {
	db := &quizQueryDB{qrow: errFuncRow(pgx.ErrNoRows)}
	s := newMasteryStore(t, db)
	_, err := s.GetQuestion(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error for missing question, got nil")
	}
}

// ── CreateQuizSession ─────────────────────────────────────────────────────────

// TestCreateQuizSession_HappyPath verifies a session row is inserted and returned.
func TestCreateQuizSession_HappyPath(t *testing.T) {
	db := &quizQueryDB{qrow: quizSessionScanRow()}
	s := newMasteryStore(t, db)
	sess, err := s.CreateQuizSession(context.Background(), "user-1", nil, nil, []string{"q-1", "q-2"})
	if err != nil {
		t.Fatalf("CreateQuizSession: %v", err)
	}
	if sess.UserID != "user-1" {
		t.Errorf("UserID: got %q, want user-1", sess.UserID)
	}
	if sess.TotalQuestions != 2 {
		t.Errorf("TotalQuestions: got %d, want 2", sess.TotalQuestions)
	}
}

// TestCreateQuizSession_DBError_Propagated verifies scan errors are returned.
func TestCreateQuizSession_DBError_Propagated(t *testing.T) {
	dbErr := errors.New("insert failed")
	db := &quizQueryDB{qrow: errFuncRow(dbErr)}
	s := newMasteryStore(t, db)
	_, err := s.CreateQuizSession(context.Background(), "user-1", nil, nil, []string{"q-1"})
	if !errors.Is(err, dbErr) {
		t.Errorf("expected wrapped DB error, got %v", err)
	}
}

// ── GetQuizSession ────────────────────────────────────────────────────────────

// TestGetQuizSession_HappyPath verifies a session is returned by ID.
func TestGetQuizSession_HappyPath(t *testing.T) {
	db := &quizQueryDB{qrow: quizSessionScanRow()}
	s := newMasteryStore(t, db)
	sess, err := s.GetQuizSession(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("GetQuizSession: %v", err)
	}
	if sess.ID != "sess-1" {
		t.Errorf("ID: got %q, want sess-1", sess.ID)
	}
}

// TestGetQuizSession_NotFound_ReturnsError verifies ErrNoRows is returned.
func TestGetQuizSession_NotFound_ReturnsError(t *testing.T) {
	db := &quizQueryDB{qrow: errFuncRow(pgx.ErrNoRows)}
	s := newMasteryStore(t, db)
	_, err := s.GetQuizSession(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error for missing session, got nil")
	}
}

// ── AbandonQuizSession ────────────────────────────────────────────────────────

// TestAbandonQuizSession_HappyPath verifies Exec is called with no error.
func TestAbandonQuizSession_HappyPath(t *testing.T) {
	db := &quizQueryDB{}
	s := newMasteryStore(t, db)
	if err := s.AbandonQuizSession(context.Background(), "sess-1"); err != nil {
		t.Errorf("AbandonQuizSession: unexpected error: %v", err)
	}
}

// TestAbandonQuizSession_DBError_Propagated verifies Exec errors are returned.
func TestAbandonQuizSession_DBError_Propagated(t *testing.T) {
	execErr := errors.New("update failed")
	db := &quizQueryDB{execErr: execErr}
	s := newMasteryStore(t, db)
	err := s.AbandonQuizSession(context.Background(), "sess-1")
	if !errors.Is(err, execErr) {
		t.Errorf("expected DB error, got %v", err)
	}
}

// ── GetHintIndex ──────────────────────────────────────────────────────────────

// TestGetHintIndex_MissingKey_ReturnsZero verifies Redis Nil returns 0, nil.
func TestGetHintIndex_MissingKey_ReturnsZero(t *testing.T) {
	s := newMasteryStore(t, &execDB{}) // miniredis with empty state
	n, err := s.GetHintIndex(context.Background(), "sess-1", "q-1")
	if err != nil {
		t.Fatalf("GetHintIndex: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

// ── IncrHintIndex ─────────────────────────────────────────────────────────────

// TestIncrHintIndex_HappyPath verifies gem deduction and counter increment.
func TestIncrHintIndex_HappyPath(t *testing.T) {
	db := &rowQueueDB{rows: []pgx.Row{&intRow{value: 4}}}
	s := newMasteryStore(t, db)
	idx, gems, err := s.IncrHintIndex(context.Background(), "sess-1", "q-1", "user-1")
	if err != nil {
		t.Fatalf("IncrHintIndex: %v", err)
	}
	if gems != 4 {
		t.Errorf("gems: got %d, want 4", gems)
	}
	if idx != 0 {
		t.Errorf("idx: got %d, want 0", idx)
	}
}

// TestIncrHintIndex_NoGems_ReturnsErrNoGems verifies ErrNoRows → ErrNoGems.
func TestIncrHintIndex_NoGems_ReturnsErrNoGems(t *testing.T) {
	db := &rowQueueDB{rows: []pgx.Row{errFuncRow(pgx.ErrNoRows)}}
	s := newMasteryStore(t, db)
	_, _, err := s.IncrHintIndex(context.Background(), "sess-1", "q-1", "user-1")
	if !errors.Is(err, store.ErrNoGems) {
		t.Errorf("expected ErrNoGems, got %v", err)
	}
}

// TestIncrHintIndex_DBError_Propagated verifies other DB errors are wrapped.
func TestIncrHintIndex_DBError_Propagated(t *testing.T) {
	dbErr := errors.New("deduct gem failed")
	db := &rowQueueDB{rows: []pgx.Row{errFuncRow(dbErr)}}
	s := newMasteryStore(t, db)
	_, _, err := s.IncrHintIndex(context.Background(), "sess-1", "q-1", "user-1")
	if !errors.Is(err, dbErr) {
		t.Errorf("expected wrapped DB error, got %v", err)
	}
}

// ── RecordAnswer ──────────────────────────────────────────────────────────────

// TestRecordAnswer_BeginError_Propagated verifies Begin failures surface.
func TestRecordAnswer_BeginError_Propagated(t *testing.T) {
	beginErr := errors.New("pool exhausted")
	db := &txDB{txErr: beginErr}
	s := newMasteryStore(t, db)
	_, err := s.RecordAnswer(context.Background(), "sess-1", "user-1", "q-1", "A", true, 0, 10, nil)
	if !errors.Is(err, beginErr) {
		t.Errorf("expected begin error, got %v", err)
	}
}

// TestRecordAnswer_ExecError_Propagated verifies INSERT answer failures surface.
func TestRecordAnswer_ExecError_Propagated(t *testing.T) {
	execErr := errors.New("insert answer failed")
	tx := &mockTx{execErr: execErr}
	db := &txDB{tx: tx}
	s := newMasteryStore(t, db)
	_, err := s.RecordAnswer(context.Background(), "sess-1", "user-1", "q-1", "A", true, 0, 10, nil)
	if err == nil {
		t.Fatal("expected error from Exec failure, got nil")
	}
}

// TestRecordAnswer_SelectError_Propagated verifies UPDATE session scan failures surface.
func TestRecordAnswer_SelectError_Propagated(t *testing.T) {
	selectErr := errors.New("update session failed")
	tx := &txWithQueryRow{mockTx: &mockTx{}, queryRow: errFuncRow(selectErr)}
	db := &txDB{tx: tx}
	s := newMasteryStore(t, db)
	_, err := s.RecordAnswer(context.Background(), "sess-1", "user-1", "q-1", "A", true, 0, 10, nil)
	if err == nil {
		t.Fatal("expected error from SELECT failure, got nil")
	}
}
