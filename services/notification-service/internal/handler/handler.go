package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/teacherslounge/notification-service/internal/middleware"
	"github.com/teacherslounge/notification-service/internal/model"
	"github.com/teacherslounge/notification-service/internal/provider"
)

// Storer is the interface the handler needs from the store layer.
type Storer interface {
	CreateNotification(ctx context.Context, n *model.Notification) error
	ListNotifications(ctx context.Context, userID string, limit, offset int) ([]model.Notification, error)
	UnreadCount(ctx context.Context, userID string) (int, error)
	MarkRead(ctx context.Context, userID, notificationID string) error
	MarkAllRead(ctx context.Context, userID string) (int64, error)
	GetPreferences(ctx context.Context, userID string) (*model.Preferences, error)
	UpsertPreferences(ctx context.Context, p *model.Preferences) error
	SaveDeviceToken(ctx context.Context, userID, token, platform string) error
	DeleteDeviceToken(ctx context.Context, userID, token string) error
	GetDeviceTokens(ctx context.Context, userID string) ([]model.DeviceToken, error)
}

// Handler holds dependencies.
type Handler struct {
	store    Storer
	fcm      *provider.FCMClient
	sendgrid *provider.SendGridClient
	logger   *zap.Logger
}

// New creates a Handler.
func New(store Storer, fcm *provider.FCMClient, sg *provider.SendGridClient, logger *zap.Logger) *Handler {
	return &Handler{store: store, fcm: fcm, sendgrid: sg, logger: logger}
}

// SendNotification handles POST /notifications/send.
// Dispatches to requested channels, always stores an in-app notification.
func (h *Handler) SendNotification(w http.ResponseWriter, r *http.Request) {
	var req model.SendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" || req.Title == "" || req.Body == "" {
		writeError(w, http.StatusBadRequest, "user_id, title, and body required")
		return
	}
	if len(req.Channels) == 0 {
		req.Channels = []model.Channel{model.ChannelInApp}
	}

	// Check user preferences
	prefs, err := h.store.GetPreferences(r.Context(), req.UserID)
	if err != nil {
		h.logger.Error("get preferences", zap.String("user_id", req.UserID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var results []model.ChannelResult

	for _, ch := range req.Channels {
		if !h.channelEnabled(prefs, ch, req.Category) {
			results = append(results, model.ChannelResult{
				Channel: ch,
				Success: false,
				Error:   "disabled by user preferences",
			})
			continue
		}

		switch ch {
		case model.ChannelInApp:
			result := h.sendInApp(r.Context(), &req)
			results = append(results, result)

		case model.ChannelPush:
			result := h.sendPush(r.Context(), &req)
			results = append(results, result)

		case model.ChannelEmail:
			result := h.sendEmail(r.Context(), &req)
			results = append(results, result)

		default:
			results = append(results, model.ChannelResult{
				Channel: ch,
				Success: false,
				Error:   "unknown channel",
			})
		}
	}

	writeJSON(w, http.StatusOK, model.SendResponse{Results: results})
}

func (h *Handler) sendInApp(ctx context.Context, req *model.SendRequest) model.ChannelResult {
	n := &model.Notification{
		ID:        generateID(),
		UserID:    req.UserID,
		Channel:   model.ChannelInApp,
		Title:     req.Title,
		Body:      req.Body,
		Category:  req.Category,
		CreatedAt: time.Now().UTC(),
	}
	if err := h.store.CreateNotification(ctx, n); err != nil {
		h.logger.Error("create in-app notification", zap.Error(err))
		return model.ChannelResult{Channel: model.ChannelInApp, Success: false, Error: "store error"}
	}
	return model.ChannelResult{Channel: model.ChannelInApp, Success: true}
}

func (h *Handler) sendPush(ctx context.Context, req *model.SendRequest) model.ChannelResult {
	if !h.fcm.Enabled() {
		return model.ChannelResult{Channel: model.ChannelPush, Success: false, Error: "FCM not configured"}
	}

	tokens, err := h.store.GetDeviceTokens(ctx, req.UserID)
	if err != nil {
		h.logger.Error("get device tokens", zap.String("user_id", req.UserID), zap.Error(err))
		return model.ChannelResult{Channel: model.ChannelPush, Success: false, Error: "failed to get device tokens"}
	}
	if len(tokens) == 0 {
		return model.ChannelResult{Channel: model.ChannelPush, Success: false, Error: "no registered devices"}
	}

	var lastErr error
	sent := 0
	for _, dt := range tokens {
		if err := h.fcm.Send(ctx, dt.Token, req.Title, req.Body, req.Data); err != nil {
			h.logger.Warn("fcm send failed", zap.String("token", dt.Token[:8]+"..."), zap.Error(err))
			lastErr = err
		} else {
			sent++
		}
	}

	if sent == 0 && lastErr != nil {
		return model.ChannelResult{Channel: model.ChannelPush, Success: false, Error: lastErr.Error()}
	}
	return model.ChannelResult{Channel: model.ChannelPush, Success: true}
}

func (h *Handler) sendEmail(ctx context.Context, req *model.SendRequest) model.ChannelResult {
	if !h.sendgrid.Enabled() {
		return model.ChannelResult{Channel: model.ChannelEmail, Success: false, Error: "SendGrid not configured"}
	}

	// Look up user email from JWT claims stored in context, or use the request directly
	// In a real integration, we'd call the user-service to resolve user_id -> email.
	// For now, email must be provided by the caller (internal service-to-service).
	claims := middleware.ClaimsFromContext(ctx)
	var email string
	if claims != nil {
		email = claims.Email
	}
	if email == "" {
		return model.ChannelResult{Channel: model.ChannelEmail, Success: false, Error: "no email address available"}
	}

	subject := req.EmailSubject
	if subject == "" {
		subject = req.Title
	}

	if err := h.sendgrid.Send(ctx, email, subject, req.Body); err != nil {
		h.logger.Error("sendgrid send", zap.Error(err))
		return model.ChannelResult{Channel: model.ChannelEmail, Success: false, Error: "email delivery failed"}
	}
	return model.ChannelResult{Channel: model.ChannelEmail, Success: true}
}

func (h *Handler) channelEnabled(prefs *model.Preferences, ch model.Channel, category string) bool {
	// Check category-level overrides first
	if category != "" && prefs.CategoryOverrides != nil {
		if catOverrides, ok := prefs.CategoryOverrides[category]; ok {
			if enabled, ok := catOverrides[ch]; ok {
				return enabled
			}
		}
	}

	// Fall back to global settings
	switch ch {
	case model.ChannelPush:
		return prefs.PushEnabled
	case model.ChannelEmail:
		return prefs.EmailEnabled
	case model.ChannelInApp:
		return prefs.InAppEnabled
	}
	return true
}

// ListNotifications handles GET /notifications.
func (h *Handler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	if limit > 100 {
		limit = 100
	}

	notifications, err := h.store.ListNotifications(r.Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("list notifications", zap.String("user_id", userID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	unread, err := h.store.UnreadCount(r.Context(), userID)
	if err != nil {
		h.logger.Error("unread count", zap.String("user_id", userID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, model.NotificationListResponse{
		Notifications: notifications,
		UnreadCount:   unread,
	})
}

// MarkRead handles PATCH /notifications/{id}/read.
func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	notifID := chi.URLParam(r, "id")
	if notifID == "" {
		writeError(w, http.StatusBadRequest, "notification id required")
		return
	}

	if err := h.store.MarkRead(r.Context(), userID, notifID); err != nil {
		h.logger.Error("mark read", zap.String("id", notifID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// MarkAllRead handles POST /notifications/read-all.
func (h *Handler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	count, err := h.store.MarkAllRead(r.Context(), userID)
	if err != nil {
		h.logger.Error("mark all read", zap.String("user_id", userID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]int64{"marked": count})
}

// GetPreferences handles GET /notifications/preferences.
func (h *Handler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	prefs, err := h.store.GetPreferences(r.Context(), userID)
	if err != nil {
		h.logger.Error("get preferences", zap.String("user_id", userID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, prefs)
}

// UpdatePreferences handles PUT /notifications/preferences.
func (h *Handler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req model.UpdatePreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Fetch current, apply partial update
	prefs, err := h.store.GetPreferences(r.Context(), userID)
	if err != nil {
		h.logger.Error("get preferences for update", zap.String("user_id", userID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if req.PushEnabled != nil {
		prefs.PushEnabled = *req.PushEnabled
	}
	if req.EmailEnabled != nil {
		prefs.EmailEnabled = *req.EmailEnabled
	}
	if req.InAppEnabled != nil {
		prefs.InAppEnabled = *req.InAppEnabled
	}
	if req.CategoryOverrides != nil {
		prefs.CategoryOverrides = req.CategoryOverrides
	}

	if err := h.store.UpsertPreferences(r.Context(), prefs); err != nil {
		h.logger.Error("upsert preferences", zap.String("user_id", userID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, prefs)
}

// RegisterDevice handles POST /notifications/devices.
func (h *Handler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req model.RegisterTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" || req.Platform == "" {
		writeError(w, http.StatusBadRequest, "token and platform required")
		return
	}

	if err := h.store.SaveDeviceToken(r.Context(), userID, req.Token, req.Platform); err != nil {
		h.logger.Error("save device token", zap.String("user_id", userID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "registered"})
}

// UnregisterDevice handles DELETE /notifications/devices/{token}.
func (h *Handler) UnregisterDevice(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	token := chi.URLParam(r, "token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "token required")
		return
	}

	if err := h.store.DeleteDeviceToken(r.Context(), userID, token); err != nil {
		h.logger.Error("delete device token", zap.String("user_id", userID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Health handles GET /health.
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

func queryInt(r *http.Request, key string, fallback int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
