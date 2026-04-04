package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/xp"
)

// Storer is the interface the handler needs from the store layer.
type Storer interface {
	GetXPAndLevel(ctx context.Context, userID string) (int64, int, error)
	UpsertXP(ctx context.Context, userID string, newXP int64, newLevel int) error
	GetProfile(ctx context.Context, userID string) (*model.Profile, error)
	StreakCheckin(ctx context.Context, userID string) (current, longest int, reset bool, err error)
	LeaderboardUpdate(ctx context.Context, userID string, xpVal int64) error
	LeaderboardTop10(ctx context.Context, userID string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error)
	RandomQuote(ctx context.Context) (*model.Quote, error)
}

// Handler holds the store and logger.
type Handler struct {
	store  Storer
	logger *zap.Logger
}

// New creates a Handler.
func New(store Storer, logger *zap.Logger) *Handler {
	return &Handler{store: store, logger: logger}
}

// GainXP handles POST /gaming/xp
func (h *Handler) GainXP(w http.ResponseWriter, r *http.Request) {
	var req model.GainXPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" || req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "user_id and positive amount required")
		return
	}

	// Only the authenticated user can gain XP for themselves.
	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" || callerID != req.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	currentXP, currentLevel, err := h.store.GetXPAndLevel(r.Context(), req.UserID)
	if err != nil {
		h.logger.Error("get xp/level", zap.String("user_id", req.UserID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	newXP, newLevel, leveledUp := xp.Apply(currentXP, currentLevel, req.Amount)

	if err := h.store.UpsertXP(r.Context(), req.UserID, newXP, newLevel); err != nil {
		h.logger.Error("upsert xp", zap.String("user_id", req.UserID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, model.GainXPResponse{
		NewXP:    newXP,
		LevelUp:  leveledUp,
		NewLevel: newLevel,
	})
}

// GetProfile handles GET /gaming/profile/{userId}
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")

	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" || callerID != userID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	profile, err := h.store.GetProfile(r.Context(), userID)
	if err != nil {
		h.logger.Error("get profile", zap.String("user_id", userID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

// StreakCheckin handles POST /gaming/streak/checkin
func (h *Handler) StreakCheckin(w http.ResponseWriter, r *http.Request) {
	var req model.StreakCheckinRequest
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

	current, longest, reset, err := h.store.StreakCheckin(r.Context(), req.UserID)
	if err != nil {
		h.logger.Error("streak checkin", zap.String("user_id", req.UserID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, model.StreakCheckinResponse{
		CurrentStreak: current,
		LongestStreak: longest,
		Reset:         reset,
	})
}

// LeaderboardUpdate handles POST /gaming/leaderboard/update
func (h *Handler) LeaderboardUpdate(w http.ResponseWriter, r *http.Request) {
	var req model.LeaderboardUpdateRequest
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

	if err := h.store.LeaderboardUpdate(r.Context(), req.UserID, req.XP); err != nil {
		h.logger.Error("leaderboard update", zap.String("user_id", req.UserID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetLeaderboard handles GET /gaming/leaderboard
func (h *Handler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	// caller's user ID for rank lookup (optional — not required for viewing leaderboard)
	callerID := middleware.UserIDFromContext(r.Context())

	top10, userRank, err := h.store.LeaderboardTop10(r.Context(), callerID)
	if err != nil {
		h.logger.Error("get leaderboard", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, model.LeaderboardResponse{
		Top10:    top10,
		UserRank: userRank,
	})
}

// RandomQuote handles GET /gaming/quotes/random
func (h *Handler) RandomQuote(w http.ResponseWriter, r *http.Request) {
	quote, err := h.store.RandomQuote(r.Context())
	if err != nil {
		h.logger.Error("random quote", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, quote)
}

// Health handles GET /health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
