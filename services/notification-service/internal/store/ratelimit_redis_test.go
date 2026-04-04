package store_test

// Tests for the real store.RateLimiter using miniredis.
// These exercise the actual Incr/Expire/Decr logic — not a hand-rolled fake.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/middleware"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/store"
)

// newTestRedis starts an in-process miniredis server and returns a connected
// go-redis client. The server is automatically stopped when the test ends.
func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestRealRateLimiter_AllowsUpToLimit(t *testing.T) {
	rdb := newTestRedis(t)
	rl := store.NewRateLimiter(rdb)
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		allowed, err := rl.Allow(ctx, "user-real-1")
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
		if !allowed {
			t.Fatalf("call %d: expected allowed, got denied", i)
		}
	}
}

func TestRealRateLimiter_BlocksFourthCall(t *testing.T) {
	rdb := newTestRedis(t)
	rl := store.NewRateLimiter(rdb)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if _, err := rl.Allow(ctx, "user-real-2"); err != nil {
			t.Fatalf("setup call %d: %v", i, err)
		}
	}

	allowed, err := rl.Allow(ctx, "user-real-2")
	if err != nil {
		t.Fatalf("4th call: unexpected error: %v", err)
	}
	if allowed {
		t.Fatal("expected 4th call to be denied, got allowed")
	}
}

func TestRealRateLimiter_CounterDoesNotExceedLimitOnBlock(t *testing.T) {
	rdb := newTestRedis(t)
	rl := store.NewRateLimiter(rdb)
	ctx := context.Background()
	user := "user-real-count"

	for i := 0; i < 3; i++ {
		rl.Allow(ctx, user) //nolint:errcheck
	}
	// Blocked call should roll back the Incr
	rl.Allow(ctx, user) //nolint:errcheck

	count, err := rdb.Get(ctx, fmt.Sprintf("ratelimit:%s:notif", user)).Int()
	if err != nil {
		t.Fatalf("get counter: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected counter=3 after blocked call, got %d", count)
	}
}

func TestRealRateLimiter_KeyHasTTL(t *testing.T) {
	rdb := newTestRedis(t)
	rl := store.NewRateLimiter(rdb)
	ctx := context.Background()
	user := "user-real-ttl"

	if _, err := rl.Allow(ctx, user); err != nil {
		t.Fatalf("allow: %v", err)
	}

	ttl := rdb.TTL(ctx, fmt.Sprintf("ratelimit:%s:notif", user)).Val()
	if ttl <= 0 {
		t.Fatalf("expected positive TTL on rate-limit key, got %v", ttl)
	}
	// Should be close to 24h
	if ttl > 25*time.Hour || ttl < 23*time.Hour {
		t.Fatalf("TTL out of expected 24h range: %v", ttl)
	}
}

func TestRealRateLimiter_IndependentUsers(t *testing.T) {
	rdb := newTestRedis(t)
	rl := store.NewRateLimiter(rdb)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		rl.Allow(ctx, "user-real-a") //nolint:errcheck
	}

	allowed, err := rl.Allow(ctx, "user-real-b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("user-b should be allowed even though user-a is rate-limited")
	}
}

// TestPushEndpoint_Returns429WhenRateLimitExceeded tests the full HTTP stack:
// real RateLimiter → PushRateLimit middleware → Push handler.
func TestPushEndpoint_Returns429WhenRateLimitExceeded(t *testing.T) {
	rdb := newTestRedis(t)
	rl := store.NewRateLimiter(rdb)
	limiter := middleware.PushRateLimit(rl)

	// Stub handler that always returns 200
	stub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := limiter(stub)

	makeRequest := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/notify/push", nil)
		// Inject userID into context as the auth middleware would
		ctx := middleware.WithUserID(req.Context(), "user-429-test")
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr
	}

	// First 3 should succeed
	for i := 1; i <= 3; i++ {
		rr := makeRequest()
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, rr.Code)
		}
	}

	// 4th should be rate-limited
	rr := makeRequest()
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on 4th request, got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header on 429 response")
	}
}
