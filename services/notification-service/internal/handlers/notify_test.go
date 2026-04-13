package handlers_test

import (
	"bytes"
	"encoding/json"
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

// testUpgrader is a permissive upgrader for setting up in-test WebSocket servers.
var testUpgrader = &websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// jsonBody encodes v as JSON into a *bytes.Buffer.
func jsonBody(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

// connectTestMember starts an httptest WebSocket server that registers the
// server-side connection with h under memberID.  Returns the client connection
// and a teardown function that closes both sides.
func connectTestMember(t *testing.T, h *hub.Hub, memberID string) (*websocket.Conn, func()) {
	t.Helper()
	ready := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		h.Register(memberID, conn)
		close(ready)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				h.Deregister(memberID, conn)
				return
			}
		}
	}))

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	client, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		srv.Close()
		t.Fatalf("dial: %v", err)
	}
	<-ready
	return client, func() {
		_ = client.Close() //nolint:errcheck
		srv.Close()
	}
}

// TestBossUnlock_ValidRequest_Returns204 verifies that a complete, valid request
// returns 204 No Content.
func TestBossUnlock_ValidRequest_Returns204(t *testing.T) {
	h := hub.New()
	nh := handlers.NewNotifyHandler(h, zap.NewNop())

	body := jsonBody(map[string]string{
		"member_id":  "mem-1",
		"boss_id":    "boss-42",
		"chapter_id": "ch-7",
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/notify/boss-unlock", body)
	rr := httptest.NewRecorder()

	nh.BossUnlock(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestBossUnlock_InvalidJSON_Returns400 verifies that malformed JSON returns 400.
func TestBossUnlock_InvalidJSON_Returns400(t *testing.T) {
	h := hub.New()
	nh := handlers.NewNotifyHandler(h, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/internal/notify/boss-unlock",
		strings.NewReader("not-valid-json"))
	rr := httptest.NewRecorder()

	nh.BossUnlock(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// TestBossUnlock_MissingMemberID_Returns400 verifies that an empty member_id returns 400.
func TestBossUnlock_MissingMemberID_Returns400(t *testing.T) {
	h := hub.New()
	nh := handlers.NewNotifyHandler(h, zap.NewNop())

	body := jsonBody(map[string]string{"boss_id": "boss-1", "chapter_id": "ch-1"})
	req := httptest.NewRequest(http.MethodPost, "/internal/notify/boss-unlock", body)
	rr := httptest.NewRecorder()

	nh.BossUnlock(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// TestBossUnlock_MissingBossID_Returns400 verifies that an empty boss_id returns 400.
func TestBossUnlock_MissingBossID_Returns400(t *testing.T) {
	h := hub.New()
	nh := handlers.NewNotifyHandler(h, zap.NewNop())

	body := jsonBody(map[string]string{"member_id": "mem-1", "chapter_id": "ch-1"})
	req := httptest.NewRequest(http.MethodPost, "/internal/notify/boss-unlock", body)
	rr := httptest.NewRecorder()

	nh.BossUnlock(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// TestBossUnlock_MissingChapterID_Returns400 verifies that an empty chapter_id returns 400.
func TestBossUnlock_MissingChapterID_Returns400(t *testing.T) {
	h := hub.New()
	nh := handlers.NewNotifyHandler(h, zap.NewNop())

	body := jsonBody(map[string]string{"member_id": "mem-1", "boss_id": "boss-1"})
	req := httptest.NewRequest(http.MethodPost, "/internal/notify/boss-unlock", body)
	rr := httptest.NewRecorder()

	nh.BossUnlock(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// TestBossUnlock_BroadcastsPayloadToConnectedMember verifies the WebSocket
// delivery of the boss_unlocked event to an active session.
func TestBossUnlock_BroadcastsPayloadToConnectedMember(t *testing.T) {
	h := hub.New()
	client, teardown := connectTestMember(t, h, "mem-ws")
	defer teardown()

	nh := handlers.NewNotifyHandler(h, zap.NewNop())

	body := jsonBody(map[string]string{
		"member_id":  "mem-ws",
		"boss_id":    "boss-99",
		"chapter_id": "ch-1",
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/notify/boss-unlock", body)
	rr := httptest.NewRecorder()

	nh.BossUnlock(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}

	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck
	_, msg, err := client.ReadMessage()
	if err != nil {
		t.Fatalf("expected to receive broadcast: %v", err)
	}

	var payload hub.BossUnlockPayload
	if err := json.Unmarshal(msg, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.BossID != "boss-99" {
		t.Errorf("BossID = %q, want %q", payload.BossID, "boss-99")
	}
	if payload.Event != "boss_unlocked" {
		t.Errorf("Event = %q, want %q", payload.Event, "boss_unlocked")
	}
}

// TestBossUnlock_NoConnectedMember_StillReturns204 verifies that broadcasting
// to a member with no WebSocket sessions is a safe no-op returning 204.
func TestBossUnlock_NoConnectedMember_StillReturns204(t *testing.T) {
	h := hub.New()
	nh := handlers.NewNotifyHandler(h, zap.NewNop())

	body := jsonBody(map[string]string{
		"member_id":  "offline-member",
		"boss_id":    "boss-1",
		"chapter_id": "ch-1",
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/notify/boss-unlock", body)
	rr := httptest.NewRecorder()

	nh.BossUnlock(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
}
