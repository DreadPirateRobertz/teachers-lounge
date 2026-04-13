package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"

	"github.com/teacherslounge/gaming-service/internal/handler"
	"github.com/teacherslounge/gaming-service/internal/wsbattle"
)

// activeSession, battleStore, newBattleHandler, and attackRequest are
// defined in battle_test.go in this same _test package.

const wsTestSecret = "ws-test-secret-do-not-use-in-prod"

// makeWSToken issues a signed JWT that matches what user-service would emit so
// the WS handler's parseBattleWSToken can validate the caller.
func makeWSToken(t *testing.T, userID string, exp time.Duration) string {
	t.Helper()
	claims := jwt.MapClaims{
		"uid": userID,
		"aud": "teacherslounge-services",
		"exp": time.Now().Add(exp).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(wsTestSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// newWSServer spins up an httptest.Server that mounts the BattleWS handler at
// the same path the production router uses.
func newWSServer(t *testing.T, h *handler.Handler) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	r.Get("/gaming/battle/{battleId}/ws", h.BattleWS(wsTestSecret))
	return httptest.NewServer(r)
}

// dialWS opens a websocket connection to the test server with the given token
// and battle_id. It returns the connection so tests can read frames off it.
func dialWS(t *testing.T, srv *httptest.Server, battleID, token string) (*websocket.Conn, *http.Response) {
	t.Helper()
	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/gaming/battle/" + battleID + "/ws?token=" + token
	c, resp, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		return nil, resp
	}
	return c, resp
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestBattleWS_ConnectionEstablished_ReceivesJoinedFrame(t *testing.T) {
	s := &battleStore{session: activeSession()}
	h := newBattleHandler(s, nil)
	srv := newWSServer(t, h)
	defer srv.Close()

	token := makeWSToken(t, "u1", time.Hour)
	conn, _ := dialWS(t, srv, "sess-1", token)
	if conn == nil {
		t.Fatal("dial failed")
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	var frame map[string]string
	if err := conn.ReadJSON(&frame); err != nil {
		t.Fatalf("read joined frame: %v", err)
	}
	if frame["type"] != "joined" {
		t.Fatalf("expected type=joined, got %+v", frame)
	}
	if frame["battle_id"] != "sess-1" {
		t.Fatalf("expected battle_id=sess-1, got %q", frame["battle_id"])
	}

	// Subscriber should be registered in the hub.
	if got := h.Hub().SubscriberCount("sess-1"); got != 1 {
		t.Fatalf("expected 1 subscriber, got %d", got)
	}
}

func TestBattleWS_BroadcastEventDelivered(t *testing.T) {
	s := &battleStore{session: activeSession()}
	h := newBattleHandler(s, nil)
	srv := newWSServer(t, h)
	defer srv.Close()

	token := makeWSToken(t, "u1", time.Hour)
	conn, _ := dialWS(t, srv, "sess-1", token)
	if conn == nil {
		t.Fatal("dial failed")
	}
	defer conn.Close()

	// Consume the joined frame.
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	var joined map[string]string
	_ = conn.ReadJSON(&joined)

	// Wait for the WS runner to actually register with the hub before broadcasting.
	// The joined frame is written before Subscribe's count is observable only
	// very briefly; poll so the test is robust on slow CI.
	deadline := time.Now().Add(time.Second)
	for h.Hub().SubscriberCount("sess-1") == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	h.Hub().Broadcast("sess-1", wsbattle.EventDamage, handler.DamagePayload{
		PlayerDamageDealt: 10,
		BossHP:            190,
		PlayerHP:          100,
		Turn:              1,
		Phase:             "active",
		AnswerCorrect:     true,
	})

	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read event: %v", err)
	}
	var evt struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &evt); err != nil {
		t.Fatalf("unmarshal event: %v — body=%s", err, raw)
	}
	if evt.Type != wsbattle.EventDamage {
		t.Fatalf("type: want %q got %q", wsbattle.EventDamage, evt.Type)
	}
	var dp handler.DamagePayload
	if err := json.Unmarshal(evt.Payload, &dp); err != nil {
		t.Fatalf("unmarshal damage payload: %v", err)
	}
	if dp.PlayerDamageDealt != 10 || dp.BossHP != 190 {
		t.Fatalf("unexpected payload: %+v", dp)
	}
}

func TestBattleWS_DisconnectCleansUpSubscriber(t *testing.T) {
	s := &battleStore{session: activeSession()}
	h := newBattleHandler(s, nil)
	srv := newWSServer(t, h)
	defer srv.Close()

	token := makeWSToken(t, "u1", time.Hour)
	conn, _ := dialWS(t, srv, "sess-1", token)
	if conn == nil {
		t.Fatal("dial failed")
	}

	// Drain the joined frame, then wait for Subscribe to register.
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	var joined map[string]string
	_ = conn.ReadJSON(&joined)

	deadline := time.Now().Add(time.Second)
	for h.Hub().SubscriberCount("sess-1") == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if h.Hub().SubscriberCount("sess-1") != 1 {
		t.Fatalf("pre-close: expected 1 subscriber")
	}

	_ = conn.Close()

	// The server's read loop should observe the close and Unsubscribe.
	deadline = time.Now().Add(2 * time.Second)
	for h.Hub().SubscriberCount("sess-1") > 0 {
		if time.Now().After(deadline) {
			t.Fatalf("subscriber not removed after client disconnect: count=%d", h.Hub().SubscriberCount("sess-1"))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestBattleWS_MissingTokenReturns401(t *testing.T) {
	s := &battleStore{session: activeSession()}
	h := newBattleHandler(s, nil)
	srv := newWSServer(t, h)
	defer srv.Close()

	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/gaming/battle/sess-1/ws"
	_, resp, err := websocket.DefaultDialer.Dial(u, nil)
	if err == nil {
		t.Fatal("expected dial to fail without token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		got := "<nil>"
		if resp != nil {
			got = resp.Status
		}
		t.Fatalf("want 401, got %s", got)
	}
}

func TestBattleWS_InvalidTokenReturns401(t *testing.T) {
	s := &battleStore{session: activeSession()}
	h := newBattleHandler(s, nil)
	srv := newWSServer(t, h)
	defer srv.Close()

	_, resp := dialWS(t, srv, "sess-1", "not-a-real-jwt")
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		got := "<nil>"
		if resp != nil {
			got = resp.Status
		}
		t.Fatalf("want 401, got %s", got)
	}
}

func TestBattleWS_ForeignUserReturns403(t *testing.T) {
	s := &battleStore{session: activeSession()} // owner is "u1"
	h := newBattleHandler(s, nil)
	srv := newWSServer(t, h)
	defer srv.Close()

	token := makeWSToken(t, "other-user", time.Hour)
	_, resp := dialWS(t, srv, "sess-1", token)
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		got := "<nil>"
		if resp != nil {
			got = resp.Status
		}
		t.Fatalf("want 403, got %s", got)
	}
}

func TestBattleWS_UnknownBattleReturns404(t *testing.T) {
	s := &battleStore{session: nil} // GetBattleSession returns nil
	h := newBattleHandler(s, nil)
	srv := newWSServer(t, h)
	defer srv.Close()

	token := makeWSToken(t, "u1", time.Hour)
	_, resp := dialWS(t, srv, "ghost-battle", token)
	if resp == nil || resp.StatusCode != http.StatusNotFound {
		got := "<nil>"
		if resp != nil {
			got = resp.Status
		}
		t.Fatalf("want 404, got %s", got)
	}
}

func TestAttack_BroadcastsDamageAndPhase_OnVictory(t *testing.T) {
	sess := activeSession()
	sess.BossHP = 5 // one-shot
	s := &battleStore{session: sess}
	h := newBattleHandler(s, nil)

	sub := h.Hub().Subscribe("sess-1")

	rr := httptest.NewRecorder()
	h.Attack(rr, attackRequest("sess-1", true, 100))

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}

	seen := map[string]bool{}
	deadline := time.After(time.Second)
	for len(seen) < 3 {
		select {
		case evt, ok := <-sub.C:
			if !ok {
				t.Fatalf("hub channel closed; events seen=%v", seen)
			}
			seen[evt.Type] = true
		case <-deadline:
			t.Fatalf("timed out; events seen=%v (want damage+phase_transition+loot_roll)", seen)
		}
	}
	for _, want := range []string{wsbattle.EventDamage, wsbattle.EventPhaseTransition, wsbattle.EventLootRoll} {
		if !seen[want] {
			t.Fatalf("missing event %q (seen=%v)", want, seen)
		}
	}
}
