package handler

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/event"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/model"
)

// Trigger handles POST /notify/trigger.
// Called by internal game services (gaming-service, tutoring-service) to fire
// contextual notifications when game events occur. Enforces the per-user daily
// push rate limit (3/day), then fans out to push, email, and in-app channels.
// Channel errors are logged but never fail the HTTP response — partial delivery
// is expected and acceptable.
func (h *Handler) Trigger(w http.ResponseWriter, r *http.Request) {
	var req model.TriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.UserID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	if req.EventType == "" {
		http.Error(w, "event_type is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Enforce daily push rate limit before attempting any delivery.
	allowed, err := h.limiter.Allow(ctx, req.UserID)
	if err != nil {
		// Fail open on Redis errors — a limiter outage must not silence all
		// event-driven notifications.
		h.logger.Warn("rate limiter error — failing open", zap.String("user_id", req.UserID), zap.Error(err))
	} else if !allowed {
		w.Header().Set("Retry-After", "86400")
		w.Header().Set("X-RateLimit-Limit", "3")
		w.Header().Set("X-RateLimit-Remaining", "0")
		http.Error(w, "push rate limit exceeded (3 per 24h)", http.StatusTooManyRequests)
		return
	}

	pushContent, emailContent := event.Content(event.Type(req.EventType), req.Payload)

	var resp model.TriggerResponse

	// ── Push ──────────────────────────────────────────────────────────────────
	tokens, err := h.store.GetPushTokens(ctx, req.UserID)
	if err != nil {
		h.logger.Warn("get push tokens for trigger", zap.String("user_id", req.UserID), zap.Error(err))
	}
	for _, token := range tokens {
		if sendErr := h.pusher.Send(ctx, token, pushContent.Title, pushContent.Body, nil); sendErr != nil {
			h.logger.Warn("push send failed in trigger",
				zap.String("user_id", req.UserID),
				zap.Error(sendErr),
			)
			continue
		}
		resp.PushSent++
	}

	// ── Email ─────────────────────────────────────────────────────────────────
	if req.ToEmail != "" {
		if sendErr := h.emailer.Send(ctx, req.ToEmail, emailContent.Subject, emailContent.HTMLBody); sendErr != nil {
			h.logger.Warn("email send failed in trigger",
				zap.String("user_id", req.UserID),
				zap.String("to", req.ToEmail),
				zap.Error(sendErr),
			)
		} else {
			resp.EmailSent = true
		}
	}

	// ── In-app ────────────────────────────────────────────────────────────────
	_, inAppErr := h.store.CreateNotification(ctx, &model.Notification{
		UserID:  req.UserID,
		Type:    req.EventType,
		Message: pushContent.Body,
	})
	if inAppErr != nil {
		h.logger.Warn("create in-app notification for trigger",
			zap.String("user_id", req.UserID),
			zap.Error(inAppErr),
		)
	} else {
		resp.InAppSent = true
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("encode trigger response", zap.Error(err))
	}
}
