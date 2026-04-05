package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

// stubRateLimitBackend is a controllable in-memory counter.
type stubRateLimitBackend struct {
	count atomic.Int64
	err   error
}

func (s *stubRateLimitBackend) IncrWithTTL(_ context.Context, _ string, _ time.Duration) (int64, error) {
	if s.err != nil {
		return 0, s.err
	}
	return s.count.Add(1), nil
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func ctxWithUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), ctxKeyUserID{}, userID)
	return r.WithContext(ctx)
}

// TestRateLimit_AllowsUnderLimit verifies requests pass while under the limit.
func TestRateLimit_AllowsUnderLimit(t *testing.T) {
	backend := &stubRateLimitBackend{}
	uid := uuid.New().String()
	mw := RateLimit(backend, "admin_audit", 5, time.Hour)

	for i := 0; i < 5; i++ {
		req := ctxWithUserID(httptest.NewRequest(http.MethodGet, "/admin/audit", nil), uid)
		rec := httptest.NewRecorder()
		mw(okHandler()).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}
}

// TestRateLimit_Returns429WhenExceeded verifies the (limit+1)th request is rejected.
func TestRateLimit_Returns429WhenExceeded(t *testing.T) {
	backend := &stubRateLimitBackend{}
	uid := uuid.New().String()
	mw := RateLimit(backend, "admin_audit", 3, time.Hour)

	for i := 0; i < 3; i++ {
		req := ctxWithUserID(httptest.NewRequest(http.MethodGet, "/admin/audit", nil), uid)
		rec := httptest.NewRecorder()
		mw(okHandler()).ServeHTTP(rec, req)
	}

	req := ctxWithUserID(httptest.NewRequest(http.MethodGet, "/admin/audit", nil), uid)
	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header on 429")
	}
}

// TestRateLimit_FailsOpenOnBackendError verifies requests proceed when Redis is unavailable.
func TestRateLimit_FailsOpenOnBackendError(t *testing.T) {
	backend := &stubRateLimitBackend{err: fmt.Errorf("redis down")}
	uid := uuid.New().String()
	mw := RateLimit(backend, "admin_audit", 1, time.Hour)

	req := ctxWithUserID(httptest.NewRequest(http.MethodGet, "/admin/audit", nil), uid)
	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (fail open), got %d", rec.Code)
	}
}

// TestRateLimit_PassesThroughWhenNoUserID verifies unauthenticated requests are not blocked.
func TestRateLimit_PassesThroughWhenNoUserID(t *testing.T) {
	backend := &stubRateLimitBackend{}
	mw := RateLimit(backend, "admin_audit", 1, time.Hour)

	// No user in context.
	req := httptest.NewRequest(http.MethodGet, "/admin/audit", nil)
	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for unauthenticated pass-through, got %d", rec.Code)
	}
}
