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

// ── Batch fake: implements pgx.Rows over a slice of (bossID, mastery) rows ───

type batchRow struct {
	bossID  string
	mastery float64
}

type batchRows struct {
	data []batchRow
	pos  int
	err  error
}

func (r *batchRows) Next() bool {
	r.pos++
	return r.pos-1 < len(r.data)
}
func (r *batchRows) Err() error                                  { return r.err }
func (r *batchRows) Close()                                      {}
func (r *batchRows) CommandTag() pgconn.CommandTag               { return pgconn.CommandTag{} }
func (r *batchRows) Conn() *pgx.Conn                             { return nil }
func (r *batchRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *batchRows) Values() ([]any, error)                      { return nil, nil }
func (r *batchRows) RawValues() [][]byte                         { return nil }
func (r *batchRows) Scan(dest ...any) error {
	if r.pos == 0 || r.pos-1 >= len(r.data) {
		return fmt.Errorf("batchRows.Scan: no current row")
	}
	row := r.data[r.pos-1]
	if len(dest) < 2 {
		return fmt.Errorf("batchRows.Scan: need 2 destinations")
	}
	*(dest[0].(*string)) = row.bossID
	*(dest[1].(*float64)) = row.mastery
	return nil
}

// batchDB captures the Query args and returns canned rows.
type batchDB struct {
	rows     *batchRows
	queryErr error
	lastSQL  string
	lastArgs []any
	calls    int
}

func (d *batchDB) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	d.calls++
	d.lastSQL = sql
	d.lastArgs = args
	if d.queryErr != nil {
		return nil, d.queryErr
	}
	return d.rows, nil
}
func (d *batchDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row { return nil }
func (d *batchDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (d *batchDB) Begin(_ context.Context) (pgx.Tx, error) { return nil, nil }

// ── GetChapterMasteryBatch tests ──────────────────────────────────────────────

func TestGetChapterMasteryBatch_HappyPath_PerBossAverages(t *testing.T) {
	db := &batchDB{rows: &batchRows{data: []batchRow{
		{bossID: "the_atom", mastery: 0.80},
		{bossID: "the_bonder", mastery: 0.55},
	}}}
	s := newMasteryStore(t, db)

	got, err := s.GetChapterMasteryBatch(context.Background(), "user-1", map[string][]string{
		"the_atom":   {"chemistry.atoms.*"},
		"the_bonder": {"chemistry.bonding.*"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["the_atom"] != 0.80 {
		t.Errorf("the_atom: got %v, want 0.80", got["the_atom"])
	}
	if got["the_bonder"] != 0.55 {
		t.Errorf("the_bonder: got %v, want 0.55", got["the_bonder"])
	}
	if db.calls != 1 {
		t.Errorf("expected 1 Query call (batch), got %d", db.calls)
	}
}

func TestGetChapterMasteryBatch_MissingBoss_ReturnsZero(t *testing.T) {
	// DB returns one boss; the other has no matching concepts.
	// Batch must still include the absent boss with 0.0 so the handler
	// can render a full trail without gaps.
	db := &batchDB{rows: &batchRows{data: []batchRow{
		{bossID: "the_atom", mastery: 0.80},
	}}}
	s := newMasteryStore(t, db)

	got, err := s.GetChapterMasteryBatch(context.Background(), "user-1", map[string][]string{
		"the_atom":  {"chemistry.atoms.*"},
		"name_lord": {"chemistry.nomenclature.*"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["the_atom"] != 0.80 {
		t.Errorf("the_atom: got %v, want 0.80", got["the_atom"])
	}
	v, present := got["name_lord"]
	if !present {
		t.Error("name_lord missing; batch must include every input boss")
	}
	if v != 0.0 {
		t.Errorf("name_lord: got %v, want 0.0 for missing boss", v)
	}
}

func TestGetChapterMasteryBatch_EmptyInput_NoDBCall(t *testing.T) {
	db := &batchDB{queryErr: fmt.Errorf("should not be called")}
	s := newMasteryStore(t, db)

	got, err := s.GetChapterMasteryBatch(context.Background(), "user-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("empty input: got %d entries, want 0", len(got))
	}
	if db.calls != 0 {
		t.Errorf("empty input must not hit DB, got %d calls", db.calls)
	}
}

func TestGetChapterMasteryBatch_AllBossesZeroPaths_NoDBCall(t *testing.T) {
	db := &batchDB{queryErr: fmt.Errorf("should not be called")}
	s := newMasteryStore(t, db)

	got, err := s.GetChapterMasteryBatch(context.Background(), "user-1", map[string][]string{
		"the_atom":   nil,
		"the_bonder": {},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if db.calls != 0 {
		t.Errorf("all-empty-paths must not hit DB, got %d calls", db.calls)
	}
	if got["the_atom"] != 0.0 || got["the_bonder"] != 0.0 {
		t.Errorf("expected both 0.0, got %v", got)
	}
}

func TestGetChapterMasteryBatch_DBError_Propagated(t *testing.T) {
	dbErr := errors.New("ltree query failed")
	db := &batchDB{queryErr: dbErr}
	s := newMasteryStore(t, db)

	_, err := s.GetChapterMasteryBatch(context.Background(), "user-1", map[string][]string{
		"the_atom": {"chemistry.atoms.*"},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Errorf("error chain should wrap DB error, got %v", err)
	}
}

func TestGetChapterMasteryBatch_ArgsForwarded_ParallelArrays(t *testing.T) {
	// Two bosses, one with 2 paths — verify the flattened parallel arrays
	// reach the driver with matching lengths and correct (boss_id, path)
	// pairing.
	db := &batchDB{rows: &batchRows{data: nil}}
	s := newMasteryStore(t, db)

	input := map[string][]string{
		"the_atom":   {"chemistry.atoms.*"},
		"the_bonder": {"chemistry.bonding.polarity.*", "chemistry.bonding.shapes.*"},
	}
	_, err := s.GetChapterMasteryBatch(context.Background(), "user-1", input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(db.lastArgs) != 3 {
		t.Fatalf("expected 3 SQL args (userID, bossIDs, paths), got %d", len(db.lastArgs))
	}
	if db.lastArgs[0] != "user-1" {
		t.Errorf("arg[0] userID: got %v, want user-1", db.lastArgs[0])
	}
	bossIDs, ok1 := db.lastArgs[1].([]string)
	paths, ok2 := db.lastArgs[2].([]string)
	if !ok1 || !ok2 {
		t.Fatalf("args must be []string, got %T and %T", db.lastArgs[1], db.lastArgs[2])
	}
	if len(bossIDs) != 3 || len(paths) != 3 {
		t.Errorf("expected 3 flattened rows, got bossIDs=%d paths=%d", len(bossIDs), len(paths))
	}
	for i := range bossIDs {
		bID := bossIDs[i]
		path := paths[i]
		found := false
		for _, p := range input[bID] {
			if p == path {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("row %d: path %q not in input paths for %q", i, path, bID)
		}
	}
}
