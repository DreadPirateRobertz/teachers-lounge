package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/cache"
	"github.com/teacherslounge/user-service/internal/middleware"
	"github.com/teacherslounge/user-service/internal/models"
	"github.com/teacherslounge/user-service/internal/rediskeys"
	"github.com/teacherslounge/user-service/internal/store"
)

// UsersHandler handles user profile endpoints (GET/PATCH /users/...).
type UsersHandler struct {
	store store.Storer
	cache cache.Cacher
}

// NewUsersHandler creates a UsersHandler backed by the given store and cache.
func NewUsersHandler(s store.Storer, c cache.Cacher) *UsersHandler {
	return &UsersHandler{store: s, cache: c}
}

// GET /users/{id}/profile
func (h *UsersHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	user, err := h.loadUser(r, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	profile, err := h.store.GetLearningProfile(r.Context(), userID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	sub, _ := h.store.GetSubscriptionByUserID(r.Context(), userID)

	writeJSON(w, http.StatusOK, map[string]any{
		"user":             toUserResponse(user, sub),
		"learning_profile": profile,
	})
}

// PATCH /users/{id}/preferences
func (h *UsersHandler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req models.UpdatePreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Update user display fields if present
	if req.DisplayName != nil || req.AvatarEmoji != nil {
		_, err = h.store.UpdateUser(r.Context(), userID, store.UpdateUserParams{
			DisplayName: req.DisplayName,
			AvatarEmoji: req.AvatarEmoji,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update user")
			return
		}
	}

	// Update learning profile if any preference fields present
	if req.LearningStylePrefs != nil || req.FelderSilvermanDials != nil || req.ExplanationPreferences != nil {
		err = h.store.UpdateLearningProfile(r.Context(), userID, store.UpdateProfileParams{
			LearningStylePreferences: nilIfEmpty(req.LearningStylePrefs),
			FelderSilvermanDials:     nilIfEmpty(req.FelderSilvermanDials),
			ExplanationPreferences:   nilIfEmptyStr(req.ExplanationPreferences),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update preferences")
			return
		}
	}

	// Invalidate session cache so next request gets fresh data
	claims := middleware.ClaimsFromCtx(r.Context())
	if claims != nil {
		_ = h.cache.DeleteSession(r.Context(), rediskeys.Session(claims.UserID))
	}

	w.WriteHeader(http.StatusNoContent)
}

// PATCH /users/{id}/onboarding — mark the first-run wizard as complete.
//
// Idempotent: safe to call multiple times.  Returns 204 on success.
// The onboarded_at timestamp is set to NOW() on the first call; subsequent
// calls leave it unchanged (COALESCE in SQL).
func (h *UsersHandler) CompleteOnboarding(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	if err := h.store.CompleteOnboarding(r.Context(), userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update onboarding status")
		return
	}

	// Invalidate session cache so the next profile fetch includes the updated flag.
	claims := middleware.ClaimsFromCtx(r.Context())
	if claims != nil {
		_ = h.cache.DeleteSession(r.Context(), rediskeys.Session(claims.UserID))
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /users/{id}/export — triggers async GDPR data export
func (h *UsersHandler) ExportData(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	// Write audit log entry
	_ = h.store.WriteAuditLog(r.Context(), store.AuditLogParams{
		AccessorID:   &userID,
		StudentID:    &userID,
		Action:       "export_request",
		DataAccessed: "all_user_data",
		Purpose:      "gdpr_right_to_portability",
		IPAddress:    realIP(r),
	})

	jobID, err := h.store.CreateExportJob(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create export job")
		return
	}

	// TODO: publish job ID to Pub/Sub topic for async processing
	writeJSON(w, http.StatusAccepted, map[string]string{
		"job_id":  jobID.String(),
		"message": "export job queued — you will receive an email when ready",
	})
}

// DELETE /users/{id} — GDPR right to erasure (cascading delete via FK)
func (h *UsersHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	// Audit the deletion before it happens
	_ = h.store.WriteAuditLog(r.Context(), store.AuditLogParams{
		AccessorID:   &userID,
		StudentID:    &userID,
		Action:       "account_delete",
		DataAccessed: "all_user_data",
		Purpose:      "gdpr_right_to_erasure",
		IPAddress:    realIP(r),
	})

	// Revoke all refresh tokens first
	_ = h.store.RevokeAllUserTokens(r.Context(), userID)

	// Clear session cache
	_ = h.cache.DeleteSession(r.Context(), rediskeys.Session(userID.String()))

	// Delete user — FK CASCADE handles all child rows
	if err := h.store.DeleteUser(r.Context(), userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete account")
		return
	}

	// Clear refresh cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
	})

	w.WriteHeader(http.StatusNoContent)
}

// GetFullExport handles GET /users/{id}/export — returns the user's full
// GDPR data package synchronously as JSON, bypassing the async job flow.
//
// Owner-only access is already enforced by middleware.RequireSelf on the
// /users/{id} route group, so here we only need to verify the user still
// exists in the store (returning 404 for stale tokens / post-deletion),
// build the export via the same store.BuildUserExport path as the async
// endpoint, write an audit entry for GDPR portability, and return the
// UserExport payload.
//
// The async POST /users/{id}/export + GET /users/{id}/export/{jobID}
// flow remains the primary path for large exports; this endpoint exists
// for CLI/API clients that prefer a single round trip.
func (h *UsersHandler) GetFullExport(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	// Reuse the user record loaded by middleware.LoadUser when present;
	// otherwise fetch directly. Returns 404 for stale tokens whose user
	// has been deleted — prevents a half-populated export.
	user, err := h.loadUser(r, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	jobID, err := h.store.CreateExportJob(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create export job")
		return
	}

	// BuildUserExport accepts the pre-fetched user so it does not issue a
	// second GetUserByID — closes the TOCTOU window where a delete between
	// the existence check and the build would surface mid-export.
	export, err := h.store.BuildUserExport(r.Context(), jobID, user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build export")
		return
	}

	// GDPR audit-must-succeed: if we cannot record the disclosure, we must
	// not disclose. Other audit sites in this service use best-effort (_ =)
	// because they log reads of resources the caller already owns; this
	// endpoint returns the full export payload, so a missing audit record
	// would be a compliance gap rather than an operational nuisance.
	if err := h.store.WriteAuditLog(r.Context(), store.AuditLogParams{
		AccessorID:   &userID,
		StudentID:    &userID,
		Action:       models.AuditActionExportView,
		DataAccessed: "all_user_data",
		Purpose:      "gdpr_right_to_portability",
		IPAddress:    realIP(r),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record audit log")
		return
	}

	writeJSON(w, http.StatusOK, export)
}

// GET /users/{id}/export/{jobID} — retrieve a completed data export.
// On first call for a pending job, triggers synchronous data collection.
func (h *UsersHandler) GetExport(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	jobIDStr := chi.URLParam(r, "jobID")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	job, err := h.store.GetExportJob(r.Context(), jobID, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "export job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// If already complete, return cached result.
	if job.Status == models.ExportStatusComplete && job.ResultData != nil {
		writeJSON(w, http.StatusOK, job)
		return
	}

	// Pending or processing: build synchronously (Pub/Sub not yet wired).
	user, err := h.loadUser(r, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	export, err := h.store.BuildUserExport(r.Context(), jobID, user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build export")
		return
	}

	// Audit the export retrieval.
	_ = h.store.WriteAuditLog(r.Context(), store.AuditLogParams{
		AccessorID:   &userID,
		StudentID:    &userID,
		Action:       models.AuditActionExportView,
		DataAccessed: "all_user_data",
		Purpose:      "gdpr_right_to_portability",
		IPAddress:    realIP(r),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"job_id":     jobID,
		"status":     models.ExportStatusComplete,
		"export":     export,
	})
}

// ============================================================
// HELPERS
// ============================================================

func parseUserIDParam(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "id"))
}

// loadUser returns the user cached in the request context by
// middleware.LoadUser, falling back to a direct store fetch when the
// middleware was not wired (e.g. unit tests that exercise a single handler).
// In production this collapses two round-trips into one.
func (h *UsersHandler) loadUser(r *http.Request, userID uuid.UUID) (*models.User, error) {
	if u := middleware.UserFromCtx(r.Context()); u != nil && u.ID == userID {
		return u, nil
	}
	return h.store.GetUserByID(r.Context(), userID)
}

func nilIfEmpty(m map[string]float64) *map[string]float64 {
	if len(m) == 0 {
		return nil
	}
	return &m
}

func nilIfEmptyStr(m map[string]string) *map[string]string {
	if len(m) == 0 {
		return nil
	}
	return &m
}
