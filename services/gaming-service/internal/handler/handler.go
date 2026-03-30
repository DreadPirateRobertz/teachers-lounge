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
	GetDailyXP(ctx context.Context, userID string) (int64, error)
	IncrDailyXP(ctx context.Context, userID string, amount int64) (int64, error)
	GetCurrentStreak(ctx context.Context, userID string) (int, error)
	LogXPEvent(ctx context.Context, userID, event string, baseXP, awarded int64, multiplier float64, capped bool) error
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

// AwardXP handles POST /gaming/xp/award — the event-driven XP pipeline.
// It resolves the base XP from the event type, applies streak multipliers,
// enforces the daily cap, updates all state, and returns the full breakdown.
func (h *Handler) AwardXP(w http.ResponseWriter, r *http.Request) {
	var req model.XPAwardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" || req.Event == "" {
		writeError(w, http.StatusBadRequest, "user_id and event required")
		return
	}

	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" || callerID != req.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	eventType := xp.EventType(req.Event)
	if !xp.ValidEvent(eventType) {
		writeError(w, http.StatusBadRequest, "unknown event type")
		return
	}

	ctx := r.Context()

	// Fetch current streak for multiplier calculation.
	streakDays, err := h.store.GetCurrentStreak(ctx, req.UserID)
	if err != nil {
		h.logger.Error("get streak", zap.String("user_id", req.UserID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Fetch daily XP spent so far.
	dailySoFar, err := h.store.GetDailyXP(ctx, req.UserID)
	if err != nil {
		h.logger.Error("get daily xp", zap.String("user_id", req.UserID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Calculate award with multiplier and daily cap.
	awarded, capped := xp.CalculateAward(eventType, streakDays, dailySoFar)
	multiplier := xp.StreakMultiplier(streakDays)
	baseXPVal := xp.BaseXPFor(eventType)

	if awarded > 0 {
		// Update daily XP counter.
		dailyTotal, err := h.store.IncrDailyXP(ctx, req.UserID, awarded)
		if err != nil {
			h.logger.Error("incr daily xp", zap.String("user_id", req.UserID), zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		dailySoFar = dailyTotal

		// Apply XP to profile.
		currentXP, currentLevel, err := h.store.GetXPAndLevel(ctx, req.UserID)
		if err != nil {
			h.logger.Error("get xp/level", zap.String("user_id", req.UserID), zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		newXP, newLevel, leveledUp := xp.Apply(currentXP, currentLevel, awarded)

		if err := h.store.UpsertXP(ctx, req.UserID, newXP, newLevel); err != nil {
			h.logger.Error("upsert xp", zap.String("user_id", req.UserID), zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		// Update leaderboard.
		if err := h.store.LeaderboardUpdate(ctx, req.UserID, newXP); err != nil {
			h.logger.Error("leaderboard update", zap.String("user_id", req.UserID), zap.Error(err))
			// Non-fatal: leaderboard is eventually consistent.
		}

		// Log the event for audit.
		if err := h.store.LogXPEvent(ctx, req.UserID, req.Event, baseXPVal, awarded, multiplier, capped); err != nil {
			h.logger.Error("log xp event", zap.String("user_id", req.UserID), zap.Error(err))
			// Non-fatal: audit logging should not block the response.
		}

		writeJSON(w, http.StatusOK, model.XPAwardResponse{
			Event:      req.Event,
			BaseXP:     baseXPVal,
			Multiplier: multiplier,
			Awarded:    awarded,
			DailyTotal: dailySoFar,
			DailyCap:   xp.DailyCap,
			Capped:     capped,
			NewXP:      newXP,
			NewLevel:   newLevel,
			LevelUp:    leveledUp,
		})
		return
	}

	// Awarded 0 — cap already reached.
	currentXP, currentLevel, _ := h.store.GetXPAndLevel(ctx, req.UserID)
	writeJSON(w, http.StatusOK, model.XPAwardResponse{
		Event:      req.Event,
		BaseXP:     baseXPVal,
		Multiplier: multiplier,
		Awarded:    0,
		DailyTotal: dailySoFar,
		DailyCap:   xp.DailyCap,
		Capped:     true,
		NewXP:      currentXP,
		NewLevel:   currentLevel,
		LevelUp:    false,
	})
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
