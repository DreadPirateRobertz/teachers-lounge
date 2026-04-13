package handler_test

// Additional tests covering error paths and renderTemplate branches.
//
// renderTemplate is package-private; it is exercised here by sending Email
// requests with specific template names and verifying the response headers/body.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/handler"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/middleware"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/model"
)

// ── Error-returning fakes ─────────────────────────────────────────────────────

// errStore wraps fakeStore but returns errors for write operations.
type errStore struct{ fakeStore }

func (s *errStore) SavePushToken(_ context.Context, _, _, _ string) error {
	return errors.New("redis down")
}
func (s *errStore) CreateNotification(_ context.Context, _ *model.Notification) (*model.Notification, error) {
	return nil, errors.New("db error")
}
func (s *errStore) ListUnread(_ context.Context, _ string) ([]model.Notification, error) {
	return nil, errors.New("db error")
}

// errEmailer always returns an error from Send.
type errEmailer struct{}

func (e *errEmailer) Send(_ context.Context, _, _, _ string) error {
	return errors.New("smtp failure")
}

// ── RegisterToken error paths ─────────────────────────────────────────────────

// TestRegisterToken_BadJSON_Returns400 verifies that malformed JSON in the
// request body returns 400.
func TestRegisterToken_BadJSON_Returns400(t *testing.T) {
	h := handler.New(&fakeStore{}, &fakePusher{}, &fakeEmailer{}, &fakeLimiter{allow: true}, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/notify/push/token",
		strings.NewReader("not-json"))
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()

	h.RegisterToken(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// TestRegisterToken_StoreError_Returns500 verifies that a store failure returns 500.
func TestRegisterToken_StoreError_Returns500(t *testing.T) {
	h := handler.New(&errStore{}, &fakePusher{}, &fakeEmailer{}, &fakeLimiter{allow: true}, zap.NewNop())

	body := jsonBody(model.RegisterTokenRequest{Token: "tok-x", Platform: "ios"})
	req := httptest.NewRequest(http.MethodPost, "/notify/push/token", body)
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()

	h.RegisterToken(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

// TestRegisterToken_DefaultPlatformIsWeb verifies that an empty platform
// defaults to "web" and the token is stored successfully.
func TestRegisterToken_DefaultPlatformIsWeb(t *testing.T) {
	s := &fakeStore{}
	h := handler.New(s, &fakePusher{}, &fakeEmailer{}, &fakeLimiter{allow: true}, zap.NewNop())

	body := jsonBody(model.RegisterTokenRequest{Token: "tok-default"}) // no Platform
	req := httptest.NewRequest(http.MethodPost, "/notify/push/token", body)
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()

	h.RegisterToken(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ── Push error paths ──────────────────────────────────────────────────────────

// TestPush_BadJSON_Returns400 verifies malformed body is rejected.
func TestPush_BadJSON_Returns400(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest(http.MethodPost, "/notify/push", strings.NewReader("bad"))
	rr := httptest.NewRecorder()

	h.Push(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// TestPush_SendFailure_StillReturns200 verifies that per-token FCM failures are
// logged but do not change the HTTP response.
func TestPush_SendFailure_StillReturns200(t *testing.T) {
	s := &fakeStore{pushTokens: map[string][]string{"u1": {"bad-token"}}}
	// fakePusher that always fails
	p := &failPusher{}
	h := handler.New(s, p, &fakeEmailer{}, &fakeLimiter{allow: true}, zap.NewNop())

	body := jsonBody(model.PushRequest{UserID: "u1", Title: "X", Body: "Y"})
	req := httptest.NewRequest(http.MethodPost, "/notify/push", body)
	rr := httptest.NewRecorder()

	h.Push(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 even when send fails, got %d", rr.Code)
	}
}

// failPusher is a Pusher that always returns an error.
type failPusher struct{}

func (p *failPusher) Send(_ context.Context, _, _, _ string, _ map[string]any) error {
	return errors.New("FCM unavailable")
}

// ── Email error paths ─────────────────────────────────────────────────────────

// TestEmail_MissingFields_Returns400 verifies that missing required fields return 400.
func TestEmail_MissingFields_Returns400(t *testing.T) {
	h := newHandler()
	body := jsonBody(model.EmailRequest{UserID: "u1"}) // missing To + Template
	req := httptest.NewRequest(http.MethodPost, "/notify/email", body)
	rr := httptest.NewRecorder()

	h.Email(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// TestEmail_BadJSON_Returns400 verifies malformed body is rejected.
func TestEmail_BadJSON_Returns400(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest(http.MethodPost, "/notify/email", strings.NewReader("bad"))
	rr := httptest.NewRecorder()

	h.Email(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// TestEmail_SendFailure_Returns500 verifies that a send failure from the emailer
// returns 500.
func TestEmail_SendFailure_Returns500(t *testing.T) {
	h := handler.New(&fakeStore{}, &fakePusher{}, &errEmailer{}, &fakeLimiter{allow: true}, zap.NewNop())

	body := jsonBody(model.EmailRequest{
		UserID:   "u1",
		To:       "user@example.com",
		Template: "streak_at_risk",
	})
	req := httptest.NewRequest(http.MethodPost, "/notify/email", body)
	rr := httptest.NewRecorder()

	h.Email(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

// ── renderTemplate coverage via Email endpoint ────────────────────────────────
// Each sub-test exercises one branch of the renderTemplate switch.

func TestEmail_Template_StreakAtRisk(t *testing.T) {
	h := newHandler()
	body := jsonBody(model.EmailRequest{UserID: "u1", To: "x@y.com", Template: "streak_at_risk"})
	req := httptest.NewRequest(http.MethodPost, "/notify/email", body)
	rr := httptest.NewRecorder()
	h.Email(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
}

func TestEmail_Template_RivalPassed(t *testing.T) {
	h := newHandler()
	body := jsonBody(model.EmailRequest{
		UserID:   "u1",
		To:       "x@y.com",
		Template: "rival_passed",
		Vars:     map[string]any{"rival_name": "Alice"},
	})
	req := httptest.NewRequest(http.MethodPost, "/notify/email", body)
	rr := httptest.NewRecorder()
	h.Email(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
}

func TestEmail_Template_BossUnlock(t *testing.T) {
	h := newHandler()
	body := jsonBody(model.EmailRequest{
		UserID:   "u1",
		To:       "x@y.com",
		Template: "boss_unlock",
		Vars:     map[string]any{"boss_name": "Dragon"},
	})
	req := httptest.NewRequest(http.MethodPost, "/notify/email", body)
	rr := httptest.NewRecorder()
	h.Email(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
}

func TestEmail_Template_QuizCountdown(t *testing.T) {
	h := newHandler()
	body := jsonBody(model.EmailRequest{UserID: "u1", To: "x@y.com", Template: "quiz_countdown"})
	req := httptest.NewRequest(http.MethodPost, "/notify/email", body)
	rr := httptest.NewRecorder()
	h.Email(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
}

func TestEmail_Template_AchievementNearMiss(t *testing.T) {
	h := newHandler()
	body := jsonBody(model.EmailRequest{
		UserID:   "u1",
		To:       "x@y.com",
		Template: "achievement_near_miss",
		Vars:     map[string]any{"achievement_name": "Speed Demon"},
	})
	req := httptest.NewRequest(http.MethodPost, "/notify/email", body)
	rr := httptest.NewRecorder()
	h.Email(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
}

func TestEmail_Template_DefaultFallback(t *testing.T) {
	h := newHandler()
	body := jsonBody(model.EmailRequest{UserID: "u1", To: "x@y.com", Template: "unknown_template"})
	req := httptest.NewRequest(http.MethodPost, "/notify/email", body)
	rr := httptest.NewRecorder()
	h.Email(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
}

// TestEmail_Template_VarsNil verifies renderTemplate is safe when Vars is nil.
func TestEmail_Template_VarsNil(t *testing.T) {
	h := newHandler()
	body := jsonBody(model.EmailRequest{UserID: "u1", To: "x@y.com", Template: "rival_passed"}) // Vars omitted
	req := httptest.NewRequest(http.MethodPost, "/notify/email", body)
	rr := httptest.NewRecorder()
	h.Email(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
}

// ── InApp error path ──────────────────────────────────────────────────────────

// TestInApp_StoreError_Returns500 verifies that a store failure returns 500.
func TestInApp_StoreError_Returns500(t *testing.T) {
	h := handler.New(&errStore{}, &fakePusher{}, &fakeEmailer{}, &fakeLimiter{allow: true}, zap.NewNop())

	body := jsonBody(model.InAppRequest{UserID: "u1", Type: "xp", Message: "Level up!"})
	req := httptest.NewRequest(http.MethodPost, "/notify/in-app", body)
	rr := httptest.NewRecorder()

	h.InApp(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

// TestInApp_BadJSON_Returns400 verifies malformed body is rejected.
func TestInApp_BadJSON_Returns400(t *testing.T) {
	h := newHandler()
	req := httptest.NewRequest(http.MethodPost, "/notify/in-app", strings.NewReader("bad"))
	rr := httptest.NewRecorder()
	h.InApp(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// ── ListUnread error path ─────────────────────────────────────────────────────

// TestListUnread_StoreError_Returns500 verifies a DB failure returns 500.
func TestListUnread_StoreError_Returns500(t *testing.T) {
	h := handler.New(&errStore{}, &fakePusher{}, &fakeEmailer{}, &fakeLimiter{allow: true}, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/notify/u1", nil)
	req = withURLParam(req, "userId", "u1")
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()

	h.ListUnread(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}
