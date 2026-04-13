// Package hub manages WebSocket connections grouped by member ID.
// Multiple sessions per member are supported — a Broadcast reaches all of them.
package hub

import (
	"sync"

	"github.com/gorilla/websocket"
)

// BossUnlockPayload is the JSON message sent to WebSocket clients when a boss
// is unlocked for a member.
type BossUnlockPayload struct {
	Event     string `json:"event"`      // always "boss_unlocked"
	BossID    string `json:"boss_id"`
	ChapterID string `json:"chapter_id"`
	MemberID  string `json:"member_id"`
}

// Hub maintains the set of active WebSocket connections, keyed by member ID.
// It is safe for concurrent use.
type Hub struct {
	mu      sync.RWMutex
	members map[string]map[*websocket.Conn]struct{}
}

// New returns an initialized, empty Hub.
func New() *Hub {
	return &Hub{
		members: make(map[string]map[*websocket.Conn]struct{}),
	}
}

// Register adds conn to the set of WebSocket sessions for memberID.
// Calling Register for the same conn twice is a no-op.
func (h *Hub) Register(memberID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.members[memberID] == nil {
		h.members[memberID] = make(map[*websocket.Conn]struct{})
	}
	h.members[memberID][conn] = struct{}{}
}

// Deregister removes conn from the set of WebSocket sessions for memberID.
// It is safe to call Deregister for a connection that was never registered.
func (h *Hub) Deregister(memberID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conns, ok := h.members[memberID]
	if !ok {
		return
	}
	delete(conns, conn)
	if len(conns) == 0 {
		delete(h.members, memberID)
	}
}

// Broadcast sends payload as a JSON message to every WebSocket connection
// registered for memberID. Connections that fail to write are deregistered
// so they do not block future broadcasts.
func (h *Hub) Broadcast(memberID string, payload any) {
	h.mu.RLock()
	conns := make([]*websocket.Conn, 0, len(h.members[memberID]))
	for c := range h.members[memberID] {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	var failed []*websocket.Conn
	for _, conn := range conns {
		if err := conn.WriteJSON(payload); err != nil {
			failed = append(failed, conn)
		}
	}

	if len(failed) == 0 {
		return
	}

	// Remove dead connections outside the read lock.
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, conn := range failed {
		conns, ok := h.members[memberID]
		if !ok {
			break
		}
		delete(conns, conn)
		if len(conns) == 0 {
			delete(h.members, memberID)
		}
	}
}

// SessionCount returns the number of active WebSocket sessions for memberID.
func (h *Hub) SessionCount(memberID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.members[memberID])
}
