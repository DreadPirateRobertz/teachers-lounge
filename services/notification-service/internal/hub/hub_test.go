package hub_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/hub"
)

var testUpgrader = &websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// connectMember starts a test WebSocket server that registers the server-side
// connection with h under memberID. Returns the client-side connection and a
// teardown function. The server keeps the connection alive until the client
// closes it.
func connectMember(t *testing.T, h *hub.Hub, memberID string) (clientConn *websocket.Conn, teardown func()) {
	t.Helper()

	ready := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		h.Register(memberID, conn)
		close(ready)
		// Drain until closed — server must not block writes to the client.
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

	// Wait until the server has registered the connection before returning.
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("server did not register connection in time")
	}

	teardown = func() {
		_ = client.Close() //nolint:errcheck // best-effort cleanup in test teardown
		srv.Close()
	}
	return client, teardown
}

// readJSON reads one JSON message from conn and unmarshals it into dst.
func readJSON(t *testing.T, conn *websocket.Conn, dst any) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck // best-effort test deadline
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	if err := json.Unmarshal(msg, dst); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

// TestRegisterAndSessionCount verifies that Register increments the session count.
func TestRegisterAndSessionCount(t *testing.T) {
	h := hub.New()

	_, teardown := connectMember(t, h, "member-1")
	defer teardown()

	if got := h.SessionCount("member-1"); got != 1 {
		t.Fatalf("SessionCount = %d, want 1", got)
	}
}

// TestDeregister verifies that Deregister removes the connection so the session
// count returns to zero when no sessions remain.
func TestDeregister(t *testing.T) {
	h := hub.New()

	client, teardown := connectMember(t, h, "member-2")
	defer teardown()

	// Close the client; the server goroutine will deregister on read error.
	_ = client.Close() //nolint:errcheck // best-effort cleanup in test

	// Allow the server goroutine time to react.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if h.SessionCount("member-2") == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("SessionCount = %d after close, want 0", h.SessionCount("member-2"))
}

// TestDeregisterUnknown verifies that Deregister is a safe no-op for unknown members.
func TestDeregisterUnknown(t *testing.T) {
	h := hub.New()
	h.Deregister("ghost", &websocket.Conn{}) // must not panic
	if got := h.SessionCount("ghost"); got != 0 {
		t.Fatalf("SessionCount = %d, want 0", got)
	}
}

// TestBroadcast verifies that Broadcast delivers the payload as JSON to the
// registered client connection.
func TestBroadcast(t *testing.T) {
	h := hub.New()

	client, teardown := connectMember(t, h, "member-3")
	defer teardown()

	want := hub.BossUnlockPayload{
		Event:     "boss_unlocked",
		BossID:    "boss-42",
		ChapterID: "chapter-7",
		MemberID:  "member-3",
	}
	h.Broadcast("member-3", want)

	var got hub.BossUnlockPayload
	readJSON(t, client, &got)

	if got.Event != want.Event {
		t.Errorf("Event = %q, want %q", got.Event, want.Event)
	}
	if got.BossID != want.BossID {
		t.Errorf("BossID = %q, want %q", got.BossID, want.BossID)
	}
	if got.ChapterID != want.ChapterID {
		t.Errorf("ChapterID = %q, want %q", got.ChapterID, want.ChapterID)
	}
	if got.MemberID != want.MemberID {
		t.Errorf("MemberID = %q, want %q", got.MemberID, want.MemberID)
	}
}

// TestMultiSession verifies that Broadcast delivers to all sessions registered
// for the same member.
func TestMultiSession(t *testing.T) {
	h := hub.New()

	const memberID = "member-multi"
	const numSessions = 3

	clients := make([]*websocket.Conn, numSessions)
	for i := range numSessions {
		client, teardown := connectMember(t, h, memberID)
		defer teardown()
		clients[i] = client
	}

	if got := h.SessionCount(memberID); got != numSessions {
		t.Fatalf("SessionCount = %d, want %d", got, numSessions)
	}

	payload := hub.BossUnlockPayload{
		Event:     "boss_unlocked",
		BossID:    "b1",
		ChapterID: "c1",
		MemberID:  memberID,
	}
	h.Broadcast(memberID, payload)

	// All clients should receive the message concurrently.
	var wg sync.WaitGroup
	for i, client := range clients {
		wg.Add(1)
		go func(idx int, c *websocket.Conn) {
			defer wg.Done()
			var got hub.BossUnlockPayload
			_ = c.SetReadDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck // best-effort test deadline
			_, msg, err := c.ReadMessage()
			if err != nil {
				t.Errorf("client %d read: %v", idx, err)
				return
			}
			if err := json.Unmarshal(msg, &got); err != nil {
				t.Errorf("client %d unmarshal: %v", idx, err)
				return
			}
			if got.Event != "boss_unlocked" {
				t.Errorf("client %d: Event = %q, want %q", idx, got.Event, "boss_unlocked")
			}
		}(i, client)
	}
	wg.Wait()
}

// TestBroadcastToUnknownMember verifies that broadcasting to a member with no
// sessions is a safe no-op.
func TestBroadcastToUnknownMember(t *testing.T) {
	h := hub.New()
	payload := hub.BossUnlockPayload{Event: "boss_unlocked"}
	h.Broadcast("nobody", payload) // must not panic
}

// TestBroadcast_DeregistersDeadConnection verifies that when a WriteJSON fails
// (dead connection), the hub automatically removes that connection so it does
// not block future broadcasts.
func TestBroadcast_DeregistersDeadConnection(t *testing.T) {
	h := hub.New()

	// serverConn receives the server-side WebSocket connection after upgrade.
	serverConn := make(chan *websocket.Conn, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		h.Register("member-dead", conn)
		serverConn <- conn
		// Block — do NOT read so that client-close triggers no deregister here.
		select {}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	client, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = client.Close() }() //nolint:errcheck

	sc := <-serverConn

	if h.SessionCount("member-dead") != 1 {
		t.Fatal("expected 1 registered session before broadcast")
	}

	// Force the server-side write to fail by setting a write deadline in the past.
	if err := sc.SetWriteDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatalf("SetWriteDeadline: %v", err)
	}

	// Broadcast — WriteJSON fails → hub should remove the dead connection.
	h.Broadcast("member-dead", hub.BossUnlockPayload{Event: "boss_unlocked"})

	if got := h.SessionCount("member-dead"); got != 0 {
		t.Fatalf("expected 0 sessions after broadcast to dead conn, got %d", got)
	}
}
