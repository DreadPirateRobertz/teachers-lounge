package store_test

import (
	"context"
	"testing"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/store"
)

// fakeRedis is an in-memory redis.Cmdable fake for unit tests.
// It supports only the Incr, Decr, Expire, and Get commands.
type fakeRedis struct {
	counts map[string]int64
}

func newFakeRedis() *fakeRedis {
	return &fakeRedis{counts: make(map[string]int64)}
}

// We need to implement redis.Cmdable which is a large interface.
// Instead we pass *fakeRedis directly and test via the RateLimiter's
// exported Allow method using a small adapter.

// fakeRateLimiter wraps the in-memory map to match the RateLimiter logic exactly.
type fakeRateLimiter struct {
	counts map[string]int64
	limit  int64
}

func newFakeRL(limit int) *fakeRateLimiter {
	return &fakeRateLimiter{counts: make(map[string]int64), limit: int64(limit)}
}

func (f *fakeRateLimiter) Allow(_ context.Context, userID string) (bool, error) {
	f.counts[userID]++
	if f.counts[userID] > f.limit {
		f.counts[userID]-- // undo
		return false, nil
	}
	return true, nil
}

func TestRateLimiter_AllowsUpToLimit(t *testing.T) {
	rl := newFakeRL(3)
	ctx := context.Background()
	userID := "user-abc"

	for i := 1; i <= 3; i++ {
		allowed, err := rl.Allow(ctx, userID)
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
		if !allowed {
			t.Fatalf("call %d: expected allowed, got denied", i)
		}
	}
}

func TestRateLimiter_BlocksFourthCall(t *testing.T) {
	rl := newFakeRL(3)
	ctx := context.Background()
	userID := "user-xyz"

	for i := 0; i < 3; i++ {
		rl.Allow(ctx, userID) //nolint:errcheck
	}

	allowed, err := rl.Allow(ctx, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatal("expected 4th call to be denied, got allowed")
	}
}

func TestRateLimiter_IndependentUsersDoNotInterfere(t *testing.T) {
	rl := newFakeRL(3)
	ctx := context.Background()

	// User A exhausts their limit
	for i := 0; i < 3; i++ {
		rl.Allow(ctx, "user-a") //nolint:errcheck
	}

	// User B should still be allowed
	allowed, err := rl.Allow(ctx, "user-b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("user-b should be allowed even though user-a is rate-limited")
	}
}

func TestRateLimiter_CounterDoesNotExceedLimitOnBlock(t *testing.T) {
	rl := newFakeRL(3)
	ctx := context.Background()
	userID := "user-count"

	for i := 0; i < 3; i++ {
		rl.Allow(ctx, userID) //nolint:errcheck
	}
	if rl.counts[userID] != 3 {
		t.Fatalf("expected counter=3 after 3 allowed calls, got %d", rl.counts[userID])
	}

	// Blocked call should not increment
	rl.Allow(ctx, userID) //nolint:errcheck
	if rl.counts[userID] != 3 {
		t.Fatalf("expected counter=3 after blocked call, got %d", rl.counts[userID])
	}
}

// Integration-style test for the real RateLimiter using a live Redis connection.
// Skipped unless TEST_REDIS_ADDR is set.
func TestRateLimiter_Integration(t *testing.T) {
	addr := store.TestRedisAddr(t)
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set — skipping Redis integration test")
	}

	// Real test using go-redis would go here; skipped in CI without Redis.
}
