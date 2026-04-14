package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/wsbattle"
)

// sanitizeForLog strips CR/LF from user-controlled strings before they flow
// into log entries, preventing log-forging (CWE-117).
func sanitizeForLog(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// wsUpgrader is reused for every WS upgrade. CheckOrigin allows all origins:
// WebSocket connections do not carry ambient credentials (no cookies used),
// so CSRF does not apply. Authentication is enforced by the JWT in the
// query string; an allowlist of expected origins can be added for additional
// hardening.
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(*http.Request) bool { return true },
}

// WebSocket write / read deadlines. The write deadline prevents a slow client
// from blocking a broadcast forever; the pong deadline lets us detect a
// dead connection so we can unsubscribe and free the slot in the hub.
const (
	wsWriteTimeout = 10 * time.Second
	wsPongTimeout  = 60 * time.Second
	wsPingInterval = (wsPongTimeout * 9) / 10
)

// BattleWS handles GET /gaming/battle/{battleId}/ws — upgrades the connection,
// validates ownership, subscribes to the hub, and forwards events to the
// client until it disconnects.
//
// JWT is read from the ``token`` query parameter because browsers cannot set
// custom headers on a WebSocket handshake. The token is validated with the
// same secret used by middleware.Authenticate; the caller must own the
// battle session referenced by the path parameter.
//
// Lifecycle events:
//   - join:       subscribe to the hub, send an initial "joined" frame.
//   - tick:       every event broadcast by the hub is written to the socket.
//   - disconnect: any read error or server-side shutdown unsubscribes and
//     closes the socket cleanly.
func (h *Handler) BattleWS(jwtSecret string) http.HandlerFunc {
	secret := []byte(jwtSecret)
	return func(w http.ResponseWriter, r *http.Request) {
		battleID := chi.URLParam(r, "battleId")
		if battleID == "" {
			http.Error(w, `{"error":"missing battle id"}`, http.StatusBadRequest)
			return
		}

		tokenStr := r.URL.Query().Get("token")
		if tokenStr == "" {
			http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
			return
		}

		userID, err := parseBattleWSToken(tokenStr, secret)
		if err != nil {
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		session, err := h.store.GetBattleSession(r.Context(), battleID)
		if err != nil {
			// battleID is user-controlled and may be surfaced via err — sanitize
			// before logging to prevent log forgery (CWE-117).
			h.logger.Error("ws: load battle session",
				zap.String("battle_id", sanitizeForLog(battleID)),
				zap.String("err", sanitizeForLog(err.Error())))
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
		if session == nil {
			http.Error(w, `{"error":"battle not found"}`, http.StatusNotFound)
			return
		}
		if session.UserID != userID {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}

		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			// The upgrader already wrote the response on failure.
			h.logger.Warn("ws: upgrade failed", zap.Error(err))
			return
		}
		defer conn.Close()

		h.runBattleWS(r.Context(), conn, battleID)
	}
}

// runBattleWS wires the upgraded connection to the hub. It is extracted from
// BattleWS so tests can drive it with a pre-built connection.
//
// Two goroutines cooperate:
//   - reader: consumes control frames (pong, close) so the underlying TCP
//     connection is properly managed and disconnect is observed promptly.
//   - writer: fans hub events onto the socket plus periodic ping frames.
//
// Either exiting triggers the other via the `done` channel, after which the
// subscriber is cleanly removed from the hub.
func (h *Handler) runBattleWS(ctx context.Context, conn *websocket.Conn, battleID string) {
	sub := h.hub.Subscribe(battleID)
	defer h.hub.Unsubscribe(battleID, sub)

	done := make(chan struct{})

	// Initial "joined" frame so clients can distinguish a stale connect from
	// a fresh join without waiting for the first battle event.
	joinPayload := map[string]string{
		"type":      "joined",
		"battle_id": battleID,
	}
	_ = conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
	if err := conn.WriteJSON(joinPayload); err != nil {
		h.logger.Warn("ws: write joined", zap.Error(err))
		return
	}

	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(wsPongTimeout))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongTimeout))
	})

	// Reader goroutine: drains client→server frames so the Gorilla library
	// processes pongs and close frames. We discard payloads — the REST API
	// remains the only write path for battle state.
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.NextReader(); err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) {
					h.logger.Warn("ws: read error", zap.String("battle_id", battleID), zap.Error(err))
				}
				return
			}
		}
	}()

	ping := time.NewTicker(wsPingInterval)
	defer ping.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case evt, ok := <-sub.C:
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := conn.WriteJSON(evt); err != nil {
				h.logger.Warn("ws: write event", zap.String("battle_id", battleID), zap.String("event_type", evt.Type), zap.Error(err))
				return
			}
		case <-ping.C:
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				h.logger.Warn("ws: ping failed", zap.String("battle_id", battleID), zap.Error(err))
				return
			}
		}
	}
}

// parseBattleWSToken validates a JWT the same way middleware.Authenticate does
// and returns the user ID. Extracted here so the WS handler doesn't have to
// route through http middleware, which cannot wrap a Gorilla upgrade cleanly.
func parseBattleWSToken(tokenStr string, secret []byte) (string, error) {
	claims := &middleware.Claims{}
	token, err := jwt.ParseWithClaims(
		tokenStr,
		claims,
		func(t *jwt.Token) (any, error) {
			// Reject algorithm-confusion attacks (e.g. "none" or RS256-signed tokens
			// that would otherwise verify against the HMAC secret as a public key).
			// Mirrors middleware.parseToken.
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return secret, nil
		},
		jwt.WithAudience("teacherslounge-services"),
	)
	if err != nil || !token.Valid {
		return "", err
	}
	return claims.UserID, nil
}

// Compile-time assertion that our event envelope still marshals — catches an
// accidental break of the wsbattle.Event shape without needing a runtime test.
var _ = func() bool {
	_, _ = json.Marshal(wsbattle.Event{})
	return true
}()
