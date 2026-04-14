package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/model"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/push"
)

// staleFCMErrors contains the FCM error codes that indicate a token has been
// permanently invalidated and should be removed from storage.
var staleFCMErrors = []string{"InvalidRegistration", "NotRegistered"}

// isStaleTokenError reports whether the FCM error message indicates a token
// that can never be delivered to and should be purged from storage.
func isStaleTokenError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, code := range staleFCMErrors {
		if strings.Contains(msg, code) {
			return true
		}
	}
	return false
}

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
	// DeletePushToken purges a stale FCM token returned by FCM as
	// InvalidRegistration or NotRegistered so it is not retried.
	DeletePushToken(ctx context.Context, userID, token string) error
	// UpdateLastStreakReminderAt stamps last_streak_reminder_at to NOW() for
	// the given user after a successful reminder fan-out so subsequent cron
	// runs within the same window are skipped.
	UpdateLastStreakReminderAt(ctx context.Context, userID string) error
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
		sentForUser := 0
		for _, token := range tokens {
			if err := h.pusher.Send(ctx, token, streakPushTitle, streakPushBody, nil); err != nil {
				h.logger.Warn("streak-reminder: push send failed",
					zap.String("user_id", u.UserID), zap.Error(err))
				resp.Failed++
				// Purge tokens that FCM has permanently rejected so they are
				// not retried on the next cron run.
				if isStaleTokenError(err) {
					if delErr := h.store.DeletePushToken(ctx, u.UserID, token); delErr != nil {
						h.logger.Warn("streak-reminder: delete stale token",
							zap.String("user_id", u.UserID), zap.Error(delErr))
					}
				}
				continue
			}
			resp.Sent++
			sentForUser++
		}
		// Stamp last_streak_reminder_at after at least one push was dispatched
		// so the deduplication guard suppresses duplicate reminders when the
		// cron fires again within the same 20–24h window.
		if sentForUser > 0 {
			if stampErr := h.store.UpdateLastStreakReminderAt(ctx, u.UserID); stampErr != nil {
				h.logger.Warn("streak-reminder: stamp last_streak_reminder_at",
					zap.String("user_id", u.UserID), zap.Error(stampErr))
			}
		}
	}

	// Buffer the JSON body before writing any headers so that an encode
	// failure does not leave the client with a 200 status and a broken body.
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(resp); err != nil {
		h.logger.Error("streak-reminder: encode response", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(buf.Bytes())
}
