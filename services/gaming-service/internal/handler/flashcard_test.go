package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/handler"
	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
)

// ── flashcardStore fake ───────────────────────────────────────────────────────

// flashcardStore is a minimal Storer stub that covers only the flashcard
// handler methods. All other Storer methods are forwarded to noopStore.
type flashcardStore struct {
	noopStore
	// cards is the in-memory flashcard map keyed by ID.
	cards map[string]*model.Flashcard
	// sessionCards maps sessionID → slice of cards (for FlashcardsForSession).
	sessionCards map[string][]*model.Flashcard
	// sessions is the quiz session map keyed by sessionID.
	sessions map[string]*model.QuizSession
	// questions is the question map keyed by ID.
	questions map[string]*model.Question
	// createErr, if non-nil, is returned by CreateFlashcard.
	createErr error
	// reviewFn, if non-nil, overrides ReviewFlashcard behaviour.
	reviewFn func(cardID, userID string, quality int) (*model.Flashcard, error)
}

func (f *flashcardStore) CreateFlashcard(_ context.Context, card *model.Flashcard) (*model.Flashcard, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	card.ID = "card-" + card.Front[:3]
	card.CreatedAt = time.Now().UTC()
	if f.cards == nil {
		f.cards = make(map[string]*model.Flashcard)
	}
	f.cards[card.ID] = card
	return card, nil
}

func (f *flashcardStore) GetFlashcard(_ context.Context, id string) (*model.Flashcard, error) {
	c, ok := f.cards[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return c, nil
}

func (f *flashcardStore) ListFlashcards(_ context.Context, userID string) ([]*model.Flashcard, error) {
	var out []*model.Flashcard
	for _, c := range f.cards {
		if c.UserID == userID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *flashcardStore) DueFlashcards(_ context.Context, userID string) ([]*model.Flashcard, error) {
	now := time.Now().UTC()
	var out []*model.Flashcard
	for _, c := range f.cards {
		if c.UserID == userID && !c.NextReviewAt.After(now) {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *flashcardStore) ReviewFlashcard(_ context.Context, cardID, userID string, quality int) (*model.Flashcard, error) {
	if f.reviewFn != nil {
		return f.reviewFn(cardID, userID, quality)
	}
	c, ok := f.cards[cardID]
	if !ok {
		return nil, errors.New("not found")
	}
	c.IntervalDays = quality + 1
	c.EaseFactor = 2.5
	return c, nil
}

func (f *flashcardStore) FlashcardsForSession(_ context.Context, sessionID string) ([]*model.Flashcard, error) {
	return f.sessionCards[sessionID], nil
}

func (f *flashcardStore) AllFlashcardsForExport(_ context.Context, userID string) ([]*model.Flashcard, error) {
	var out []*model.Flashcard
	for _, c := range f.cards {
		if c.UserID == userID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *flashcardStore) GetQuizSession(_ context.Context, sessionID string) (*model.QuizSession, error) {
	s, ok := f.sessions[sessionID]
	if !ok {
		return nil, errors.New("not found")
	}
	return s, nil
}

func (f *flashcardStore) GetQuestion(_ context.Context, questionID string) (*model.Question, error) {
	q, ok := f.questions[questionID]
	if !ok {
		return nil, errors.New("not found")
	}
	return q, nil
}

// ── noopStore — satisfies the full Storer interface with no-ops ───────────────

type noopStore struct{}

func (noopStore) GetXPAndLevel(_ context.Context, _ string) (int64, int, error) { return 0, 1, nil }
func (noopStore) UpsertXP(_ context.Context, _ string, _ int64, _ int) error    { return nil }
func (noopStore) GetProfile(_ context.Context, _ string) (*model.Profile, error) {
	return nil, nil
}
func (noopStore) StreakCheckin(_ context.Context, _ string) (int, int, bool, error) {
	return 0, 0, false, nil
}
func (noopStore) CreateStreakFreeze(_ context.Context, _ string, _ int) (int, time.Time, error) {
	return 0, time.Time{}, nil
}
func (noopStore) IsStreakFrozen(_ context.Context, _ string) (bool, error) { return false, nil }
func (noopStore) LeaderboardUpdate(_ context.Context, _ string, _ int64) error { return nil }
func (noopStore) LeaderboardUpdateCourse(_ context.Context, _, _ string, _ int64) error {
	return nil
}
func (noopStore) LeaderboardTop10(_ context.Context, _ string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	return nil, nil, nil
}
func (noopStore) LeaderboardGetPeriod(_ context.Context, _, _ string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	return nil, nil, nil
}
func (noopStore) LeaderboardGetCourse(_ context.Context, _, _ string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	return nil, nil, nil
}
func (noopStore) LeaderboardGetFriends(_ context.Context, _ string, _ []string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	return nil, nil, nil
}
func (noopStore) RandomQuote(_ context.Context) (*model.Quote, error)             { return nil, nil }
func (noopStore) RandomQuoteForUser(_ context.Context, _, _ string) (*model.Quote, error) {
	return nil, nil
}
func (noopStore) GetRandomQuestions(_ context.Context, _ string, _ int) ([]*model.Question, error) {
	return nil, nil
}
func (noopStore) GetQuestion(_ context.Context, _ string) (*model.Question, error) {
	return nil, nil
}
func (noopStore) CreateQuizSession(_ context.Context, _ string, _, _ *string, _ []string) (*model.QuizSession, error) {
	return nil, nil
}
func (noopStore) GetQuizSession(_ context.Context, _ string) (*model.QuizSession, error) {
	return nil, nil
}
func (noopStore) RecordAnswer(_ context.Context, _, _, _, _ string, _ bool, _, _ int, _ *int) (*model.QuizSession, error) {
	return nil, nil
}
func (noopStore) GetHintIndex(_ context.Context, _, _ string) (int, error)     { return 0, nil }
func (noopStore) IncrHintIndex(_ context.Context, _, _, _ string) (int, int, error) {
	return 0, 0, nil
}
func (noopStore) GetDailyQuests(_ context.Context, _ string) ([]model.QuestState, error) {
	return nil, nil
}
func (noopStore) UpdateQuestProgress(_ context.Context, _ string, _ string) ([]model.QuestState, int, int, error) {
	return nil, 0, 0, nil
}
func (noopStore) AwardQuestRewards(_ context.Context, _ string, _, _ int) (int64, int, bool, int, error) {
	return 0, 0, false, 0, nil
}
func (noopStore) SaveBattleSession(_ context.Context, _ *model.BattleSession) error { return nil }
func (noopStore) GetBattleSession(_ context.Context, _ string) (*model.BattleSession, error) {
	return nil, nil
}
func (noopStore) DeleteBattleSession(_ context.Context, _ string) error { return nil }
func (noopStore) RecordBattleResult(_ context.Context, _ *model.BattleResult) error { return nil }
func (noopStore) DeductGems(_ context.Context, _ string, _ int) (int, error)        { return 0, nil }
func (noopStore) SaveTaunt(_ context.Context, _ string, _ int, _ string) error      { return nil }
func (noopStore) GetRandomTaunt(_ context.Context, _ string, _ int) (string, bool, error) {
	return "", false, nil
}
func (noopStore) GrantAchievement(_ context.Context, _, _, _ string) (*model.Achievement, bool, error) {
	return nil, false, nil
}
func (noopStore) GetAchievements(_ context.Context, _ string) ([]model.Achievement, error) {
	return nil, nil
}
func (noopStore) AddCosmeticItem(_ context.Context, _, _, _ string) error { return nil }
func (noopStore) CreateAssessmentSession(_ context.Context, _ string) (*model.AssessmentSession, error) {
	return nil, nil
}
func (noopStore) GetAssessmentSession(_ context.Context, _ string) (*model.AssessmentSession, error) {
	return nil, nil
}
func (noopStore) RecordAssessmentAnswer(_ context.Context, _, _, _, _ string) (*model.AssessmentSession, error) {
	return nil, nil
}
func (noopStore) CreateFlashcard(_ context.Context, _ *model.Flashcard) (*model.Flashcard, error) {
	return nil, nil
}
func (noopStore) GetFlashcard(_ context.Context, _ string) (*model.Flashcard, error) {
	return nil, nil
}
func (noopStore) ListFlashcards(_ context.Context, _ string) ([]*model.Flashcard, error) {
	return nil, nil
}
func (noopStore) DueFlashcards(_ context.Context, _ string) ([]*model.Flashcard, error) {
	return nil, nil
}
func (noopStore) ReviewFlashcard(_ context.Context, _, _ string, _ int) (*model.Flashcard, error) {
	return nil, nil
}
func (noopStore) FlashcardsForSession(_ context.Context, _ string) ([]*model.Flashcard, error) {
	return nil, nil
}
func (noopStore) AllFlashcardsForExport(_ context.Context, _ string) ([]*model.Flashcard, error) {
	return nil, nil
}
func (noopStore) BuyPowerUp(_ context.Context, _ string, _ model.PowerUpType, _ int) (int, int, error) {
	return 0, 0, nil
}
func (noopStore) GetDefeatedBossIDs(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (noopStore) GetChapterMastery(_ context.Context, _ string, _ []string) (float64, error) {
	return 0.0, nil
}
func (noopStore) GetChapterMasteryBatch(_ context.Context, _ string, _ map[string][]string) (map[string]float64, error) {
	return map[string]float64{}, nil
}
func (noopStore) GetBattle(_ context.Context, _ string) (*model.BattleSession, error) {
	return nil, nil
}
func (noopStore) UpdateBattleState(_ context.Context, _ *model.BattleSession) error { return nil }

// ── Helpers ───────────────────────────────────────────────────────────────────

// newFlashcardHandler creates a Handler wired with the given store.
func newFlashcardHandler(s handler.Storer) *handler.Handler {
	return handler.New(s, nil, zap.NewNop())
}

// completedSession returns a quiz session that is in completed state and owns questionIDs.
func completedSession(sessionID, userID string, questionIDs []string) *model.QuizSession {
	return &model.QuizSession{
		ID:          sessionID,
		UserID:      userID,
		Status:      "completed",
		QuestionIDs: questionIDs,
	}
}

// makeQuestion builds a minimal Question with a correct option.
func makeQuestion(id, text, correctKey string) *model.Question {
	return &model.Question{
		ID:          id,
		Question:    text,
		Options:     []model.QuizOption{{Key: correctKey, Text: "the answer"}},
		CorrectKey:  correctKey,
		Explanation: "because reasons",
		Topic:       "math",
	}
}

// makeCard returns a flashcard owned by userID with NextReviewAt in the past (due now).
func makeCard(id, userID string) *model.Flashcard {
	return &model.Flashcard{
		ID:           id,
		UserID:       userID,
		Front:        "What is 2+2?",
		Back:         "4",
		Source:       "manual",
		EaseFactor:   2.5,
		IntervalDays: 1,
		NextReviewAt: time.Now().Add(-time.Hour),
		CreatedAt:    time.Now(),
	}
}

// ── GenerateFlashcards tests ──────────────────────────────────────────────────

// TestGenerateFlashcards_HappyPath verifies that cards are created for each
// question in a completed session.
func TestGenerateFlashcards_HappyPath(t *testing.T) {
	s := &flashcardStore{
		sessions:  map[string]*model.QuizSession{"sess-1": completedSession("sess-1", "u1", []string{"q1", "q2"})},
		questions: map[string]*model.Question{"q1": makeQuestion("q1", "1+1?", "B"), "q2": makeQuestion("q2", "2+2?", "C")},
		cards:     map[string]*model.Flashcard{},
	}
	h := newFlashcardHandler(s)

	body, _ := json.Marshal(model.GenerateFlashcardsRequest{SessionID: "sess-1"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/flashcards/generate", bytes.NewBuffer(body))
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()

	h.GenerateFlashcards(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d: %s", rr.Code, rr.Body)
	}
	var resp model.GenerateFlashcardsResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Created != 2 {
		t.Errorf("expected 2 cards created, got %d", resp.Created)
	}
}

// TestGenerateFlashcards_MissingAuth verifies 403 when context has no user ID.
func TestGenerateFlashcards_MissingAuth(t *testing.T) {
	s := &flashcardStore{}
	h := newFlashcardHandler(s)

	body, _ := json.Marshal(model.GenerateFlashcardsRequest{SessionID: "sess-1"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/flashcards/generate", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.GenerateFlashcards(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// TestGenerateFlashcards_SessionNotCompleted verifies 409 for pending sessions.
func TestGenerateFlashcards_SessionNotCompleted(t *testing.T) {
	sess := completedSession("sess-1", "u1", []string{"q1"})
	sess.Status = "pending"
	s := &flashcardStore{
		sessions: map[string]*model.QuizSession{"sess-1": sess},
	}
	h := newFlashcardHandler(s)

	body, _ := json.Marshal(model.GenerateFlashcardsRequest{SessionID: "sess-1"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/flashcards/generate", bytes.NewBuffer(body))
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()

	h.GenerateFlashcards(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rr.Code)
	}
}

// TestGenerateFlashcards_IdempotentSkipsExisting verifies that existing cards
// for the same session are not duplicated.
func TestGenerateFlashcards_IdempotentSkipsExisting(t *testing.T) {
	qID := "q1"
	existing := &model.Flashcard{
		ID:         "card-existing",
		UserID:     "u1",
		QuestionID: &qID,
		Front:      "1+1?",
	}
	s := &flashcardStore{
		sessions:     map[string]*model.QuizSession{"sess-1": completedSession("sess-1", "u1", []string{"q1"})},
		questions:    map[string]*model.Question{"q1": makeQuestion("q1", "1+1?", "B")},
		cards:        map[string]*model.Flashcard{"card-existing": existing},
		sessionCards: map[string][]*model.Flashcard{"sess-1": {existing}},
	}
	h := newFlashcardHandler(s)

	body, _ := json.Marshal(model.GenerateFlashcardsRequest{SessionID: "sess-1"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/flashcards/generate", bytes.NewBuffer(body))
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()

	h.GenerateFlashcards(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d", rr.Code)
	}
	var resp model.GenerateFlashcardsResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Created != 0 {
		t.Errorf("expected 0 new cards (all skipped), got %d", resp.Created)
	}
}

// ── ListFlashcards tests ──────────────────────────────────────────────────────

// TestListFlashcards_HappyPath verifies the due count and totals are correct.
func TestListFlashcards_HappyPath(t *testing.T) {
	dueCard := makeCard("c1", "u1")
	futureCard := makeCard("c2", "u1")
	futureCard.NextReviewAt = time.Now().Add(24 * time.Hour)

	s := &flashcardStore{
		cards: map[string]*model.Flashcard{"c1": dueCard, "c2": futureCard},
	}
	h := newFlashcardHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/gaming/flashcards", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()

	h.ListFlashcards(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rr.Code, rr.Body)
	}
	var resp model.ListFlashcardsResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Total != 2 {
		t.Errorf("expected 2 total, got %d", resp.Total)
	}
	if resp.DueCount != 1 {
		t.Errorf("expected 1 due, got %d", resp.DueCount)
	}
}

// TestListFlashcards_Unauthorized verifies 403 when no user in context.
func TestListFlashcards_Unauthorized(t *testing.T) {
	h := newFlashcardHandler(&flashcardStore{})
	req := httptest.NewRequest(http.MethodGet, "/gaming/flashcards", nil)
	rr := httptest.NewRecorder()
	h.ListFlashcards(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// ── DueFlashcards tests ───────────────────────────────────────────────────────

// TestDueFlashcards_ReturnsDueOnly verifies that only due cards are returned.
func TestDueFlashcards_ReturnsDueOnly(t *testing.T) {
	dueCard := makeCard("c1", "u1")
	futureCard := makeCard("c2", "u1")
	futureCard.NextReviewAt = time.Now().Add(24 * time.Hour)

	s := &flashcardStore{
		cards: map[string]*model.Flashcard{"c1": dueCard, "c2": futureCard},
	}
	h := newFlashcardHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/gaming/flashcards/due", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()

	h.DueFlashcards(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rr.Code)
	}
	var resp model.ListFlashcardsResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.DueCount != 1 {
		t.Errorf("expected 1 due card, got %d", resp.DueCount)
	}
}

// ── ReviewFlashcard tests ─────────────────────────────────────────────────────

// TestReviewFlashcard_HappyPath verifies a valid review updates the card.
func TestReviewFlashcard_HappyPath(t *testing.T) {
	card := makeCard("c1", "u1")
	s := &flashcardStore{
		cards: map[string]*model.Flashcard{"c1": card},
	}
	h := newFlashcardHandler(s)

	body, _ := json.Marshal(model.ReviewFlashcardRequest{Quality: 4})
	req := httptest.NewRequest(http.MethodPost, "/gaming/flashcards/c1/review", bytes.NewBuffer(body))
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	// Set chi URL parameter.
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("cardId", "c1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()

	h.ReviewFlashcard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rr.Code, rr.Body)
	}
	var resp model.ReviewFlashcardResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Card == nil {
		t.Error("expected card in response, got nil")
	}
}

// TestReviewFlashcard_InvalidQuality verifies 400 for quality out of range.
func TestReviewFlashcard_InvalidQuality(t *testing.T) {
	h := newFlashcardHandler(&flashcardStore{})

	body, _ := json.Marshal(model.ReviewFlashcardRequest{Quality: 6})
	req := httptest.NewRequest(http.MethodPost, "/gaming/flashcards/c1/review", bytes.NewBuffer(body))
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("cardId", "c1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()

	h.ReviewFlashcard(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestReviewFlashcard_ForbiddenCrossUser verifies 403 when the caller does not
// own the card.
func TestReviewFlashcard_ForbiddenCrossUser(t *testing.T) {
	card := makeCard("c1", "other-user")
	s := &flashcardStore{
		cards: map[string]*model.Flashcard{"c1": card},
	}
	h := newFlashcardHandler(s)

	body, _ := json.Marshal(model.ReviewFlashcardRequest{Quality: 3})
	req := httptest.NewRequest(http.MethodPost, "/gaming/flashcards/c1/review", bytes.NewBuffer(body))
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("cardId", "c1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()

	h.ReviewFlashcard(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 got %d", rr.Code)
	}
}

// ── ExportAnki tests ──────────────────────────────────────────────────────────

// TestExportAnki_HappyPath verifies a valid export returns an .apkg binary.
func TestExportAnki_HappyPath(t *testing.T) {
	card := makeCard("c1", "u1")
	s := &flashcardStore{
		cards: map[string]*model.Flashcard{"c1": card},
	}
	h := newFlashcardHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/gaming/flashcards/export/anki", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()

	h.ExportAnki(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", rr.Code, rr.Body)
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "application/octet-stream" {
		t.Errorf("expected application/octet-stream, got %s", ct)
	}
	if rr.Body.Len() == 0 {
		t.Error("expected non-empty .apkg body")
	}
}

// TestExportAnki_Unauthorized verifies 403 when no user in context.
func TestExportAnki_Unauthorized(t *testing.T) {
	h := newFlashcardHandler(&flashcardStore{})
	req := httptest.NewRequest(http.MethodGet, "/gaming/flashcards/export/anki", nil)
	rr := httptest.NewRecorder()
	h.ExportAnki(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// ── DueFlashcards error path ──────────────────────────────────────────────────

// TestDueFlashcards_Unauthorized verifies 403 when no user in context.
func TestDueFlashcards_Unauthorized(t *testing.T) {
	h := newFlashcardHandler(&flashcardStore{})
	req := httptest.NewRequest(http.MethodGet, "/gaming/flashcards/due", nil)
	rr := httptest.NewRecorder()
	h.DueFlashcards(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// ── ListFlashcards error path ─────────────────────────────────────────────────

// TestListFlashcards_EmptyList verifies 200 with zero totals when no cards exist.
func TestListFlashcards_EmptyList(t *testing.T) {
	h := newFlashcardHandler(&flashcardStore{cards: map[string]*model.Flashcard{}})
	req := httptest.NewRequest(http.MethodGet, "/gaming/flashcards", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()
	h.ListFlashcards(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp model.ListFlashcardsResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Total != 0 {
		t.Errorf("expected 0 total, got %d", resp.Total)
	}
}

// ── ReviewFlashcard error path ────────────────────────────────────────────────

// TestReviewFlashcard_CardNotFound verifies the store error surfaces as expected.
func TestReviewFlashcard_CardNotFound(t *testing.T) {
	s := &flashcardStore{cards: map[string]*model.Flashcard{}}
	h := newFlashcardHandler(s)
	body, _ := json.Marshal(model.ReviewFlashcardRequest{Quality: 3})
	req := httptest.NewRequest(http.MethodPost, "/gaming/flashcards/missing/review", bytes.NewBuffer(body))
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("cardId", "missing")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()
	h.ReviewFlashcard(rr, req)
	// Store returns "not found" error → handler returns 500 (internal store error path)
	if rr.Code == http.StatusOK {
		t.Error("expected non-200 response for missing card")
	}
}

// TestGenerateFlashcards_StoreCreateError verifies 500 when CreateFlashcard fails.
func TestGenerateFlashcards_StoreCreateError(t *testing.T) {
	s := &flashcardStore{
		sessions:  map[string]*model.QuizSession{"sess-1": completedSession("sess-1", "u1", []string{"q1"})},
		questions: map[string]*model.Question{"q1": makeQuestion("q1", "1+1?", "B")},
		cards:     map[string]*model.Flashcard{},
		createErr: errors.New("db write failed"),
	}
	h := newFlashcardHandler(s)
	body, _ := json.Marshal(model.GenerateFlashcardsRequest{SessionID: "sess-1"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/flashcards/generate", bytes.NewBuffer(body))
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()
	h.GenerateFlashcards(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// TestGenerateFlashcards_SessionStoreError verifies 404 when GetQuizSession fails.
func TestGenerateFlashcards_SessionStoreError(t *testing.T) {
	s := &flashcardStore{
		sessions: map[string]*model.QuizSession{}, // session not found → store returns error
	}
	h := newFlashcardHandler(s)
	body, _ := json.Marshal(model.GenerateFlashcardsRequest{SessionID: "missing-sess"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/flashcards/generate", bytes.NewBuffer(body))
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()
	h.GenerateFlashcards(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// TestListFlashcards_StoreError verifies 500 when ListFlashcards store call fails.
// The noopStore is not used here — we need a store that returns an error from ListFlashcards.
type listErrorStore struct{ noopStore }

func (listErrorStore) ListFlashcards(_ context.Context, _ string) ([]*model.Flashcard, error) {
	return nil, errors.New("db error")
}

func TestListFlashcards_StoreError(t *testing.T) {
	h := newFlashcardHandler(listErrorStore{})
	req := httptest.NewRequest(http.MethodGet, "/gaming/flashcards", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()
	h.ListFlashcards(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// TestDueFlashcards_StoreError verifies 500 when DueFlashcards store call fails.
type dueErrorStore struct{ noopStore }

func (dueErrorStore) DueFlashcards(_ context.Context, _ string) ([]*model.Flashcard, error) {
	return nil, errors.New("db error")
}

func TestDueFlashcards_StoreError(t *testing.T) {
	h := newFlashcardHandler(dueErrorStore{})
	req := httptest.NewRequest(http.MethodGet, "/gaming/flashcards/due", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()
	h.DueFlashcards(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// TestReviewFlashcard_CardNilReturns404 verifies 404 when GetFlashcard returns (nil, nil).
// noopStore returns (nil, nil) for GetFlashcard, triggering the nil card path.
func TestReviewFlashcard_CardNilReturns404(t *testing.T) {
	h := newFlashcardHandler(noopStore{})
	body, _ := json.Marshal(model.ReviewFlashcardRequest{Quality: 3})
	req := httptest.NewRequest(http.MethodPost, "/gaming/flashcards/c1/review", bytes.NewBuffer(body))
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("cardId", "c1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()
	h.ReviewFlashcard(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nil card, got %d", rr.Code)
	}
}

// TestReviewFlashcard_ReviewStoreError verifies 500 when ReviewFlashcard store call fails.
func TestReviewFlashcard_ReviewStoreError(t *testing.T) {
	card := makeCard("c1", "u1")
	s := &flashcardStore{
		cards: map[string]*model.Flashcard{"c1": card},
		reviewFn: func(_, _ string, _ int) (*model.Flashcard, error) {
			return nil, errors.New("review write failed")
		},
	}
	h := newFlashcardHandler(s)
	body, _ := json.Marshal(model.ReviewFlashcardRequest{Quality: 3})
	req := httptest.NewRequest(http.MethodPost, "/gaming/flashcards/c1/review", bytes.NewBuffer(body))
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("cardId", "c1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()
	h.ReviewFlashcard(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// TestExportAnki_StoreError verifies 500 when AllFlashcardsForExport fails.
type exportErrorStore struct{ noopStore }

func (exportErrorStore) AllFlashcardsForExport(_ context.Context, _ string) ([]*model.Flashcard, error) {
	return nil, errors.New("db error")
}

func TestExportAnki_StoreError(t *testing.T) {
	h := newFlashcardHandler(exportErrorStore{})
	req := httptest.NewRequest(http.MethodGet, "/gaming/flashcards/export/anki", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), "u1"))
	rr := httptest.NewRecorder()
	h.ExportAnki(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}
