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

type fakeStore struct {
	notifications []model.Notification
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

// ── Helpers ───────────────────────────────────────────────────────────────────

func newHandler() *handler.Handler {
	return handler.New(&fakeStore{}, zap.NewNop())
}

func jsonBody(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

// ── Tests ─────────────────────────────────────────────────────────────────────

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

func TestEmail_ValidRequest_Returns200(t *testing.T) {
	h := newHandler()
	body := jsonBody(model.EmailRequest{UserID: "u1", Template: "welcome"})
	req := httptest.NewRequest(http.MethodPost, "/notify/email", body)
	rr := httptest.NewRecorder()

	h.Email(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestInApp_ValidRequest_Returns201(t *testing.T) {
	s := &fakeStore{}
	h := handler.New(s, zap.NewNop())

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

func TestListUnread_ReturnsStoredNotifications(t *testing.T) {
	s := &fakeStore{
		notifications: []model.Notification{
			{ID: "n1", UserID: "u1", Type: "xp", Message: "Level up!", Read: false},
			{ID: "n2", UserID: "u2", Type: "xp", Message: "Other user", Read: false},
		},
	}
	h := handler.New(s, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/notify/u1", nil)
	req = withURLParam(req, "userId", "u1")
	// Inject matching caller identity into context
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()

	h.ListUnread(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var result []model.Notification
	json.NewDecoder(rr.Body).Decode(&result)
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
	h := handler.New(s, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/notify/u1", nil)
	req = withURLParam(req, "userId", "u1")
	// Caller is u2 trying to read u1's notifications
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
	// No user ID in context
	rr := httptest.NewRecorder()

	h.ListUnread(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when no caller identity, got %d", rr.Code)
	}
}

func TestHealth_Returns200(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	h.Health(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// withURLParam injects a chi URL param into the request context.
// Avoids pulling in chi as a test dependency for simple URL param reads.
func withURLParam(r *http.Request, key, value string) *http.Request {
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, chiCtx))
}
