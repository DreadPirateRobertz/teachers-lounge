package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/handler"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/middleware"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/model"
)

// ── Fakes ────────────────────────────────────────────────────────────────────

// fakeStore is an in-memory implementation of handler.NotifStore for testing.
type fakeStore struct {
	notifications []model.Notification
	pushTokens    map[string][]string // userID -> []token
}

func (f *fakeStore) CreateNotification(_ context.Context, n *model.Notification) (*model.Notification, error) {
	n.ID = "fake-id-001"
	f.notifications = append(f.notifications, *n)
	return n, nil
}

func (f *fakeStore) ListUnread(_ context.Context, userID string) ([]model.Notification, error) {
	var out []model.Notification
	for _, n := range f.notifications {
		if n.UserID == userID && !n.Read {
			out = append(out, n)
		}
	}
	return out, nil
}

// SavePushToken records the token for the given user.
func (f *fakeStore) SavePushToken(_ context.Context, userID, token, _ string) error {
	if f.pushTokens == nil {
		f.pushTokens = make(map[string][]string)
	}
	for _, t := range f.pushTokens[userID] {
		if t == token {
			return nil // already present — idempotent
		}
	}
	f.pushTokens[userID] = append(f.pushTokens[userID], token)
	return nil
}

// GetPushTokens returns all tokens registered for the given user.
func (f *fakeStore) GetPushTokens(_ context.Context, userID string) ([]string, error) {
	return f.pushTokens[userID], nil
}

// fakePusher records Send calls for assertion in tests.
type fakePusher struct {
	calls []pushCall
}

type pushCall struct {
	token string
	title string
	body  string
}

// Send records the call and returns nil.
func (p *fakePusher) Send(_ context.Context, token, title, body string, _ map[string]any) error {
	p.calls = append(p.calls, pushCall{token: token, title: title, body: body})
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func newHandler() *handler.Handler {
	return handler.New(&fakeStore{}, &fakePusher{}, &fakeEmailer{}, &fakeLimiter{allow: true}, zap.NewNop())
}

func jsonBody(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

// ── Push tests ────────────────────────────────────────────────────────────────

func TestPush_ValidRequest_Returns200(t *testing.T) {
	h := newHandler()
	body := jsonBody(model.PushRequest{UserID: "u1", Title: "Hi", Body: "Test"})
	req := httptest.NewRequest(http.MethodPost, "/notify/push", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Push(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPush_MissingFields_Returns400(t *testing.T) {
	h := newHandler()
	body := jsonBody(model.PushRequest{UserID: "u1"}) // missing Title + Body
	req := httptest.NewRequest(http.MethodPost, "/notify/push", body)
	rr := httptest.NewRecorder()

	h.Push(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestPush_DeliversToRegisteredTokens(t *testing.T) {
	s := &fakeStore{
		pushTokens: map[string][]string{
			"u1": {"tok-a", "tok-b"},
		},
	}
	p := &fakePusher{}
	h := handler.New(s, p, &fakeEmailer{}, &fakeLimiter{allow: true}, zap.NewNop())

	body := jsonBody(model.PushRequest{UserID: "u1", Title: "Streak!", Body: "3 days"})
	req := httptest.NewRequest(http.MethodPost, "/notify/push", body)
	rr := httptest.NewRecorder()

	h.Push(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(p.calls) != 2 {
		t.Fatalf("expected 2 FCM sends, got %d", len(p.calls))
	}
	if p.calls[0].token != "tok-a" || p.calls[1].token != "tok-b" {
		t.Fatalf("unexpected tokens: %+v", p.calls)
	}
}

func TestPush_NoTokens_Returns200WithoutSend(t *testing.T) {
	p := &fakePusher{}
	h := handler.New(&fakeStore{}, p, &fakeEmailer{}, &fakeLimiter{allow: true}, zap.NewNop())

	body := jsonBody(model.PushRequest{UserID: "u1", Title: "Hi", Body: "World"})
	req := httptest.NewRequest(http.MethodPost, "/notify/push", body)
	rr := httptest.NewRecorder()

	h.Push(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if len(p.calls) != 0 {
		t.Fatalf("expected 0 FCM sends when no tokens, got %d", len(p.calls))
	}
}

// ── Email tests ───────────────────────────────────────────────────────────────

func TestEmail_ValidRequest_Returns202(t *testing.T) {
	h := newHandler()
	body := jsonBody(model.EmailRequest{UserID: "u1", To: "test@example.com", Template: "welcome"})
	req := httptest.NewRequest(http.MethodPost, "/notify/email", body)
	rr := httptest.NewRecorder()

	h.Email(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ── In-app tests ──────────────────────────────────────────────────────────────

func TestInApp_ValidRequest_Returns201(t *testing.T) {
	s := &fakeStore{}
	h := handler.New(s, &fakePusher{}, &fakeEmailer{}, &fakeLimiter{allow: true}, zap.NewNop())

	body := jsonBody(model.InAppRequest{UserID: "u1", Type: "streak", Message: "You're on fire!"})
	req := httptest.NewRequest(http.MethodPost, "/notify/in-app", body)
	rr := httptest.NewRecorder()

	h.InApp(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(s.notifications) != 1 {
		t.Fatalf("expected 1 notification stored, got %d", len(s.notifications))
	}
}

func TestInApp_MissingFields_Returns400(t *testing.T) {
	h := newHandler()
	body := jsonBody(model.InAppRequest{UserID: "u1"}) // missing Type + Message
	req := httptest.NewRequest(http.MethodPost, "/notify/in-app", body)
	rr := httptest.NewRecorder()

	h.InApp(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// ── RegisterToken tests ───────────────────────────────────────────────────────

func TestRegisterToken_ValidRequest_Returns204(t *testing.T) {
	s := &fakeStore{}
	h := handler.New(s, &fakePusher{}, &fakeEmailer{}, &fakeLimiter{allow: true}, zap.NewNop())

	body := jsonBody(model.RegisterTokenRequest{Token: "tok-xyz", Platform: "ios"})
	req := httptest.NewRequest(http.MethodPost, "/notify/push/token", body)
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()

	h.RegisterToken(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(s.pushTokens["u1"]) != 1 || s.pushTokens["u1"][0] != "tok-xyz" {
		t.Fatalf("expected token stored, got %v", s.pushTokens)
	}
}

func TestRegisterToken_MissingToken_Returns400(t *testing.T) {
	h := newHandler()
	body := jsonBody(model.RegisterTokenRequest{Platform: "android"}) // missing Token
	req := httptest.NewRequest(http.MethodPost, "/notify/push/token", body)
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()

	h.RegisterToken(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestRegisterToken_NoCallerInContext_Returns401(t *testing.T) {
	h := newHandler()
	body := jsonBody(model.RegisterTokenRequest{Token: "tok-xyz", Platform: "ios"})
	req := httptest.NewRequest(http.MethodPost, "/notify/push/token", body)
	// No user ID in context
	rr := httptest.NewRecorder()

	h.RegisterToken(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestRegisterToken_Idempotent_StoresSameTokenOnce(t *testing.T) {
	s := &fakeStore{}
	h := handler.New(s, &fakePusher{}, &fakeEmailer{}, &fakeLimiter{allow: true}, zap.NewNop())

	reg := func() {
		body := jsonBody(model.RegisterTokenRequest{Token: "tok-dup", Platform: "ios"})
		req := httptest.NewRequest(http.MethodPost, "/notify/push/token", body)
		req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
		rr := httptest.NewRecorder()
		h.RegisterToken(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", rr.Code)
		}
	}

	reg()
	reg()

	if len(s.pushTokens["u1"]) != 1 {
		t.Fatalf("expected token stored once, got %d", len(s.pushTokens["u1"]))
	}
}

// ── ListUnread tests ──────────────────────────────────────────────────────────

func TestListUnread_ReturnsStoredNotifications(t *testing.T) {
	s := &fakeStore{
		notifications: []model.Notification{
			{ID: "n1", UserID: "u1", Type: "xp", Message: "Level up!", Read: false},
			{ID: "n2", UserID: "u2", Type: "xp", Message: "Other user", Read: false},
		},
	}
	h := handler.New(s, &fakePusher{}, &fakeEmailer{}, &fakeLimiter{allow: true}, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/notify/u1", nil)
	req = withURLParam(req, "userId", "u1")
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()

	h.ListUnread(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var result []model.Notification
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 notification for u1, got %d", len(result))
	}
	if result[0].ID != "n1" {
		t.Fatalf("expected n1, got %s", result[0].ID)
	}
}

func TestListUnread_ForbiddenWhenCallerMismatch(t *testing.T) {
	s := &fakeStore{
		notifications: []model.Notification{
			{ID: "n1", UserID: "u1", Type: "xp", Message: "Level up!", Read: false},
		},
	}
	h := handler.New(s, &fakePusher{}, &fakeEmailer{}, &fakeLimiter{allow: true}, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/notify/u1", nil)
	req = withURLParam(req, "userId", "u1")
	req = req.WithContext(middleware.WithUserID(req.Context(), "u2"))
	rr := httptest.NewRecorder()

	h.ListUnread(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestListUnread_ForbiddenWhenNoCallerInContext(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest(http.MethodGet, "/notify/u1", nil)
	req = withURLParam(req, "userId", "u1")
	rr := httptest.NewRecorder()

	h.ListUnread(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when no caller identity, got %d", rr.Code)
	}
}

// ── Health test ───────────────────────────────────────────────────────────────

func TestHealth_Returns200(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	h.Health(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// withURLParam injects a chi URL param into the request context.
func withURLParam(r *http.Request, key, value string) *http.Request {
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, chiCtx))
}
