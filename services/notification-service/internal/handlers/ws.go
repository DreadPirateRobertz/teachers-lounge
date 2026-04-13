// Package handlers provides HTTP and WebSocket handlers for the notification
// service's WebSocket push endpoints.
package handlers

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/hub"
)

// pongWait is the time allowed to read the next pong message from the client.
const pongWait = 60 * time.Second

// Authenticator extracts a verified member ID from a raw JWT token string.
// Returns an error when the token is missing, malformed, or expired.
type Authenticator interface {
	MemberID(token string) (string, error)
}

// WSHandler upgrades HTTP connections to WebSocket, authenticates the caller
// via JWT, and registers the session with the hub.
type WSHandler struct {
	hub      *hub.Hub
	auth     Authenticator
	upgrader websocket.Upgrader
	logger   *zap.Logger
}

// NewWSHandler returns a configured WSHandler.
//
// hub holds the broadcast registry; auth validates JWT tokens issued by the
// user-service; logger is used for connection lifecycle events.
func NewWSHandler(h *hub.Hub, auth Authenticator, logger *zap.Logger) *WSHandler {
	return &WSHandler{
		hub:  h,
		auth: auth,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			// CheckOrigin is permissive for now; restrict to trusted origins in production.
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		logger: logger,
	}
}

// ServeHTTP handles GET /ws?token=<jwt>.
//
// Flow:
//  1. Validate the ?token query parameter via Authenticator.
//  2. Upgrade the HTTP connection to WebSocket.
//  3. Register the connection with the hub under the authenticated member ID.
//  4. Hold the read pump open until the client disconnects or times out.
//  5. Deregister on exit so the hub does not broadcast to dead connections.
//
// Reconnect logic is the client's responsibility — the server does not
// auto-reconnect, but periodic pings keep the TCP connection alive.
func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "token query parameter required", http.StatusUnauthorized)
		return
	}

	memberID, err := h.auth.MemberID(token)
	if err != nil {
		h.logger.Warn("ws: invalid token", zap.Error(err))
		http.Error(w, "invalid or expired token", http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade writes its own error response; just log.
		h.logger.Error("ws: upgrade failed", zap.Error(err))
		return
	}

	h.hub.Register(memberID, conn)
	defer func() {
		h.hub.Deregister(memberID, conn)
		conn.Close()
		h.logger.Info("ws: client disconnected", zap.String("member_id", memberID))
	}()

	h.logger.Info("ws: client connected", zap.String("member_id", memberID))

	// Configure read deadline so dead connections are detected after pongWait.
	_ = conn.SetReadDeadline(time.Now().Add(pongWait)) //nolint:errcheck // best-effort deadline
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait)) //nolint:errcheck // best-effort deadline
		return nil
	})

	// Read pump — drain incoming frames (clients send pong frames automatically;
	// any other data is ignored). The loop exits when the connection closes or
	// the read deadline expires.
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseNormalClosure,
			) {
				h.logger.Warn("ws: unexpected close",
					zap.String("member_id", memberID),
					zap.Error(err),
				)
			}
			return
		}
	}
}
