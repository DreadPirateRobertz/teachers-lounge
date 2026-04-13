// Package wsbattle is an in-memory pub/sub hub for boss-battle events.
//
// The gaming-service REST endpoints (Attack, ActivatePowerUp, finishBattle)
// remain the source of truth for battle state; the hub exists so connected
// WebSocket clients can be notified of state changes in real time.
//
// A single *Hub is created at server startup and shared across all request
// handlers. Subscribers register against a battle ID; Broadcast fans out to
// every subscriber for that battle. The hub is safe for concurrent use and
// never blocks a publisher on a slow subscriber — per-subscriber channels
// are buffered and slow subscribers are dropped rather than back-pressured.
package wsbattle

import (
	"sync"
	"time"
)

// Event categories broadcast over the WebSocket. Kept as string constants so
// front-end code can switch on them directly.
const (
	EventDamage          = "damage"
	EventPhaseTransition = "phase_transition"
	EventLootRoll        = "loot_roll"
	EventPowerUp         = "power_up"
)

// Event is the envelope every subscriber receives. Payload is opaque to the
// hub — handlers marshal their own struct into the interface.
type Event struct {
	Type      string      `json:"type"`
	BattleID  string      `json:"battle_id"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}

// Subscriber is a live connection's inbox. Hub.Broadcast does a non-blocking
// send on C; when the buffer is full the send is dropped silently so one slow
// client cannot stall the publisher.
type Subscriber struct {
	id string
	C  chan Event
}

// ID returns the opaque identifier assigned by Subscribe. Callers should treat
// this as a handle for Unsubscribe only.
func (s *Subscriber) ID() string { return s.id }

// Hub fans battle events out to connected WebSocket clients. The zero value is
// not usable; call New.
type Hub struct {
	mu   sync.RWMutex
	subs map[string]map[string]*Subscriber // battleID → subscriberID → *Subscriber
	// nextID is a monotonically increasing counter used to mint subscriber IDs
	// so tests and Unsubscribe calls have a deterministic handle.
	nextID uint64
	// now allows tests to freeze the clock used for Event.Timestamp.
	now func() time.Time
}

// New returns a ready-to-use Hub with no subscribers.
func New() *Hub {
	return &Hub{
		subs: make(map[string]map[string]*Subscriber),
		now:  time.Now,
	}
}

// sendBuffer is the per-subscriber channel depth. Picked generously so a
// bursty turn (damage + phase + loot emitted back-to-back) does not drop
// frames under normal scheduling latency.
const sendBuffer = 16

// Subscribe registers a new subscriber for the given battle and returns it
// along with its handle. The returned Subscriber.C must be drained by the
// caller; events are dropped if the buffer fills.
func (h *Hub) Subscribe(battleID string) *Subscriber {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.nextID++
	sub := &Subscriber{
		id: subscriberID(h.nextID),
		C:  make(chan Event, sendBuffer),
	}

	if h.subs[battleID] == nil {
		h.subs[battleID] = make(map[string]*Subscriber)
	}
	h.subs[battleID][sub.id] = sub
	return sub
}

// Unsubscribe removes a subscriber registered via Subscribe. It is safe to
// call Unsubscribe multiple times or with an unknown handle.
//
// The subscriber's channel is closed so any goroutine ranging over it can exit
// cleanly without additional coordination.
func (h *Hub) Unsubscribe(battleID string, sub *Subscriber) {
	if sub == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	bucket, ok := h.subs[battleID]
	if !ok {
		return
	}
	if existing, ok := bucket[sub.id]; ok {
		delete(bucket, sub.id)
		close(existing.C)
	}
	if len(bucket) == 0 {
		delete(h.subs, battleID)
	}
}

// SubscriberCount returns the number of live subscribers for a given battle.
// Intended for tests and diagnostics; production code should not rely on it.
func (h *Hub) SubscriberCount(battleID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs[battleID])
}

// Broadcast fans out an event of the given type to every subscriber of the
// battle. Sends are non-blocking — if a subscriber's buffer is full the event
// is dropped for that subscriber only.
//
// Broadcasting to a battle with no subscribers is a silent no-op.
func (h *Hub) Broadcast(battleID, eventType string, payload interface{}) {
	// Hold the read lock for the whole send. Unsubscribe (which closes the
	// channel) takes the write lock, so while we hold RLock no channel can
	// be closed underneath us — this prevents the "send on closed channel"
	// race between Broadcast and Unsubscribe. The send itself is non-blocking
	// (select default) so concurrent broadcasters don't queue on slow clients.
	h.mu.RLock()
	defer h.mu.RUnlock()

	bucket := h.subs[battleID]
	if len(bucket) == 0 {
		return
	}

	evt := Event{
		Type:      eventType,
		BattleID:  battleID,
		Timestamp: h.now(),
		Payload:   payload,
	}
	for _, s := range bucket {
		select {
		case s.C <- evt:
		default:
			// Buffer full — drop this event for the slow subscriber. The
			// client will see a gap but the hub stays responsive.
		}
	}
}

// subscriberID encodes a counter as a short string. Separate from Subscribe so
// tests can reproduce IDs if needed.
func subscriberID(n uint64) string {
	// Hex is compact and avoids locale-dependent formatting; front-end never
	// reads this value so a minimal encoding is fine.
	const hex = "0123456789abcdef"
	if n == 0 {
		return "0"
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = hex[n&0xf]
		n >>= 4
	}
	return string(buf[i:])
}
