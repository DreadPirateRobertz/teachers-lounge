package handler

import (
	"sync"

	"github.com/gorilla/websocket"
	"github.com/teacherslounge/gaming-service/internal/model"
)

// wsConn wraps a gorilla WebSocket connection with a write mutex so that
// concurrent goroutines can safely call WriteJSON without racing.
type wsConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// writeEvent marshals event as JSON and sends it to the client.
// It is safe to call from multiple goroutines.
func (c *wsConn) writeEvent(event model.BattleEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(event)
}

// close shuts down the underlying connection.
func (c *wsConn) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.Close()
}

// Hub tracks active WebSocket connections grouped by battle ID.
// All methods are safe for concurrent use.
type Hub struct {
	mu      sync.RWMutex
	battles map[string]map[*wsConn]struct{}
}

// newHub creates an empty Hub.
func newHub() *Hub {
	return &Hub{battles: make(map[string]map[*wsConn]struct{})}
}

// register adds conn to the set of listeners for battleID.
func (h *Hub) register(battleID string, c *wsConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.battles[battleID] == nil {
		h.battles[battleID] = make(map[*wsConn]struct{})
	}
	h.battles[battleID][c] = struct{}{}
}

// unregister removes conn from the set of listeners for battleID.
func (h *Hub) unregister(battleID string, c *wsConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.battles[battleID]; ok {
		delete(conns, c)
		if len(conns) == 0 {
			delete(h.battles, battleID)
		}
	}
}

// broadcast sends event to every connection registered for battleID.
// Connections that fail to receive the message are closed and unregistered.
func (h *Hub) broadcast(battleID string, event model.BattleEvent) {
	h.mu.RLock()
	conns := make([]*wsConn, 0, len(h.battles[battleID]))
	for c := range h.battles[battleID] {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	var dead []*wsConn
	for _, c := range conns {
		if err := c.writeEvent(event); err != nil {
			dead = append(dead, c)
		}
	}
	for _, c := range dead {
		c.close()
		h.unregister(battleID, c)
	}
}

// connCount returns the number of active connections for battleID (for testing).
func (h *Hub) connCount(battleID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.battles[battleID])
}
