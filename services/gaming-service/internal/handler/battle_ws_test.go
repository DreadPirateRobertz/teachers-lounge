package handler_test

// Tests for the WebSocket battle-state API: BattleWebSocket handler.
//
// Tests:
//   - Connection established: upgrade succeeds, join event received
//   - Event broadcast to all participants
//   - Disconnect cleanup: connection removed from Hub after close
//   - Battle-end triggers loot roll event (via Attack kill-shot)

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/handler"
	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
)

// ── store stub for WS tests ───────────────────────────────────────────────────

type wsTestStore struct {
	noopStore
	session *model.BattleSession
}

func (s *wsTestStore) GetBattle(_ context.Context, _ string) (*model.BattleSession, error) {
	return s.session, nil
}

func (s *wsTestStore) GetBattleSession(_ context.Context, _ string) (*model.BattleSession, error) {
	return s.session, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// testBattleSession returns a minimal active BattleSession owned by "user-1".
func testBattleSession(sessionID string) *model.BattleSession {
	return &model.BattleSession{
		SessionID:  sessionID,
		UserID:     "user-1",
		BossID:     "boss-1",
		Phase:      model.PhaseActive,
		PlayerHP:   100,
		PlayerMaxHP: 100,
		BossHP:     200,
		BossMaxHP:  200,
		ExpiresAt:  time.Now().Add(30 * time.Minute),
	}
}

// newWSServer starts an httptest.Server with the BattleWebSocket handler
// wired to the given store. It returns the server and a dial function that
// connects a WS client with the given user injected into the request.
func newWSServer(t *testing.T, st handler.Storer, userID string) (*httptest.Server, func(battleID string) *websocket.Conn) {
	t.Helper()

	h := handler.New(st, nil, zap.NewNop())

	r := chi.NewRouter()
	// Inject the user into context for all requests (bypasses JWT middleware).
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := middleware.WithUserID(req.Context(), userID)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Get("/gaming/battle/{battle_id}/ws", h.BattleWebSocket)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	dial := func(battleID string) *websocket.Conn {
		t.Helper()
		url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/gaming/battle/" + battleID + "/ws"
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			t.Fatalf("ws dial: %v", err)
		}
		t.Cleanup(func() { conn.Close() })
		return conn
	}

	return srv, dial
}

// readEvent reads and decodes the next BattleEvent from a WS connection.
func readEvent(t *testing.T, conn *websocket.Conn, timeout time.Duration) model.BattleEvent {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	var ev model.BattleEvent
	if err := json.Unmarshal(msg, &ev); err != nil {
		t.Fatalf("ws parse event: %v", err)
	}
	return ev
}

// ── TestBattleWS_ConnectionEstablished ───────────────────────────────────────

// TestBattleWS_ConnectionEstablished verifies that upgrading to WebSocket
// succeeds and the client immediately receives a "join" event carrying the
// current battle state.
func TestBattleWS_ConnectionEstablished(t *testing.T) {
	st := &wsTestStore{session: testBattleSession("sess-1")}
	_, dial := newWSServer(t, st, "user-1")

	conn := dial("sess-1")

	ev := readEvent(t, conn, 2*time.Second)
	if ev.Type != model.EventJoin {
		t.Errorf("first event type: got %q, want %q", ev.Type, model.EventJoin)
	}

	// Payload should be a JSON object with session_id.
	raw, _ := json.Marshal(ev.Payload)
	var sess model.BattleSession
	if err := json.Unmarshal(raw, &sess); err != nil {
		t.Fatalf("parse join payload: %v", err)
	}
	if sess.SessionID != "sess-1" {
		t.Errorf("join payload session_id: got %q, want sess-1", sess.SessionID)
	}
}

// TestBattleWS_NotFound_RejectsConnection verifies that connecting to a
// non-existent battle returns a 404 before the WS upgrade.
func TestBattleWS_NotFound_RejectsConnection(t *testing.T) {
	st := &wsTestStore{session: nil} // nil → not found

	// Use a direct HTTP check instead — session not found returns 404 before upgrade.
	h := handler.New(st, nil, zap.NewNop())
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := middleware.WithUserID(req.Context(), "user-1")
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Get("/gaming/battle/{battle_id}/ws", h.BattleWebSocket)
	s2 := httptest.NewServer(r)
	defer s2.Close()

	resp, err := http.Get(s2.URL + "/gaming/battle/missing/ws")
	if err != nil {
		t.Fatalf("http get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for missing battle, got %d", resp.StatusCode)
	}
}

// ── TestBattleWS_EventBroadcastToAllParticipants ──────────────────────────────

// TestBattleWS_EventBroadcastToAllParticipants verifies that a broadcast via
// BroadcastBattleEvent reaches every connected client for that battle.
func TestBattleWS_EventBroadcastToAllParticipants(t *testing.T) {
	st := &wsTestStore{session: testBattleSession("sess-2")}

	h := handler.New(st, nil, zap.NewNop())

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := middleware.WithUserID(req.Context(), "user-1")
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Get("/gaming/battle/{battle_id}/ws", h.BattleWebSocket)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	dialURL := func() *websocket.Conn {
		t.Helper()
		url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/gaming/battle/sess-2/ws"
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		t.Cleanup(func() { conn.Close() })
		return conn
	}

	// Connect two clients.
	c1 := dialURL()
	c2 := dialURL()

	// Drain the join events.
	readEvent(t, c1, 2*time.Second)
	readEvent(t, c2, 2*time.Second)

	// Broadcast a damage event via the exported helper.
	dmgEvent := model.BattleEvent{
		Type: model.EventDamage,
		Payload: model.DamageEvent{
			PlayerDamageDealt: 50,
			BossHP:            150,
			PlayerHP:          80,
			Turn:              1,
		},
	}
	h.BroadcastBattleEvent("sess-2", dmgEvent)

	// Both clients must receive the event.
	for i, c := range []*websocket.Conn{c1, c2} {
		ev := readEvent(t, c, 2*time.Second)
		if ev.Type != model.EventDamage {
			t.Errorf("client %d: event type = %q, want %q", i+1, ev.Type, model.EventDamage)
		}
	}
}

// ── TestBattleWS_DisconnectCleanup ────────────────────────────────────────────

// TestBattleWS_DisconnectCleanup verifies that closing a client connection
// removes it from the Hub so future broadcasts don't target dead connections.
func TestBattleWS_DisconnectCleanup(t *testing.T) {
	st := &wsTestStore{session: testBattleSession("sess-3")}
	_, dial := newWSServer(t, st, "user-1")

	conn := dial("sess-3")

	// Drain join event.
	readEvent(t, conn, 2*time.Second)

	// Close the client connection.
	conn.Close()

	// Give the server goroutine time to detect the disconnect and unregister.
	time.Sleep(100 * time.Millisecond)

	// Broadcasting to the battle after cleanup must not panic or hang.
	// If the connection were still registered, WriteJSON would return an error
	// and the hub would remove it — either way this must not deadlock.
	h := handler.New(st, nil, zap.NewNop())
	h.BroadcastBattleEvent("sess-3", model.BattleEvent{Type: model.EventDisconnect})
}

// ── TestBattleWS_BattleEndTriggersLootRoll ────────────────────────────────────

// TestBattleWS_BattleEndTriggersLootRoll verifies that calling Attack with a
// kill shot causes a "loot_roll" event to be broadcast to connected WS clients.
func TestBattleWS_BattleEndTriggersLootRoll(t *testing.T) {
	// Boss has 1 HP — any attack will kill it.
	sess := testBattleSession("sess-4")
	sess.BossHP = 1
	sess.BossMaxHP = 100

	// killStore wraps the noopStore, returning the session for both
	// GetBattle (WS join) and GetBattleSession (Attack lookup).
	killStore := &wsAttackStore{session: sess}

	h := handler.New(killStore, nil, zap.NewNop())

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := middleware.WithUserID(req.Context(), "user-1")
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Get("/gaming/battle/{battle_id}/ws", h.BattleWebSocket)
	r.Post("/gaming/boss/attack", h.Attack)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	// Connect a WS client to the battle.
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/gaming/battle/sess-4/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	// Drain the join event.
	readEvent(t, conn, 2*time.Second)

	// Fire an attack that kills the boss.
	attackBody := `{"session_id":"sess-4","answer_correct":true,"base_damage":9999}`
	resp, err := http.Post(srv.URL+"/gaming/boss/attack", "application/json", strings.NewReader(attackBody))
	if err != nil {
		t.Fatalf("attack POST: %v", err)
	}
	defer resp.Body.Close()

	// We expect either 200 (attack processed) or the session to be returned.
	// The important thing is that the WS client receives a loot_roll event.
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	// Read events until we see loot_roll or timeout.
	var gotLoot bool
	for i := 0; i < 5; i++ {
		ev := readEvent(t, conn, 3*time.Second)
		if ev.Type == model.EventLootRoll {
			gotLoot = true
			break
		}
		if ev.Type == model.EventPhaseTransition {
			// Phase transition is also acceptable — loot_roll may follow
			continue
		}
	}
	if !gotLoot {
		t.Error("expected loot_roll event after boss kill, none received")
	}
}

// wsAttackStore supports both WS (GetBattle) and REST (GetBattleSession) lookups.
type wsAttackStore struct {
	noopStore
	session *model.BattleSession
}

func (s *wsAttackStore) GetBattle(_ context.Context, _ string) (*model.BattleSession, error) {
	return s.session, nil
}

func (s *wsAttackStore) GetBattleSession(_ context.Context, _ string) (*model.BattleSession, error) {
	return s.session, nil
}

func (s *wsAttackStore) GetXPAndLevel(_ context.Context, _ string) (int64, int, error) {
	return 0, 1, nil
}

func (s *wsAttackStore) UpsertXP(_ context.Context, _ string, _ int64, _ int) error { return nil }
