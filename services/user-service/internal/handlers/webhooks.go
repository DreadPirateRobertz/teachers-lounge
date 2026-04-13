package handlers

import (
	"io"
	"net/http"

	"github.com/teacherslounge/user-service/internal/billing"
)

// WebhookHandler handles inbound Stripe webhook events at POST /webhooks/stripe.
type WebhookHandler struct {
	billing *billing.Client
}

// NewWebhookHandler creates a WebhookHandler using the given Stripe billing client.
func NewWebhookHandler(b *billing.Client) *WebhookHandler {
	return &WebhookHandler{billing: b}
}

// POST /webhooks/stripe
// Stripe sends subscription lifecycle events here.
// Endpoint must NOT require authentication — Stripe signs the payload.
func (h *WebhookHandler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	// Stripe recommends reading max 65536 bytes to avoid DoS
	payload, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	sigHeader := r.Header.Get("Stripe-Signature")
	if sigHeader == "" {
		writeError(w, http.StatusBadRequest, "missing Stripe-Signature header")
		return
	}

	if err := h.billing.HandleWebhook(r.Context(), payload, sigHeader); err != nil {
		// Return 400 so Stripe retries on signature failure.
		// Return 200 on business-logic errors to prevent infinite retries.
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Always return 200 quickly — Stripe retries on non-2xx.
	writeJSON(w, http.StatusOK, map[string]string{"received": "true"})
}
