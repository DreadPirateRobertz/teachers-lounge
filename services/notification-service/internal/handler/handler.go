package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/middleware"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/model"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/push"
)

// NotifStore is the subset of store.Store the handler depends on.
type NotifStore interface {
	CreateNotification(ctx context.Context, n *model.Notification) (*model.Notification, error)
	ListUnread(ctx context.Context, userID string) ([]model.Notification, error)
	SavePushToken(ctx context.Context, userID, token, platform string) error
	GetPushTokens(ctx context.Context, userID string) ([]string, error)
}

// Handler holds dependencies for all HTTP handlers.
type Handler struct {
	store  NotifStore
	pusher push.Pusher
	logger *zap.Logger
}

// New returns a configured Handler.
// pusher is used by the Push handler to deliver FCM notifications; pass a
// push.LogPusher when FCM credentials are not configured.
func New(store NotifStore, pusher push.Pusher, logger *zap.Logger) *Handler {
	return &Handler{store: store, pusher: pusher, logger: logger}
}

// RegisterToken handles POST /notify/push/token.
// Registers or refreshes a device push token for the authenticated user.
func (h *Handler) RegisterToken(w http.ResponseWriter, r *http.Request) {
	var req model.RegisterTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.UserID == "" || req.Token == "" {
		http.Error(w, "user_id and token are required", http.StatusBadRequest)
		return
	}
	platform := req.Platform
	if platform == "" {
		platform = "web"
	}

	if err := h.store.SavePushToken(r.Context(), req.UserID, req.Token, platform); err != nil {
		h.logger.Error("save push token", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "registered"}); err != nil {
		h.logger.Error("encode register-token response", zap.Error(err))
	}
}

// Push handles POST /notify/push.
// Looks up all FCM device tokens registered for the user and delivers the
// notification to each one via the configured Pusher. Individual token
// failures are logged but do not fail the overall request — partial delivery
// is treated as success at the HTTP layer.
func (h *Handler) Push(w http.ResponseWriter, r *http.Request) {
	var req model.PushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.UserID == "" || req.Title == "" || req.Body == "" {
		http.Error(w, "user_id, title, and body are required", http.StatusBadRequest)
		return
	}

	tokens, err := h.store.GetPushTokens(r.Context(), req.UserID)
	if err != nil {
		h.logger.Warn("get push tokens", zap.String("user_id", req.UserID), zap.Error(err))
		// Fail open — a token-lookup error should not block the response.
	}

	for _, token := range tokens {
		if err := h.pusher.Send(r.Context(), token, req.Title, req.Body, req.Data); err != nil {
			prefix := token
			if len(prefix) > 8 {
				prefix = prefix[:8]
			}
			h.logger.Warn("push send failed",
				zap.String("user_id", req.UserID),
				zap.String("token_prefix", prefix),
				zap.Error(err),
			)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "sent"}); err != nil {
		h.logger.Error("encode push response", zap.Error(err))
	}
}

// Email handles POST /notify/email.
// Phase 1 stub: validates request, logs, returns 200.
// Real SendGrid/Resend integration is a follow-on bead.
func (h *Handler) Email(w http.ResponseWriter, r *http.Request) {
	var req model.EmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.UserID == "" || req.Template == "" {
		http.Error(w, "user_id and template are required", http.StatusBadRequest)
		return
	}

	h.logger.Info("email notification queued",
		zap.String("user_id", req.UserID),
		zap.String("template", req.Template),
	)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "queued"}); err != nil {
		h.logger.Error("encode email response", zap.Error(err))
	}
}

// InApp handles POST /notify/in-app.
// Writes a notification row to Postgres.
func (h *Handler) InApp(w http.ResponseWriter, r *http.Request) {
	var req model.InAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.UserID == "" || req.Type == "" || req.Message == "" {
		http.Error(w, "user_id, type, and message are required", http.StatusBadRequest)
		return
	}

	n, err := h.store.CreateNotification(r.Context(), &model.Notification{
		UserID:  req.UserID,
		Type:    req.Type,
		Message: req.Message,
	})
	if err != nil {
		h.logger.Error("create in-app notification", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(n); err != nil {
		h.logger.Error("encode in-app response", zap.Error(err))
	}
}

// ListUnread handles GET /notify/{userId}.
// Returns unread in-app notifications for the authenticated user.
// The caller may only fetch their own notifications — path userId must match
// the identity in the request context (set by auth middleware).
func (h *Handler) ListUnread(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	if userID == "" {
		http.Error(w, "userId path param required", http.StatusBadRequest)
		return
	}

	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" || callerID != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	notifications, err := h.store.ListUnread(r.Context(), userID)
	if err != nil {
		h.logger.Error("list unread notifications", zap.Error(err), zap.String("user_id", userID))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if notifications == nil {
		notifications = []model.Notification{}
	}
	if err := json.NewEncoder(w).Encode(notifications); err != nil {
		h.logger.Error("encode list-unread response", zap.Error(err))
	}
}

// Health handles GET /health.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		h.logger.Error("encode health response", zap.Error(err))
	}
}
