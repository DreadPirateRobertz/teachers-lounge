package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
)

var upgrader = websocket.Upgrader{
	// ReadBufferSize and WriteBufferSize tune the per-connection I/O buffers.
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// CheckOrigin allows all origins in development; production callers should
	// set a more restrictive policy via CORS at the ingress level.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// pongWait is how long the server waits for a pong response to a ping.
const pongWait = 60 * time.Second

// pingPeriod is how often the server sends pings to keep the connection alive.
// Must be less than pongWait.
const pingPeriod = 45 * time.Second

// BattleWebSocket handles GET /gaming/battle/{battle_id}/ws
//
// It upgrades the request to a WebSocket connection and registers the client
// with the battle Hub for the given battle. While connected the client
// receives real-time battle events (damage, phase transitions, loot rolls)
// broadcast by the Attack and ForfeitBattle handlers.
//
// The handler sends an initial "join" event with the current battle state and
// then blocks in a read-pump loop, responding to pings and detecting
// disconnection. On exit the connection is unregistered from the Hub.
func (h *Handler) BattleWebSocket(w http.ResponseWriter, r *http.Request) {
	battleID := chi.URLParam(r, "battle_id")
	callerID := middleware.UserIDFromContext(r.Context())

	// Verify the battle exists and belongs to the caller.
	session, err := h.store.GetBattle(r.Context(), battleID)
	if err != nil {
		h.logger.Error("ws: get battle", zap.String("battle_id", battleID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if session == nil {
		writeError(w, http.StatusNotFound, "battle not found or expired")
		return
	}
	if session.UserID != callerID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	// Upgrade to WebSocket.
	raw, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("ws: upgrade", zap.String("battle_id", battleID), zap.Error(err))
		return
	}

	c := &wsConn{conn: raw}

	// Register in the hub before sending the join event so no events are missed.
	h.hub.register(battleID, c)
	defer h.hub.unregister(battleID, c)
	defer c.close()

	// Send initial session state.
	if err := c.writeEvent(model.BattleEvent{
		Type:    model.EventJoin,
		Payload: session,
	}); err != nil {
		h.logger.Warn("ws: write join event", zap.String("battle_id", battleID), zap.Error(err))
		return
	}

	// Read pump: keep the connection alive with ping/pong and detect disconnect.
	raw.SetReadDeadline(time.Now().Add(pongWait))
	raw.SetPongHandler(func(string) error {
		return raw.SetReadDeadline(time.Now().Add(pongWait))
	})

	pingTicker := time.NewTicker(pingPeriod)
	defer pingTicker.Stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := raw.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-pingTicker.C:
			c.mu.Lock()
			err := raw.WriteMessage(websocket.PingMessage, nil)
			c.mu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

// BroadcastBattleEvent sends event to every WebSocket client connected to the
// given battle. Called by Attack and finishBattle after mutating session state.
func (h *Handler) BroadcastBattleEvent(battleID string, event model.BattleEvent) {
	h.hub.broadcast(battleID, event)
}
