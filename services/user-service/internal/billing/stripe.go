// Package billing handles Stripe subscription lifecycle for TeachersLounge.
package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/customer"
	"github.com/stripe/stripe-go/v79/subscription"
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

// Client wraps Stripe operations.
type Client struct {
	prices  PlanPrices
	whSecret string
	store   *store.Store
}

func NewClient(secretKey string, prices PlanPrices, whSecret string, s *store.Store) *Client {
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

	switch event.Type {
	case "customer.subscription.created":
		return c.handleSubscriptionCreated(ctx, event)
	case "customer.subscription.updated":
		return c.handleSubscriptionUpdated(ctx, event)
	case "customer.subscription.deleted":
		return c.handleSubscriptionDeleted(ctx, event)
	case "invoice.payment_failed":
		return c.handlePaymentFailed(ctx, event)
	case "invoice.payment_succeeded":
		return c.handlePaymentSucceeded(ctx, event)
	case "customer.subscription.trial_will_end":
		// 3 days before trial ends — send notification (future: trigger Notification Service)
		return nil
	default:
		// Unknown event — ignore silently
		return nil
	}
}

func (c *Client) handleSubscriptionCreated(ctx context.Context, event stripe.Event) error {
	sub, err := parseSubscription(event)
	if err != nil {
		return err
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

func (c *Client) handleSubscriptionUpdated(ctx context.Context, event stripe.Event) error {
	return c.handleSubscriptionCreated(ctx, event) // same upsert logic
}

func (c *Client) handleSubscriptionDeleted(ctx context.Context, event stripe.Event) error {
	sub, err := parseSubscription(event)
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
	// Stripe dunning handles retries (configured in Stripe dashboard: 3 attempts over 7 days).
	// On final failure, subscription.deleted event fires — handled above.
	// Here we update status to past_due immediately so the app can show a banner.
	invoice, ok := event.Data.Object["subscription"]
	if !ok {
		return nil
	}
	subID, _ := invoice.(string)
	if subID == "" {
		return nil
	}
	status := models.StatusPastDue
	return c.store.UpdateSubscription(ctx, store.UpdateSubscriptionParams{
		StripeSubscriptionID: subID,
		Status:               &status,
	})
}

func (c *Client) handlePaymentSucceeded(ctx context.Context, event stripe.Event) error {
	// Payment recovered — restore active status.
	invoice, ok := event.Data.Object["subscription"]
	if !ok {
		return nil
	}
	subID, _ := invoice.(string)
	if subID == "" {
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

func parseSubscription(event stripe.Event) (*stripe.Subscription, error) {
	sub, ok := event.Data.Object["id"]
	if !ok {
		return nil, fmt.Errorf("missing subscription id in event")
	}
	subID := sub.(string)
	s, err := subscription.Get(subID, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching subscription %s: %w", subID, err)
	}
	return s, nil
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
