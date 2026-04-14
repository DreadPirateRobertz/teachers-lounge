package store_test

// Tests for assessment.go — CreateAssessmentSession, GetAssessmentSession,
// RecordAssessmentAnswer.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// ── assessment session row helper ─────────────────────────────────────────────

// assessmentSessionScanRow returns a funcRow that scans a valid AssessmentSession.
func assessmentSessionScanRow() pgx.Row {
	now := time.Now()
	return &funcRow{fn: func(dest ...any) error {
		*(dest[0].(*string)) = "asess-1"    // id
		*(dest[1].(*string)) = "user-1"     // user_id
		*(dest[2].(*string)) = "in_progress" // status
		*(dest[3].(*int)) = 0               // current_index
		*(dest[4].(*int)) = 4               // total_questions
		*(dest[5].(*int)) = 0               // xp_earned
		*(dest[6].(*[]byte)) = nil           // results (NULL)
		*(dest[7].(*time.Time)) = now        // started_at
		*(dest[8].(**time.Time)) = nil       // completed_at
		return nil
	}}
}

// ── CreateAssessmentSession ───────────────────────────────────────────────────

// TestCreateAssessmentSession_HappyPath verifies a session is created and returned.
func TestCreateAssessmentSession_HappyPath(t *testing.T) {
	db := &rowQueueDB{rows: []pgx.Row{assessmentSessionScanRow()}}
	s := newMasteryStore(t, db)
	sess, err := s.CreateAssessmentSession(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("CreateAssessmentSession: %v", err)
	}
	if sess.UserID != "user-1" {
		t.Errorf("UserID: got %q, want user-1", sess.UserID)
	}
	if sess.TotalQuestions != 4 {
		t.Errorf("TotalQuestions: got %d, want 4", sess.TotalQuestions)
	}
}

// TestCreateAssessmentSession_DBError_Propagated verifies scan errors are returned.
func TestCreateAssessmentSession_DBError_Propagated(t *testing.T) {
	dbErr := errors.New("insert failed")
	db := &rowQueueDB{rows: []pgx.Row{errFuncRow(dbErr)}}
	s := newMasteryStore(t, db)
	_, err := s.CreateAssessmentSession(context.Background(), "user-1")
	if !errors.Is(err, dbErr) {
		t.Errorf("expected wrapped DB error, got %v", err)
	}
}

// ── GetAssessmentSession ──────────────────────────────────────────────────────

// TestGetAssessmentSession_HappyPath verifies a session is returned by ID.
func TestGetAssessmentSession_HappyPath(t *testing.T) {
	db := &rowQueueDB{rows: []pgx.Row{assessmentSessionScanRow()}}
	s := newMasteryStore(t, db)
	sess, err := s.GetAssessmentSession(context.Background(), "asess-1")
	if err != nil {
		t.Fatalf("GetAssessmentSession: %v", err)
	}
	if sess.ID != "asess-1" {
		t.Errorf("ID: got %q, want asess-1", sess.ID)
	}
}

// TestGetAssessmentSession_NotFound_ReturnsError verifies that ErrNoRows is
// wrapped and returned (not swallowed).
func TestGetAssessmentSession_NotFound_ReturnsError(t *testing.T) {
	db := &rowQueueDB{rows: []pgx.Row{errFuncRow(pgx.ErrNoRows)}}
	s := newMasteryStore(t, db)
	_, err := s.GetAssessmentSession(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error for missing session, got nil")
	}
}

// ── RecordAssessmentAnswer — error paths ──────────────────────────────────────

// TestRecordAssessmentAnswer_BeginError_Propagated verifies Begin failures surface.
func TestRecordAssessmentAnswer_BeginError_Propagated(t *testing.T) {
	beginErr := errors.New("pool exhausted")
	s := newMasteryStore(t, &txDB{txErr: beginErr})
	_, err := s.RecordAssessmentAnswer(context.Background(), "sess-1", "user-1", "q-1", "A")
	if !errors.Is(err, beginErr) {
		t.Errorf("expected begin error, got %v", err)
	}
}

// TestRecordAssessmentAnswer_ExecError_Propagated verifies INSERT answer failures surface.
func TestRecordAssessmentAnswer_ExecError_Propagated(t *testing.T) {
	execErr := errors.New("insert answer failed")
	tx := &mockTx{execErr: execErr}
	s := newMasteryStore(t, &txDB{tx: tx})
	_, err := s.RecordAssessmentAnswer(context.Background(), "sess-1", "user-1", "q-1", "A")
	if err == nil {
		t.Fatal("expected error from Exec failure, got nil")
	}
}

// TestRecordAssessmentAnswer_SelectError_Propagated verifies SELECT FOR UPDATE failures surface.
func TestRecordAssessmentAnswer_SelectError_Propagated(t *testing.T) {
	selectErr := errors.New("select failed")
	tx := &txWithQueryRow{mockTx: &mockTx{}, queryRow: errFuncRow(selectErr)}
	s := newMasteryStore(t, &txDB{tx: tx})
	_, err := s.RecordAssessmentAnswer(context.Background(), "sess-1", "user-1", "q-1", "A")
	if err == nil {
		t.Fatal("expected error from SELECT failure, got nil")
	}
}
