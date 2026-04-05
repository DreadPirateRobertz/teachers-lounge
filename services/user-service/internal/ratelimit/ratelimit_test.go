package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/teacherslounge/user-service/internal/ratelimit"
)

func newTestLimiter(t *testing.T) (*ratelimit.Limiter, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return ratelimit.New(rdb), mr
}

// TestAllow_UnderLimit verifies requests pass while tokens remain.
func TestAllow_UnderLimit(t *testing.T) {
	lim, _ := newTestLimiter(t)
	b := ratelimit.Bucket{Name: "test", Capacity: 3, Rate: 1}

	for i := 0; i < 3; i++ {
		res, err := lim.Allow(context.Background(), b, "1.2.3.4")
		if err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
		if !res.Allowed {
			t.Fatalf("request %d: expected allowed", i+1)
		}
	}
}

// TestAllow_ExceedsCapacity verifies the (capacity+1)th request is denied.
func TestAllow_ExceedsCapacity(t *testing.T) {
	lim, _ := newTestLimiter(t)
	b := ratelimit.Bucket{Name: "test", Capacity: 2, Rate: 1.0 / 3600}

	for i := 0; i < 2; i++ {
		lim.Allow(context.Background(), b, "1.2.3.4") //nolint:errcheck
	}

	res, err := lim.Allow(context.Background(), b, "1.2.3.4")
	if err != nil {
		t.Fatalf("3rd allow: %v", err)
	}
	if res.Allowed {
		t.Error("expected denied after capacity exhausted")
	}
	if res.RetryAfter == 0 {
		t.Error("expected non-zero RetryAfter when denied")
	}
}

// TestAllow_IndependentSubjects verifies two IPs have independent buckets.
func TestAllow_IndependentSubjects(t *testing.T) {
	lim, _ := newTestLimiter(t)
	b := ratelimit.Bucket{Name: "user_create", Capacity: 1, Rate: 1.0 / 3600}

	// Exhaust IP-A
	lim.Allow(context.Background(), b, "10.0.0.1") //nolint:errcheck
	resA, _ := lim.Allow(context.Background(), b, "10.0.0.1")
	if resA.Allowed {
		t.Fatal("10.0.0.1 should be exhausted")
	}

	// IP-B should be unaffected
	resB, err := lim.Allow(context.Background(), b, "10.0.0.2")
	if err != nil {
		t.Fatal(err)
	}
	if !resB.Allowed {
		t.Error("10.0.0.2 should have its own independent bucket")
	}
}

// TestAllow_RefillAfterTime verifies tokens refill after the clock advances.
func TestAllow_RefillAfterTime(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	fakeNow := float64(time.Now().Unix())
	lim := ratelimit.New(rdb)
	lim.NowFunc = func() float64 { return fakeNow }

	b := ratelimit.Bucket{Name: "refill", Capacity: 2, Rate: 1} // 1 token/sec

	// Exhaust bucket
	lim.Allow(context.Background(), b, "1.1.1.1") //nolint:errcheck
	lim.Allow(context.Background(), b, "1.1.1.1") //nolint:errcheck
	res, _ := lim.Allow(context.Background(), b, "1.1.1.1")
	if res.Allowed {
		t.Fatal("should be exhausted")
	}

	// Advance clock 3 seconds → at least 2 tokens refilled
	fakeNow += 3

	res, err = lim.Allow(context.Background(), b, "1.1.1.1")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Allowed {
		t.Error("expected allowed after time-based refill")
	}
}

// TestBucketUserCreate_Config verifies the exported constant has sensible parameters.
func TestBucketUserCreate_Config(t *testing.T) {
	b := ratelimit.BucketUserCreate
	if b.Capacity <= 0 {
		t.Errorf("BucketUserCreate.Capacity must be positive, got %v", b.Capacity)
	}
	if b.Rate <= 0 {
		t.Errorf("BucketUserCreate.Rate must be positive, got %v", b.Rate)
	}
	if b.Name == "" {
		t.Error("BucketUserCreate.Name must not be empty")
	}

	// First request should be allowed (fresh bucket)
	lim, _ := newTestLimiter(t)
	res, err := lim.Allow(context.Background(), b, "203.0.113.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Allowed {
		t.Error("first request on fresh bucket should be allowed")
	}
}

// TestBucketUserCreate_BlocksAfterBurst verifies registration is blocked after
// burst capacity is exhausted (clock frozen).
func TestBucketUserCreate_BlocksAfterBurst(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	fakeNow := float64(time.Now().Unix())
	lim := ratelimit.New(rdb)
	lim.NowFunc = func() float64 { return fakeNow } // freeze clock

	b := ratelimit.BucketUserCreate
	ip := "198.51.100.42"

	// Drain full capacity
	for i := 0; i < int(b.Capacity); i++ {
		res, err := lim.Allow(context.Background(), b, ip)
		if err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
		if !res.Allowed {
			t.Fatalf("request %d: expected allowed within burst capacity", i+1)
		}
	}

	// Next request must be denied
	res, err := lim.Allow(context.Background(), b, ip)
	if err != nil {
		t.Fatalf("over-burst request: %v", err)
	}
	if res.Allowed {
		t.Error("expected denied after burst exhausted")
	}
}
