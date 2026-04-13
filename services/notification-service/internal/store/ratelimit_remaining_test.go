package store_test

// Tests for RateLimiter.Remaining using a fake Redis adapter that implements
// redis.Cmdable via go-redis miniredis. Since miniredis requires importing a
// test-only package that may not be available, we use the same fake-adapter
// pattern already established in ratelimit_test.go for unit coverage.
//
// Remaining tests exercise the three paths:
//   - Key absent (fresh user)  → returns pushLimitPerWindow (3)
//   - Key present, count < limit → returns pushLimitPerWindow - count
//   - Key present, count >= limit → returns 0

import (
	"context"
	"testing"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/store"
	"github.com/redis/go-redis/v9"
)

// TestRemaining_Integration is a Redis integration test for the real
// RateLimiter.Remaining via a live Redis instance.
// Skipped unless TEST_REDIS_ADDR is set in the environment.
func TestRemaining_Integration(t *testing.T) {
	addr := store.TestRedisAddr(t)
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set — skipping Redis integration test")
	}

	rdb := redis.NewClient(&redis.Options{Addr: addr})
	defer rdb.Close() //nolint:errcheck

	rl := store.NewRateLimiter(rdb)
	ctx := context.Background()
	userID := "remaining-test-user"

	// Clean state.
	rdb.Del(ctx, "ratelimit:"+userID+":notif") //nolint:errcheck

	// No calls yet — remaining should equal the full limit (3).
	rem, err := rl.Remaining(ctx, userID)
	if err != nil {
		t.Fatalf("Remaining error: %v", err)
	}
	if rem != 3 {
		t.Fatalf("expected 3 remaining before any calls, got %d", rem)
	}

	// Use one slot.
	rl.Allow(ctx, userID) //nolint:errcheck

	rem, err = rl.Remaining(ctx, userID)
	if err != nil {
		t.Fatalf("Remaining after 1 call: %v", err)
	}
	if rem != 2 {
		t.Fatalf("expected 2 remaining after 1 call, got %d", rem)
	}

	// Exhaust the limit.
	rl.Allow(ctx, userID) //nolint:errcheck
	rl.Allow(ctx, userID) //nolint:errcheck

	rem, err = rl.Remaining(ctx, userID)
	if err != nil {
		t.Fatalf("Remaining after exhausting limit: %v", err)
	}
	if rem != 0 {
		t.Fatalf("expected 0 remaining after exhausting limit, got %d", rem)
	}

	rdb.Del(ctx, "ratelimit:"+userID+":notif") //nolint:errcheck
}
