package billing_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v79"
	"github.com/teacherslounge/user-service/internal/billing"
	"github.com/teacherslounge/user-service/internal/models"
	"github.com/teacherslounge/user-service/internal/store"
)

// ============================================================
// MOCK STORER
// ============================================================

type mockStorer struct {
	calls  []store.UpdateSubscriptionParams
	err    error
}

func (m *mockStorer) UpdateSubscription(_ context.Context, p store.UpdateSubscriptionParams) error {
	m.calls = append(m.calls, p)
	return m.err
}

// ============================================================
// HELPERS
// ============================================================

const (
	testPriceMonthly    = "price_monthly"
	testPriceQuarterly  = "price_quarterly"
	testPriceSemesterly = "price_semesterly"
	testWebhookSecret   = "whsec_test" // not validated in unit tests (we call handleWebhook directly)
	testSubID           = "sub_test123"
)

func newTestClient(s billing.Storer) *billing.Client {
	return billing.NewClient("sk_test_dummy", billing.PlanPrices{
		Monthly:    testPriceMonthly,
		Quarterly:  testPriceQuarterly,
		Semesterly: testPriceSemesterly,
	}, testWebhookSecret, s)
}

// buildSubscriptionEvent builds a raw stripe.Event for subscription events.
func buildSubscriptionEvent(eventType string, sub *stripe.Subscription) stripe.Event {
	raw, _ := json.Marshal(sub)
	return stripe.Event{
		ID:   "evt_" + uuid.NewString(),
		Type: stripe.EventType(eventType),
		Data: &stripe.EventData{
			Raw: raw,
		},
	}
}

// buildInvoiceEvent builds a raw stripe.Event for invoice events.
func buildInvoiceEvent(eventType, subID string) stripe.Event {
	invoice := &stripe.Invoice{
		ID:           "in_test",
		Subscription: &stripe.Subscription{ID: subID},
	}
	raw, _ := json.Marshal(invoice)
	return stripe.Event{
		ID:   "evt_" + uuid.NewString(),
		Type: stripe.EventType(eventType),
		Data: &stripe.EventData{
			Raw: raw,
		},
	}
}

func makeSubscription(status stripe.SubscriptionStatus, priceID string) *stripe.Subscription {
	periodStart := time.Now().Unix()
	periodEnd := time.Now().Add(30 * 24 * time.Hour).Unix()
	return &stripe.Subscription{
		ID:                 testSubID,
		Status:             status,
		CurrentPeriodStart: periodStart,
		CurrentPeriodEnd:   periodEnd,
		Items: &stripe.SubscriptionItemList{
			Data: []*stripe.SubscriptionItem{
				{Price: &stripe.Price{ID: priceID}},
			},
		},
	}
}

// ============================================================
// TESTS: subscription.created → active
// ============================================================

func TestHandleWebhook_SubscriptionCreated_Monthly(t *testing.T) {
	s := &mockStorer{}
	c := newTestClient(s)

	sub := makeSubscription("active", testPriceMonthly)
	event := buildSubscriptionEvent("customer.subscription.created", sub)
	if err := invokeEvent(c, event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(s.calls) != 1 {
		t.Fatalf("expected 1 UpdateSubscription call, got %d", len(s.calls))
	}
	call := s.calls[0]
	if call.StripeSubscriptionID != testSubID {
		t.Errorf("expected sub ID %s, got %s", testSubID, call.StripeSubscriptionID)
	}
	if *call.Status != models.StatusActive {
		t.Errorf("expected status active, got %s", *call.Status)
	}
	if *call.Plan != models.PlanMonthly {
		t.Errorf("expected plan monthly, got %s", *call.Plan)
	}
}

func TestHandleWebhook_SubscriptionCreated_Trialing(t *testing.T) {
	s := &mockStorer{}
	c := newTestClient(s)

	sub := makeSubscription("trialing", testPriceQuarterly)
	sub.TrialEnd = time.Now().Add(14 * 24 * time.Hour).Unix()
	event := buildSubscriptionEvent("customer.subscription.created", sub)
	if err := invokeEvent(c, event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	call := s.calls[0]
	if *call.Status != models.StatusTrialing {
		t.Errorf("expected status trialing, got %s", *call.Status)
	}
	if call.TrialEnd == nil {
		t.Error("expected TrialEnd to be set")
	}
}

// ============================================================
// TESTS: subscription.updated → plan change
// ============================================================

func TestHandleWebhook_SubscriptionUpdated_PlanChange(t *testing.T) {
	s := &mockStorer{}
	c := newTestClient(s)

	sub := makeSubscription("active", testPriceSemesterly)
	event := buildSubscriptionEvent("customer.subscription.updated", sub)
	if err := invokeEvent(c, event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	call := s.calls[0]
	if *call.Plan != models.PlanSemesterly {
		t.Errorf("expected semesterly, got %s", *call.Plan)
	}
}

// ============================================================
// TESTS: subscription.deleted → cancelled
// ============================================================

func TestHandleWebhook_SubscriptionDeleted(t *testing.T) {
	s := &mockStorer{}
	c := newTestClient(s)

	sub := makeSubscription("canceled", testPriceMonthly)
	event := buildSubscriptionEvent("customer.subscription.deleted", sub)
	if err := invokeEvent(c, event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	call := s.calls[0]
	if *call.Status != models.StatusCancelled {
		t.Errorf("expected status cancelled, got %s", *call.Status)
	}
	if call.CancelledAt == nil {
		t.Error("expected CancelledAt to be set")
	}
}

// ============================================================
// TESTS: invoice.payment_failed → past_due
// ============================================================

func TestHandleWebhook_PaymentFailed_SetsPastDue(t *testing.T) {
	s := &mockStorer{}
	c := newTestClient(s)

	event := buildInvoiceEvent("invoice.payment_failed", testSubID)
	if err := invokeEvent(c, event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	call := s.calls[0]
	if *call.Status != models.StatusPastDue {
		t.Errorf("expected status past_due, got %s", *call.Status)
	}
	if call.StripeSubscriptionID != testSubID {
		t.Errorf("expected sub ID %s, got %s", testSubID, call.StripeSubscriptionID)
	}
}

// ============================================================
// TESTS: invoice.payment_succeeded → active (recovery)
// ============================================================

func TestHandleWebhook_PaymentSucceeded_RestoresActive(t *testing.T) {
	s := &mockStorer{}
	c := newTestClient(s)

	event := buildInvoiceEvent("invoice.payment_succeeded", testSubID)
	if err := invokeEvent(c, event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	call := s.calls[0]
	if *call.Status != models.StatusActive {
		t.Errorf("expected status active, got %s", *call.Status)
	}
}

// ============================================================
// TESTS: unknown event type ignored
// ============================================================

func TestHandleWebhook_UnknownEvent_NoOp(t *testing.T) {
	s := &mockStorer{}
	c := newTestClient(s)

	event := stripe.Event{
		ID:   "evt_unknown",
		Type: "charge.succeeded",
		Data: &stripe.EventData{Raw: []byte(`{}`)},
	}
	if err := invokeEvent(c, event); err != nil {
		t.Fatalf("unexpected error for unknown event: %v", err)
	}
	if len(s.calls) != 0 {
		t.Errorf("expected 0 store calls for unknown event, got %d", len(s.calls))
	}
}

// ============================================================
// TESTS: no sync Stripe API call (event data used directly)
// ============================================================

func TestHandleWebhook_NoStripeAPICall(t *testing.T) {
	// If parseSubscriptionFromEvent works correctly, no Stripe API call is made.
	// We verify this by using an invalid stripe.Key — a real API call would fail with auth error.
	s := &mockStorer{}
	c := billing.NewClient("sk_invalid_key_that_will_fail_if_called", billing.PlanPrices{
		Monthly:    testPriceMonthly,
		Quarterly:  testPriceQuarterly,
		Semesterly: testPriceSemesterly,
	}, testWebhookSecret, s)

	sub := makeSubscription("active", testPriceMonthly)
	event := buildSubscriptionEvent("customer.subscription.created", sub)
	// Should succeed without making any Stripe API calls
	if err := invokeEvent(c, event); err != nil {
		t.Fatalf("expected no error (no Stripe API call), got: %v", err)
	}
}

// invokeEvent bypasses webhook signature validation (not needed in unit tests)
// by calling the exported HandleWebhookEvent test helper.
// Since HandleWebhook requires a valid signature, we use a package-internal helper.
func invokeEvent(c *billing.Client, event stripe.Event) error {
	return c.HandleWebhookEvent(context.Background(), event)
}
