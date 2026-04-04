package store

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	pushLimitPerWindow = 3
	pushWindow         = 24 * time.Hour
)

// RateLimiter enforces per-user push notification limits via Redis.
// Key format: ratelimit:{userID}:notif
// Each key is an integer counter with a 24-hour TTL.
type RateLimiter struct {
	rdb redis.Cmdable
}

// NewRateLimiter returns a RateLimiter backed by the given Redis client.
func NewRateLimiter(rdb redis.Cmdable) *RateLimiter {
	return &RateLimiter{rdb: rdb}
}

// Allow returns true if the user is within the push rate limit and increments
// their counter. Returns false (and does not increment) if the limit is reached.
// Returns an error only on Redis failure.
func (r *RateLimiter) Allow(ctx context.Context, userID string) (bool, error) {
	key := fmt.Sprintf("ratelimit:%s:notif", userID)

	// Increment first; if this is the first call in the window, set TTL.
	count, err := r.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("ratelimit: incr %s: %w", key, err)
	}

	if count == 1 {
		// First notification in this window — set the expiry.
		// This error is fatal: without a TTL the key never expires and the user
		// would be permanently locked out once they hit the limit.
		if err := r.rdb.Expire(ctx, key, pushWindow).Err(); err != nil {
			// Roll back the increment so we don't leave a keyless counter behind.
			r.rdb.Decr(ctx, key) //nolint:errcheck — best-effort rollback
			return false, fmt.Errorf("ratelimit: set ttl %s: %w", key, err)
		}
	}

	if count > pushLimitPerWindow {
		// Undo the increment so the count stays accurate.
		r.rdb.Decr(ctx, key) //nolint:errcheck — best-effort
		return false, nil
	}
	return true, nil
}

// Remaining returns how many push notifications the user has left in the
// current window. Used for informational headers (X-RateLimit-Remaining).
func (r *RateLimiter) Remaining(ctx context.Context, userID string) (int, error) {
	key := fmt.Sprintf("ratelimit:%s:notif", userID)
	count, err := r.rdb.Get(ctx, key).Int()
	if err == redis.Nil {
		return pushLimitPerWindow, nil
	}
	if err != nil {
		return 0, fmt.Errorf("ratelimit: get %s: %w", key, err)
	}
	remaining := pushLimitPerWindow - count
	if remaining < 0 {
		return 0, nil
	}
	return remaining, nil
}
