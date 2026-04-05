package store_test

// Tests for RandomQuoteForUser in store.go (tl-pcw).
//
// We use:
//   - mockDB   — implements store.DB; returns canned pgx.Row responses without
//                a real Postgres connection (all SQL calls routed through it)
//   - miniredis — real in-process Redis for the dedup SET logic
//
// Full integration tests (real SELECT from scifi_quotes) belong in the e2e suite.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"

	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/store"
)

// ── DB mock ───────────────────────────────────────────────────────────────────

// fakeRow implements pgx.Row by returning canned values or a canned error.
type fakeRow struct {
	id          int
	quote       string
	attribution string
	context     string
	err         error
}

// Scan satisfies pgx.Row. Expects dest to be [*int, *string, *string, *string]
// matching the SELECT id, quote, attribution, context pattern in store.go.
func (r *fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*int)) = r.id
	*(dest[1].(*string)) = r.quote
	*(dest[2].(*string)) = r.attribution
	*(dest[3].(*string)) = r.context
	return nil
}

// seqDB sequences through a list of fakeRows, returning them one per QueryRow call.
// After the list is exhausted every subsequent call returns pgx.ErrNoRows.
type seqDB struct {
	rows []*fakeRow
	pos  int
}

func (d *seqDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if d.pos >= len(d.rows) {
		return &fakeRow{err: pgx.ErrNoRows}
	}
	r := d.rows[d.pos]
	d.pos++
	return r
}

func (d *seqDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (d *seqDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}

func (d *seqDB) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newQuoteStore(t *testing.T, db store.DB) (*store.Store, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return store.New(db, rdb), mr
}

// seenKey mirrors the key format used by store.go — must stay in sync.
func seenKey(userID string) string {
	return fmt.Sprintf("quotes:seen:%s:%s", userID, time.Now().UTC().Format("2006-01-02"))
}

func seenMembers(t *testing.T, mr *miniredis.Miniredis, userID string) []string {
	t.Helper()
	members, err := mr.Members(seenKey(userID))
	if err != nil {
		// Key does not exist — empty set.
		return nil
	}
	return members
}

func wantQuote(t *testing.T, q *model.Quote, id int, text, context string) {
	t.Helper()
	if q == nil {
		t.Fatal("expected quote, got nil")
	}
	if q.ID != id {
		t.Errorf("quote.ID: got %d, want %d", q.ID, id)
	}
	if q.Quote != text {
		t.Errorf("quote.Quote: got %q, want %q", q.Quote, text)
	}
	if q.Context != context {
		t.Errorf("quote.Context: got %q, want %q", q.Context, context)
	}
}

// ── happy path ────────────────────────────────────────────────────────────────

func TestRandomQuoteForUser_HappyPath_ReturnsQuoteAndTracksInRedis(t *testing.T) {
	db := &seqDB{rows: []*fakeRow{
		{id: 7, quote: "Fear is the mind-killer.", attribution: "Paul — Dune", context: "session_start"},
	}}
	s, mr := newQuoteStore(t, db)
	defer mr.Close()

	ctx := context.Background()
	q, err := s.RandomQuoteForUser(ctx, "user-1", "session_start")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantQuote(t, q, 7, "Fear is the mind-killer.", "session_start")

	// Quote ID must be added to the seen set.
	seen := seenMembers(t, mr, "user-1")
	if len(seen) != 1 || seen[0] != "7" {
		t.Errorf("seen set: got %v, want [7]", seen)
	}
}

func TestRandomQuoteForUser_EmptyContext_AnyContextReturned(t *testing.T) {
	db := &seqDB{rows: []*fakeRow{
		{id: 3, quote: "Make it so.", attribution: "Picard — TNG", context: "boss_fight"},
	}}
	s, mr := newQuoteStore(t, db)
	defer mr.Close()

	q, err := s.RandomQuoteForUser(context.Background(), "user-2", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantQuote(t, q, 3, "Make it so.", "boss_fight")
}

// ── seen-set exclusion ────────────────────────────────────────────────────────

func TestRandomQuoteForUser_SeenIDsExcluded_FreshQuoteReturned(t *testing.T) {
	// First call returns ErrNoRows (all matching unseen quotes exhausted),
	// then after reset the second call returns a quote.
	db := &seqDB{rows: []*fakeRow{
		{err: pgx.ErrNoRows}, // no unseen quotes
		{id: 9, quote: "The enemy's gate is down.", attribution: "Ender", context: "victory"},
	}}
	s, mr := newQuoteStore(t, db)
	defer mr.Close()

	ctx := context.Background()
	userID := "user-reset"

	// Pre-populate seen set — store should clear it and retry.
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	_ = rdb.SAdd(ctx, seenKey(userID), "1", "2", "3")

	q, err := s.RandomQuoteForUser(ctx, userID, "victory")
	if err != nil {
		t.Fatalf("unexpected error after reset: %v", err)
	}
	wantQuote(t, q, 9, "The enemy's gate is down.", "victory")

	// Seen set must be cleared and then populated with the new quote.
	seen := seenMembers(t, mr, userID)
	if len(seen) != 1 || seen[0] != "9" {
		t.Errorf("after reset: seen set should contain only [9], got %v", seen)
	}
}

func TestRandomQuoteForUser_AccumulatesSeenAcrossCalls(t *testing.T) {
	db := &seqDB{rows: []*fakeRow{
		{id: 10, quote: "Q1", attribution: "A", context: "correct"},
		{id: 11, quote: "Q2", attribution: "B", context: "correct"},
		{id: 12, quote: "Q3", attribution: "C", context: "correct"},
	}}
	s, mr := newQuoteStore(t, db)
	defer mr.Close()

	ctx := context.Background()
	userID := "user-accum"

	for i, wantID := range []int{10, 11, 12} {
		q, err := s.RandomQuoteForUser(ctx, userID, "correct")
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i+1, err)
		}
		if q.ID != wantID {
			t.Errorf("call %d: got quote ID %d, want %d", i+1, q.ID, wantID)
		}
	}

	// After 3 calls, seen set must have 3 IDs.
	seen := seenMembers(t, mr, userID)
	if len(seen) != 3 {
		t.Errorf("expected 3 seen IDs, got %d: %v", len(seen), seen)
	}
}

// ── reset-after-exhaustion ────────────────────────────────────────────────────

func TestRandomQuoteForUser_FreshUser_EmptySeenBeforeCall(t *testing.T) {
	db := &seqDB{rows: []*fakeRow{
		{id: 5, quote: "Live long.", attribution: "Spock", context: "streak"},
	}}
	s, mr := newQuoteStore(t, db)
	defer mr.Close()

	ctx := context.Background()
	userID := "user-fresh"

	// No seen entries before the call.
	seen := seenMembers(t, mr, userID)
	if len(seen) != 0 {
		t.Errorf("pre-call: expected empty seen set, got %v", seen)
	}

	_, err := s.RandomQuoteForUser(ctx, userID, "streak")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// One entry after.
	seen = seenMembers(t, mr, userID)
	if len(seen) != 1 {
		t.Errorf("post-call: expected 1 seen ID, got %v", seen)
	}
}

// ── TTL ───────────────────────────────────────────────────────────────────────

func TestRandomQuoteForUser_SeenKeyHas25HourTTL(t *testing.T) {
	db := &seqDB{rows: []*fakeRow{
		{id: 1, quote: "Q", attribution: "A", context: "session_start"},
	}}
	s, mr := newQuoteStore(t, db)
	defer mr.Close()

	userID := "user-ttl"
	_, err := s.RandomQuoteForUser(context.Background(), userID, "session_start")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ttl := mr.TTL(seenKey(userID))
	const want = 25 * time.Hour
	// Allow a 5-second window for test execution time.
	if ttl < want-5*time.Second || ttl > want {
		t.Errorf("TTL = %v, want ~%v", ttl, want)
	}
}

// ── error paths ───────────────────────────────────────────────────────────────

func TestRandomQuoteForUser_DBError_Propagated(t *testing.T) {
	// First call returns a hard DB error (not ErrNoRows).
	db := &seqDB{rows: []*fakeRow{
		{err: fmt.Errorf("connection refused")},
	}}
	s, mr := newQuoteStore(t, db)
	defer mr.Close()

	_, err := s.RandomQuoteForUser(context.Background(), "user-err", "correct")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRandomQuoteForUser_AllExhaustedAndStillNoneAfterReset_ReturnsError(t *testing.T) {
	// Both attempts return ErrNoRows — context has no quotes at all.
	db := &seqDB{rows: []*fakeRow{
		{err: pgx.ErrNoRows},
		{err: pgx.ErrNoRows},
	}}
	s, mr := newQuoteStore(t, db)
	defer mr.Close()

	_, err := s.RandomQuoteForUser(context.Background(), "user-none", "unknown_context")
	if err == nil {
		t.Fatal("expected error when no quotes exist for context, got nil")
	}
}

// ── DifferentUsers_Independent ────────────────────────────────────────────────

func TestRandomQuoteForUser_DifferentUsers_IndependentSeenSets(t *testing.T) {
	db := &seqDB{rows: []*fakeRow{
		{id: 20, quote: "Qa", attribution: "A", context: "wrong"},
		{id: 21, quote: "Qb", attribution: "B", context: "wrong"},
	}}
	s, mr := newQuoteStore(t, db)
	defer mr.Close()

	ctx := context.Background()
	_, _ = s.RandomQuoteForUser(ctx, "alice", "wrong")
	_, _ = s.RandomQuoteForUser(ctx, "bob", "wrong")

	aliceSeen := seenMembers(t, mr, "alice")
	bobSeen := seenMembers(t, mr, "bob")

	if len(aliceSeen) != 1 {
		t.Errorf("alice: expected 1 seen ID, got %v", aliceSeen)
	}
	if len(bobSeen) != 1 {
		t.Errorf("bob: expected 1 seen ID, got %v", bobSeen)
	}
	if len(aliceSeen) > 0 && len(bobSeen) > 0 && aliceSeen[0] == bobSeen[0] {
		// Could happen if both calls return the same quote ID — fine.
		// IDs 20 and 21 are different, so this won't happen with seqDB.
		t.Logf("both users got the same seen ID (could happen with random DB)")
	}
}
