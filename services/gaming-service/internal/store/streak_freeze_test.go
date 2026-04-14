package store_test

// Tests for CreateStreakFreeze and IsStreakFrozen in streak_freeze.go (tl-2n5).
//
// Uses a lightweight intRow fake for Postgres (returns a canned gem balance) and
// miniredis for the Redis freeze-key operations. Full integration tests (real
// UPDATE against gaming_profiles) belong in the e2e suite.

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"

	"github.com/teacherslounge/gaming-service/internal/store"
)

// ── DB fakes ──────────────────────────────────────────────────────────────────

// intRow is a pgx.Row that scans a single int into dest[0].
type intRow struct {
	value int
	err   error
}

func (r *intRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) == 0 {
		return fmt.Errorf("intRow.Scan: no destination")
	}
	p, ok := dest[0].(*int)
	if !ok {
		return fmt.Errorf("intRow.Scan: dest[0] is not *int")
	}
	*p = r.value
	return nil
}

// staticDB always returns the same pgx.Row regardless of the query.
type staticDB struct {
	row pgx.Row
}

func (d *staticDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return d.row
}

func (d *staticDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}

func (d *staticDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (d *staticDB) Begin(_ context.Context) (pgx.Tx, error) {
	return nil, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// newFreezeStore builds a Store wired with a static DB row and a real miniredis.
// The returned *miniredis.Miniredis lets tests control TTL expiry via FastForward.
func newFreezeStore(t *testing.T, row pgx.Row) (*store.Store, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return store.New(&staticDB{row: row}, rdb), mr
}

// ── CreateStreakFreeze tests ───────────────────────────────────────────────────

// TestCreateStreakFreeze_Success verifies that a successful purchase deducts gems
// and records an active freeze key in Redis.
func TestCreateStreakFreeze_Success(t *testing.T) {
	const wantGemsLeft = 75 // 125 - 50
	s, mr := newFreezeStore(t, &intRow{value: wantGemsLeft})
	ctx := context.Background()

	got, err := s.CreateStreakFreeze(ctx, "user-alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != wantGemsLeft {
		t.Errorf("gems_left: got %d, want %d", got, wantGemsLeft)
	}

	// Freeze key must be present in Redis.
	frozen, err := s.IsStreakFrozen(ctx, "user-alice")
	if err != nil {
		t.Fatalf("IsStreakFrozen error: %v", err)
	}
	if !frozen {
		t.Error("expected freeze key to be set after CreateStreakFreeze")
	}

	// Verify the key carries a TTL (miniredis tracks it).
	ttl := mr.TTL("streak:freeze:user-alice")
	if ttl <= 0 {
		t.Errorf("expected positive TTL on freeze key, got %v", ttl)
	}
}

// TestCreateStreakFreeze_InsufficientCoins verifies ErrInsufficientCoins is
// returned when the UPDATE matches no rows (gems < cost).
func TestCreateStreakFreeze_InsufficientCoins(t *testing.T) {
	s, _ := newFreezeStore(t, &intRow{err: pgx.ErrNoRows})

	_, err := s.CreateStreakFreeze(context.Background(), "user-poor")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !errors.Is(err, store.ErrInsufficientCoins) {
		t.Errorf("want ErrInsufficientCoins, got %v", err)
	}
}

// TestCreateStreakFreeze_DBError verifies unexpected DB errors are propagated.
func TestCreateStreakFreeze_DBError(t *testing.T) {
	dbErr := errors.New("connection reset by peer")
	s, _ := newFreezeStore(t, &intRow{err: dbErr})

	_, err := s.CreateStreakFreeze(context.Background(), "user-flaky")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if errors.Is(err, store.ErrInsufficientCoins) {
		t.Error("unexpected ErrInsufficientCoins — should be a wrapped DB error")
	}
}

// ── IsStreakFrozen tests ──────────────────────────────────────────────────────

// TestIsStreakFrozen_Active verifies that a key manually set in Redis is
// detected as frozen.
func TestIsStreakFrozen_Active(t *testing.T) {
	s, mr := newFreezeStore(t, &intRow{value: 0})
	ctx := context.Background()

	// Manually insert the freeze key (simulates a prior purchase).
	_ = mr.Set("streak:freeze:user-bob", "1")
	mr.SetTTL("streak:freeze:user-bob", 24*time.Hour)

	frozen, err := s.IsStreakFrozen(ctx, "user-bob")
	if err != nil {
		t.Fatalf("IsStreakFrozen error: %v", err)
	}
	if !frozen {
		t.Error("expected frozen=true when key exists")
	}
}

// TestIsStreakFrozen_Expired verifies that IsStreakFrozen returns false when the
// Redis key is absent (simulates TTL expiry via miniredis FastForward).
func TestIsStreakFrozen_Expired(t *testing.T) {
	s, mr := newFreezeStore(t, &intRow{value: 0})
	ctx := context.Background()

	_ = mr.Set("streak:freeze:user-carol", "1")
	mr.SetTTL("streak:freeze:user-carol", 1*time.Second)

	// Fast-forward past the TTL to simulate expiry.
	mr.FastForward(2 * time.Second)

	frozen, err := s.IsStreakFrozen(ctx, "user-carol")
	if err != nil {
		t.Fatalf("IsStreakFrozen error: %v", err)
	}
	if frozen {
		t.Error("expected frozen=false after TTL expiry")
	}
}

// TestIsStreakFrozen_NotSet verifies false is returned when no freeze was ever purchased.
func TestIsStreakFrozen_NotSet(t *testing.T) {
	s, _ := newFreezeStore(t, &intRow{value: 0})

	frozen, err := s.IsStreakFrozen(context.Background(), "user-dave")
	if err != nil {
		t.Fatalf("IsStreakFrozen error: %v", err)
	}
	if frozen {
		t.Error("expected frozen=false for user with no freeze")
	}
}
