package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/store"
)

// ── pgx.Tx mock ───────────────────────────────────────────────────────────────

type mockTx struct {
	execErr   error
	commitErr error
	execCalls int
}

func (t *mockTx) Begin(_ context.Context) (pgx.Tx, error)  { return t, nil }
func (t *mockTx) Commit(_ context.Context) error            { return t.commitErr }
func (t *mockTx) Rollback(_ context.Context) error          { return nil }
func (t *mockTx) Conn() *pgx.Conn                           { return nil }
func (t *mockTx) LargeObjects() pgx.LargeObjects            { return pgx.LargeObjects{} }
func (t *mockTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults { return nil }
func (t *mockTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *mockTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *mockTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) { return nil, nil }
func (t *mockTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row        { return nil }
func (t *mockTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	t.execCalls++
	return pgconn.CommandTag{}, t.execErr
}

// txWithQueryRow extends mockTx so tests can inject a row response.
type txWithQueryRow struct {
	*mockTx
	queryRow pgx.Row
}

func (t *txWithQueryRow) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return t.queryRow
}

// ── txDB ──────────────────────────────────────────────────────────────────────

type txDB struct {
	tx     pgx.Tx
	txErr  error
	intRow pgx.Row
}

func (d *txDB) Begin(_ context.Context) (pgx.Tx, error) { return d.tx, d.txErr }
func (d *txDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row { return d.intRow }
func (d *txDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) { return nil, nil }
func (d *txDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

// ── intRow ────────────────────────────────────────────────────────────────────

type intRow struct {
	value int
	err   error
}

func (r *intRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*int)) = r.value
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newBattleStore(t *testing.T, db store.DB) *store.Store {
	t.Helper()
	return newMasteryStore(t, db)
}

func battleSession(id string) *model.BattleSession {
	return &model.BattleSession{
		SessionID: id,
		UserID:    "user-abc",
		BossID:    "the_atom",
		Phase:     model.PhaseActive,
		PlayerHP:  100,
		BossHP:    80,
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}
}

// ── SaveBattleSession ─────────────────────────────────────────────────────────

func TestSaveBattleSession_HappyPath(t *testing.T) {
	s := newBattleStore(t, &txDB{})
	if err := s.SaveBattleSession(context.Background(), battleSession("sess-1")); err != nil {
		t.Fatalf("SaveBattleSession: %v", err)
	}
}

func TestSaveBattleSession_ExpiredExpiresAt_UsesFallbackTTL(t *testing.T) {
	s := newBattleStore(t, &txDB{})
	sess := battleSession("sess-exp")
	sess.ExpiresAt = time.Now().Add(-5 * time.Minute)
	if err := s.SaveBattleSession(context.Background(), sess); err != nil {
		t.Fatalf("SaveBattleSession: %v", err)
	}
	got, err := s.GetBattleSession(context.Background(), "sess-exp")
	if err != nil {
		t.Fatalf("GetBattleSession: %v", err)
	}
	if got == nil {
		t.Error("expected session to be stored, got nil")
	}
}

// ── GetBattleSession ──────────────────────────────────────────────────────────

func TestGetBattleSession_HappyPath(t *testing.T) {
	s := newBattleStore(t, &txDB{})
	sess := battleSession("sess-2")
	if err := s.SaveBattleSession(context.Background(), sess); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.GetBattleSession(context.Background(), "sess-2")
	if err != nil {
		t.Fatalf("GetBattleSession: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil session")
	}
	if got.SessionID != "sess-2" {
		t.Errorf("SessionID: got %q, want sess-2", got.SessionID)
	}
}

func TestGetBattleSession_NotFound_ReturnsNil(t *testing.T) {
	s := newBattleStore(t, &txDB{})
	got, err := s.GetBattleSession(context.Background(), "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

// ── DeleteBattleSession ───────────────────────────────────────────────────────

func TestDeleteBattleSession_RemovesKey(t *testing.T) {
	s := newBattleStore(t, &txDB{})
	sess := battleSession("sess-del")
	if err := s.SaveBattleSession(context.Background(), sess); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := s.DeleteBattleSession(context.Background(), "sess-del"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got, err := s.GetBattleSession(context.Background(), "sess-del")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestDeleteBattleSession_MissingKey_NoError(t *testing.T) {
	s := newBattleStore(t, &txDB{})
	if err := s.DeleteBattleSession(context.Background(), "never-existed"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── RecordBattleResult ────────────────────────────────────────────────────────

func TestRecordBattleResult_Victory_ExecsInsertAndUpdate(t *testing.T) {
	tx := &mockTx{}
	s := newBattleStore(t, &txDB{tx: tx})
	result := &model.BattleResult{
		SessionID: "sess-win", UserID: "u", BossID: "b",
		Won: true, TurnsUsed: 5, XPEarned: 100, GemsEarned: 10,
		FinishedAt: time.Now(),
	}
	if err := s.RecordBattleResult(context.Background(), result); err != nil {
		t.Fatalf("RecordBattleResult: %v", err)
	}
	if tx.execCalls != 2 {
		t.Errorf("want 2 Exec calls for victory, got %d", tx.execCalls)
	}
}

func TestRecordBattleResult_Defeat_ExecsInsertOnly(t *testing.T) {
	tx := &mockTx{}
	s := newBattleStore(t, &txDB{tx: tx})
	result := &model.BattleResult{
		UserID: "u", BossID: "b", Won: false, FinishedAt: time.Now(),
	}
	if err := s.RecordBattleResult(context.Background(), result); err != nil {
		t.Fatalf("RecordBattleResult: %v", err)
	}
	if tx.execCalls != 1 {
		t.Errorf("want 1 Exec call for defeat, got %d", tx.execCalls)
	}
}

func TestRecordBattleResult_BeginError_Propagated(t *testing.T) {
	beginErr := errors.New("pool exhausted")
	s := newBattleStore(t, &txDB{txErr: beginErr})
	if err := s.RecordBattleResult(context.Background(), &model.BattleResult{FinishedAt: time.Now()}); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRecordBattleResult_ExecError_Propagated(t *testing.T) {
	tx := &mockTx{execErr: errors.New("constraint")}
	s := newBattleStore(t, &txDB{tx: tx})
	if err := s.RecordBattleResult(context.Background(), &model.BattleResult{Won: true, FinishedAt: time.Now()}); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRecordBattleResult_CommitError_Propagated(t *testing.T) {
	tx := &mockTx{commitErr: errors.New("commit failed")}
	s := newBattleStore(t, &txDB{tx: tx})
	if err := s.RecordBattleResult(context.Background(), &model.BattleResult{FinishedAt: time.Now()}); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── DeductGems ────────────────────────────────────────────────────────────────

func TestDeductGems_HappyPath(t *testing.T) {
	s := newBattleStore(t, &txDB{intRow: &intRow{value: 90}})
	remaining, err := s.DeductGems(context.Background(), "user-abc", 10)
	if err != nil {
		t.Fatalf("DeductGems: %v", err)
	}
	if remaining != 90 {
		t.Errorf("remaining: got %d, want 90", remaining)
	}
}

func TestDeductGems_InsufficientGems_ReturnsError(t *testing.T) {
	s := newBattleStore(t, &txDB{intRow: &intRow{err: errors.New("no rows")}})
	if _, err := s.DeductGems(context.Background(), "user-abc", 999); err == nil {
		t.Fatal("expected error, got nil")
	}
}
