package store_test

// Tests for GetChapterMastery in boss_progression.go (tl-7wv).
//
// Uses a lightweight mockDB that implements store.DB, returning canned pgx.Row
// responses without a real Postgres connection. Full ltree-matching is covered
// by the e2e suite; here we verify the Go-side contract: empty-paths
// short-circuit, happy-path average scan, no-rows → 0.0, hard-error
// propagation, and correct forwarding of SQL args.

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"

	"github.com/teacherslounge/gaming-service/internal/store"
)

// ── DB mock ───────────────────────────────────────────────────────────────────

// floatRow scans a single float64 into dest[0].
type floatRow struct {
	value float64
	err   error
}

func (r *floatRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) == 0 {
		return fmt.Errorf("floatRow.Scan: no destination provided")
	}
	p, ok := dest[0].(*float64)
	if !ok {
		return fmt.Errorf("floatRow.Scan: dest[0] is not *float64")
	}
	*p = r.value
	return nil
}

// captureDB records the last QueryRow invocation so tests can assert the
// SQL + args passed by the production code.
type captureDB struct {
	row      pgx.Row
	lastSQL  string
	lastArgs []any
	calls    int
}

func (d *captureDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	d.calls++
	d.lastSQL = sql
	d.lastArgs = args
	return d.row
}

func (d *captureDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}

func (d *captureDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (d *captureDB) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newMasteryStore(t *testing.T, db store.DB) *store.Store {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return store.New(db, rdb)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestGetChapterMastery_HappyPath_ReturnsAverage(t *testing.T) {
	db := &captureDB{row: &floatRow{value: 0.73}}
	s := newMasteryStore(t, db)

	got, err := s.GetChapterMastery(context.Background(), "user-1", []string{"chemistry.bonding.*"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0.73 {
		t.Errorf("mastery: got %v, want 0.73", got)
	}
	if db.calls != 1 {
		t.Errorf("expected 1 QueryRow call, got %d", db.calls)
	}
}

func TestGetChapterMastery_EmptyPaths_ShortCircuitsWithoutQuerying(t *testing.T) {
	// No DB should be touched when there are no paths — this is the spec
	// escape hatch for bosses that haven't been mapped to chapters yet.
	db := &captureDB{row: &floatRow{err: fmt.Errorf("should not be called")}}
	s := newMasteryStore(t, db)

	got, err := s.GetChapterMastery(context.Background(), "user-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0.0 {
		t.Errorf("empty paths: got %v, want 0.0", got)
	}
	if db.calls != 0 {
		t.Errorf("expected 0 QueryRow calls with empty paths, got %d", db.calls)
	}
}

func TestGetChapterMastery_NoRows_ReturnsZero(t *testing.T) {
	// COALESCE should mean pgx.ErrNoRows never surfaces for this query, but
	// the code defends against it explicitly — verify that contract.
	db := &captureDB{row: &floatRow{err: pgx.ErrNoRows}}
	s := newMasteryStore(t, db)

	got, err := s.GetChapterMastery(context.Background(), "user-new", []string{"chemistry.reactions.*"})
	if err != nil {
		t.Fatalf("ErrNoRows should be swallowed, got error: %v", err)
	}
	if got != 0.0 {
		t.Errorf("no rows: got %v, want 0.0", got)
	}
}

func TestGetChapterMastery_DBError_Propagated(t *testing.T) {
	dbErr := errors.New("connection refused")
	db := &captureDB{row: &floatRow{err: dbErr}}
	s := newMasteryStore(t, db)

	_, err := s.GetChapterMastery(context.Background(), "user-1", []string{"chemistry.bonding.*"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Errorf("error chain should wrap the underlying DB error, got %v", err)
	}
}

func TestGetChapterMastery_MultiPath_ForwardsAllPathsAsSecondArg(t *testing.T) {
	// The handler passes every BossDef.ChapterConceptPaths entry as a single
	// lquery[] parameter — this test pins that behavior so a future refactor
	// can't silently drop paths.
	db := &captureDB{row: &floatRow{value: 0.42}}
	s := newMasteryStore(t, db)

	paths := []string{"chemistry.bonding.*", "chemistry.bonding.polarity.*", "chemistry.bonding.shapes.*"}
	_, err := s.GetChapterMastery(context.Background(), "user-1", paths)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(db.lastArgs) != 2 {
		t.Fatalf("expected 2 SQL args (userID, paths), got %d: %v", len(db.lastArgs), db.lastArgs)
	}
	if db.lastArgs[0] != "user-1" {
		t.Errorf("arg[0] userID: got %v, want user-1", db.lastArgs[0])
	}
	got, ok := db.lastArgs[1].([]string)
	if !ok {
		t.Fatalf("arg[1]: expected []string, got %T", db.lastArgs[1])
	}
	if len(got) != len(paths) {
		t.Errorf("arg[1] length: got %d, want %d", len(got), len(paths))
	}
	for i, p := range paths {
		if got[i] != p {
			t.Errorf("arg[1][%d]: got %q, want %q", i, got[i], p)
		}
	}
}

func TestGetChapterMastery_ZeroMastery_ReturnsZero(t *testing.T) {
	// User exists in student_concept_mastery but averages 0.0 — distinct
	// from the no-rows case. Must not be confused with an error.
	db := &captureDB{row: &floatRow{value: 0.0}}
	s := newMasteryStore(t, db)

	got, err := s.GetChapterMastery(context.Background(), "user-1", []string{"chemistry.bonding.*"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 0.0 {
		t.Errorf("got %v, want 0.0", got)
	}
}
