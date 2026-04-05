package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/teacherslounge/gaming-service/internal/ratelimit"
)

func newTestLimiter(t *testing.T) (*ratelimit.Limiter, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return ratelimit.New(rdb), mr
}

// TestAllow_UnderLimit verifies requests are allowed while tokens remain.
func TestAllow_UnderLimit(t *testing.T) {
	lim, _ := newTestLimiter(t)
	b := ratelimit.Bucket{Name: "test", Capacity: 5, Rate: 1}

	for i := 0; i < 5; i++ {
		res, err := lim.Allow(context.Background(), b, "user-a")
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i+1, err)
		}
		if !res.Allowed {
			t.Fatalf("request %d: expected allowed, got denied", i+1)
		}
	}
}

// TestAllow_ExceedsCapacity verifies the bucket rejects the (capacity+1)th request.
func TestAllow_ExceedsCapacity(t *testing.T) {
	lim, _ := newTestLimiter(t)
	b := ratelimit.Bucket{Name: "test", Capacity: 3, Rate: 1.0 / 60}

	for i := 0; i < 3; i++ {
		res, err := lim.Allow(context.Background(), b, "user-b")
		if err != nil {
			t.Fatalf("allow %d: %v", i, err)
		}
		if !res.Allowed {
			t.Fatalf("expected allowed on request %d", i+1)
		}
	}

	res, err := lim.Allow(context.Background(), b, "user-b")
	if err != nil {
		t.Fatalf("4th allow: %v", err)
	}
	if res.Allowed {
		t.Error("expected denied after capacity exhausted")
	}
	if res.RetryAfter == 0 {
		t.Error("expected non-zero RetryAfter when denied")
	}
}

// TestAllow_RemainingDecreases verifies remaining token count decreases per request.
func TestAllow_RemainingDecreases(t *testing.T) {
	lim, _ := newTestLimiter(t)
	b := ratelimit.Bucket{Name: "remaining", Capacity: 5, Rate: 1}

	var prev = 5
	for i := 0; i < 5; i++ {
		res, err := lim.Allow(context.Background(), b, "user-c")
		if err != nil {
			t.Fatal(err)
		}
		if !res.Allowed {
			t.Fatalf("expected allowed on request %d", i+1)
		}
		if res.Remaining >= prev {
			t.Errorf("request %d: remaining %d should be < previous %d", i+1, res.Remaining, prev)
		}
		prev = res.Remaining
	}
}

// TestAllow_IndependentBuckets verifies two users have independent buckets.
func TestAllow_IndependentBuckets(t *testing.T) {
	lim, _ := newTestLimiter(t)
	b := ratelimit.Bucket{Name: "iso", Capacity: 2, Rate: 1.0 / 60}

	// Exhaust user-x
	for i := 0; i < 2; i++ {
		lim.Allow(context.Background(), b, "user-x") //nolint:errcheck
	}

	// user-y should still be allowed
	res, err := lim.Allow(context.Background(), b, "user-y")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Allowed {
		t.Error("user-y should be unaffected by user-x exhausting their bucket")
	}
}

// TestAllow_IndependentBucketNames verifies the same user has separate buckets per name.
func TestAllow_IndependentBucketNames(t *testing.T) {
	lim, _ := newTestLimiter(t)
	xp := ratelimit.Bucket{Name: "xp", Capacity: 1, Rate: 1.0 / 60}
	quiz := ratelimit.Bucket{Name: "quiz", Capacity: 2, Rate: 1.0 / 60}

	// Exhaust xp bucket
	lim.Allow(context.Background(), xp, "user-d") //nolint:errcheck
	resXP, _ := lim.Allow(context.Background(), xp, "user-d")
	if resXP.Allowed {
		t.Error("xp bucket should be exhausted")
	}

	// quiz bucket for same user should still allow
	resQuiz, err := lim.Allow(context.Background(), quiz, "user-d")
	if err != nil {
		t.Fatal(err)
	}
	if !resQuiz.Allowed {
		t.Error("quiz bucket should be independent from xp bucket")
	}
}

// TestAllow_RefillAfterFastForward verifies tokens refill after time advances.
// Uses NowFunc injection to control the clock deterministically — miniredis
// FastForward does not affect redis.call('TIME') inside Lua scripts.
func TestAllow_RefillAfterFastForward(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	fakeNow := float64(time.Now().Unix())
	lim := ratelimit.New(rdb)
	lim.NowFunc = func() float64 { return fakeNow }

	b := ratelimit.Bucket{Name: "refill", Capacity: 2, Rate: 1} // 1 token/sec

	// Exhaust bucket
	lim.Allow(context.Background(), b, "user-e") //nolint:errcheck
	lim.Allow(context.Background(), b, "user-e") //nolint:errcheck
	res, _ := lim.Allow(context.Background(), b, "user-e")
	if res.Allowed {
		t.Fatal("bucket should be empty after exhaustion")
	}

	// Advance fake clock by 3 seconds — should refill at least 2 tokens (rate=1/s)
	fakeNow += 3

	// After refill, request should succeed
	res, err = lim.Allow(context.Background(), b, "user-e")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Allowed {
		t.Error("expected allowed after time-based refill")
	}
}

// TestAllow_PredefinedBuckets verifies the exported Bucket constants are valid.
func TestAllow_PredefinedBuckets(t *testing.T) {
	lim, _ := newTestLimiter(t)
	buckets := []ratelimit.Bucket{
		ratelimit.BucketXP,
		ratelimit.BucketQuizStart,
		ratelimit.BucketQuizAnswer,
	}
	for _, b := range buckets {
		res, err := lim.Allow(context.Background(), b, "user-predef")
		if err != nil {
			t.Errorf("bucket %q: unexpected error: %v", b.Name, err)
		}
		if !res.Allowed {
			t.Errorf("bucket %q: first request should be allowed", b.Name)
		}
	}
}
