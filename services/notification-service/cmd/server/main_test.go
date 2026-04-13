package main

// Tests for pure helper functions in cmd/server/main.go:
//   - envOr: returns env value when set, fallback otherwise.
//   - configFromEnv: builds a config struct from environment variables.
//   - stubAuthMiddleware: injects X-User-ID header into request context.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/middleware"
)

// TestEnvOr_ReturnsFallbackWhenUnset verifies that envOr returns the fallback
// value when the environment variable is not set.
func TestEnvOr_ReturnsFallbackWhenUnset(t *testing.T) {
	os.Unsetenv("TEST_ENVOR_KEY") //nolint:errcheck
	got := envOr("TEST_ENVOR_KEY", "default-val")
	if got != "default-val" {
		t.Fatalf("expected %q, got %q", "default-val", got)
	}
}

// TestEnvOr_ReturnsEnvValueWhenSet verifies that envOr returns the environment
// variable value when it is set.
func TestEnvOr_ReturnsEnvValueWhenSet(t *testing.T) {
	t.Setenv("TEST_ENVOR_KEY", "from-env")
	got := envOr("TEST_ENVOR_KEY", "default-val")
	if got != "from-env" {
		t.Fatalf("expected %q, got %q", "from-env", got)
	}
}

// TestEnvOr_EmptyEnvUseFallback verifies that envOr treats an empty string env
// var as unset and returns the fallback.
func TestEnvOr_EmptyEnvUseFallback(t *testing.T) {
	t.Setenv("TEST_ENVOR_EMPTY", "")
	got := envOr("TEST_ENVOR_EMPTY", "fallback")
	if got != "fallback" {
		t.Fatalf("expected %q, got %q", "fallback", got)
	}
}

// TestConfigFromEnv_Defaults verifies that configFromEnv returns expected
// default values when no environment variables are set.
func TestConfigFromEnv_Defaults(t *testing.T) {
	// Unset all config keys so we hit every fallback.
	envKeys := []string{
		"PORT", "DATABASE_URL", "REDIS_ADDR", "REDIS_PASSWORD",
		"FCM_SERVER_KEY", "SENDGRID_API_KEY", "FROM_EMAIL", "JWT_SECRET",
	}
	for _, k := range envKeys {
		os.Unsetenv(k) //nolint:errcheck
	}

	cfg := configFromEnv()

	if cfg.port != "9000" {
		t.Errorf("port = %q, want %q", cfg.port, "9000")
	}
	if cfg.redisAddr != "redis:6379" {
		t.Errorf("redisAddr = %q, want %q", cfg.redisAddr, "redis:6379")
	}
	if cfg.jwtSecret != "dev-secret-change-me-in-production" {
		t.Errorf("jwtSecret = %q, unexpected", cfg.jwtSecret)
	}
	// Optional fields should be empty when unset.
	if cfg.fcmServerKey != "" {
		t.Errorf("fcmServerKey expected empty, got %q", cfg.fcmServerKey)
	}
	if cfg.sendgridAPIKey != "" {
		t.Errorf("sendgridAPIKey expected empty, got %q", cfg.sendgridAPIKey)
	}
	if cfg.fromEmail != "" {
		t.Errorf("fromEmail expected empty, got %q", cfg.fromEmail)
	}
}

// TestConfigFromEnv_OverridesFromEnv verifies that configFromEnv picks up
// values set in the environment.
func TestConfigFromEnv_OverridesFromEnv(t *testing.T) {
	t.Setenv("PORT", "8080")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	t.Setenv("FCM_SERVER_KEY", "fcm-key-123")

	cfg := configFromEnv()

	if cfg.port != "8080" {
		t.Errorf("port = %q, want %q", cfg.port, "8080")
	}
	if cfg.redisAddr != "localhost:6379" {
		t.Errorf("redisAddr = %q, want %q", cfg.redisAddr, "localhost:6379")
	}
	if cfg.fcmServerKey != "fcm-key-123" {
		t.Errorf("fcmServerKey = %q, want %q", cfg.fcmServerKey, "fcm-key-123")
	}
}

// TestStubAuthMiddleware_MissingHeader_Returns401 verifies that a request
// without the X-User-ID header is rejected with 401.
func TestStubAuthMiddleware_MissingHeader_Returns401(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := stubAuthMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/notify/push", nil)
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

// TestStubAuthMiddleware_ValidHeader_InjectsUserID verifies that a request
// with X-User-ID is forwarded with the user ID in context.
func TestStubAuthMiddleware_ValidHeader_InjectsUserID(t *testing.T) {
	const wantID = "member-42"
	var gotID string

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = middleware.UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	mw := stubAuthMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/notify/push", nil)
	req.Header.Set("X-User-ID", wantID)
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if gotID != wantID {
		t.Fatalf("user ID in context = %q, want %q", gotID, wantID)
	}
}
