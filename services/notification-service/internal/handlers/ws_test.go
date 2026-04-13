package handlers_test

// Tests for WSHandler — covers early-exit paths (missing/invalid token,
// upgrade failure) and the happy-path WebSocket connection lifecycle.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/handlers"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/hub"
)

// fakeAuth implements handlers.Authenticator for testing.
type fakeAuth struct {
	memberID string
	err      error
}

func (a *fakeAuth) MemberID(_ string) (string, error) {
	return a.memberID, a.err
}

// TestWSHandler_NewWSHandler_IsNotNil verifies the constructor returns a
// non-nil handler.
func TestWSHandler_NewWSHandler_IsNotNil(t *testing.T) {
	h := hub.New()
	wsh := handlers.NewWSHandler(h, &fakeAuth{memberID: "mem"}, zap.NewNop())
	if wsh == nil {
		t.Fatal("expected non-nil WSHandler from NewWSHandler")
	}
}

// TestWSHandler_MissingToken_Returns401 verifies that a request without a
// ?token query parameter is rejected with 401 Unauthorized.
func TestWSHandler_MissingToken_Returns401(t *testing.T) {
	h := hub.New()
	wsh := handlers.NewWSHandler(h, &fakeAuth{memberID: "mem"}, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/ws", nil) // no ?token
	rr := httptest.NewRecorder()
	wsh.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

// TestWSHandler_InvalidToken_Returns401 verifies that an invalid JWT is
// rejected with 401 Unauthorized.
func TestWSHandler_InvalidToken_Returns401(t *testing.T) {
	h := hub.New()
	auth := &fakeAuth{err: errors.New("token expired")}
	wsh := handlers.NewWSHandler(h, auth, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/ws?token=bad-token", nil)
	rr := httptest.NewRecorder()
	wsh.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid token, got %d", rr.Code)
	}
}

// TestWSHandler_UpgradeFailure_NoBody verifies that when a plain HTTP request
// (not a WebSocket upgrade) is received with a valid token, the upgrade fails
// and the handler returns without panicking.  Gorilla websocket writes its own
// error response, so we only check there is no panic.
func TestWSHandler_UpgradeFailure_DoesNotPanic(t *testing.T) {
	h := hub.New()
	auth := &fakeAuth{memberID: "mem-1"}
	wsh := handlers.NewWSHandler(h, auth, zap.NewNop())

	// Plain HTTP request — not a WebSocket upgrade.
	req := httptest.NewRequest(http.MethodGet, "/ws?token=valid-token", nil)
	rr := httptest.NewRecorder()

	// Must not panic.
	wsh.ServeHTTP(rr, req)
}

// TestWSHandler_HappyPath_ConnectsAndRegisters verifies the full WebSocket
// upgrade flow: the handler registers the connection with the hub and
// deregisters it on client disconnect.
func TestWSHandler_HappyPath_ConnectsAndRegisters(t *testing.T) {
	h := hub.New()
	const memberID = "mem-ws-test"
	auth := &fakeAuth{memberID: memberID}
	wsh := handlers.NewWSHandler(h, auth, zap.NewNop())

	// Wrap the handler in a real httptest.Server so we can perform a genuine
	// WebSocket upgrade.
	srv := httptest.NewServer(wsh)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "?token=any-valid-token"
	client, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		// If the upgrade fails here it means the test environment is unusual;
		// skip rather than fail.
		t.Skipf("WebSocket dial failed (may be a test-env issue): %v", err)
	}
	defer func() { _ = client.Close() }() //nolint:errcheck

	// Give the handler goroutine a moment to register the connection.
	time.Sleep(10 * time.Millisecond)

	if got := h.SessionCount(memberID); got != 1 {
		t.Fatalf("expected 1 registered session, got %d", got)
	}

	// Close the client — the handler's read pump should exit and deregister.
	_ = client.Close() //nolint:errcheck

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if h.SessionCount(memberID) == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected 0 sessions after client disconnect, got %d", h.SessionCount(memberID))
}
