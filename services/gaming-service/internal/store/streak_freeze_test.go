package store_test

// Store-level tests for CreateStreakFreeze + IsStreakFrozen (tl-2n5).
//
// Uses captureDB from boss_progression_test.go so we verify SQL args + branch
// semantics without spinning up a real Postgres. The two-query classification
// path on UPDATE→no-rows is exercised by swapping the DB's row between
// QueryRow calls.

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/teacherslounge/gaming-service/internal/store"
)

// ── row stubs ─────────────────────────────────────────────────────────────────

// updateRow scans (gems int, expires time.Time) for the happy path of
// CreateStreakFreeze's UPDATE ... RETURNING.
type updateRow struct {
	gems    int
	expires time.Time
	err     error
}

func (r *updateRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 2 {
		return fmt.Errorf("updateRow.Scan: want 2 dests, got %d", len(dest))
	}
	gp, ok := dest[0].(*int)
	if !ok {
		return fmt.Errorf("updateRow.Scan: dest[0] is not *int")
	}
	ep, ok := dest[1].(*time.Time)
	if !ok {
		return fmt.Errorf("updateRow.Scan: dest[1] is not *time.Time")
	}
	*gp = r.gems
	*ep = r.expires
	return nil
}

// classifyRow scans (gems int, streak_frozen_until *time.Time) for the
// failure-classification SELECT.
type classifyRow struct {
	gems     int
	existing *time.Time
	err      error
}

func (r *classifyRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 2 {
		return fmt.Errorf("classifyRow.Scan: want 2 dests, got %d", len(dest))
	}
	gp, ok := dest[0].(*int)
	if !ok {
		return fmt.Errorf("classifyRow.Scan: dest[0] is not *int")
	}
	ep, ok := dest[1].(**time.Time)
	if !ok {
		return fmt.Errorf("classifyRow.Scan: dest[1] is not **time.Time")
	}
	*gp = r.gems
	*ep = r.existing
	return nil
}

// boolRow scans a single bool for IsStreakFrozen.
type boolRow struct {
	value bool
	err   error
}

func (r *boolRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	bp, ok := dest[0].(*bool)
	if !ok {
		return fmt.Errorf("boolRow.Scan: dest[0] is not *bool")
	}
	*bp = r.value
	return nil
}

// seqDB returns queued rows in FIFO order; allows a single test to drive
// both the UPDATE and the classification SELECT of CreateStreakFreeze.
type seqDB struct {
	rows     []pgx.Row
	lastSQLs []string
	lastArgs [][]any
}

func (d *seqDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	d.lastSQLs = append(d.lastSQLs, sql)
	d.lastArgs = append(d.lastArgs, args)
	if len(d.rows) == 0 {
		return &updateRow{err: fmt.Errorf("seqDB: no rows queued")}
	}
	r := d.rows[0]
	d.rows = d.rows[1:]
	return r
}

func (d *seqDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (d *seqDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (d *seqDB) Begin(_ context.Context) (pgx.Tx, error) { return nil, nil }

// newFreezeStore wires a store.Store around a fake DB. Redis is unused by
// the streak-freeze methods so we reuse newMasteryStore's miniredis helper.
func newFreezeStore(t *testing.T, db store.DB) *store.Store {
	t.Helper()
	return newMasteryStore(t, db)
}

// ── CreateStreakFreeze ────────────────────────────────────────────────────────

func TestCreateStreakFreeze_HappyPath_DeductsAndReturnsExpiry(t *testing.T) {
	expected := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	db := &captureDB{row: &updateRow{gems: 150, expires: expected}}

	s := newFreezeStore(t, db)
	gemsLeft, expires, err := s.CreateStreakFreeze(context.Background(), "u1", 50)
	if err != nil {
		t.Fatalf("CreateStreakFreeze: %v", err)
	}
	if gemsLeft != 150 {
		t.Errorf("gemsLeft: want 150, got %d", gemsLeft)
	}
	if !expires.Equal(expected) {
		t.Errorf("expires: want %s, got %s", expected, expires)
	}

	// SQL should be a single UPDATE RETURNING with the user + gem cost + duration.
	if db.calls != 1 {
		t.Errorf("QueryRow calls: want 1, got %d", db.calls)
	}
	if len(db.lastArgs) < 3 || db.lastArgs[0] != "u1" || db.lastArgs[1] != 50 {
		t.Errorf("unexpected args: %+v", db.lastArgs)
	}
}

func TestCreateStreakFreeze_NoGems_ReturnsErrNoGems(t *testing.T) {
	// UPDATE returns no rows → classify SELECT reports gems < cost + no
	// active freeze → CreateStreakFreeze must surface ErrNoGems.
	db := &seqDB{rows: []pgx.Row{
		&updateRow{err: pgx.ErrNoRows},
		&classifyRow{gems: 10, existing: nil},
	}}
	s := newFreezeStore(t, db)

	_, _, err := s.CreateStreakFreeze(context.Background(), "u1", 50)
	if !errors.Is(err, store.ErrNoGems) {
		t.Fatalf("expected ErrNoGems, got %v", err)
	}
}

func TestCreateStreakFreeze_AlreadyFrozen_ReturnsErrAlreadyFrozen(t *testing.T) {
	future := time.Now().Add(6 * time.Hour)
	db := &seqDB{rows: []pgx.Row{
		&updateRow{err: pgx.ErrNoRows},
		&classifyRow{gems: 500, existing: &future},
	}}
	s := newFreezeStore(t, db)

	_, _, err := s.CreateStreakFreeze(context.Background(), "u1", 50)
	if !errors.Is(err, store.ErrAlreadyFrozen) {
		t.Fatalf("expected ErrAlreadyFrozen, got %v", err)
	}
}

func TestCreateStreakFreeze_ExpiredFreeze_AllowedToRepurchase(t *testing.T) {
	// If streak_frozen_until is in the past, the UPDATE's guard passes and
	// we get the happy path. This test pins that behavior so a future
	// refactor can't silently make expired freezes block new ones.
	expected := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	db := &captureDB{row: &updateRow{gems: 100, expires: expected}}
	s := newFreezeStore(t, db)

	gemsLeft, _, err := s.CreateStreakFreeze(context.Background(), "u1", 50)
	if err != nil {
		t.Fatalf("CreateStreakFreeze: %v", err)
	}
	if gemsLeft != 100 {
		t.Errorf("gemsLeft: want 100, got %d", gemsLeft)
	}
}

func TestCreateStreakFreeze_DBError_Propagated(t *testing.T) {
	boom := errors.New("db exploded")
	db := &captureDB{row: &updateRow{err: boom}}
	s := newFreezeStore(t, db)

	_, _, err := s.CreateStreakFreeze(context.Background(), "u1", 50)
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected wrapped boom, got %v", err)
	}
}

// ── IsStreakFrozen ────────────────────────────────────────────────────────────

func TestIsStreakFrozen_TrueWhenActive(t *testing.T) {
	db := &captureDB{row: &boolRow{value: true}}
	s := newFreezeStore(t, db)

	frozen, err := s.IsStreakFrozen(context.Background(), "u1")
	if err != nil {
		t.Fatalf("IsStreakFrozen: %v", err)
	}
	if !frozen {
		t.Error("want frozen=true")
	}
}

func TestIsStreakFrozen_FalseWhenExpired(t *testing.T) {
	db := &captureDB{row: &boolRow{value: false}}
	s := newFreezeStore(t, db)

	frozen, err := s.IsStreakFrozen(context.Background(), "u1")
	if err != nil {
		t.Fatalf("IsStreakFrozen: %v", err)
	}
	if frozen {
		t.Error("want frozen=false")
	}
}

func TestIsStreakFrozen_NoRow_ReturnsFalseNoError(t *testing.T) {
	// New user with no gaming_profiles row should be treated as not-frozen
	// rather than surfacing a hard error. Callers that create the row on
	// first interaction remain free to do so later.
	db := &captureDB{row: &boolRow{err: pgx.ErrNoRows}}
	s := newFreezeStore(t, db)

	frozen, err := s.IsStreakFrozen(context.Background(), "new-user")
	if err != nil {
		t.Fatalf("IsStreakFrozen: %v", err)
	}
	if frozen {
		t.Error("want frozen=false for missing row")
	}
}

func TestIsStreakFrozen_DBError_Propagated(t *testing.T) {
	boom := errors.New("db exploded")
	db := &captureDB{row: &boolRow{err: boom}}
	s := newFreezeStore(t, db)

	_, err := s.IsStreakFrozen(context.Background(), "u1")
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected wrapped boom, got %v", err)
	}
}

func TestIsStreakFrozen_ArgsForwarded(t *testing.T) {
	db := &captureDB{row: &boolRow{value: false}}
	s := newFreezeStore(t, db)

	_, err := s.IsStreakFrozen(context.Background(), "user-xyz")
	if err != nil {
		t.Fatalf("IsStreakFrozen: %v", err)
	}
	if len(db.lastArgs) != 1 || db.lastArgs[0] != "user-xyz" {
		t.Errorf("unexpected args: %+v", db.lastArgs)
	}
}
