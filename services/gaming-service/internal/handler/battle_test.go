package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/handler"
	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/taunt"
)

// ── battleStore fake ──────────────────────────────────────────────────────────

// battleStore is a minimal Storer stub that covers only the methods exercised
// by the Attack handler. All other Storer methods are no-ops.
type battleStore struct {
	session    *model.BattleSession
	savedTaunt string
	// tauntPool is keyed by "bossID:round". Present entry means cache hit.
	tauntPool map[string]string
	// tauntSaved is closed by SaveTaunt so tests can wait for the async persist.
	tauntSaved chan struct{}
}

func (b *battleStore) GetBattleSession(_ context.Context, _ string) (*model.BattleSession, error) {
	return b.session, nil
}
func (b *battleStore) SaveBattleSession(_ context.Context, s *model.BattleSession) error {
	b.session = s
	return nil
}
func (b *battleStore) DeleteBattleSession(_ context.Context, _ string) error { return nil }
func (b *battleStore) RecordBattleResult(_ context.Context, _ *model.BattleResult) error {
	return nil
}
func (b *battleStore) GetXPAndLevel(_ context.Context, _ string) (int64, int, error) {
	return 0, 1, nil
}
func (b *battleStore) UpsertXP(_ context.Context, _ string, _ int64, _ int) error { return nil }
func (b *battleStore) DeductGems(_ context.Context, _ string, _ int) (int, error) { return 5, nil }

// SaveTaunt records the taunt so tests can assert it was persisted.
// Closes tauntSaved (if set) to unblock tests waiting on the async goroutine.
func (b *battleStore) SaveTaunt(_ context.Context, bossID string, round int, t string) error {
	b.savedTaunt = t
	if b.tauntSaved != nil {
		close(b.tauntSaved)
	}
	return nil
}

// GetRandomTaunt returns a cached taunt from the test pool, or (false) on miss.
func (b *battleStore) GetRandomTaunt(_ context.Context, bossID string, round int) (string, bool, error) {
	key := fmt.Sprintf("%s:%d", bossID, round)
	t, ok := b.tauntPool[key]
	return t, ok, nil
}

// ── Unused Storer methods — satisfy the interface ─────────────────────────────

func (b *battleStore) GetProfile(_ context.Context, _ string) (*model.Profile, error) {
	return nil, nil
}
func (b *battleStore) StreakCheckin(_ context.Context, _ string) (int, int, bool, error) {
	return 0, 0, false, nil
}
func (b *battleStore) LeaderboardUpdate(_ context.Context, _ string, _ int64) error { return nil }
func (b *battleStore) LeaderboardUpdateCourse(_ context.Context, _, _ string, _ int64) error {
	return nil
}
func (b *battleStore) LeaderboardTop10(_ context.Context, _ string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	return nil, nil, nil
}
func (b *battleStore) LeaderboardGetPeriod(_ context.Context, _, _ string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	return nil, nil, nil
}
func (b *battleStore) LeaderboardGetCourse(_ context.Context, _, _ string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	return nil, nil, nil
}
func (b *battleStore) LeaderboardGetFriends(_ context.Context, _ string, _ []string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	return nil, nil, nil
}
func (b *battleStore) RandomQuote(_ context.Context) (*model.Quote, error) { return nil, nil }
func (b *battleStore) GetRandomQuestions(_ context.Context, _ string, _ int) ([]*model.Question, error) {
	return nil, nil
}
func (b *battleStore) GetQuestion(_ context.Context, _ string) (*model.Question, error) {
	return nil, nil
}
func (b *battleStore) CreateQuizSession(_ context.Context, _ string, _, _ *string, _ []string) (*model.QuizSession, error) {
	return nil, nil
}
func (b *battleStore) GetQuizSession(_ context.Context, _ string) (*model.QuizSession, error) {
	return nil, nil
}
func (b *battleStore) RecordAnswer(_ context.Context, _, _, _, _ string, _ bool, _, _ int, _ *int) (*model.QuizSession, error) {
	return nil, nil
}
func (b *battleStore) GetHintIndex(_ context.Context, _, _ string) (int, error) { return 0, nil }
func (b *battleStore) IncrHintIndex(_ context.Context, _, _, _ string) (int, int, error) {
	return 0, 0, nil
}
func (b *battleStore) GetDailyQuests(_ context.Context, _ string) ([]model.QuestState, error) {
	return nil, nil
}
func (b *battleStore) UpdateQuestProgress(_ context.Context, _ string, _ string) ([]model.QuestState, int, int, error) {
	return nil, 0, 0, nil
}
func (b *battleStore) AwardQuestRewards(_ context.Context, _ string, _, _ int) (int64, int, bool, int, error) {
	return 0, 0, false, 0, nil
}
func (b *battleStore) CreateAssessmentSession(_ context.Context, _ string) (*model.AssessmentSession, error) {
	return nil, nil
}
func (b *battleStore) GetAssessmentSession(_ context.Context, _ string) (*model.AssessmentSession, error) {
	return nil, nil
}
func (b *battleStore) RecordAssessmentAnswer(_ context.Context, _, _, _, _ string) (*model.AssessmentSession, error) {
	return nil, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// activeSession returns a BattleSession with enough HP to survive one wrong
// answer without ending the battle.
func activeSession() *model.BattleSession {
	return &model.BattleSession{
		SessionID:    "sess-1",
		UserID:       "u1",
		BossID:       "algebra_dragon",
		Phase:        model.PhaseActive,
		PlayerHP:     100,
		PlayerMaxHP:  100,
		BossHP:       200,
		BossMaxHP:    200,
		BossAttack:   5,
		BossDefense:  5,
		Turn:         0,
		ActivePowers: []model.ActivePowerUp{},
		XPReward:     500,
		GemReward:    10,
		StartedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(30 * time.Minute),
	}
}

// attackRequest builds a POST /gaming/boss/attack request with the caller u1
// injected into context (bypassing JWT middleware).
func attackRequest(sessionID string, correct bool, baseDmg int) *http.Request {
	b, _ := json.Marshal(model.AttackRequest{
		SessionID:     sessionID,
		AnswerCorrect: correct,
		BaseDamage:    baseDmg,
	})
	req := httptest.NewRequest(http.MethodPost, "/gaming/boss/attack", bytes.NewBuffer(b))
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	return req
}

// newBattleHandler creates a Handler wired with the given store and generator.
func newBattleHandler(s handler.Storer, g taunt.Generator) *handler.Handler {
	return handler.New(s, g, zap.NewNop())
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestAttack_WrongAnswer_TauntServedFromCache(t *testing.T) {
	s := &battleStore{
		session: activeSession(),
		tauntPool: map[string]string{
			"algebra_dragon:1": "Your algebra fails you, as expected!",
		},
	}
	h := newBattleHandler(s, taunt.StaticGenerator{Taunt: "generated — should not appear"})

	rr := httptest.NewRecorder()
	h.Attack(rr, attackRequest("sess-1", false, 20))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp model.AttackResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Taunt != "Your algebra fails you, as expected!" {
		t.Fatalf("expected cached taunt, got %q", resp.Taunt)
	}
}

func TestAttack_WrongAnswer_TauntGeneratedAndSavedOnCacheMiss(t *testing.T) {
	s := &battleStore{
		session:    activeSession(),
		tauntPool:  map[string]string{},
		tauntSaved: make(chan struct{}),
	}
	h := newBattleHandler(s, taunt.StaticGenerator{Taunt: "fresh generated taunt"})

	rr := httptest.NewRecorder()
	h.Attack(rr, attackRequest("sess-1", false, 20))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp model.AttackResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Taunt != "fresh generated taunt" {
		t.Fatalf("expected generated taunt, got %q", resp.Taunt)
	}

	// SaveTaunt is called in a background goroutine; wait for it with a timeout.
	select {
	case <-s.tauntSaved:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for taunt to be persisted")
	}
	if s.savedTaunt != "fresh generated taunt" {
		t.Fatalf("generated taunt not persisted to store, got %q", s.savedTaunt)
	}
}

func TestAttack_CorrectAnswer_NoTauntInResponse(t *testing.T) {
	s := &battleStore{session: activeSession()}
	h := newBattleHandler(s, taunt.StaticGenerator{Taunt: "should not appear"})

	rr := httptest.NewRecorder()
	h.Attack(rr, attackRequest("sess-1", true, 20))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp model.AttackResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Taunt != "" {
		t.Fatalf("expected no taunt on correct answer, got %q", resp.Taunt)
	}
}

func TestAttack_TauntGeneratorFails_AttackStillSucceeds(t *testing.T) {
	// When the AI gateway is unreachable, the attack response must still be 200
	// with an empty taunt rather than a 500.
	s := &battleStore{
		session:   activeSession(),
		tauntPool: map[string]string{},
	}
	// Point at a server that's already closed so every request errors.
	deadSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadSrv.Close()
	gen := taunt.NewLiteLLMGenerator(deadSrv.URL, "key",
		taunt.WithHTTPClient(deadSrv.Client()),
	)

	h := newBattleHandler(s, gen)

	rr := httptest.NewRecorder()
	h.Attack(rr, attackRequest("sess-1", false, 20))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 even on taunt failure, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAttack_SessionNotFound_Returns404(t *testing.T) {
	s := &battleStore{session: nil}
	h := newBattleHandler(s, taunt.StaticGenerator{})

	rr := httptest.NewRecorder()
	h.Attack(rr, attackRequest("missing", false, 0))

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}
