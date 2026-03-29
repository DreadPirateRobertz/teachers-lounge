package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/model"
)

// NotifStore is the subset of store.Store the handler depends on.
type NotifStore interface {
	CreateNotification(ctx context.Context, n *model.Notification) (*model.Notification, error)
	ListUnread(ctx context.Context, userID string) ([]model.Notification, error)
}

// Handler holds dependencies for all HTTP handlers.
type Handler struct {
	store  NotifStore
	logger *zap.Logger
}

// New returns a configured Handler.
func New(store NotifStore, logger *zap.Logger) *Handler {
	return &Handler{store: store, logger: logger}
}

// Push handles POST /notify/push.
// Phase 1 stub: validates request, enforces rate limit (via middleware), logs, returns 200.
// Real FCM integration is Phase 8.
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

	h.logger.Info("push notification stub",
		zap.String("user_id", req.UserID),
		zap.String("title", req.Title),
		zap.String("body", req.Body),
	)

	// TODO(Phase 8): send via FCM
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "queued"})
}

// Email handles POST /notify/email.
// Phase 1 stub: validates request, logs, returns 200.
// Real SendGrid/Resend integration is Phase 8.
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

	h.logger.Info("email notification stub",
		zap.String("user_id", req.UserID),
		zap.String("template", req.Template),
	)

	// TODO(Phase 8): send via SendGrid/Resend
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "queued"})
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
	json.NewEncoder(w).Encode(n)
}

// ListUnread handles GET /notify/{userId}.
// Returns the list of unread in-app notifications for the user.
func (h *Handler) ListUnread(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	if userID == "" {
		http.Error(w, "userId path param required", http.StatusBadRequest)
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
	json.NewEncoder(w).Encode(notifications)
}

// Health handles GET /health.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
