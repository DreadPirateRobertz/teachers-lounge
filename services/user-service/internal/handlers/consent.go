package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/teacherslounge/user-service/internal/models"
	"github.com/teacherslounge/user-service/internal/store"
)

// GetConsent handles GET /users/{id}/consent.
//
// Returns the consent status for the user: whether they are a minor, the
// guardian email on file, and whether guardian consent has been given.
func (h *UsersHandler) GetConsent(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	user, err := h.store.GetUserByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	isMinor := user.IsMinor()
	status := models.ConsentStatus{
		IsMinor:           isMinor,
		GuardianEmail:     user.GuardianEmail,
		GuardianConsentAt: user.GuardianConsentAt,
		ConsentRequired:   isMinor,
		ConsentGiven:      isMinor && user.GuardianConsentAt != nil,
	}

	writeJSON(w, http.StatusOK, status)
}

// UpdateConsent handles PATCH /users/{id}/consent.
//
// Records guardian consent for a minor user. The guardian_email in the request
// body must match the guardian_email stored on the account. Idempotent: calling
// this multiple times has no additional effect beyond the first acceptance.
func (h *UsersHandler) UpdateConsent(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req models.UpdateConsentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.GuardianEmail == "" {
		writeError(w, http.StatusBadRequest, "guardian_email is required")
		return
	}

	// Verify user is a minor before recording consent.
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

	// Write FERPA audit trail for consent.
	_ = h.store.WriteAuditLog(r.Context(), store.AuditLogParams{
		AccessorID:   &userID,
		StudentID:    &userID,
		Action:       models.AuditActionConsentGiven,
		DataAccessed: "guardian_consent",
		Purpose:      "ferpa_k12_parental_consent",
		IPAddress:    realIP(r),
	})

	w.WriteHeader(http.StatusNoContent)
}
