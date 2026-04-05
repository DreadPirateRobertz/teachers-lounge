package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/teacherslounge/user-service/internal/ratelimit"
)

// stubIPLimiter is an in-memory fake for IPRateLimiter.
type stubIPLimiter struct {
	allowed    bool
	remaining  int
	retryAfter time.Duration
	err        error
}

func (s *stubIPLimiter) Allow(_ context.Context, _ ratelimit.Bucket, _ string) (ratelimit.Result, error) {
	return ratelimit.Result{
		Allowed:    s.allowed,
		Remaining:  s.remaining,
		RetryAfter: s.retryAfter,
	}, s.err
}

// TestIPRateLimit_AllowsWhenTokensAvailable verifies the middleware passes requests
// through when the limiter allows them.
func TestIPRateLimit_AllowsWhenTokensAvailable(t *testing.T) {
	lim := &stubIPLimiter{allowed: true, remaining: 4}
	mw := IPRateLimit(lim, ratelimit.BucketUserCreate)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", nil)
	req.RemoteAddr = "203.0.113.1"
	rec := httptest.NewRecorder()

	mw(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-RateLimit-Remaining") != "4" {
		t.Errorf("expected X-RateLimit-Remaining: 4, got %q", rec.Header().Get("X-RateLimit-Remaining"))
	}
}

// TestIPRateLimit_Returns429WhenExhausted verifies 429 is returned with headers
// when the token bucket is empty.
func TestIPRateLimit_Returns429WhenExhausted(t *testing.T) {
	lim := &stubIPLimiter{allowed: false, retryAfter: 2 * time.Minute}
	mw := IPRateLimit(lim, ratelimit.BucketUserCreate)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", nil)
	req.RemoteAddr = "198.51.100.1"
	rec := httptest.NewRecorder()

	mw(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header on 429")
	}
	if rec.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("expected X-RateLimit-Limit header on 429")
	}
	if rec.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("expected X-RateLimit-Reset header on 429")
	}
}

// TestIPRateLimit_FailsOpenOnLimiterError verifies that requests proceed when
// Redis is unavailable (fail-open behaviour).
func TestIPRateLimit_FailsOpenOnLimiterError(t *testing.T) {
	lim := &stubIPLimiter{err: fmt.Errorf("redis: connection refused")}
	mw := IPRateLimit(lim, ratelimit.BucketUserCreate)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", nil)
	req.RemoteAddr = "10.0.0.1"
	rec := httptest.NewRecorder()

	mw(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (fail open), got %d", rec.Code)
	}
}

// TestIPRateLimit_KeysOnRemoteAddr verifies two different IPs get independent
// treatment (first IP exhausted, second IP allowed).
func TestIPRateLimit_KeysOnRemoteAddr(t *testing.T) {
	callCount := 0
	// Alternate: first call exhausted, subsequent calls allowed.
	lim := &callCountingLimiter{results: []ratelimit.Result{
		{Allowed: false, RetryAfter: time.Minute}, // IP-A call
		{Allowed: true, Remaining: 3},             // IP-B call
	}}

	mw := IPRateLimit(lim, ratelimit.BucketUserCreate)

	// IP-A is exhausted
	reqA := httptest.NewRequest(http.MethodPost, "/auth/register", nil)
	reqA.RemoteAddr = "192.0.2.1"
	recA := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(recA, reqA)
	if recA.Code != http.StatusTooManyRequests {
		t.Errorf("IP-A: expected 429, got %d", recA.Code)
	}

	// IP-B is allowed
	reqB := httptest.NewRequest(http.MethodPost, "/auth/register", nil)
	reqB.RemoteAddr = "192.0.2.2"
	recB := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(recB, reqB)
	if recB.Code != http.StatusOK {
		t.Errorf("IP-B: expected 200, got %d", recB.Code)
	}

	_ = callCount
}

// callCountingLimiter returns pre-configured results in sequence.
type callCountingLimiter struct {
	results []ratelimit.Result
	idx     int
}

func (c *callCountingLimiter) Allow(_ context.Context, _ ratelimit.Bucket, _ string) (ratelimit.Result, error) {
	if c.idx >= len(c.results) {
		return ratelimit.Result{Allowed: true}, nil
	}
	res := c.results[c.idx]
	c.idx++
	return res, nil
}
