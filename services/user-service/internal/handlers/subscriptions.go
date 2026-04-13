package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/teacherslounge/user-service/internal/billing"
	"github.com/teacherslounge/user-service/internal/models"
	"github.com/teacherslounge/user-service/internal/store"
)

// SubscriptionsHandler handles subscription endpoints (GET/POST /users/.../subscription).
type SubscriptionsHandler struct {
	store   store.Storer
	billing billing.SubscriptionManager
}

// NewSubscriptionsHandler creates a SubscriptionsHandler backed by the given store and billing client.
func NewSubscriptionsHandler(s store.Storer, b billing.SubscriptionManager) *SubscriptionsHandler {
	return &SubscriptionsHandler{store: s, billing: b}
}

// GET /users/{id}/subscription
func (h *SubscriptionsHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	sub, err := h.store.GetSubscriptionByUserID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "subscription not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, toSubscriptionResponse(sub))
}

// POST /users/{id}/subscription/cancel
func (h *SubscriptionsHandler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	sub, err := h.store.GetSubscriptionByUserID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "subscription not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if !sub.IsActive() {
		writeError(w, http.StatusUnprocessableEntity, "subscription is not active")
		return
	}

	updated, err := h.billing.CancelSubscription(r.Context(), sub)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to cancel subscription")
		return
	}

	if err := h.store.UpdateSubscriptionByUserID(r.Context(), userID, store.UpdateSubscriptionParams{
		Status: &updated.Status,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist subscription update")
		return
	}

	writeJSON(w, http.StatusOK, toSubscriptionResponse(updated))
}

// POST /users/{id}/subscription/reactivate
func (h *SubscriptionsHandler) ReactivateSubscription(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	sub, err := h.store.GetSubscriptionByUserID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "subscription not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	updated, err := h.billing.ReactivateSubscription(r.Context(), sub)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reactivate subscription")
		return
	}

	if err := h.store.UpdateSubscriptionByUserID(r.Context(), userID, store.UpdateSubscriptionParams{
		Status: &updated.Status,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist subscription update")
		return
	}

	writeJSON(w, http.StatusOK, toSubscriptionResponse(updated))
}

// ============================================================
// HELPERS
// ============================================================

type subscriptionResponse struct {
	Plan            string  `json:"plan"`
	Status          string  `json:"status"`
	TrialEndsAt     *string `json:"trial_ends_at,omitempty"`
	NextBillingDate *string `json:"next_billing_date,omitempty"`
}

func toSubscriptionResponse(sub *models.Subscription) *subscriptionResponse {
	resp := &subscriptionResponse{
		Plan:   string(sub.Plan),
		Status: string(sub.Status),
	}
	if sub.TrialEnd != nil {
		s := sub.TrialEnd.Format(time.RFC3339)
		resp.TrialEndsAt = &s
	}
	if sub.CurrentPeriodEnd != nil {
		s := sub.CurrentPeriodEnd.Format(time.RFC3339)
		resp.NextBillingDate = &s
	}
	return resp
}
