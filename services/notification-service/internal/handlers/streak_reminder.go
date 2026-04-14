package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/model"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/push"
)

// Streak reminder cron parameters.
//
// The window is deliberately narrow (20–24h since last study) so a single
// daily or hourly cron tick produces at most one reminder per user before the
// 24h streak-loss cutoff.
const (
	streakMinAgeHours = 20
	streakMaxAgeHours = 24
	streakPushTitle   = "Don't break your streak!"
	streakPushBody    = "Log in and answer a few questions before your streak resets."
)

// StreakReminderStore is the subset of store.Store the StreakReminder handler
// depends on. Defined here so tests can substitute an in-memory fake without
// spinning up a real Postgres pool.
type StreakReminderStore interface {
	GetUsersAtRiskOfStreakLoss(ctx context.Context, minAgeHours, maxAgeHours int) ([]model.UserAtRisk, error)
	GetPushTokens(ctx context.Context, userID string) ([]string, error)
}

// StreakReminderHandler serves POST /internal/notify/streak-reminder.
//
// It is cron-triggered (no user auth context) and network-scoped to the
// internal router, matching the security posture of BossUnlock.
type StreakReminderHandler struct {
	store  StreakReminderStore
	pusher push.Pusher
	logger *zap.Logger
}

// NewStreakReminderHandler returns a configured StreakReminderHandler.
//
// store sources the at-risk user set and their registered FCM tokens.
// pusher delivers reminders; pass push.LogPusher when FCM is not configured.
func NewStreakReminderHandler(store StreakReminderStore, pusher push.Pusher, logger *zap.Logger) *StreakReminderHandler {
	return &StreakReminderHandler{store: store, pusher: pusher, logger: logger}
}

// Serve handles POST /internal/notify/streak-reminder.
//
// For each user whose active streak is about to lapse, it resolves their
// registered FCM device tokens and dispatches a push reminder. Per-token
// delivery errors are counted under Failed but do not short-circuit the
// remaining work — the cron must be able to reach every at-risk user even
// when one FCM send fails transiently.
//
// Returns JSON {at_risk, sent, failed}. The handler returns 500 only on
// catastrophic failures (the at-risk query itself errored); FCM transport
// problems are surfaced via the Failed count.
func (h *StreakReminderHandler) Serve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	atRisk, err := h.store.GetUsersAtRiskOfStreakLoss(ctx, streakMinAgeHours, streakMaxAgeHours)
	if err != nil {
		h.logger.Error("streak-reminder: at-risk query", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := model.StreakReminderResponse{AtRisk: len(atRisk)}
	for _, u := range atRisk {
		tokens, err := h.store.GetPushTokens(ctx, u.UserID)
		if err != nil {
			h.logger.Warn("streak-reminder: get push tokens",
				zap.String("user_id", u.UserID), zap.Error(err))
			continue
		}
		for _, token := range tokens {
			if err := h.pusher.Send(ctx, token, streakPushTitle, streakPushBody, nil); err != nil {
				h.logger.Warn("streak-reminder: push send failed",
					zap.String("user_id", u.UserID), zap.Error(err))
				resp.Failed++
				continue
			}
			resp.Sent++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("streak-reminder: encode response", zap.Error(err))
	}
}
