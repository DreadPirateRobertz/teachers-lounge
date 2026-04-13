package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/store"
)

// StreakFreeze handles POST /gaming/streak/freeze.
// Spends store.StreakFreezeCost gems to activate a 24-hour streak freeze that
// prevents the next missed day from resetting the caller's streak.
func (h *Handler) StreakFreeze(w http.ResponseWriter, r *http.Request) {
	var req model.StreakFreezeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id required")
		return
	}

	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" || callerID != req.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	gemsLeft, err := h.store.CreateStreakFreeze(r.Context(), callerID)
	if err != nil {
		if errors.Is(err, store.ErrInsufficientCoins) {
			writeError(w, http.StatusUnprocessableEntity, "not enough coins")
			return
		}
		h.logger.Error("streak freeze", zap.String("user_id", callerID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, model.StreakFreezeResponse{
		Active:   true,
		GemsLeft: gemsLeft,
	})
}
