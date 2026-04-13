// Package billing handles Stripe subscription lifecycle for TeachersLounge.
package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/customer"
	"github.com/stripe/stripe-go/v79/webhook"
	"github.com/teacherslounge/user-service/internal/models"
	"github.com/teacherslounge/user-service/internal/store"
)

// Plan metadata — Stripe Price IDs injected from config.
type PlanPrices struct {
	Monthly    string
	Quarterly  string
	Semesterly string
}

// Storer is the subset of store.Store that billing needs.
// Defined here to allow test doubles.
type Storer interface {
	UpdateSubscription(ctx context.Context, p store.UpdateSubscriptionParams) error
}

// Client wraps Stripe operations.
type Client struct {
	prices   PlanPrices
	whSecret string
	store    Storer
}

// NewClient creates a billing Client configured with the given Stripe secret key,
// plan price IDs, webhook signing secret, and user store.
func NewClient(secretKey string, prices PlanPrices, whSecret string, s Storer) *Client {
	stripe.Key = secretKey
	return &Client{
		prices:   prices,
		whSecret: whSecret,
		store:    s,
	}
}

// ============================================================
// CUSTOMER CREATION
// ============================================================

// CreateCustomer creates a Stripe customer for a new user and starts their trial.
// Returns the Stripe customer ID.
func (c *Client) CreateCustomer(ctx context.Context, user *models.User, trialDays int) (string, error) {
	params := &stripe.CustomerParams{
		Email: stripe.String(user.Email),
		Name:  stripe.String(user.DisplayName),
		Metadata: map[string]string{
			"user_id": user.ID.String(),
		},
	}
	cust, err := customer.New(params)
	if err != nil {
		return "", fmt.Errorf("creating stripe customer: %w", err)
	}
	return cust.ID, nil
}

// ============================================================
// WEBHOOK HANDLING
// ============================================================

// HandleWebhook validates the Stripe webhook signature and dispatches the event.
func (c *Client) HandleWebhook(ctx context.Context, payload []byte, sigHeader string) error {
	event, err := webhook.ConstructEvent(payload, sigHeader, c.whSecret)
	if err != nil {
		return fmt.Errorf("invalid webhook signature: %w", err)
	}
	return c.HandleWebhookEvent(ctx, event)
}

// HandleWebhookEvent dispatches a pre-parsed event. Exposed for testing.
func (c *Client) HandleWebhookEvent(ctx context.Context, event stripe.Event) error {
	switch event.Type {
	case "customer.subscription.created":
		return c.handleSubscriptionUpsert(ctx, event)
	case "customer.subscription.updated":
		return c.handleSubscriptionUpsert(ctx, event)
	case "customer.subscription.deleted":
		return c.handleSubscriptionDeleted(ctx, event)
	case "invoice.payment_failed":
		return c.handlePaymentFailed(ctx, event)
	case "invoice.payment_succeeded":
		return c.handlePaymentSucceeded(ctx, event)
	case "customer.subscription.trial_will_end":
		// 3 days before trial ends — future: trigger Notification Service
		return nil
	default:
		return nil
	}
}

// handleSubscriptionUpsert handles subscription.created and subscription.updated.
// Parses the subscription object directly from event.Data.Raw — no Stripe API call.
func (c *Client) handleSubscriptionUpsert(ctx context.Context, event stripe.Event) error {
	sub, err := parseSubscriptionFromEvent(event)
	if err != nil {
		return err
	}

	if len(sub.Items.Data) == 0 {
		return fmt.Errorf("subscription %s has no items", sub.ID)
	}
	plan, err := c.planFromPriceID(sub.Items.Data[0].Price.ID)
	if err != nil {
		return err
	}

	status := mapStripeStatus(string(sub.Status))
	start := time.Unix(sub.CurrentPeriodStart, 0)
	end := time.Unix(sub.CurrentPeriodEnd, 0)

	var trialEnd *time.Time
	if sub.TrialEnd != 0 {
		t := time.Unix(sub.TrialEnd, 0)
		trialEnd = &t
	}

	subIDStr := sub.ID
	return c.store.UpdateSubscription(ctx, store.UpdateSubscriptionParams{
		StripeSubscriptionID:    sub.ID,
		NewStripeSubscriptionID: &subIDStr,
		Plan:                    &plan,
		Status:                  &status,
		CurrentPeriodStart:      &start,
		CurrentPeriodEnd:        &end,
		TrialEnd:                trialEnd,
	})
}

func (c *Client) handleSubscriptionDeleted(ctx context.Context, event stripe.Event) error {
	sub, err := parseSubscriptionFromEvent(event)
	if err != nil {
		return err
	}
	status := models.StatusCancelled
	now := time.Now()
	return c.store.UpdateSubscription(ctx, store.UpdateSubscriptionParams{
		StripeSubscriptionID: sub.ID,
		Status:               &status,
		CancelledAt:          &now,
	})
}

func (c *Client) handlePaymentFailed(ctx context.Context, event stripe.Event) error {
	subID, err := subscriptionIDFromInvoiceEvent(event)
	if err != nil || subID == "" {
		return nil // invoice may not have a subscription (one-time charge)
	}
	status := models.StatusPastDue
	return c.store.UpdateSubscription(ctx, store.UpdateSubscriptionParams{
		StripeSubscriptionID: subID,
		Status:               &status,
	})
}

func (c *Client) handlePaymentSucceeded(ctx context.Context, event stripe.Event) error {
	subID, err := subscriptionIDFromInvoiceEvent(event)
	if err != nil || subID == "" {
		return nil
	}
	status := models.StatusActive
	return c.store.UpdateSubscription(ctx, store.UpdateSubscriptionParams{
		StripeSubscriptionID: subID,
		Status:               &status,
	})
}

// ============================================================
// HELPERS
// ============================================================

// parseSubscriptionFromEvent decodes the subscription object embedded in the
// event payload. This avoids a synchronous Stripe API call per webhook.
func parseSubscriptionFromEvent(event stripe.Event) (*stripe.Subscription, error) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return nil, fmt.Errorf("decoding subscription from event %s: %w", event.ID, err)
	}
	if sub.ID == "" {
		return nil, fmt.Errorf("subscription id missing in event %s", event.ID)
	}
	return &sub, nil
}

// subscriptionIDFromInvoiceEvent extracts the subscription ID from an invoice event.
func subscriptionIDFromInvoiceEvent(event stripe.Event) (string, error) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return "", fmt.Errorf("decoding invoice from event %s: %w", event.ID, err)
	}
	if invoice.Subscription == nil {
		return "", nil
	}
	return invoice.Subscription.ID, nil
}

func (c *Client) planFromPriceID(priceID string) (models.SubscriptionPlan, error) {
	switch priceID {
	case c.prices.Monthly:
		return models.PlanMonthly, nil
	case c.prices.Quarterly:
		return models.PlanQuarterly, nil
	case c.prices.Semesterly:
		return models.PlanSemesterly, nil
	default:
		return "", fmt.Errorf("unknown price ID: %s", priceID)
	}
}

func mapStripeStatus(s string) models.SubscriptionStatus {
	switch s {
	case "trialing":
		return models.StatusTrialing
	case "active":
		return models.StatusActive
	case "past_due":
		return models.StatusPastDue
	case "canceled", "cancelled":
		return models.StatusCancelled
	case "unpaid":
		return models.StatusExpired
	default:
		return models.StatusActive
	}
}
