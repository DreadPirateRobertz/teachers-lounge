package handler_test

// Tests for handler.go: GainXP, GetProfile, StreakCheckin, GetDailyQuests,
// UpdateQuestProgress, GetAchievements, and Health.
//
// Each store stub embeds noopStore (defined in flashcard_test.go) and overrides
// only the methods exercised by the handler under test. Function fields on the
// stubs allow individual tests to inject specific return values or errors.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/handler"
	"github.com/teacherslounge/gaming-service/internal/model"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// newHandler creates a Handler wired with the given store, no taunter.
func newHandler(s handler.Storer) *handler.Handler {
	return handler.New(s, nil, zap.NewNop())
}

// withURLParam injects a chi URL parameter into the request context.
func withURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// ── xpStore — stub for GainXP tests ──────────────────────────────────────────

type xpStore struct {
	noopStore
	getXPFn    func(ctx context.Context, userID string) (int64, int, error)
	upsertXPFn func(ctx context.Context, userID string, newXP int64, newLevel int) error
}

func (s *xpStore) GetXPAndLevel(ctx context.Context, userID string) (int64, int, error) {
	if s.getXPFn != nil {
		return s.getXPFn(ctx, userID)
	}
	return 0, 1, nil
}

func (s *xpStore) UpsertXP(ctx context.Context, userID string, newXP int64, newLevel int) error {
	if s.upsertXPFn != nil {
		return s.upsertXPFn(ctx, userID, newXP, newLevel)
	}
	return nil
}

// ── GainXP tests ──────────────────────────────────────────────────────────────

// TestGainXP_HappyPath verifies XP is applied and the response encodes correctly.
func TestGainXP_HappyPath(t *testing.T) {
	s := &xpStore{
		getXPFn: func(_ context.Context, _ string) (int64, int, error) { return 50, 1, nil },
	}
	h := newHandler(s)

	body, _ := json.Marshal(model.GainXPRequest{UserID: "u1", Amount: 100})
	req := httptest.NewRequest(http.MethodPost, "/gaming/xp", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()

	h.GainXP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rr.Code, rr.Body)
	}
	var resp model.GainXPResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.NewXP <= 50 {
		t.Errorf("expected new XP > 50, got %d", resp.NewXP)
	}
}

// TestGainXP_InvalidBody verifies 400 when the request body is not valid JSON.
func TestGainXP_InvalidBody(t *testing.T) {
	h := newHandler(&xpStore{})
	req := httptest.NewRequest(http.MethodPost, "/gaming/xp", bytes.NewBufferString("not-json"))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.GainXP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestGainXP_MissingUserID verifies 400 when user_id is empty.
func TestGainXP_MissingUserID(t *testing.T) {
	h := newHandler(&xpStore{})
	body, _ := json.Marshal(model.GainXPRequest{Amount: 10})
	req := httptest.NewRequest(http.MethodPost, "/gaming/xp", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.GainXP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestGainXP_ZeroAmount verifies 400 when amount is zero (not positive).
func TestGainXP_ZeroAmount(t *testing.T) {
	h := newHandler(&xpStore{})
	body, _ := json.Marshal(model.GainXPRequest{UserID: "u1", Amount: 0})
	req := httptest.NewRequest(http.MethodPost, "/gaming/xp", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.GainXP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestGainXP_NegativeAmount verifies 400 when amount is negative.
func TestGainXP_NegativeAmount(t *testing.T) {
	h := newHandler(&xpStore{})
	body, _ := json.Marshal(model.GainXPRequest{UserID: "u1", Amount: -5})
	req := httptest.NewRequest(http.MethodPost, "/gaming/xp", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.GainXP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestGainXP_ForbiddenCallerMismatch verifies 403 when caller != user_id in body.
func TestGainXP_ForbiddenCallerMismatch(t *testing.T) {
	h := newHandler(&xpStore{})
	body, _ := json.Marshal(model.GainXPRequest{UserID: "other-user", Amount: 10})
	req := httptest.NewRequest(http.MethodPost, "/gaming/xp", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.GainXP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// TestGainXP_NoAuth verifies 403 when no user is in the context.
func TestGainXP_NoAuth(t *testing.T) {
	h := newHandler(&xpStore{})
	body, _ := json.Marshal(model.GainXPRequest{UserID: "u1", Amount: 10})
	req := httptest.NewRequest(http.MethodPost, "/gaming/xp", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	h.GainXP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// TestGainXP_GetXPAndLevelError verifies 500 when the store fails to fetch XP.
func TestGainXP_GetXPAndLevelError(t *testing.T) {
	s := &xpStore{
		getXPFn: func(_ context.Context, _ string) (int64, int, error) {
			return 0, 0, errors.New("db error")
		},
	}
	h := newHandler(s)
	body, _ := json.Marshal(model.GainXPRequest{UserID: "u1", Amount: 10})
	req := httptest.NewRequest(http.MethodPost, "/gaming/xp", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.GainXP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// TestGainXP_UpsertXPError verifies 500 when the store fails to persist the new XP.
func TestGainXP_UpsertXPError(t *testing.T) {
	s := &xpStore{
		getXPFn: func(_ context.Context, _ string) (int64, int, error) { return 0, 1, nil },
		upsertXPFn: func(_ context.Context, _ string, _ int64, _ int) error {
			return errors.New("write failed")
		},
	}
	h := newHandler(s)
	body, _ := json.Marshal(model.GainXPRequest{UserID: "u1", Amount: 10})
	req := httptest.NewRequest(http.MethodPost, "/gaming/xp", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.GainXP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// ── profileStore — stub for GetProfile tests ──────────────────────────────────

type profileStore struct {
	noopStore
	profileFn func(ctx context.Context, userID string) (*model.Profile, error)
}

func (s *profileStore) GetProfile(ctx context.Context, userID string) (*model.Profile, error) {
	if s.profileFn != nil {
		return s.profileFn(ctx, userID)
	}
	return &model.Profile{UserID: userID, Level: 5, XP: 1000}, nil
}

// TestGetProfile_HappyPath verifies the profile is returned for the authenticated user.
func TestGetProfile_HappyPath(t *testing.T) {
	h := newHandler(&profileStore{})
	req := httptest.NewRequest(http.MethodGet, "/gaming/profile/u1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "userId", "u1")
	rr := httptest.NewRecorder()

	h.GetProfile(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rr.Code, rr.Body)
	}
	var profile model.Profile
	if err := json.NewDecoder(rr.Body).Decode(&profile); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if profile.UserID != "u1" {
		t.Errorf("expected user_id=u1, got %q", profile.UserID)
	}
}

// TestGetProfile_ForbiddenCrossUser verifies 403 when caller requests another user's profile.
func TestGetProfile_ForbiddenCrossUser(t *testing.T) {
	h := newHandler(&profileStore{})
	req := httptest.NewRequest(http.MethodGet, "/gaming/profile/other", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "userId", "other")
	rr := httptest.NewRecorder()
	h.GetProfile(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// TestGetProfile_NoAuth verifies 403 when no auth context is set.
func TestGetProfile_NoAuth(t *testing.T) {
	h := newHandler(&profileStore{})
	req := httptest.NewRequest(http.MethodGet, "/gaming/profile/u1", nil)
	req = withURLParam(req, "userId", "u1")
	rr := httptest.NewRecorder()
	h.GetProfile(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// TestGetProfile_StoreError verifies 500 when the store returns an error.
func TestGetProfile_StoreError(t *testing.T) {
	s := &profileStore{
		profileFn: func(_ context.Context, _ string) (*model.Profile, error) {
			return nil, errors.New("db offline")
		},
	}
	h := newHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/gaming/profile/u1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "userId", "u1")
	rr := httptest.NewRecorder()
	h.GetProfile(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// ── streakStore — stub for StreakCheckin tests ────────────────────────────────

type streakStore struct {
	noopStore
	checkinFn func(ctx context.Context, userID string) (current, longest int, reset bool, err error)
}

func (s *streakStore) StreakCheckin(ctx context.Context, userID string) (int, int, bool, error) {
	if s.checkinFn != nil {
		return s.checkinFn(ctx, userID)
	}
	return 5, 10, false, nil
}

// TestStreakCheckin_HappyPath verifies a normal checkin returns streak counts.
func TestStreakCheckin_HappyPath(t *testing.T) {
	h := newHandler(&streakStore{})
	body, _ := json.Marshal(model.StreakCheckinRequest{UserID: "u1"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/streak/checkin", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()

	h.StreakCheckin(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rr.Code, rr.Body)
	}
	var resp model.StreakCheckinResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.CurrentStreak != 5 {
		t.Errorf("expected current_streak=5, got %d", resp.CurrentStreak)
	}
}

// TestStreakCheckin_ResetTrue verifies the reset flag is propagated when streak was broken.
func TestStreakCheckin_ResetTrue(t *testing.T) {
	s := &streakStore{
		checkinFn: func(_ context.Context, _ string) (int, int, bool, error) { return 1, 7, true, nil },
	}
	h := newHandler(s)
	body, _ := json.Marshal(model.StreakCheckinRequest{UserID: "u1"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/streak/checkin", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()

	h.StreakCheckin(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rr.Code)
	}
	var resp model.StreakCheckinResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if !resp.Reset {
		t.Error("expected reset=true")
	}
}

// TestStreakCheckin_InvalidBody verifies 400 for malformed JSON.
func TestStreakCheckin_InvalidBody(t *testing.T) {
	h := newHandler(&streakStore{})
	req := httptest.NewRequest(http.MethodPost, "/gaming/streak/checkin", bytes.NewBufferString("{bad"))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.StreakCheckin(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestStreakCheckin_MissingUserID verifies 400 when user_id is empty.
func TestStreakCheckin_MissingUserID(t *testing.T) {
	h := newHandler(&streakStore{})
	body, _ := json.Marshal(model.StreakCheckinRequest{})
	req := httptest.NewRequest(http.MethodPost, "/gaming/streak/checkin", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.StreakCheckin(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestStreakCheckin_ForbiddenMismatch verifies 403 when caller != user_id in body.
func TestStreakCheckin_ForbiddenMismatch(t *testing.T) {
	h := newHandler(&streakStore{})
	body, _ := json.Marshal(model.StreakCheckinRequest{UserID: "other"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/streak/checkin", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.StreakCheckin(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// TestStreakCheckin_StoreError verifies 500 when the store returns an error.
func TestStreakCheckin_StoreError(t *testing.T) {
	s := &streakStore{
		checkinFn: func(_ context.Context, _ string) (int, int, bool, error) {
			return 0, 0, false, errors.New("redis timeout")
		},
	}
	h := newHandler(s)
	body, _ := json.Marshal(model.StreakCheckinRequest{UserID: "u1"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/streak/checkin", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.StreakCheckin(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// ── questStore — stub for GetDailyQuests / UpdateQuestProgress tests ─────────

type questStore struct {
	noopStore
	getDailyQuestsFn    func(ctx context.Context, userID string) ([]model.QuestState, error)
	updateQuestFn       func(ctx context.Context, userID, action string) ([]model.QuestState, int, int, error)
	awardQuestRewardsFn func(ctx context.Context, userID string, xpDelta, gemsDelta int) (int64, int, bool, int, error)
}

func (s *questStore) GetDailyQuests(ctx context.Context, userID string) ([]model.QuestState, error) {
	if s.getDailyQuestsFn != nil {
		return s.getDailyQuestsFn(ctx, userID)
	}
	return []model.QuestState{{ID: "q1", Title: "Study", Target: 3}}, nil
}

func (s *questStore) UpdateQuestProgress(ctx context.Context, userID, action string) ([]model.QuestState, int, int, error) {
	if s.updateQuestFn != nil {
		return s.updateQuestFn(ctx, userID, action)
	}
	return []model.QuestState{}, 0, 0, nil
}

func (s *questStore) AwardQuestRewards(ctx context.Context, userID string, xpDelta, gemsDelta int) (int64, int, bool, int, error) {
	if s.awardQuestRewardsFn != nil {
		return s.awardQuestRewardsFn(ctx, userID, xpDelta, gemsDelta)
	}
	return 200, 3, false, 10, nil
}

// TestGetDailyQuests_HappyPath verifies quests are returned for an authenticated user.
func TestGetDailyQuests_HappyPath(t *testing.T) {
	h := newHandler(&questStore{})
	req := httptest.NewRequest(http.MethodGet, "/gaming/quests/daily", nil)
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()

	h.GetDailyQuests(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rr.Code, rr.Body)
	}
	var resp model.DailyQuestsResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.Quests) == 0 {
		t.Error("expected at least one quest in response")
	}
}

// TestGetDailyQuests_NoAuth verifies 403 when context has no user ID.
func TestGetDailyQuests_NoAuth(t *testing.T) {
	h := newHandler(&questStore{})
	req := httptest.NewRequest(http.MethodGet, "/gaming/quests/daily", nil)
	rr := httptest.NewRecorder()
	h.GetDailyQuests(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// TestGetDailyQuests_StoreError verifies 500 when the store returns an error.
func TestGetDailyQuests_StoreError(t *testing.T) {
	s := &questStore{
		getDailyQuestsFn: func(_ context.Context, _ string) ([]model.QuestState, error) {
			return nil, errors.New("db error")
		},
	}
	h := newHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/gaming/quests/daily", nil)
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.GetDailyQuests(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// TestUpdateQuestProgress_NoRewards verifies a progress update with no completions.
func TestUpdateQuestProgress_NoRewards(t *testing.T) {
	h := newHandler(&questStore{})
	body, _ := json.Marshal(model.QuestProgressRequest{UserID: "u1", Action: "answer_correct"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quests/progress", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()

	h.UpdateQuestProgress(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rr.Code, rr.Body)
	}
	var resp model.QuestProgressResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.XPAwarded != 0 || resp.GemsAwarded != 0 {
		t.Errorf("expected zero rewards, got xp=%d gems=%d", resp.XPAwarded, resp.GemsAwarded)
	}
}

// TestUpdateQuestProgress_WithRewards verifies AwardQuestRewards is called when quests complete.
func TestUpdateQuestProgress_WithRewards(t *testing.T) {
	awardCalled := false
	s := &questStore{
		updateQuestFn: func(_ context.Context, _, _ string) ([]model.QuestState, int, int, error) {
			return []model.QuestState{}, 50, 10, nil
		},
		awardQuestRewardsFn: func(_ context.Context, _ string, xp, gems int) (int64, int, bool, int, error) {
			awardCalled = true
			if xp != 50 || gems != 10 {
				return 0, 0, false, 0, errors.New("unexpected reward amounts")
			}
			return 500, 4, true, 20, nil
		},
	}
	h := newHandler(s)
	body, _ := json.Marshal(model.QuestProgressRequest{UserID: "u1", Action: "complete_session"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quests/progress", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()

	h.UpdateQuestProgress(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rr.Code, rr.Body)
	}
	if !awardCalled {
		t.Error("expected AwardQuestRewards to be called when rewards > 0")
	}
	var resp model.QuestProgressResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if !resp.LevelUp {
		t.Error("expected level_up=true in response")
	}
}

// TestUpdateQuestProgress_InvalidBody verifies 400 for malformed JSON.
func TestUpdateQuestProgress_InvalidBody(t *testing.T) {
	h := newHandler(&questStore{})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quests/progress", bytes.NewBufferString("bad-json"))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.UpdateQuestProgress(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestUpdateQuestProgress_MissingFields verifies 400 when user_id or action is empty.
func TestUpdateQuestProgress_MissingFields(t *testing.T) {
	h := newHandler(&questStore{})
	body, _ := json.Marshal(model.QuestProgressRequest{UserID: "u1"}) // action empty
	req := httptest.NewRequest(http.MethodPost, "/gaming/quests/progress", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.UpdateQuestProgress(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestUpdateQuestProgress_Forbidden verifies 403 when caller != user_id.
func TestUpdateQuestProgress_Forbidden(t *testing.T) {
	h := newHandler(&questStore{})
	body, _ := json.Marshal(model.QuestProgressRequest{UserID: "other", Action: "login"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quests/progress", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.UpdateQuestProgress(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// TestUpdateQuestProgress_UpdateStoreError verifies 500 when UpdateQuestProgress fails.
func TestUpdateQuestProgress_UpdateStoreError(t *testing.T) {
	s := &questStore{
		updateQuestFn: func(_ context.Context, _, _ string) ([]model.QuestState, int, int, error) {
			return nil, 0, 0, errors.New("db write failed")
		},
	}
	h := newHandler(s)
	body, _ := json.Marshal(model.QuestProgressRequest{UserID: "u1", Action: "login"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quests/progress", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.UpdateQuestProgress(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// TestUpdateQuestProgress_AwardRewardsError verifies 500 when AwardQuestRewards fails.
func TestUpdateQuestProgress_AwardRewardsError(t *testing.T) {
	s := &questStore{
		updateQuestFn: func(_ context.Context, _, _ string) ([]model.QuestState, int, int, error) {
			return []model.QuestState{}, 50, 0, nil
		},
		awardQuestRewardsFn: func(_ context.Context, _ string, _, _ int) (int64, int, bool, int, error) {
			return 0, 0, false, 0, errors.New("ledger offline")
		},
	}
	h := newHandler(s)
	body, _ := json.Marshal(model.QuestProgressRequest{UserID: "u1", Action: "answer_correct"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quests/progress", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.UpdateQuestProgress(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// ── achievementStore — stub for GetAchievements tests ────────────────────────

type achievementStore struct {
	noopStore
	getAchievementsFn func(ctx context.Context, userID string) ([]model.Achievement, error)
}

func (s *achievementStore) GetAchievements(ctx context.Context, userID string) ([]model.Achievement, error) {
	if s.getAchievementsFn != nil {
		return s.getAchievementsFn(ctx, userID)
	}
	return []model.Achievement{{UserID: userID, BadgeName: "first-win"}}, nil
}

// TestGetAchievements_HappyPath verifies achievements are returned for the correct user.
func TestGetAchievements_HappyPath(t *testing.T) {
	h := newHandler(&achievementStore{})
	req := httptest.NewRequest(http.MethodGet, "/gaming/achievements/u1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "userId", "u1")
	rr := httptest.NewRecorder()

	h.GetAchievements(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rr.Code, rr.Body)
	}
	var resp model.AchievementsResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.Achievements) == 0 {
		t.Error("expected at least one achievement")
	}
}

// TestGetAchievements_ForbiddenCrossUser verifies 403 when caller != userId param.
func TestGetAchievements_ForbiddenCrossUser(t *testing.T) {
	h := newHandler(&achievementStore{})
	req := httptest.NewRequest(http.MethodGet, "/gaming/achievements/other", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "userId", "other")
	rr := httptest.NewRecorder()
	h.GetAchievements(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// TestGetAchievements_NoAuth verifies 403 when no auth context is set.
func TestGetAchievements_NoAuth(t *testing.T) {
	h := newHandler(&achievementStore{})
	req := httptest.NewRequest(http.MethodGet, "/gaming/achievements/u1", nil)
	req = withURLParam(req, "userId", "u1")
	rr := httptest.NewRecorder()
	h.GetAchievements(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// TestGetAchievements_StoreError verifies 500 when the store returns an error.
func TestGetAchievements_StoreError(t *testing.T) {
	s := &achievementStore{
		getAchievementsFn: func(_ context.Context, _ string) ([]model.Achievement, error) {
			return nil, errors.New("db error")
		},
	}
	h := newHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/gaming/achievements/u1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "userId", "u1")
	rr := httptest.NewRecorder()
	h.GetAchievements(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// ── Health tests ──────────────────────────────────────────────────────────────

// TestHealth_ReturnsOK verifies the health endpoint always responds 200.
func TestHealth_ReturnsOK(t *testing.T) {
	h := newHandler(&noopStore{})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	h.Health(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rr.Code)
	}
	body := rr.Body.String()
	if body != `{"status":"ok"}` {
		t.Errorf("unexpected body: %q", body)
	}
}
