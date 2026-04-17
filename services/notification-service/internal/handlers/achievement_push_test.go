package handlers_test

// Unit tests for AchievementPushHandler (LevelUp + QuestComplete).
//
// Both endpoints share the same dispatch path, so tests exercise the shared
// flow (guard ordering, dedup short-circuit, stale-token purge, async fanout)
// on LevelUp and spot-check QuestComplete for payload/title differences.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/handlers"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/model"
)

// ── fakes ────────────────────────────────────────────────────────────────────

// fakeAchievementStore is an in-memory AchievementPushStore.
type fakeAchievementStore struct {
	mu sync.Mutex

	tokens    map[string][]string
	tokensErr error

	// markStamp[userID+event] = (stamped, err). Default: (true, nil).
	markLevelUpOverride      func(userID string) (bool, error)
	markQuestCompleteOverride func(userID string) (bool, error)

	deletedTokens []deletedToken
}

type deletedToken struct{ userID, token string }

func (f *fakeAchievementStore) GetPushTokens(_ context.Context, userID string) ([]string, error) {
	if f.tokensErr != nil {
		return nil, f.tokensErr
	}
	return f.tokens[userID], nil
}

func (f *fakeAchievementStore) DeletePushToken(_ context.Context, userID, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedTokens = append(f.deletedTokens, deletedToken{userID, token})
	return nil
}

func (f *fakeAchievementStore) MarkLevelUpNotified(_ context.Context, userID string, _ time.Duration) (bool, error) {
	if f.markLevelUpOverride != nil {
		return f.markLevelUpOverride(userID)
	}
	return true, nil
}

func (f *fakeAchievementStore) MarkQuestCompleteNotified(_ context.Context, userID string, _ time.Duration) (bool, error) {
	if f.markQuestCompleteOverride != nil {
		return f.markQuestCompleteOverride(userID)
	}
	return true, nil
}

// recordingPusher captures every Send call and can be configured to fail
// for specific tokens (simulating FCM InvalidRegistration etc).
type recordingPusher struct {
	mu     sync.Mutex
	calls  []pushCallRecord
	failOn map[string]error
}

type pushCallRecord struct {
	token, title, body string
}

func (p *recordingPusher) Send(_ context.Context, token, title, body string, _ map[string]any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, pushCallRecord{token, title, body})
	if err := p.failOn[token]; err != nil {
		return err
	}
	return nil
}

func (p *recordingPusher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func levelUpRequest(body any) *http.Request {
	b, _ := json.Marshal(body)
	return httptest.NewRequest(http.MethodPost, "/internal/push/level-up", bytes.NewReader(b))
}

func questCompleteRequest(body any) *http.Request {
	b, _ := json.Marshal(body)
	return httptest.NewRequest(http.MethodPost, "/internal/push/quest-complete", bytes.NewReader(b))
}

func decodeDispatch(t *testing.T, rr *httptest.ResponseRecorder) model.PushDispatchResponse {
	t.Helper()
	var out model.PushDispatchResponse
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v (body=%q)", err, rr.Body.String())
	}
	return out
}

// ── LevelUp tests ────────────────────────────────────────────────────────────

func TestLevelUp_BadJSON_Returns400(t *testing.T) {
	h := handlers.NewAchievementPushHandler(&fakeAchievementStore{}, &recordingPusher{}, zap.NewNop())
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/push/level-up", strings.NewReader("{not json"))
	h.LevelUp(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestLevelUp_MissingUserID_Returns400(t *testing.T) {
	h := handlers.NewAchievementPushHandler(&fakeAchievementStore{}, &recordingPusher{}, zap.NewNop())
	rr := httptest.NewRecorder()
	h.LevelUp(rr, levelUpRequest(model.LevelUpRequest{NewLevel: 3}))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestLevelUp_NonPositiveLevel_Returns400(t *testing.T) {
	h := handlers.NewAchievementPushHandler(&fakeAchievementStore{}, &recordingPusher{}, zap.NewNop())
	rr := httptest.NewRecorder()
	h.LevelUp(rr, levelUpRequest(model.LevelUpRequest{UserID: "u1", NewLevel: 0}))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestLevelUp_NoTokens_SkippedNoStamp(t *testing.T) {
	stampCalled := false
	store := &fakeAchievementStore{
		tokens: map[string][]string{}, // user has no devices
		markLevelUpOverride: func(userID string) (bool, error) {
			stampCalled = true
			return true, nil
		},
	}
	pusher := &recordingPusher{}
	h := handlers.NewAchievementPushHandler(store, pusher, zap.NewNop())

	rr := httptest.NewRecorder()
	h.LevelUp(rr, levelUpRequest(model.LevelUpRequest{UserID: "u1", NewLevel: 5}))
	h.WaitForFanout()

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
	got := decodeDispatch(t, rr)
	if !got.Skipped || got.Reason != "no_tokens" {
		t.Errorf("got %+v, want Skipped=true Reason=no_tokens", got)
	}
	if stampCalled {
		t.Error("stamp should not run when user has zero tokens (blocks future retries once they register)")
	}
	if pusher.count() != 0 {
		t.Errorf("expected no FCM calls, got %d", pusher.count())
	}
}

func TestLevelUp_DuplicateWithinWindow_SkippedNoFanout(t *testing.T) {
	store := &fakeAchievementStore{
		tokens: map[string][]string{"u1": {"tok-a"}},
		markLevelUpOverride: func(userID string) (bool, error) {
			return false, nil // stamp says "within dedup window"
		},
	}
	pusher := &recordingPusher{}
	h := handlers.NewAchievementPushHandler(store, pusher, zap.NewNop())

	rr := httptest.NewRecorder()
	h.LevelUp(rr, levelUpRequest(model.LevelUpRequest{UserID: "u1", NewLevel: 5}))
	h.WaitForFanout()

	got := decodeDispatch(t, rr)
	if !got.Skipped || got.Reason != "duplicate" {
		t.Errorf("got %+v, want Skipped=true Reason=duplicate", got)
	}
	if pusher.count() != 0 {
		t.Errorf("expected no FCM calls on dedup hit, got %d", pusher.count())
	}
}

func TestLevelUp_AcceptedFansOutToAllTokens(t *testing.T) {
	store := &fakeAchievementStore{
		tokens: map[string][]string{"u1": {"tok-a", "tok-b", "tok-c"}},
	}
	pusher := &recordingPusher{}
	h := handlers.NewAchievementPushHandler(store, pusher, zap.NewNop())

	rr := httptest.NewRecorder()
	h.LevelUp(rr, levelUpRequest(model.LevelUpRequest{UserID: "u1", NewLevel: 7}))
	h.WaitForFanout()

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
	got := decodeDispatch(t, rr)
	if got.Skipped {
		t.Errorf("expected not skipped, got %+v", got)
	}
	if pusher.count() != 3 {
		t.Errorf("expected 3 FCM calls, got %d", pusher.count())
	}
	// Body should carry the new level.
	for _, c := range pusher.calls {
		if !strings.Contains(c.body, "7") {
			t.Errorf("body %q missing level 7", c.body)
		}
	}
}

func TestLevelUp_StaleTokenIsPurgedAfterFanout(t *testing.T) {
	store := &fakeAchievementStore{
		tokens: map[string][]string{"u1": {"good", "stale"}},
	}
	pusher := &recordingPusher{
		failOn: map[string]error{
			"stale": errors.New("InvalidRegistration"),
		},
	}
	h := handlers.NewAchievementPushHandler(store, pusher, zap.NewNop())

	rr := httptest.NewRecorder()
	h.LevelUp(rr, levelUpRequest(model.LevelUpRequest{UserID: "u1", NewLevel: 2}))
	h.WaitForFanout()

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
	if n := len(store.deletedTokens); n != 1 {
		t.Fatalf("expected 1 stale token purge, got %d", n)
	}
	if store.deletedTokens[0].token != "stale" {
		t.Errorf("purged wrong token: %+v", store.deletedTokens[0])
	}
}

func TestLevelUp_StoreErrorOnMark_Returns500(t *testing.T) {
	store := &fakeAchievementStore{
		tokens: map[string][]string{"u1": {"tok-a"}},
		markLevelUpOverride: func(_ string) (bool, error) {
			return false, errors.New("db down")
		},
	}
	h := handlers.NewAchievementPushHandler(store, &recordingPusher{}, zap.NewNop())

	rr := httptest.NewRecorder()
	h.LevelUp(rr, levelUpRequest(model.LevelUpRequest{UserID: "u1", NewLevel: 3}))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestLevelUp_StoreErrorOnGetTokens_Returns500(t *testing.T) {
	store := &fakeAchievementStore{tokensErr: errors.New("db down")}
	h := handlers.NewAchievementPushHandler(store, &recordingPusher{}, zap.NewNop())

	rr := httptest.NewRecorder()
	h.LevelUp(rr, levelUpRequest(model.LevelUpRequest{UserID: "u1", NewLevel: 3}))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

// ── QuestComplete tests ──────────────────────────────────────────────────────

func TestQuestComplete_BadJSON_Returns400(t *testing.T) {
	h := handlers.NewAchievementPushHandler(&fakeAchievementStore{}, &recordingPusher{}, zap.NewNop())
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/push/quest-complete", strings.NewReader("{"))
	h.QuestComplete(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestQuestComplete_MissingQuestTitle_Returns400(t *testing.T) {
	h := handlers.NewAchievementPushHandler(&fakeAchievementStore{}, &recordingPusher{}, zap.NewNop())
	rr := httptest.NewRecorder()
	h.QuestComplete(rr, questCompleteRequest(model.QuestCompleteRequest{UserID: "u1", XPReward: 50}))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestQuestComplete_AcceptedIncludesTitleAndXPInBody(t *testing.T) {
	store := &fakeAchievementStore{tokens: map[string][]string{"u1": {"tok-a"}}}
	pusher := &recordingPusher{}
	h := handlers.NewAchievementPushHandler(store, pusher, zap.NewNop())

	rr := httptest.NewRecorder()
	h.QuestComplete(rr, questCompleteRequest(model.QuestCompleteRequest{
		UserID: "u1", QuestTitle: "Answer 5 questions", XPReward: 100,
	}))
	h.WaitForFanout()

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
	if pusher.count() != 1 {
		t.Fatalf("expected 1 FCM call, got %d", pusher.count())
	}
	body := pusher.calls[0].body
	if !strings.Contains(body, "Answer 5 questions") || !strings.Contains(body, "100") {
		t.Errorf("body %q missing quest title or XP", body)
	}
}

func TestQuestComplete_DuplicateHit_Skipped(t *testing.T) {
	store := &fakeAchievementStore{
		tokens: map[string][]string{"u1": {"tok-a"}},
		markQuestCompleteOverride: func(_ string) (bool, error) { return false, nil },
	}
	pusher := &recordingPusher{}
	h := handlers.NewAchievementPushHandler(store, pusher, zap.NewNop())

	rr := httptest.NewRecorder()
	h.QuestComplete(rr, questCompleteRequest(model.QuestCompleteRequest{
		UserID: "u1", QuestTitle: "X", XPReward: 10,
	}))
	h.WaitForFanout()

	got := decodeDispatch(t, rr)
	if !got.Skipped || got.Reason != "duplicate" {
		t.Errorf("got %+v, want Skipped=true Reason=duplicate", got)
	}
	if pusher.count() != 0 {
		t.Errorf("expected no FCM calls, got %d", pusher.count())
	}
}
