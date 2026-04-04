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
	"github.com/teacherslounge/user-service/internal/store"

	"github.com/teacherslounge/user-service/internal/rediskeys"
)

type UsersHandler struct {
	store store.Storer
	cache cache.Cacher
}

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

	user, err := h.store.GetUserByID(r.Context(), userID)
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

// ============================================================
// HELPERS
// ============================================================

func parseUserIDParam(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "id"))
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
