package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/ratelimit"
)

// stubLimiter is a controllable test double for the RateLimiter interface.
type stubLimiter struct {
	result ratelimit.Result
	err    error
}

func (s *stubLimiter) Allow(_ context.Context, _ ratelimit.Bucket, _ string) (ratelimit.Result, error) {
	return s.result, s.err
}

func newOKHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// contextWithUser injects a user ID the way Authenticate middleware does.
func contextWithUser(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), ctxKeyUserID{}, userID)
	return r.WithContext(ctx)
}

func TestRateLimit_AllowedRequest(t *testing.T) {
	lim := &stubLimiter{result: ratelimit.Result{Allowed: true, Remaining: 9}}
	mw := RateLimit(lim, ratelimit.BucketXP, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/gaming/xp", nil)
	req = contextWithUser(req, "user-1")
	rec := httptest.NewRecorder()

	mw(newOKHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-RateLimit-Remaining") != "9" {
		t.Errorf("expected X-RateLimit-Remaining=9, got %q", rec.Header().Get("X-RateLimit-Remaining"))
	}
}

func TestRateLimit_DeniedRequest_Returns429(t *testing.T) {
	lim := &stubLimiter{result: ratelimit.Result{Allowed: false, Remaining: 0, RetryAfter: 6_000_000_000}} // 6s
	mw := RateLimit(lim, ratelimit.BucketXP, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/gaming/xp", nil)
	req = contextWithUser(req, "user-2")
	rec := httptest.NewRecorder()

	mw(newOKHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header on 429")
	}
	if rec.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("expected X-RateLimit-Remaining=0, got %q", rec.Header().Get("X-RateLimit-Remaining"))
	}
}

func TestRateLimit_MissingUserID_Returns401(t *testing.T) {
	lim := &stubLimiter{result: ratelimit.Result{Allowed: true}}
	mw := RateLimit(lim, ratelimit.BucketXP, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/gaming/xp", nil)
	// No user ID in context.
	rec := httptest.NewRecorder()

	mw(newOKHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRateLimit_LimiterError_FailsOpen(t *testing.T) {
	lim := &stubLimiter{err: context.DeadlineExceeded}
	mw := RateLimit(lim, ratelimit.BucketXP, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/gaming/xp", nil)
	req = contextWithUser(req, "user-3")
	rec := httptest.NewRecorder()

	mw(newOKHandler()).ServeHTTP(rec, req)

	// Fail open: request proceeds despite Redis error.
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (fail open), got %d", rec.Code)
	}
}

func TestRateLimit_XRateLimitLimitHeader(t *testing.T) {
	lim := &stubLimiter{result: ratelimit.Result{Allowed: true, Remaining: 4}}
	b := ratelimit.Bucket{Name: "test", Capacity: 5, Rate: 1}
	mw := RateLimit(lim, b, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req = contextWithUser(req, "user-4")
	rec := httptest.NewRecorder()

	mw(newOKHandler()).ServeHTTP(rec, req)

	if rec.Header().Get("X-RateLimit-Limit") != "5" {
		t.Errorf("expected X-RateLimit-Limit=5, got %q", rec.Header().Get("X-RateLimit-Limit"))
	}
}
