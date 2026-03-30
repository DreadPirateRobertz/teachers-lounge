package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/teacherslounge/notification-service/internal/middleware"
	"github.com/teacherslounge/notification-service/internal/model"
	"github.com/teacherslounge/notification-service/internal/provider"
)

// mockStore implements Storer for testing.
type mockStore struct {
	notifications []model.Notification
	preferences   map[string]*model.Preferences
	deviceTokens  map[string][]model.DeviceToken
}

func newMockStore() *mockStore {
	return &mockStore{
		preferences:  make(map[string]*model.Preferences),
		deviceTokens: make(map[string][]model.DeviceToken),
	}
}

func (m *mockStore) CreateNotification(_ context.Context, n *model.Notification) error {
	m.notifications = append(m.notifications, *n)
	return nil
}

func (m *mockStore) ListNotifications(_ context.Context, userID string, limit, offset int) ([]model.Notification, error) {
	var result []model.Notification
	for _, n := range m.notifications {
		if n.UserID == userID {
			result = append(result, n)
		}
	}
	if result == nil {
		result = []model.Notification{}
	}
	if offset >= len(result) {
		return []model.Notification{}, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], nil
}

func (m *mockStore) UnreadCount(_ context.Context, userID string) (int, error) {
	count := 0
	for _, n := range m.notifications {
		if n.UserID == userID && n.ReadAt == nil {
			count++
		}
	}
	return count, nil
}

func (m *mockStore) MarkRead(_ context.Context, userID, notificationID string) error {
	now := time.Now()
	for i := range m.notifications {
		if m.notifications[i].ID == notificationID && m.notifications[i].UserID == userID {
			m.notifications[i].ReadAt = &now
			break
		}
	}
	return nil
}

func (m *mockStore) MarkAllRead(_ context.Context, userID string) (int64, error) {
	now := time.Now()
	var count int64
	for i := range m.notifications {
		if m.notifications[i].UserID == userID && m.notifications[i].ReadAt == nil {
			m.notifications[i].ReadAt = &now
			count++
		}
	}
	return count, nil
}

func (m *mockStore) GetPreferences(_ context.Context, userID string) (*model.Preferences, error) {
	if p, ok := m.preferences[userID]; ok {
		return p, nil
	}
	return model.DefaultPreferences(userID), nil
}

func (m *mockStore) UpsertPreferences(_ context.Context, p *model.Preferences) error {
	m.preferences[p.UserID] = p
	return nil
}

func (m *mockStore) SaveDeviceToken(_ context.Context, userID, token, platform string) error {
	m.deviceTokens[userID] = append(m.deviceTokens[userID], model.DeviceToken{
		UserID:   userID,
		Token:    token,
		Platform: platform,
	})
	return nil
}

func (m *mockStore) DeleteDeviceToken(_ context.Context, userID, token string) error {
	tokens := m.deviceTokens[userID]
	for i, dt := range tokens {
		if dt.Token == token {
			m.deviceTokens[userID] = append(tokens[:i], tokens[i+1:]...)
			break
		}
	}
	return nil
}

func (m *mockStore) GetDeviceTokens(_ context.Context, userID string) ([]model.DeviceToken, error) {
	return m.deviceTokens[userID], nil
}

func setupHandler() (*Handler, *mockStore) {
	ms := newMockStore()
	fcm := provider.NewFCMClient("", "")     // disabled
	sg := provider.NewSendGridClient("", "", "") // disabled
	logger, _ := zap.NewDevelopment()
	h := New(ms, fcm, sg, logger)
	return h, ms
}

func authedRequest(method, path string, body any, userID string) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, path, &buf)
	r.Header.Set("Content-Type", "application/json")
	// Inject user ID directly into context (bypass JWT parsing in tests)
	ctx := context.WithValue(r.Context(), middleware.TestCtxKeyUserID(), userID)
	return r.WithContext(ctx)
}

func TestSendNotification_InApp(t *testing.T) {
	h, ms := setupHandler()

	req := model.SendRequest{
		UserID:   "user-1",
		Channels: []model.Channel{model.ChannelInApp},
		Title:    "Test Title",
		Body:     "Test Body",
		Category: "test",
	}

	r := authedRequest(http.MethodPost, "/notifications/send", req, "user-1")
	w := httptest.NewRecorder()
	h.SendNotification(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.SendResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if !resp.Results[0].Success {
		t.Errorf("expected success, got error: %s", resp.Results[0].Error)
	}
	if len(ms.notifications) != 1 {
		t.Errorf("expected 1 stored notification, got %d", len(ms.notifications))
	}
}

func TestListNotifications(t *testing.T) {
	h, ms := setupHandler()

	// Pre-populate
	ms.notifications = []model.Notification{
		{ID: "n1", UserID: "user-1", Title: "First", Body: "body1", CreatedAt: time.Now()},
		{ID: "n2", UserID: "user-1", Title: "Second", Body: "body2", CreatedAt: time.Now()},
		{ID: "n3", UserID: "user-2", Title: "Other", Body: "body3", CreatedAt: time.Now()},
	}

	r := authedRequest(http.MethodGet, "/notifications?limit=10", nil, "user-1")
	w := httptest.NewRecorder()
	h.ListNotifications(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.NotificationListResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Notifications) != 2 {
		t.Errorf("expected 2 notifications for user-1, got %d", len(resp.Notifications))
	}
	if resp.UnreadCount != 2 {
		t.Errorf("expected 2 unread, got %d", resp.UnreadCount)
	}
}

func TestMarkRead(t *testing.T) {
	h, ms := setupHandler()

	ms.notifications = []model.Notification{
		{ID: "n1", UserID: "user-1", Title: "Test", Body: "body", CreatedAt: time.Now()},
	}

	// Set up chi context for URL param
	r := authedRequest(http.MethodPatch, "/notifications/n1/read", nil, "user-1")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "n1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.MarkRead(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
	if ms.notifications[0].ReadAt == nil {
		t.Error("expected notification to be marked as read")
	}
}

func TestMarkAllRead(t *testing.T) {
	h, ms := setupHandler()

	ms.notifications = []model.Notification{
		{ID: "n1", UserID: "user-1", Title: "A", Body: "a", CreatedAt: time.Now()},
		{ID: "n2", UserID: "user-1", Title: "B", Body: "b", CreatedAt: time.Now()},
	}

	r := authedRequest(http.MethodPost, "/notifications/read-all", nil, "user-1")
	w := httptest.NewRecorder()
	h.MarkAllRead(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	for _, n := range ms.notifications {
		if n.ReadAt == nil {
			t.Errorf("notification %s should be marked as read", n.ID)
		}
	}
}

func TestGetPreferences_Default(t *testing.T) {
	h, _ := setupHandler()

	r := authedRequest(http.MethodGet, "/notifications/preferences", nil, "user-1")
	w := httptest.NewRecorder()
	h.GetPreferences(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var prefs model.Preferences
	json.NewDecoder(w.Body).Decode(&prefs)

	if !prefs.PushEnabled || !prefs.EmailEnabled || !prefs.InAppEnabled {
		t.Error("expected all channels enabled by default")
	}
}

func TestUpdatePreferences(t *testing.T) {
	h, _ := setupHandler()

	pushOff := false
	req := model.UpdatePreferencesRequest{
		PushEnabled: &pushOff,
	}

	r := authedRequest(http.MethodPut, "/notifications/preferences", req, "user-1")
	w := httptest.NewRecorder()
	h.UpdatePreferences(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var prefs model.Preferences
	json.NewDecoder(w.Body).Decode(&prefs)

	if prefs.PushEnabled {
		t.Error("expected push_enabled to be false")
	}
	if !prefs.EmailEnabled || !prefs.InAppEnabled {
		t.Error("expected email and in_app still enabled")
	}
}

func TestSendNotification_DisabledByPreferences(t *testing.T) {
	h, ms := setupHandler()

	// Disable push for this user
	ms.preferences["user-1"] = &model.Preferences{
		UserID:       "user-1",
		PushEnabled:  false,
		EmailEnabled: true,
		InAppEnabled: true,
	}

	req := model.SendRequest{
		UserID:   "user-1",
		Channels: []model.Channel{model.ChannelPush},
		Title:    "Test",
		Body:     "Body",
	}

	r := authedRequest(http.MethodPost, "/notifications/send", req, "user-1")
	w := httptest.NewRecorder()
	h.SendNotification(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp model.SendResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if resp.Results[0].Success {
		t.Error("expected push to fail due to disabled preferences")
	}
	if resp.Results[0].Error != "disabled by user preferences" {
		t.Errorf("unexpected error: %s", resp.Results[0].Error)
	}
}

func TestRegisterDevice(t *testing.T) {
	h, ms := setupHandler()

	req := model.RegisterTokenRequest{
		Token:    "fcm-token-abc123",
		Platform: "android",
	}

	r := authedRequest(http.MethodPost, "/notifications/devices", req, "user-1")
	w := httptest.NewRecorder()
	h.RegisterDevice(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if len(ms.deviceTokens["user-1"]) != 1 {
		t.Errorf("expected 1 device token, got %d", len(ms.deviceTokens["user-1"]))
	}
}

func TestHealth(t *testing.T) {
	h, _ := setupHandler()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.Health(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
