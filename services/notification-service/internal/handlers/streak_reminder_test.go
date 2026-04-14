package handlers_test

// Unit tests for StreakReminderHandler (tl-wti).
//
// These exercise the handler against an in-memory store fake and an
// in-memory Pusher that captures every FCM call. The real Postgres path
// is covered by the store integration tests; the real FCM path is
// covered by push/push_test.go.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"go.uber.org/zap"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/handlers"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/model"
)

// ── fakes ────────────────────────────────────────────────────────────────────

// fakeStreakStore is an in-memory StreakReminderStore for handler tests.
type fakeStreakStore struct {
	atRisk        []model.UserAtRisk
	atRiskErr     error
	gotMinAge     int
	gotMaxAge     int
	tokens        map[string][]string
	tokensErr     map[string]error
	tokensCall    int
	deletedTokens []string // tokens purged via DeletePushToken
	stampedUsers  []string // users stamped via UpdateLastStreakReminderAt
}

func (f *fakeStreakStore) GetUsersAtRiskOfStreakLoss(_ context.Context, minAgeHours, maxAgeHours int) ([]model.UserAtRisk, error) {
	f.gotMinAge = minAgeHours
	f.gotMaxAge = maxAgeHours
	return f.atRisk, f.atRiskErr
}

func (f *fakeStreakStore) GetPushTokens(_ context.Context, userID string) ([]string, error) {
	f.tokensCall++
	if err := f.tokensErr[userID]; err != nil {
		return nil, err
	}
	return f.tokens[userID], nil
}

// DeletePushToken records the token as deleted; no actual storage mutation.
func (f *fakeStreakStore) DeletePushToken(_ context.Context, _, token string) error {
	f.deletedTokens = append(f.deletedTokens, token)
	return nil
}

// UpdateLastStreakReminderAt records which users had their timestamp stamped.
func (f *fakeStreakStore) UpdateLastStreakReminderAt(_ context.Context, userID string) error {
	f.stampedUsers = append(f.stampedUsers, userID)
	return nil
}

// capturePusher records every Send invocation. failOn returns an error when
// the target token matches, simulating FCM per-token failures.
type capturePusher struct {
	mu     sync.Mutex
	calls  []pushCall
	failOn map[string]error
}

type pushCall struct {
	token, title, body string
}

func (p *capturePusher) Send(_ context.Context, token, title, body string, _ map[string]any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, pushCall{token: token, title: title, body: body})
	if err := p.failOn[token]; err != nil {
		return err
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func newStreakRequest() (*httptest.ResponseRecorder, *http.Request) {
	return httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/internal/notify/streak-reminder", nil)
}

func decodeStreakResponse(t *testing.T, rr *httptest.ResponseRecorder) model.StreakReminderResponse {
	t.Helper()
	var out model.StreakReminderResponse
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

// ── tests ────────────────────────────────────────────────────────────────────

// TestStreakReminder_PassesCronWindow verifies the handler forwards the
// pre-lapse 20h/24h window to the store query.
func TestStreakReminder_PassesCronWindow(t *testing.T) {
	store := &fakeStreakStore{}
	h := handlers.NewStreakReminderHandler(store, &capturePusher{}, zap.NewNop())

	rr, req := newStreakRequest()
	h.Serve(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if store.gotMinAge != 20 || store.gotMaxAge != 24 {
		t.Errorf("window = (%d,%d), want (20,24)", store.gotMinAge, store.gotMaxAge)
	}
}

// TestStreakReminder_NoAtRiskUsers_ReturnsZeroCounts verifies an empty
// at-risk set returns 200 with zeroed counts and no FCM traffic.
func TestStreakReminder_NoAtRiskUsers_ReturnsZeroCounts(t *testing.T) {
	store := &fakeStreakStore{atRisk: nil}
	pusher := &capturePusher{}
	h := handlers.NewStreakReminderHandler(store, pusher, zap.NewNop())

	rr, req := newStreakRequest()
	h.Serve(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	got := decodeStreakResponse(t, rr)
	if got.AtRisk != 0 || got.Sent != 0 || got.Failed != 0 {
		t.Errorf("counts = %+v, want all zero", got)
	}
	if len(pusher.calls) != 0 {
		t.Errorf("expected no FCM calls, got %d", len(pusher.calls))
	}
}

// TestStreakReminder_FansOutToAllTokens verifies one push per device token
// per at-risk user, and that Sent reflects successful deliveries.
func TestStreakReminder_FansOutToAllTokens(t *testing.T) {
	store := &fakeStreakStore{
		atRisk: []model.UserAtRisk{
			{UserID: "u1", CurrentStreak: 7},
			{UserID: "u2", CurrentStreak: 3},
		},
		tokens: map[string][]string{
			"u1": {"tok-u1-a", "tok-u1-b"},
			"u2": {"tok-u2-a"},
		},
	}
	pusher := &capturePusher{}
	h := handlers.NewStreakReminderHandler(store, pusher, zap.NewNop())

	rr, req := newStreakRequest()
	h.Serve(rr, req)

	got := decodeStreakResponse(t, rr)
	if got.AtRisk != 2 {
		t.Errorf("AtRisk = %d, want 2", got.AtRisk)
	}
	if got.Sent != 3 {
		t.Errorf("Sent = %d, want 3", got.Sent)
	}
	if got.Failed != 0 {
		t.Errorf("Failed = %d, want 0", got.Failed)
	}
	if len(pusher.calls) != 3 {
		t.Fatalf("expected 3 FCM calls, got %d", len(pusher.calls))
	}
	// Title/body should match the streak reminder copy on every call.
	for _, c := range pusher.calls {
		if c.title == "" || c.body == "" {
			t.Errorf("empty title/body on push call: %+v", c)
		}
	}
}

// TestStreakReminder_PerTokenFCMErrorIsCountedNotFatal verifies that a
// failing FCM send increments Failed but does not short-circuit the loop
// or turn into a 500 — the cron must keep reaching other users.
func TestStreakReminder_PerTokenFCMErrorIsCountedNotFatal(t *testing.T) {
	store := &fakeStreakStore{
		atRisk: []model.UserAtRisk{{UserID: "u1", CurrentStreak: 5}, {UserID: "u2", CurrentStreak: 2}},
		tokens: map[string][]string{
			"u1": {"bad-token", "good-token"},
			"u2": {"good-token-2"},
		},
	}
	pusher := &capturePusher{failOn: map[string]error{"bad-token": errors.New("InvalidRegistration")}}
	h := handlers.NewStreakReminderHandler(store, pusher, zap.NewNop())

	rr, req := newStreakRequest()
	h.Serve(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 despite FCM failure, got %d", rr.Code)
	}
	got := decodeStreakResponse(t, rr)
	if got.AtRisk != 2 {
		t.Errorf("AtRisk = %d, want 2", got.AtRisk)
	}
	if got.Sent != 2 {
		t.Errorf("Sent = %d, want 2", got.Sent)
	}
	if got.Failed != 1 {
		t.Errorf("Failed = %d, want 1", got.Failed)
	}
}

// TestStreakReminder_StoreErrorReturns500 verifies a catastrophic at-risk
// query failure surfaces as 500 (distinct from per-token FCM errors).
func TestStreakReminder_StoreErrorReturns500(t *testing.T) {
	store := &fakeStreakStore{atRiskErr: errors.New("db down")}
	h := handlers.NewStreakReminderHandler(store, &capturePusher{}, zap.NewNop())

	rr, req := newStreakRequest()
	h.Serve(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

// TestStreakReminder_TokenLookupErrorIsSkippedNotFatal verifies that a
// per-user token-lookup failure is logged and skipped, not fatal. Other
// at-risk users must still be reached.
func TestStreakReminder_TokenLookupErrorIsSkippedNotFatal(t *testing.T) {
	store := &fakeStreakStore{
		atRisk: []model.UserAtRisk{{UserID: "broken"}, {UserID: "ok"}},
		tokens: map[string][]string{"ok": {"tok-ok"}},
		tokensErr: map[string]error{
			"broken": errors.New("token query timed out"),
		},
	}
	pusher := &capturePusher{}
	h := handlers.NewStreakReminderHandler(store, pusher, zap.NewNop())

	rr, req := newStreakRequest()
	h.Serve(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	got := decodeStreakResponse(t, rr)
	if got.AtRisk != 2 {
		t.Errorf("AtRisk = %d, want 2", got.AtRisk)
	}
	if got.Sent != 1 {
		t.Errorf("Sent = %d, want 1 (ok-user only)", got.Sent)
	}
}

// TestStreakReminder_StampsLastReminderAtBeforeSend verifies that when a user
// has registered push tokens, last_streak_reminder_at is stamped before the
// fan-out (not after), so FCM outages don't cause repeat hammering. Users with
// no registered tokens are not stamped — they remain eligible if they register
// a device within the at-risk window.
func TestStreakReminder_StampsLastReminderAtBeforeSend(t *testing.T) {
	store := &fakeStreakStore{
		atRisk: []model.UserAtRisk{
			{UserID: "u1", CurrentStreak: 5},
			{UserID: "u2", CurrentStreak: 3},
		},
		tokens: map[string][]string{
			"u1": {"tok-u1"},
			// u2 has no tokens — stamp should NOT be written
		},
	}
	h := handlers.NewStreakReminderHandler(store, &capturePusher{}, zap.NewNop())

	rr, req := newStreakRequest()
	h.Serve(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if len(store.stampedUsers) != 1 || store.stampedUsers[0] != "u1" {
		t.Errorf("expected only u1 stamped, got stampedUsers=%v", store.stampedUsers)
	}
}

// TestStreakReminder_StaleTokenPurgedOnInvalidRegistration verifies that when
// FCM returns an InvalidRegistration error the dead token is deleted from
// storage so it is not retried on subsequent cron runs.
func TestStreakReminder_StaleTokenPurgedOnInvalidRegistration(t *testing.T) {
	store := &fakeStreakStore{
		atRisk: []model.UserAtRisk{{UserID: "u1", CurrentStreak: 4}},
		tokens: map[string][]string{
			"u1": {"stale-token", "good-token"},
		},
	}
	pusher := &capturePusher{
		failOn: map[string]error{
			"stale-token": errors.New("fcm: delivery failed: InvalidRegistration"),
		},
	}
	h := handlers.NewStreakReminderHandler(store, pusher, zap.NewNop())

	rr, req := newStreakRequest()
	h.Serve(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if len(store.deletedTokens) != 1 || store.deletedTokens[0] != "stale-token" {
		t.Errorf("expected stale-token to be purged, got deletedTokens=%v", store.deletedTokens)
	}
	got := decodeStreakResponse(t, rr)
	if got.Sent != 1 || got.Failed != 1 {
		t.Errorf("counts = %+v, want {Sent:1 Failed:1}", got)
	}
}

// TestStreakReminder_UserWithNoTokens_NoPush verifies a user who has no
// registered FCM tokens contributes to AtRisk but not Sent/Failed.
func TestStreakReminder_UserWithNoTokens_NoPush(t *testing.T) {
	store := &fakeStreakStore{
		atRisk: []model.UserAtRisk{{UserID: "tokenless"}},
		tokens: map[string][]string{}, // no tokens for this user
	}
	pusher := &capturePusher{}
	h := handlers.NewStreakReminderHandler(store, pusher, zap.NewNop())

	rr, req := newStreakRequest()
	h.Serve(rr, req)

	got := decodeStreakResponse(t, rr)
	if got.AtRisk != 1 || got.Sent != 0 || got.Failed != 0 {
		t.Errorf("counts = %+v, want {AtRisk:1 Sent:0 Failed:0}", got)
	}
	if len(pusher.calls) != 0 {
		t.Errorf("expected zero FCM calls, got %d", len(pusher.calls))
	}
}
