package wsbattle

import (
	"sync"
	"testing"
	"time"
)

// TestSubscribeBroadcastDeliversEvent verifies the happy path: a single
// subscriber receives the exact event that Broadcast publishes.
func TestSubscribeBroadcastDeliversEvent(t *testing.T) {
	h := New()
	sub := h.Subscribe("battle-1")

	h.Broadcast("battle-1", EventDamage, map[string]int{"hp": 42})

	select {
	case evt := <-sub.C:
		if evt.Type != EventDamage {
			t.Fatalf("event type: want %q got %q", EventDamage, evt.Type)
		}
		if evt.BattleID != "battle-1" {
			t.Fatalf("battle id: want battle-1 got %q", evt.BattleID)
		}
		payload, ok := evt.Payload.(map[string]int)
		if !ok {
			t.Fatalf("payload type: want map[string]int got %T", evt.Payload)
		}
		if payload["hp"] != 42 {
			t.Fatalf("payload hp: want 42 got %d", payload["hp"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber did not receive event within 100ms")
	}
}

// TestBroadcastFansOutToMultipleSubscribers checks every subscriber of the
// same battle gets the same event.
func TestBroadcastFansOutToMultipleSubscribers(t *testing.T) {
	h := New()
	a := h.Subscribe("b")
	b := h.Subscribe("b")
	c := h.Subscribe("b")

	h.Broadcast("b", EventLootRoll, "loot")

	for i, sub := range []*Subscriber{a, b, c} {
		select {
		case evt := <-sub.C:
			if evt.Type != EventLootRoll {
				t.Fatalf("sub %d: wrong event type %q", i, evt.Type)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("sub %d: no event delivered", i)
		}
	}
}

// TestBroadcastIsolatesByBattleID ensures subscribers of one battle never
// receive events from another.
func TestBroadcastIsolatesByBattleID(t *testing.T) {
	h := New()
	sub := h.Subscribe("alpha")
	other := h.Subscribe("beta")

	h.Broadcast("alpha", EventPhaseTransition, "victory")

	select {
	case evt := <-sub.C:
		if evt.BattleID != "alpha" {
			t.Fatalf("wrong battle id: %q", evt.BattleID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("alpha subscriber got no event")
	}

	select {
	case evt := <-other.C:
		t.Fatalf("beta subscriber got spurious event: %+v", evt)
	case <-time.After(20 * time.Millisecond):
		// expected — no event for beta
	}
}

// TestBroadcastWithNoSubscribersIsNoOp verifies Broadcast does not panic or
// allocate when the battle has no subscribers.
func TestBroadcastWithNoSubscribersIsNoOp(t *testing.T) {
	h := New()
	// Must not panic, must not block.
	h.Broadcast("ghost-battle", EventDamage, nil)
}

// TestUnsubscribeRemovesSubscriber verifies the subscriber stops receiving
// events AND its channel is closed so reading goroutines can exit.
func TestUnsubscribeRemovesSubscriber(t *testing.T) {
	h := New()
	sub := h.Subscribe("b1")

	if got := h.SubscriberCount("b1"); got != 1 {
		t.Fatalf("pre-unsub count: want 1 got %d", got)
	}

	h.Unsubscribe("b1", sub)

	if got := h.SubscriberCount("b1"); got != 0 {
		t.Fatalf("post-unsub count: want 0 got %d", got)
	}

	// Channel must be closed so range-loops terminate.
	_, ok := <-sub.C
	if ok {
		t.Fatal("subscriber channel still open after Unsubscribe")
	}

	// A subsequent Broadcast must not panic (channel is closed).
	h.Broadcast("b1", EventDamage, nil)
}

// TestUnsubscribeIdempotent confirms calling Unsubscribe twice is safe.
func TestUnsubscribeIdempotent(t *testing.T) {
	h := New()
	sub := h.Subscribe("b")
	h.Unsubscribe("b", sub)
	// Second call must not panic or close a nil/already-closed channel.
	h.Unsubscribe("b", sub)
	h.Unsubscribe("unknown-battle", sub)
	h.Unsubscribe("b", nil)
}

// TestBroadcastDoesNotBlockOnFullBuffer proves the publisher is never
// back-pressured by a slow subscriber — we fill the buffer and then push one
// more event that should be dropped for that subscriber without blocking.
func TestBroadcastDoesNotBlockOnFullBuffer(t *testing.T) {
	h := New()
	sub := h.Subscribe("b")

	for i := 0; i < sendBuffer; i++ {
		h.Broadcast("b", EventDamage, i)
	}

	done := make(chan struct{})
	go func() {
		h.Broadcast("b", EventDamage, "overflow")
		close(done)
	}()

	select {
	case <-done:
		// expected — Broadcast returned without blocking
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Broadcast blocked on full subscriber buffer")
	}

	// Subscriber should have at most sendBuffer events queued (overflow dropped).
	drained := 0
	for drained < sendBuffer {
		select {
		case <-sub.C:
			drained++
		case <-time.After(20 * time.Millisecond):
			t.Fatalf("expected %d events, drained %d", sendBuffer, drained)
		}
	}
}

// TestConcurrentSubscribeBroadcastUnsubscribe stresses the hub under race
// detector — 50 subscribers churn in and out while a publisher broadcasts.
func TestConcurrentSubscribeBroadcastUnsubscribe(t *testing.T) {
	h := New()
	const battleID = "race"

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Publisher
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				h.Broadcast(battleID, EventDamage, nil)
			}
		}
	}()

	// Churning subscribers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sub := h.Subscribe(battleID)
			// Drain until Unsubscribe closes the channel.
			go func() {
				for range sub.C {
				}
			}()
			time.Sleep(5 * time.Millisecond)
			h.Unsubscribe(battleID, sub)
		}()
	}

	// Let the churners finish.
	time.Sleep(30 * time.Millisecond)
	close(stop)
	wg.Wait()

	if got := h.SubscriberCount(battleID); got != 0 {
		t.Fatalf("leaked subscribers: %d", got)
	}
}
