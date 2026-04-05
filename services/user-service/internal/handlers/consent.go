package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/teacherslounge/user-service/internal/models"
	"github.com/teacherslounge/user-service/internal/store"
)

// consentResponse is the combined response for GET /users/{id}/consent.
// It embeds both the GDPR preference bundle (tutoring/analytics/marketing)
// and the minor/guardian consent status fields so that callers decoding into
// either models.ConsentBundle or models.ConsentStatus receive the data they expect.
type consentResponse struct {
	Tutoring  *models.ConsentRecord `json:"tutoring"`
	Analytics *models.ConsentRecord `json:"analytics"`
	Marketing *models.ConsentRecord `json:"marketing"`

	IsMinor           bool       `json:"is_minor"`
	GuardianEmail     *string    `json:"guardian_email,omitempty"`
	GuardianConsentAt *time.Time `json:"guardian_consent_at,omitempty"`
	ConsentRequired   bool       `json:"consent_required"`
	ConsentGiven      bool       `json:"consent_given"`
}

// consentUpdateBody is the request body for PATCH /users/{id}/consent.
// It handles both the GDPR preferences path (tutoring/analytics/marketing)
// and the guardian consent path (guardian_email).
type consentUpdateBody struct {
	GuardianEmail string `json:"guardian_email"`
	Tutoring      *bool  `json:"tutoring,omitempty"`
	Analytics     *bool  `json:"analytics,omitempty"`
	Marketing     *bool  `json:"marketing,omitempty"`
}

// GetConsent handles GET /users/{id}/consent.
//
// Returns the combined consent state: GDPR preferences (tutoring/analytics/marketing)
// from the store, plus minor/guardian flags derived from the user record.
func (h *UsersHandler) GetConsent(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	bundle, err := h.store.GetConsent(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := consentResponse{
		Tutoring:  bundle.Tutoring,
		Analytics: bundle.Analytics,
		Marketing: bundle.Marketing,
	}

	// Augment with minor/guardian info when the user record is available.
	if user, err := h.store.GetUserByID(r.Context(), userID); err == nil {
		isMinor := user.IsMinor()
		resp.IsMinor = isMinor
		resp.GuardianEmail = user.GuardianEmail
		resp.GuardianConsentAt = user.GuardianConsentAt
		resp.ConsentRequired = isMinor
		resp.ConsentGiven = isMinor && user.GuardianConsentAt != nil
	}

	writeJSON(w, http.StatusOK, resp)
}

// UpdateConsent handles PATCH /users/{id}/consent.
//
// Supports two flows based on the request body:
//   - Guardian consent (guardian_email set): records parental consent for a minor.
//     The guardian_email must match the value on the account.
//   - GDPR preferences (tutoring/analytics/marketing set): updates the user's
//     data-processing consent preferences via store.UpdateConsent.
func (h *UsersHandler) UpdateConsent(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req consentUpdateBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.GuardianEmail != "" {
		// Guardian consent flow: verify the user is a minor first.
		user, err := h.store.GetUserByID(r.Context(), userID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "user not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if !user.IsMinor() {
			writeError(w, http.StatusUnprocessableEntity, "consent only required for minor accounts")
			return
		}

		if err := h.store.UpdateGuardianConsent(r.Context(), userID, req.GuardianEmail); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusUnprocessableEntity, "guardian_email does not match account")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to record consent")
			return
		}

		_ = h.store.WriteAuditLog(r.Context(), store.AuditLogParams{
			AccessorID:   &userID,
			StudentID:    &userID,
			Action:       models.AuditActionConsentGiven,
			DataAccessed: "guardian_consent",
			Purpose:      "ferpa_k12_parental_consent",
			IPAddress:    realIP(r),
		})

		w.WriteHeader(http.StatusNoContent)
		return
	}

	// GDPR preferences flow: update tutoring/analytics/marketing flags.
	params := store.UpdateConsentParams{
		Tutoring:  req.Tutoring,
		Analytics: req.Analytics,
		Marketing: req.Marketing,
		IPAddress: realIP(r),
		UserAgent: r.UserAgent(),
	}
	if err := h.store.UpdateConsent(r.Context(), userID, params); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update consent")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
