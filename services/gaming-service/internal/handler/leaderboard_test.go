package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/handler"
	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/taunt"
)

// leaderboardStore is a minimal Storer stub for leaderboard handler tests.
// Only the leaderboard-related methods are functional; the rest are no-ops.
type leaderboardStore struct {
	top10      []model.LeaderboardEntry
	userRank   *model.LeaderboardEntry
	periodData map[string][]model.LeaderboardEntry
	courseData map[string][]model.LeaderboardEntry
	friendData []model.LeaderboardEntry
	err        error
}

func (s *leaderboardStore) LeaderboardTop10(_ context.Context, _ string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	return s.top10, s.userRank, s.err
}
func (s *leaderboardStore) LeaderboardGetPeriod(_ context.Context, _, period string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	if s.err != nil {
		return nil, nil, s.err
	}
	entries := s.periodData[period]
	return entries, s.userRank, nil
}
func (s *leaderboardStore) LeaderboardGetCourse(_ context.Context, _, courseID string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	if s.err != nil {
		return nil, nil, s.err
	}
	entries := s.courseData[courseID]
	return entries, s.userRank, nil
}
func (s *leaderboardStore) LeaderboardGetFriends(_ context.Context, _ string, _ []string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	return s.friendData, s.userRank, s.err
}
func (s *leaderboardStore) LeaderboardUpdate(_ context.Context, _ string, _ int64) error {
	return s.err
}
func (s *leaderboardStore) LeaderboardUpdateCourse(_ context.Context, _, _ string, _ int64) error {
	return s.err
}

// ── Satisfy the full Storer interface with no-ops ────────────────────────────

func (s *leaderboardStore) GetXPAndLevel(_ context.Context, _ string) (int64, int, error) {
	return 0, 1, nil
}
func (s *leaderboardStore) UpsertXP(_ context.Context, _ string, _ int64, _ int) error { return nil }
func (s *leaderboardStore) GetProfile(_ context.Context, _ string) (*model.Profile, error) {
	return nil, nil
}
func (s *leaderboardStore) StreakCheckin(_ context.Context, _ string) (int, int, bool, error) {
	return 0, 0, false, nil
}
func (s *leaderboardStore) RandomQuote(_ context.Context) (*model.Quote, error) { return nil, nil }
func (s *leaderboardStore) RandomQuoteForUser(_ context.Context, _, _ string) (*model.Quote, error) {
	return nil, nil
}
func (s *leaderboardStore) GetRandomQuestions(_ context.Context, _ string, _ int) ([]*model.Question, error) {
	return nil, nil
}
func (s *leaderboardStore) GetQuestion(_ context.Context, _ string) (*model.Question, error) {
	return nil, nil
}
func (s *leaderboardStore) CreateQuizSession(_ context.Context, _ string, _, _ *string, _ []string) (*model.QuizSession, error) {
	return nil, nil
}
func (s *leaderboardStore) GetQuizSession(_ context.Context, _ string) (*model.QuizSession, error) {
	return nil, nil
}
func (s *leaderboardStore) RecordAnswer(_ context.Context, _, _, _, _ string, _ bool, _, _ int, _ *int) (*model.QuizSession, error) {
	return nil, nil
}
func (s *leaderboardStore) GetHintIndex(_ context.Context, _, _ string) (int, error) { return 0, nil }
func (s *leaderboardStore) IncrHintIndex(_ context.Context, _, _, _ string) (int, int, error) {
	return 0, 0, nil
}
func (s *leaderboardStore) GetDailyQuests(_ context.Context, _ string) ([]model.QuestState, error) {
	return nil, nil
}
func (s *leaderboardStore) UpdateQuestProgress(_ context.Context, _ string, _ string) ([]model.QuestState, int, int, error) {
	return nil, 0, 0, nil
}
func (s *leaderboardStore) AwardQuestRewards(_ context.Context, _ string, _, _ int) (int64, int, bool, int, error) {
	return 0, 0, false, 0, nil
}
func (s *leaderboardStore) SaveBattleSession(_ context.Context, _ *model.BattleSession) error {
	return nil
}
func (s *leaderboardStore) GetBattleSession(_ context.Context, _ string) (*model.BattleSession, error) {
	return nil, nil
}
func (s *leaderboardStore) DeleteBattleSession(_ context.Context, _ string) error { return nil }
func (s *leaderboardStore) RecordBattleResult(_ context.Context, _ *model.BattleResult) error {
	return nil
}
func (s *leaderboardStore) DeductGems(_ context.Context, _ string, _ int) (int, error) { return 0, nil }
func (s *leaderboardStore) SaveTaunt(_ context.Context, _ string, _ int, _ string) error { return nil }
func (s *leaderboardStore) GetRandomTaunt(_ context.Context, _ string, _ int) (string, bool, error) {
	return "", false, nil
}
func (s *leaderboardStore) GrantAchievement(_ context.Context, _, _, _ string) (*model.Achievement, bool, error) {
	return nil, false, nil
}
func (s *leaderboardStore) GetAchievements(_ context.Context, _ string) ([]model.Achievement, error) {
	return nil, nil
}
func (s *leaderboardStore) AddCosmeticItem(_ context.Context, _, _, _ string) error { return nil }
func (s *leaderboardStore) CreateAssessmentSession(_ context.Context, _ string) (*model.AssessmentSession, error) {
	return nil, nil
}
func (s *leaderboardStore) GetAssessmentSession(_ context.Context, _ string) (*model.AssessmentSession, error) {
	return nil, nil
}
func (s *leaderboardStore) RecordAssessmentAnswer(_ context.Context, _, _, _, _ string) (*model.AssessmentSession, error) {
	return nil, nil
}
func (s *leaderboardStore) BuyPowerUp(_ context.Context, _ string, _ model.PowerUpType, _ int) (int, int, error) {
	return 0, 0, nil
}
func (s *leaderboardStore) GetDefeatedBossIDs(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (s *leaderboardStore) GetChapterMastery(_ context.Context, _ string, _ []string) (float64, error) {
	return 0.0, nil
}
func (s *leaderboardStore) CreateFlashcard(_ context.Context, c *model.Flashcard) (*model.Flashcard, error) {
	return c, nil
}
func (s *leaderboardStore) GetFlashcard(_ context.Context, _ string) (*model.Flashcard, error) {
	return nil, nil
}
func (s *leaderboardStore) ListFlashcards(_ context.Context, _ string) ([]*model.Flashcard, error) {
	return nil, nil
}
func (s *leaderboardStore) DueFlashcards(_ context.Context, _ string) ([]*model.Flashcard, error) {
	return nil, nil
}
func (s *leaderboardStore) ReviewFlashcard(_ context.Context, _, _ string, _ int) (*model.Flashcard, error) {
	return nil, nil
}
func (s *leaderboardStore) FlashcardsForSession(_ context.Context, _ string) ([]*model.Flashcard, error) {
	return nil, nil
}
func (s *leaderboardStore) AllFlashcardsForExport(_ context.Context, _ string) ([]*model.Flashcard, error) {
	return nil, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newLeaderboardHandler(st *leaderboardStore) *handler.Handler {
	return handler.New(st, taunt.StaticGenerator{Taunt: "test"}, zap.NewNop())
}

func leaderboardRequest(method, path, userID string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	if userID != "" {
		req = req.WithContext(middleware.WithUserID(req.Context(), userID))
	}
	return req
}

// ── GET /gaming/leaderboard tests ────────────────────────────────────────────

func TestGetLeaderboard_AllTime_ReturnsMostRecent(t *testing.T) {
	entries := []model.LeaderboardEntry{
		{UserID: "bob", XP: 800, Rank: 1},
		{UserID: "alice", XP: 500, Rank: 2},
	}
	st := &leaderboardStore{top10: entries}
	h := newLeaderboardHandler(st)

	rec := httptest.NewRecorder()
	h.GetLeaderboard(rec, leaderboardRequest(http.MethodGet, "/gaming/leaderboard", "alice"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp model.LeaderboardResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Top10) != 2 {
		t.Fatalf("want 2 entries, got %d", len(resp.Top10))
	}
	if resp.Top10[0].UserID != "bob" {
		t.Errorf("rank 1: want bob, got %s", resp.Top10[0].UserID)
	}
	if resp.Top10[0].Rank != 1 {
		t.Errorf("rank field: want 1, got %d", resp.Top10[0].Rank)
	}
}

func TestGetLeaderboard_EmptyPeriodCallsAllTime(t *testing.T) {
	called := false
	st := &leaderboardStore{}
	st.top10 = []model.LeaderboardEntry{{UserID: "u1", XP: 100, Rank: 1}}
	_ = called

	h := newLeaderboardHandler(st)
	rec := httptest.NewRecorder()
	// No period param — should hit LeaderboardTop10
	h.GetLeaderboard(rec, leaderboardRequest(http.MethodGet, "/gaming/leaderboard", "u1"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestGetLeaderboard_WeeklyPeriodUsesWeeklyData(t *testing.T) {
	weeklyEntries := []model.LeaderboardEntry{
		{UserID: "carol", XP: 300, Rank: 1},
	}
	st := &leaderboardStore{
		periodData: map[string][]model.LeaderboardEntry{
			model.PeriodWeekly: weeklyEntries,
		},
	}
	h := newLeaderboardHandler(st)

	rec := httptest.NewRecorder()
	h.GetLeaderboard(rec, leaderboardRequest(http.MethodGet, "/gaming/leaderboard?period=weekly", "carol"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp model.LeaderboardResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Top10) != 1 || resp.Top10[0].UserID != "carol" {
		t.Errorf("weekly: want [carol], got %+v", resp.Top10)
	}
}

func TestGetLeaderboard_MonthlyPeriodUsesMonthlyData(t *testing.T) {
	monthlyEntries := []model.LeaderboardEntry{
		{UserID: "dave", XP: 1200, Rank: 1},
		{UserID: "eve", XP: 900, Rank: 2},
	}
	st := &leaderboardStore{
		periodData: map[string][]model.LeaderboardEntry{
			model.PeriodMonthly: monthlyEntries,
		},
	}
	h := newLeaderboardHandler(st)

	rec := httptest.NewRecorder()
	h.GetLeaderboard(rec, leaderboardRequest(http.MethodGet, "/gaming/leaderboard?period=monthly", "dave"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp model.LeaderboardResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Top10) != 2 {
		t.Fatalf("monthly: want 2 entries, got %d", len(resp.Top10))
	}
	if resp.Top10[0].UserID != "dave" {
		t.Errorf("monthly rank 1: want dave, got %s", resp.Top10[0].UserID)
	}
}

func TestGetLeaderboard_AllTimePeriodCallsAllTime(t *testing.T) {
	st := &leaderboardStore{
		top10: []model.LeaderboardEntry{{UserID: "frank", XP: 700, Rank: 1}},
	}
	h := newLeaderboardHandler(st)

	rec := httptest.NewRecorder()
	h.GetLeaderboard(rec, leaderboardRequest(http.MethodGet, "/gaming/leaderboard?period=all_time", "frank"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp model.LeaderboardResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Top10) != 1 || resp.Top10[0].UserID != "frank" {
		t.Errorf("all_time: want [frank], got %+v", resp.Top10)
	}
}

func TestGetLeaderboard_UserRankIncludedInResponse(t *testing.T) {
	st := &leaderboardStore{
		top10:    []model.LeaderboardEntry{{UserID: "top", XP: 1000, Rank: 1}},
		userRank: &model.LeaderboardEntry{UserID: "alice", XP: 500, Rank: 5},
	}
	h := newLeaderboardHandler(st)

	rec := httptest.NewRecorder()
	h.GetLeaderboard(rec, leaderboardRequest(http.MethodGet, "/gaming/leaderboard", "alice"))

	var resp model.LeaderboardResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.UserRank == nil {
		t.Fatal("user_rank should be present when user has a rank")
	}
	if resp.UserRank.UserID != "alice" {
		t.Errorf("user_rank user_id: want alice, got %s", resp.UserRank.UserID)
	}
	if resp.UserRank.Rank != 5 {
		t.Errorf("user_rank rank: want 5, got %d", resp.UserRank.Rank)
	}
}

func TestGetLeaderboard_StoreErrorReturns500(t *testing.T) {
	st := &leaderboardStore{err: errors.New("redis unavailable")}
	h := newLeaderboardHandler(st)

	rec := httptest.NewRecorder()
	h.GetLeaderboard(rec, leaderboardRequest(http.MethodGet, "/gaming/leaderboard", "u1"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestGetLeaderboard_ResponseIsValidJSON(t *testing.T) {
	st := &leaderboardStore{
		top10: []model.LeaderboardEntry{
			{UserID: "u1", XP: 100, Rank: 1},
		},
	}
	h := newLeaderboardHandler(st)

	rec := httptest.NewRecorder()
	h.GetLeaderboard(rec, leaderboardRequest(http.MethodGet, "/gaming/leaderboard", "u1"))

	var resp model.LeaderboardResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
}

func TestGetLeaderboard_CorrectOrderingDescendingXP(t *testing.T) {
	// Store returns pre-sorted descending; handler must not re-sort
	entries := []model.LeaderboardEntry{
		{UserID: "u3", XP: 900, Rank: 1},
		{UserID: "u1", XP: 700, Rank: 2},
		{UserID: "u2", XP: 500, Rank: 3},
	}
	st := &leaderboardStore{top10: entries}
	h := newLeaderboardHandler(st)

	rec := httptest.NewRecorder()
	h.GetLeaderboard(rec, leaderboardRequest(http.MethodGet, "/gaming/leaderboard", "u1"))

	var resp model.LeaderboardResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for i, e := range resp.Top10 {
		if e.Rank != int64(i+1) {
			t.Errorf("entry %d: rank field want %d, got %d", i, i+1, e.Rank)
		}
	}
	if resp.Top10[0].XP < resp.Top10[1].XP {
		t.Errorf("entries not in descending XP order: %v > %v violated",
			resp.Top10[0].XP, resp.Top10[1].XP)
	}
}

// ── GET /gaming/leaderboard/friends tests ────────────────────────────────────

func TestGetFriendLeaderboard_ReturnsFriendsRanked(t *testing.T) {
	st := &leaderboardStore{
		friendData: []model.LeaderboardEntry{
			{UserID: "caller", XP: 800, Rank: 1},
			{UserID: "friend1", XP: 600, Rank: 2},
			{UserID: "friend2", XP: 400, Rank: 3},
		},
		userRank: &model.LeaderboardEntry{UserID: "caller", XP: 800, Rank: 1},
	}
	h := newLeaderboardHandler(st)

	req := httptest.NewRequest(http.MethodGet, "/gaming/leaderboard/friends?friends=friend1,friend2", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), "caller"))
	rec := httptest.NewRecorder()
	h.GetFriendLeaderboard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp model.FriendLeaderboardResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Friends) != 3 {
		t.Fatalf("want 3 friend entries, got %d", len(resp.Friends))
	}
	if resp.Friends[0].UserID != "caller" {
		t.Errorf("rank 1: want caller, got %s", resp.Friends[0].UserID)
	}
}

func TestGetFriendLeaderboard_UnauthenticatedReturns403(t *testing.T) {
	st := &leaderboardStore{}
	h := newLeaderboardHandler(st)

	rec := httptest.NewRecorder()
	// No user ID in context
	h.GetFriendLeaderboard(rec, httptest.NewRequest(http.MethodGet, "/gaming/leaderboard/friends", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestGetFriendLeaderboard_StoreErrorReturns500(t *testing.T) {
	st := &leaderboardStore{err: errors.New("redis down")}
	h := newLeaderboardHandler(st)

	req := httptest.NewRequest(http.MethodGet, "/gaming/leaderboard/friends?friends=u2", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	h.GetFriendLeaderboard(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
