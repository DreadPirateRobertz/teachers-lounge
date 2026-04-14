package handler

import (
	"errors"
	"net/http"

	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/store"
)

// FreezeStreak handles POST /gaming/streak/freeze.
//
// Charges the caller model.StreakFreezeCost gems and sets a 24-hour
// streak_frozen_until on their gaming profile. While the freeze is
// active, StreakCheckin will not reset current_streak due to a missed
// day.
//
// Authentication: required — the caller is identified via JWT middleware;
// there is no request body, so there is no user_id to forge. A request
// without a valid caller is rejected with 403.
//
// Errors:
//   - 403 forbidden         — unauthenticated caller.
//   - 422 Unprocessable     — caller has fewer than StreakFreezeCost gems
//     (store.ErrNoGems) or already has an active freeze
//     (store.ErrAlreadyFrozen). Distinct "error" messages allow the
//     client to surface the right prompt.
//   - 500 internal error    — any other store failure.
func (h *Handler) FreezeStreak(w http.ResponseWriter, r *http.Request) {
	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	gemsLeft, expiresAt, err := h.store.CreateStreakFreeze(r.Context(), callerID, model.StreakFreezeCost)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNoGems):
			writeError(w, http.StatusUnprocessableEntity, "not enough gems")
			return
		case errors.Is(err, store.ErrAlreadyFrozen):
			writeError(w, http.StatusUnprocessableEntity, "streak already frozen")
			return
		}
		h.logger.Error("create streak freeze", zap.String("user_id", callerID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, model.StreakFreezeResponse{
		GemsLeft:  gemsLeft,
		ExpiresAt: expiresAt,
	})
}
