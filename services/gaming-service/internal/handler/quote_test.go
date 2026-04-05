package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/taunt"
)

// noopTaunter satisfies taunt.Generator with a no-op implementation for tests.
type noopTaunter struct{}

func (noopTaunter) Generate(_ context.Context, _, _, _ string, _ int) (string, error) {
	return "boo", nil
}

var _ taunt.Generator = noopTaunter{}

// quoteStorer is a minimal Storer stub for quote handler tests.
type quoteStorer struct {
	randomQuote        func(ctx context.Context) (*model.Quote, error)
	randomQuoteForUser func(ctx context.Context, userID, quotectx string) (*model.Quote, error)
}

// Satisfy the full Storer interface with no-ops for all other methods.
func (s *quoteStorer) GetXPAndLevel(ctx context.Context, u string) (int64, int, error) {
	return 0, 1, nil
}
func (s *quoteStorer) UpsertXP(ctx context.Context, u string, x int64, l int) error { return nil }
func (s *quoteStorer) GetProfile(ctx context.Context, u string) (*model.Profile, error) {
	return nil, nil
}
func (s *quoteStorer) StreakCheckin(ctx context.Context, u string) (int, int, bool, error) {
	return 0, 0, false, nil
}
func (s *quoteStorer) LeaderboardUpdate(ctx context.Context, u string, x int64) error { return nil }
func (s *quoteStorer) LeaderboardUpdateCourse(ctx context.Context, u, c string, x int64) error {
	return nil
}
func (s *quoteStorer) LeaderboardTop10(ctx context.Context, u string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	return nil, nil, nil
}
func (s *quoteStorer) LeaderboardGetPeriod(ctx context.Context, u, p string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	return nil, nil, nil
}
func (s *quoteStorer) LeaderboardGetCourse(ctx context.Context, u, c string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	return nil, nil, nil
}
func (s *quoteStorer) LeaderboardGetFriends(ctx context.Context, u string, f []string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	return nil, nil, nil
}
func (s *quoteStorer) RandomQuote(ctx context.Context) (*model.Quote, error) {
	if s.randomQuote != nil {
		return s.randomQuote(ctx)
	}
	return &model.Quote{ID: 1, Quote: "default", Attribution: "x", Context: "session_start"}, nil
}
func (s *quoteStorer) RandomQuoteForUser(ctx context.Context, userID, quotectx string) (*model.Quote, error) {
	if s.randomQuoteForUser != nil {
		return s.randomQuoteForUser(ctx, userID, quotectx)
	}
	return &model.Quote{ID: 2, Quote: "user quote", Attribution: "x", Context: quotectx}, nil
}
func (s *quoteStorer) GetRandomQuestions(ctx context.Context, t string, n int) ([]*model.Question, error) {
	return nil, nil
}
func (s *quoteStorer) GetQuestion(ctx context.Context, id string) (*model.Question, error) {
	return nil, nil
}
func (s *quoteStorer) CreateQuizSession(ctx context.Context, u string, t, c *string, qs []string) (*model.QuizSession, error) {
	return nil, nil
}
func (s *quoteStorer) GetQuizSession(ctx context.Context, id string) (*model.QuizSession, error) {
	return nil, nil
}
func (s *quoteStorer) RecordAnswer(ctx context.Context, sID, uID, qID, k string, ok bool, h, x int, ms *int) (*model.QuizSession, error) {
	return nil, nil
}
func (s *quoteStorer) GetHintIndex(ctx context.Context, sID, qID string) (int, error) { return 0, nil }
func (s *quoteStorer) IncrHintIndex(ctx context.Context, sID, qID, uID string) (int, int, error) {
	return 0, 0, nil
}
func (s *quoteStorer) GetDailyQuests(ctx context.Context, u string) ([]model.QuestState, error) {
	return nil, nil
}
func (s *quoteStorer) UpdateQuestProgress(ctx context.Context, u, a string) ([]model.QuestState, int, int, error) {
	return nil, 0, 0, nil
}
func (s *quoteStorer) AwardQuestRewards(ctx context.Context, u string, xd, gd int) (int64, int, bool, int, error) {
	return 0, 0, false, 0, nil
}
func (s *quoteStorer) SaveBattleSession(ctx context.Context, s2 *model.BattleSession) error {
	return nil
}
func (s *quoteStorer) GetBattleSession(ctx context.Context, id string) (*model.BattleSession, error) {
	return nil, nil
}
func (s *quoteStorer) DeleteBattleSession(ctx context.Context, id string) error { return nil }
func (s *quoteStorer) RecordBattleResult(ctx context.Context, r *model.BattleResult) error {
	return nil
}
func (s *quoteStorer) DeductGems(ctx context.Context, u string, a int) (int, error) { return 0, nil }
func (s *quoteStorer) SaveTaunt(ctx context.Context, bossID string, round int, tauntText string) error {
	return nil
}
func (s *quoteStorer) GetRandomTaunt(ctx context.Context, bossID string, round int) (string, bool, error) {
	return "", false, nil
}
func (s *quoteStorer) GrantAchievement(ctx context.Context, userID, achievementType, badgeName string) (*model.Achievement, bool, error) {
	return nil, false, nil
}
func (s *quoteStorer) GetAchievements(ctx context.Context, userID string) ([]model.Achievement, error) {
	return nil, nil
}
func (s *quoteStorer) AddCosmeticItem(ctx context.Context, userID, key, value string) error {
	return nil
}
func (s *quoteStorer) CreateAssessmentSession(ctx context.Context, u string) (*model.AssessmentSession, error) {
	return nil, nil
}
func (s *quoteStorer) GetAssessmentSession(ctx context.Context, id string) (*model.AssessmentSession, error) {
	return nil, nil
}
func (s *quoteStorer) RecordAssessmentAnswer(ctx context.Context, sID, uID, qID, k string) (*model.AssessmentSession, error) {
	return nil, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newQuoteHandler(st *quoteStorer) *Handler {
	return New(st, noopTaunter{}, zap.NewNop())
}

func quoteRequest(userID, quotectx string) *http.Request {
	target := "/gaming/quotes/random"
	if quotectx != "" {
		target += "?context=" + quotectx
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if userID != "" {
		req = req.WithContext(middleware.WithUserID(req.Context(), userID))
	}
	return req
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestRandomQuote_UnauthenticatedUsesPlainRandomQuote(t *testing.T) {
	called := false
	st := &quoteStorer{
		randomQuote: func(ctx context.Context) (*model.Quote, error) {
			called = true
			return &model.Quote{ID: 1, Quote: "q", Attribution: "a", Context: "session_start"}, nil
		},
	}
	h := newQuoteHandler(st)

	rec := httptest.NewRecorder()
	h.RandomQuote(rec, quoteRequest("", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !called {
		t.Error("expected RandomQuote(ctx) to be called for unauthenticated request")
	}
}

func TestRandomQuote_AuthenticatedUsesRandomQuoteForUser(t *testing.T) {
	var gotUserID, gotCtx string
	st := &quoteStorer{
		randomQuoteForUser: func(ctx context.Context, userID, quotectx string) (*model.Quote, error) {
			gotUserID = userID
			gotCtx = quotectx
			return &model.Quote{ID: 2, Quote: "q", Attribution: "a", Context: quotectx}, nil
		},
	}
	h := newQuoteHandler(st)

	rec := httptest.NewRecorder()
	h.RandomQuote(rec, quoteRequest("user-abc", "boss_fight"))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotUserID != "user-abc" {
		t.Errorf("expected userID=user-abc, got %q", gotUserID)
	}
	if gotCtx != "boss_fight" {
		t.Errorf("expected quotectx=boss_fight, got %q", gotCtx)
	}
}

func TestRandomQuote_ContextParamForwardedToStore(t *testing.T) {
	var gotCtx string
	st := &quoteStorer{
		randomQuoteForUser: func(_ context.Context, _, quotectx string) (*model.Quote, error) {
			gotCtx = quotectx
			return &model.Quote{ID: 3, Quote: "q", Attribution: "a", Context: quotectx}, nil
		},
	}
	h := newQuoteHandler(st)

	for _, ctx := range []string{"session_start", "victory", "streak", ""} {
		gotCtx = "unset"
		rec := httptest.NewRecorder()
		h.RandomQuote(rec, quoteRequest("user-xyz", ctx))
		if rec.Code != http.StatusOK {
			t.Errorf("[%s] expected 200, got %d", ctx, rec.Code)
		}
		if gotCtx != ctx {
			t.Errorf("[%s] expected quotectx=%q forwarded, got %q", ctx, ctx, gotCtx)
		}
	}
}

func TestRandomQuote_StoreErrorReturns500(t *testing.T) {
	st := &quoteStorer{
		randomQuoteForUser: func(_ context.Context, _, _ string) (*model.Quote, error) {
			return nil, errors.New("db down")
		},
	}
	h := newQuoteHandler(st)

	rec := httptest.NewRecorder()
	h.RandomQuote(rec, quoteRequest("user-abc", "correct"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestRandomQuote_ResponseBodyIsValidJSON(t *testing.T) {
	st := &quoteStorer{}
	h := newQuoteHandler(st)

	rec := httptest.NewRecorder()
	h.RandomQuote(rec, quoteRequest("user-abc", ""))

	var q model.Quote
	if err := json.NewDecoder(rec.Body).Decode(&q); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if q.ID == 0 {
		t.Error("expected non-zero quote ID in response")
	}
}
