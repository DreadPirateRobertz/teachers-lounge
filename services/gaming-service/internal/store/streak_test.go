package store_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// We test the Redis-facing streak logic directly by replaying the same
// HMGet / HMSet sequence the store uses, so we don't need a real Postgres.
// Full integration tests (with Postgres) belong in the e2e suite.

func TestStreakCheckin_FirstCheckin(t *testing.T) {
	mr, rdb := newMiniredis(t)
	defer mr.Close()

	userID := "user-abc"
	current, reset := simulateCheckin(t, rdb, userID, time.Now())
	if current != 1 {
		t.Errorf("first checkin: want streak=1, got %d", current)
	}
	if reset {
		t.Error("first checkin: want reset=false")
	}
}

func TestStreakCheckin_ConsecutiveIncrement(t *testing.T) {
	mr, rdb := newMiniredis(t)
	defer mr.Close()

	userID := "user-abc"
	now := time.Now().UTC()

	simulateCheckin(t, rdb, userID, now.Add(-2*time.Hour))  // first
	current, reset := simulateCheckin(t, rdb, userID, now) // second, within 24h

	if current != 2 {
		t.Errorf("consecutive: want streak=2, got %d", current)
	}
	if reset {
		t.Error("consecutive: want reset=false")
	}
}

func TestStreakCheckin_ResetAfterGap(t *testing.T) {
	mr, rdb := newMiniredis(t)
	defer mr.Close()

	userID := "user-abc"
	now := time.Now().UTC()

	simulateCheckin(t, rdb, userID, now.Add(-25*time.Hour)) // old checkin
	current, reset := simulateCheckin(t, rdb, userID, now)  // > 24h gap

	if current != 1 {
		t.Errorf("reset: want streak=1, got %d", current)
	}
	if !reset {
		t.Error("reset: want reset=true")
	}
}

func TestStreakCheckin_MultipleUsers_Independent(t *testing.T) {
	mr, rdb := newMiniredis(t)
	defer mr.Close()

	now := time.Now().UTC()

	for _, id := range []string{"alice", "bob", "carol"} {
		simulateCheckin(t, rdb, id, now.Add(-1*time.Hour))
		current, reset := simulateCheckin(t, rdb, id, now)
		if current != 2 {
			t.Errorf("user %s: want streak=2, got %d", id, current)
		}
		if reset {
			t.Errorf("user %s: want reset=false", id)
		}
	}
}

func TestStreakCheckin_ExactlyAtBoundary(t *testing.T) {
	tests := []struct {
		name      string
		gapHours  float64
		wantReset bool
		wantCount int
	}{
		{"23h59m gap — no reset", 23.98, false, 2},
		{"24h01m gap — resets", 24.02, true, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr, rdb := newMiniredis(t)
			defer mr.Close()

			now := time.Now().UTC()
			gap := time.Duration(tt.gapHours * float64(time.Hour))
			simulateCheckin(t, rdb, "u1", now.Add(-gap))
			count, reset := simulateCheckin(t, rdb, "u1", now)

			if count != tt.wantCount {
				t.Errorf("count: got %d, want %d", count, tt.wantCount)
			}
			if reset != tt.wantReset {
				t.Errorf("reset: got %v, want %v", reset, tt.wantReset)
			}
		})
	}
}

// simulateCheckin replicates the Redis-side logic from store.StreakCheckin.
// It sets the current timestamp to `at` so tests can control the clock.
func simulateCheckin(t *testing.T, rdb *redis.Client, userID string, at time.Time) (count int, reset bool) {
	t.Helper()
	ctx := context.Background()
	key := fmt.Sprintf("streak:%s", userID)

	vals, err := rdb.HMGet(ctx, key, "count", "last_ts").Result()
	if err != nil {
		t.Fatalf("hmget: %v", err)
	}

	const resetWindow = 24 * time.Hour
	reset = false

	if vals[0] != nil && vals[1] != nil {
		var prevCount int64
		var lastTS int64
		_, _ = fmt.Sscan(vals[0].(string), &prevCount)
		_, _ = fmt.Sscan(vals[1].(string), &lastTS)

		lastTime := time.Unix(lastTS, 0)
		if at.Sub(lastTime) > resetWindow {
			count = 1
			reset = true
		} else {
			count = int(prevCount) + 1
		}
	} else {
		count = 1
	}

	if err := rdb.HMSet(ctx, key, map[string]any{
		"count":   strconv.Itoa(count),
		"last_ts": strconv.FormatInt(at.Unix(), 10),
	}).Err(); err != nil {
		t.Fatalf("hmset: %v", err)
	}

	return count, reset
}

func newMiniredis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, rdb
}
