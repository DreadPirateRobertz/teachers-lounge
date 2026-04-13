package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/middleware"
)

// ── Fake Limiter ──────────────────────────────────────────────────────────────

// allowLimiter always permits requests.
type allowLimiter struct{}

func (l *allowLimiter) Allow(_ context.Context, _ string) (bool, error) { return true, nil }

// denyLimiter always blocks requests.
type denyLimiter struct{}

func (l *denyLimiter) Allow(_ context.Context, _ string) (bool, error) { return false, nil }

// errorLimiter returns a Redis-style error.
type errorLimiter struct{}

func (l *errorLimiter) Allow(_ context.Context, _ string) (bool, error) {
	return false, errors.New("redis: connection refused")
}

// nextHandler is a simple final handler that records whether it was called.
type nextHandler struct{ called bool }

func (h *nextHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	h.called = true
	w.WriteHeader(http.StatusOK)
}

// requestWithUserID builds a request with the given user ID injected via WithUserID.
func requestWithUserID(userID string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/notify/push", nil)
	if userID != "" {
		req = req.WithContext(middleware.WithUserID(req.Context(), userID))
	}
	return req
}

// ── PushRateLimit tests ───────────────────────────────────────────────────────

// TestPushRateLimit_AllowedRequest_PassesToNextHandler verifies that a request
// within the rate limit is forwarded to the next handler with 200.
func TestPushRateLimit_AllowedRequest_PassesToNextHandler(t *testing.T) {
	next := &nextHandler{}
	mw := middleware.PushRateLimit(&allowLimiter{})(next)

	req := requestWithUserID("user-1")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if !next.called {
		t.Fatal("expected next handler to be called for an allowed request")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// TestPushRateLimit_RateLimited_Returns429 verifies that a rate-limited request
// returns 429 Too Many Requests without reaching the next handler.
func TestPushRateLimit_RateLimited_Returns429(t *testing.T) {
	next := &nextHandler{}
	mw := middleware.PushRateLimit(&denyLimiter{})(next)

	req := requestWithUserID("user-2")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if next.called {
		t.Fatal("expected next handler NOT to be called for a rate-limited request")
	}
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rr.Code)
	}
}

// TestPushRateLimit_MissingUserID_Returns400 verifies that a request with no
// user ID in the context is rejected with 400.
func TestPushRateLimit_MissingUserID_Returns400(t *testing.T) {
	next := &nextHandler{}
	mw := middleware.PushRateLimit(&allowLimiter{})(next)

	req := requestWithUserID("") // no user ID
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if next.called {
		t.Fatal("expected next handler NOT to be called when user ID is absent")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// TestPushRateLimit_LimiterError_FailsOpen verifies that a Redis/limiter error
// does not block the request — the middleware fails open.
func TestPushRateLimit_LimiterError_FailsOpen(t *testing.T) {
	next := &nextHandler{}
	mw := middleware.PushRateLimit(&errorLimiter{})(next)

	req := requestWithUserID("user-3")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if !next.called {
		t.Fatal("expected next handler to be called on limiter error (fail-open)")
	}
}

// TestPushRateLimit_RetryAfterHeader verifies that the Retry-After header is
// set on rate-limited responses.
func TestPushRateLimit_RetryAfterHeader(t *testing.T) {
	next := &nextHandler{}
	mw := middleware.PushRateLimit(&denyLimiter{})(next)

	req := requestWithUserID("user-4")
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header on 429 response")
	}
}

// ── WithUserID / UserIDFromContext tests ──────────────────────────────────────

// TestWithUserID_RoundTrip verifies that WithUserID and UserIDFromContext
// correctly store and retrieve the user ID through the request context.
func TestWithUserID_RoundTrip(t *testing.T) {
	ctx := middleware.WithUserID(context.Background(), "member-42")
	got := middleware.UserIDFromContext(ctx)
	if got != "member-42" {
		t.Fatalf("expected %q, got %q", "member-42", got)
	}
}

// TestUserIDFromContext_EmptyWhenNotSet verifies that UserIDFromContext returns
// an empty string when no user ID has been set.
func TestUserIDFromContext_EmptyWhenNotSet(t *testing.T) {
	got := middleware.UserIDFromContext(context.Background())
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}
