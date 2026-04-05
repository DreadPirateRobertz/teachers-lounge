package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/event"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/handler"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/model"
)

// ── Additional fakes for trigger tests ───────────────────────────────────────

// fakeLimiter controls whether Allow returns true or false.
type fakeLimiter struct{ allow bool }

// Allow returns the configured value, satisfying middleware.Limiter.
func (f *fakeLimiter) Allow(_ context.Context, _ string) (bool, error) { return f.allow, nil }

// fakeEmailer records Send calls.
type fakeEmailer struct {
	calls []emailCall
	err   error
}

type emailCall struct{ to, subject string }

// Send records the call and returns the configured error.
func (f *fakeEmailer) Send(_ context.Context, to, subject, _ string) error {
	f.calls = append(f.calls, emailCall{to: to, subject: subject})
	return f.err
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func triggerBody(evType event.Type, userID, toEmail string) *bytes.Buffer {
	b, _ := json.Marshal(model.TriggerRequest{
		EventType: string(evType),
		UserID:    userID,
		ToEmail:   toEmail,
	})
	return bytes.NewBuffer(b)
}

func newFullHandler(s *fakeStore, p *fakePusher, e *fakeEmailer, l *fakeLimiter) *handler.Handler {
	return handler.New(s, p, e, l, zap.NewNop())
}

// ── Trigger tests ─────────────────────────────────────────────────────────────

func TestTrigger_ValidRequest_SendsPushAndEmail(t *testing.T) {
	s := &fakeStore{pushTokens: map[string][]string{"u1": {"tok-abc"}}}
	p := &fakePusher{}
	e := &fakeEmailer{}
	h := newFullHandler(s, p, e, &fakeLimiter{allow: true})

	req := httptest.NewRequest(http.MethodPost, "/notify/trigger",
		triggerBody(event.StreakAtRisk, "u1", "student@example.com"))
	rr := httptest.NewRecorder()

	h.Trigger(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(p.calls) != 1 {
		t.Fatalf("expected 1 push send, got %d", len(p.calls))
	}
	if len(e.calls) != 1 {
		t.Fatalf("expected 1 email send, got %d", len(e.calls))
	}
	if e.calls[0].to != "student@example.com" {
		t.Fatalf("email sent to wrong address: %q", e.calls[0].to)
	}
}

func TestTrigger_RateLimited_Returns429(t *testing.T) {
	s := &fakeStore{pushTokens: map[string][]string{"u1": {"tok"}}}
	h := newFullHandler(s, &fakePusher{}, &fakeEmailer{}, &fakeLimiter{allow: false})

	req := httptest.NewRequest(http.MethodPost, "/notify/trigger",
		triggerBody(event.StreakAtRisk, "u1", "x@x.com"))
	rr := httptest.NewRecorder()

	h.Trigger(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rr.Code)
	}
}

func TestTrigger_MissingUserID_Returns400(t *testing.T) {
	h := newFullHandler(&fakeStore{}, &fakePusher{}, &fakeEmailer{}, &fakeLimiter{allow: true})

	b, _ := json.Marshal(model.TriggerRequest{EventType: string(event.StreakAtRisk)})
	req := httptest.NewRequest(http.MethodPost, "/notify/trigger", bytes.NewBuffer(b))
	rr := httptest.NewRecorder()

	h.Trigger(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestTrigger_MissingEventType_Returns400(t *testing.T) {
	h := newFullHandler(&fakeStore{}, &fakePusher{}, &fakeEmailer{}, &fakeLimiter{allow: true})

	b, _ := json.Marshal(model.TriggerRequest{UserID: "u1"})
	req := httptest.NewRequest(http.MethodPost, "/notify/trigger", bytes.NewBuffer(b))
	rr := httptest.NewRecorder()

	h.Trigger(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestTrigger_NoTokens_EmailStillSent(t *testing.T) {
	s := &fakeStore{pushTokens: map[string][]string{}} // no tokens
	p := &fakePusher{}
	e := &fakeEmailer{}
	h := newFullHandler(s, p, e, &fakeLimiter{allow: true})

	req := httptest.NewRequest(http.MethodPost, "/notify/trigger",
		triggerBody(event.RivalPassed, "u1", "student@example.com"))
	rr := httptest.NewRecorder()

	h.Trigger(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(p.calls) != 0 {
		t.Fatalf("expected 0 push sends (no tokens), got %d", len(p.calls))
	}
	if len(e.calls) != 1 {
		t.Fatalf("expected 1 email send, got %d", len(e.calls))
	}
}

func TestTrigger_NoToEmail_SkipsEmail(t *testing.T) {
	s := &fakeStore{pushTokens: map[string][]string{"u1": {"tok"}}}
	p := &fakePusher{}
	e := &fakeEmailer{}
	h := newFullHandler(s, p, e, &fakeLimiter{allow: true})

	req := httptest.NewRequest(http.MethodPost, "/notify/trigger",
		triggerBody(event.BossUnlock, "u1", "")) // no to_email
	rr := httptest.NewRecorder()

	h.Trigger(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(e.calls) != 0 {
		t.Fatalf("expected 0 email sends when to_email empty, got %d", len(e.calls))
	}
}

func TestTrigger_EmailFailure_Returns200(t *testing.T) {
	// Email errors must not fail the HTTP response — push and in-app are more
	// important; email delivery failure is logged and ignored.
	s := &fakeStore{pushTokens: map[string][]string{"u1": {"tok"}}}
	e := &fakeEmailer{err: errors.New("sendgrid down")}
	h := newFullHandler(s, &fakePusher{}, e, &fakeLimiter{allow: true})

	req := httptest.NewRequest(http.MethodPost, "/notify/trigger",
		triggerBody(event.AchievementNearMiss, "u1", "student@example.com"))
	rr := httptest.NewRecorder()

	h.Trigger(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 even on email failure, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestTrigger_ResponseIncludesDeliveryStats(t *testing.T) {
	s := &fakeStore{pushTokens: map[string][]string{"u1": {"tok-a", "tok-b"}}}
	h := newFullHandler(s, &fakePusher{}, &fakeEmailer{}, &fakeLimiter{allow: true})

	req := httptest.NewRequest(http.MethodPost, "/notify/trigger",
		triggerBody(event.QuizCountdown, "u1", "s@example.com"))
	rr := httptest.NewRecorder()

	h.Trigger(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp model.TriggerResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.PushSent != 2 {
		t.Errorf("expected PushSent=2 (2 tokens), got %d", resp.PushSent)
	}
	if !resp.EmailSent {
		t.Error("expected EmailSent=true")
	}
}
