package billing

import (
	"context"
	"fmt"

	"github.com/stripe/stripe-go/v79"
	stripesubscription "github.com/stripe/stripe-go/v79/subscription"
	"github.com/teacherslounge/user-service/internal/models"
	"github.com/teacherslounge/user-service/internal/store"
)

// SubscriptionManager is the interface for Stripe subscription lifecycle calls.
// Extracted so handlers can accept an interface and tests can inject a mock.
type SubscriptionManager interface {
	CancelSubscription(ctx context.Context, sub *models.Subscription) (*models.Subscription, error)
	ReactivateSubscription(ctx context.Context, sub *models.Subscription) (*models.Subscription, error)
}

// Ensure *Client satisfies SubscriptionManager.
var _ SubscriptionManager = (*Client)(nil)

// CancelSubscription sets cancel_at_period_end=true so the subscription stays
// active until the end of the billing period, then cancels automatically.
// Returns the updated subscription model.
func (c *Client) CancelSubscription(ctx context.Context, sub *models.Subscription) (*models.Subscription, error) {
	if sub.StripeSubscriptionID == nil || *sub.StripeSubscriptionID == "" {
		return nil, fmt.Errorf("subscription has no Stripe ID — cannot cancel")
	}

	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(true),
	}
	_, err := stripesubscription.Update(*sub.StripeSubscriptionID, params)
	if err != nil {
		return nil, fmt.Errorf("cancelling stripe subscription: %w", err)
	}

	// Reflect the change locally — status stays active until period end.
	// The subscription.updated webhook will also fire and write to DB.
	// We write immediately so callers see an up-to-date response.
	status := models.StatusActive // still active, just scheduled for cancellation
	if err := c.store.UpdateSubscription(ctx, store.UpdateSubscriptionParams{
		StripeSubscriptionID: *sub.StripeSubscriptionID,
		Status:               &status,
	}); err != nil {
		return nil, fmt.Errorf("updating local subscription after cancel: %w", err)
	}

	sub.Status = status
	return sub, nil
}

// ReactivateSubscription removes a pending cancellation (cancel_at_period_end=false).
// Only valid when status=active and cancel_at_period_end was previously set.
func (c *Client) ReactivateSubscription(ctx context.Context, sub *models.Subscription) (*models.Subscription, error) {
	if sub.StripeSubscriptionID == nil || *sub.StripeSubscriptionID == "" {
		return nil, fmt.Errorf("subscription has no Stripe ID — cannot reactivate")
	}
	if sub.Status != models.StatusActive {
		return nil, fmt.Errorf("can only reactivate an active subscription (current status: %s)", sub.Status)
	}

	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(false),
	}
	_, err := stripesubscription.Update(*sub.StripeSubscriptionID, params)
	if err != nil {
		return nil, fmt.Errorf("reactivating stripe subscription: %w", err)
	}

	return sub, nil
}
