package handler

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

	"github.com/teacherslounge/gaming-service/internal/assessment"
	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
)

// mockStore is a test double for the Storer interface.
// Methods exercised by assessment tests have function fields; all other
// methods panic with "not implemented in mockStore".
type mockStore struct {
	createAssessmentSession func(ctx context.Context, userID string) (*model.AssessmentSession, error)
	getAssessmentSession    func(ctx context.Context, sessionID string) (*model.AssessmentSession, error)
	recordAssessmentAnswer  func(ctx context.Context, sessionID, userID, questionID, chosenKey string) (*model.AssessmentSession, error)
	getXPAndLevel           func(ctx context.Context, userID string) (int64, int, error)
	upsertXP                func(ctx context.Context, userID string, newXP int64, newLevel int) error
}

func (m *mockStore) CreateAssessmentSession(ctx context.Context, userID string) (*model.AssessmentSession, error) {
	return m.createAssessmentSession(ctx, userID)
}

func (m *mockStore) GetAssessmentSession(ctx context.Context, sessionID string) (*model.AssessmentSession, error) {
	return m.getAssessmentSession(ctx, sessionID)
}

func (m *mockStore) RecordAssessmentAnswer(ctx context.Context, sessionID, userID, questionID, chosenKey string) (*model.AssessmentSession, error) {
	return m.recordAssessmentAnswer(ctx, sessionID, userID, questionID, chosenKey)
}

func (m *mockStore) GetXPAndLevel(ctx context.Context, userID string) (int64, int, error) {
	return m.getXPAndLevel(ctx, userID)
}

func (m *mockStore) UpsertXP(ctx context.Context, userID string, newXP int64, newLevel int) error {
	return m.upsertXP(ctx, userID, newXP, newLevel)
}

// --- Storer methods not used by assessment tests: all panic ---

func (m *mockStore) GetProfile(ctx context.Context, userID string) (*model.Profile, error) {
	panic("not implemented in mockStore")
}

func (m *mockStore) StreakCheckin(ctx context.Context, userID string) (current, longest int, reset bool, err error) {
	panic("not implemented in mockStore")
}

func (m *mockStore) LeaderboardUpdate(ctx context.Context, userID string, xpVal int64) error {
	panic("not implemented in mockStore")
}

func (m *mockStore) LeaderboardUpdateCourse(ctx context.Context, userID, courseID string, xp int64) error {
	panic("not implemented in mockStore")
}

func (m *mockStore) LeaderboardTop10(ctx context.Context, userID string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	panic("not implemented in mockStore")
}

func (m *mockStore) LeaderboardGetPeriod(ctx context.Context, userID, period string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	panic("not implemented in mockStore")
}

func (m *mockStore) LeaderboardGetCourse(ctx context.Context, userID, courseID string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	panic("not implemented in mockStore")
}

func (m *mockStore) LeaderboardGetFriends(ctx context.Context, userID string, friendIDs []string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	panic("not implemented in mockStore")
}

func (m *mockStore) RandomQuote(ctx context.Context) (*model.Quote, error) {
	panic("not implemented in mockStore")
}

func (m *mockStore) GetRandomQuestions(ctx context.Context, topic string, n int) ([]*model.Question, error) {
	panic("not implemented in mockStore")
}

func (m *mockStore) GetQuestion(ctx context.Context, questionID string) (*model.Question, error) {
	panic("not implemented in mockStore")
}

func (m *mockStore) CreateQuizSession(ctx context.Context, userID string, topic, courseID *string, questionIDs []string) (*model.QuizSession, error) {
	panic("not implemented in mockStore")
}

func (m *mockStore) GetQuizSession(ctx context.Context, sessionID string) (*model.QuizSession, error) {
	panic("not implemented in mockStore")
}

func (m *mockStore) RecordAnswer(ctx context.Context, sessionID, userID, questionID, chosenKey string, isCorrect bool, hintsUsed, xpEarned int, timeMs *int) (*model.QuizSession, error) {
	panic("not implemented in mockStore")
}

func (m *mockStore) GetHintIndex(ctx context.Context, sessionID, questionID string) (int, error) {
	panic("not implemented in mockStore")
}

func (m *mockStore) IncrHintIndex(ctx context.Context, sessionID, questionID, userID string) (newIndex, gemsRemaining int, err error) {
	panic("not implemented in mockStore")
}

func (m *mockStore) GetDailyQuests(ctx context.Context, userID string) ([]model.QuestState, error) {
	panic("not implemented in mockStore")
}

func (m *mockStore) UpdateQuestProgress(ctx context.Context, userID string, action string) ([]model.QuestState, int, int, error) {
	panic("not implemented in mockStore")
}

func (m *mockStore) AwardQuestRewards(ctx context.Context, userID string, xpDelta, gemsDelta int) (newXP int64, newLevel int, leveledUp bool, newGems int, err error) {
	panic("not implemented in mockStore")
}

func (m *mockStore) StartBossBattle(ctx context.Context, userID, bossID, bossName, topic string, maxRounds, bossHP int, questionIDs []string) (*model.BossSession, error) {
	panic("not implemented in mockStore")
}

func (m *mockStore) GetBossSession(ctx context.Context, sessionID string) (*model.BossSession, error) {
	panic("not implemented in mockStore")
}

func (m *mockStore) SaveBossSession(ctx context.Context, session *model.BossSession) error {
	panic("not implemented in mockStore")
}

func (m *mockStore) CompleteBossBattle(ctx context.Context, session *model.BossSession, bonusXP int) (newXP int64, newLevel int, leveledUp bool, bossesDefeated int, err error) {
	panic("not implemented in mockStore")
}

// --- helpers ---

// newTestHandler creates a Handler backed by the given mockStore.
func newTestHandler(store *mockStore) *Handler {
	logger, _ := zap.NewDevelopment()
	return New(store, logger)
}

// addChiParam attaches a chi URL parameter to the request context so that
// chi.URLParam can retrieve it inside the handler under test.
func addChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// makeActiveSession returns a stub AssessmentSession in "active" status at
// question index 0 owned by the given userID.
func makeActiveSession(sessionID, userID string) *model.AssessmentSession {
	return &model.AssessmentSession{
		ID:             sessionID,
		UserID:         userID,
		Status:         "active",
		CurrentIndex:   0,
		TotalQuestions: assessment.TotalCount,
		XPEarned:       0,
		StartedAt:      time.Now(),
	}
}

// makeCompletedSession returns a stub AssessmentSession in "completed" status.
func makeCompletedSession(sessionID, userID string) *model.AssessmentSession {
	now := time.Now()
	return &model.AssessmentSession{
		ID:          sessionID,
		UserID:      userID,
		Status:      "completed",
		XPEarned:    50,
		StartedAt:   now,
		CompletedAt: &now,
	}
}

// decodeJSON unmarshals the response body into dst and fails the test on error.
func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.NewDecoder(rr.Body).Decode(dst); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
}

// ---- StartAssessment tests ----

// TestStartAssessment_HappyPath verifies that a valid request returns 201 with
// the new session and the first question.
func TestStartAssessment_HappyPath(t *testing.T) {
	const (
		userID    = "user-abc"
		sessionID = "sess-001"
	)

	store := &mockStore{
		createAssessmentSession: func(_ context.Context, uid string) (*model.AssessmentSession, error) {
			return makeActiveSession(sessionID, uid), nil
		},
	}
	h := newTestHandler(store)

	body, _ := json.Marshal(model.StartAssessmentRequest{UserID: userID})
	r := httptest.NewRequest(http.MethodPost, "/gaming/assessment/start", bytes.NewReader(body))
	r = r.WithContext(middleware.WithUserID(r.Context(), userID))
	rr := httptest.NewRecorder()

	h.StartAssessment(rr, r)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	var resp model.StartAssessmentResponse
	decodeJSON(t, rr, &resp)

	if resp.Session == nil {
		t.Fatal("expected session in response, got nil")
	}
	if resp.Session.ID != sessionID {
		t.Errorf("session ID: got %q, want %q", resp.Session.ID, sessionID)
	}
	if resp.Question == nil {
		t.Fatal("expected first question in response, got nil")
	}
	// The first question in the bank is ar-1.
	if resp.Question.ID != "ar-1" {
		t.Errorf("expected first question id ar-1, got %q", resp.Question.ID)
	}
	if resp.Question.Index != 0 {
		t.Errorf("expected question index 0, got %d", resp.Question.Index)
	}
	if resp.Question.Total != assessment.TotalCount {
		t.Errorf("expected total %d, got %d", assessment.TotalCount, resp.Question.Total)
	}
}

// TestStartAssessment_BadJSON verifies that malformed JSON returns 400.
func TestStartAssessment_BadJSON(t *testing.T) {
	h := newTestHandler(&mockStore{})

	r := httptest.NewRequest(http.MethodPost, "/gaming/assessment/start", bytes.NewBufferString("{invalid"))
	r = r.WithContext(middleware.WithUserID(r.Context(), "user-abc"))
	rr := httptest.NewRecorder()

	h.StartAssessment(rr, r)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestStartAssessment_Forbidden verifies that a caller cannot start an
// assessment on behalf of a different user.
func TestStartAssessment_Forbidden(t *testing.T) {
	h := newTestHandler(&mockStore{})

	body, _ := json.Marshal(model.StartAssessmentRequest{UserID: "other-user"})
	r := httptest.NewRequest(http.MethodPost, "/gaming/assessment/start", bytes.NewReader(body))
	// Authenticated as "caller", requesting for "other-user".
	r = r.WithContext(middleware.WithUserID(r.Context(), "caller"))
	rr := httptest.NewRecorder()

	h.StartAssessment(rr, r)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// ---- GetAssessmentSession tests ----

// TestGetAssessmentSession_HappyPathActive verifies that fetching an active
// session returns 200 with the session and current question.
func TestGetAssessmentSession_HappyPathActive(t *testing.T) {
	const (
		userID    = "user-abc"
		sessionID = "sess-002"
	)

	store := &mockStore{
		getAssessmentSession: func(_ context.Context, sid string) (*model.AssessmentSession, error) {
			return makeActiveSession(sid, userID), nil
		},
	}
	h := newTestHandler(store)

	r := httptest.NewRequest(http.MethodGet, "/gaming/assessment/sessions/"+sessionID, nil)
	r = addChiParam(r, "sessionId", sessionID)
	r = r.WithContext(middleware.WithUserID(r.Context(), userID))
	rr := httptest.NewRecorder()

	h.GetAssessmentSession(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp model.AssessmentSessionResponse
	decodeJSON(t, rr, &resp)

	if resp.Session == nil {
		t.Fatal("expected session in response, got nil")
	}
	if resp.Session.Status != "active" {
		t.Errorf("expected status active, got %q", resp.Session.Status)
	}
	if resp.Question == nil {
		t.Error("expected question for active session, got nil")
	}
}

// TestGetAssessmentSession_HappyPathCompleted verifies that a completed session
// returns 200 without a question.
func TestGetAssessmentSession_HappyPathCompleted(t *testing.T) {
	const (
		userID    = "user-abc"
		sessionID = "sess-003"
	)

	store := &mockStore{
		getAssessmentSession: func(_ context.Context, sid string) (*model.AssessmentSession, error) {
			return makeCompletedSession(sid, userID), nil
		},
	}
	h := newTestHandler(store)

	r := httptest.NewRequest(http.MethodGet, "/gaming/assessment/sessions/"+sessionID, nil)
	r = addChiParam(r, "sessionId", sessionID)
	r = r.WithContext(middleware.WithUserID(r.Context(), userID))
	rr := httptest.NewRecorder()

	h.GetAssessmentSession(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp model.AssessmentSessionResponse
	decodeJSON(t, rr, &resp)

	if resp.Session.Status != "completed" {
		t.Errorf("expected status completed, got %q", resp.Session.Status)
	}
	if resp.Question != nil {
		t.Error("expected no question for completed session, got one")
	}
}

// TestGetAssessmentSession_NotFound verifies that a store error returns 404.
func TestGetAssessmentSession_NotFound(t *testing.T) {
	store := &mockStore{
		getAssessmentSession: func(_ context.Context, _ string) (*model.AssessmentSession, error) {
			return nil, errors.New("not found")
		},
	}
	h := newTestHandler(store)

	r := httptest.NewRequest(http.MethodGet, "/gaming/assessment/sessions/missing", nil)
	r = addChiParam(r, "sessionId", "missing")
	r = r.WithContext(middleware.WithUserID(r.Context(), "user-abc"))
	rr := httptest.NewRecorder()

	h.GetAssessmentSession(rr, r)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// TestGetAssessmentSession_Forbidden verifies that a caller who does not own
// the session receives 403.
func TestGetAssessmentSession_Forbidden(t *testing.T) {
	const sessionID = "sess-004"

	store := &mockStore{
		getAssessmentSession: func(_ context.Context, sid string) (*model.AssessmentSession, error) {
			return makeActiveSession(sid, "owner-user"), nil
		},
	}
	h := newTestHandler(store)

	r := httptest.NewRequest(http.MethodGet, "/gaming/assessment/sessions/"+sessionID, nil)
	r = addChiParam(r, "sessionId", sessionID)
	// Authenticated as different user.
	r = r.WithContext(middleware.WithUserID(r.Context(), "other-user"))
	rr := httptest.NewRecorder()

	h.GetAssessmentSession(rr, r)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// ---- SubmitAssessmentAnswer tests ----

// TestSubmitAssessmentAnswer_HappyPathMidSession verifies that submitting a
// valid answer for a non-final question returns 200 with the next question.
func TestSubmitAssessmentAnswer_HappyPathMidSession(t *testing.T) {
	const (
		userID    = "user-abc"
		sessionID = "sess-005"
	)

	// Index 0 → question ar-1; after answering, current_index moves to 1.
	activeSession := makeActiveSession(sessionID, userID)
	advancedSession := makeActiveSession(sessionID, userID)
	advancedSession.CurrentIndex = 1

	store := &mockStore{
		getAssessmentSession: func(_ context.Context, _ string) (*model.AssessmentSession, error) {
			return activeSession, nil
		},
		recordAssessmentAnswer: func(_ context.Context, _, _, _, _ string) (*model.AssessmentSession, error) {
			return advancedSession, nil
		},
	}
	h := newTestHandler(store)

	q0 := assessment.ByIndex(0)
	body, _ := json.Marshal(model.SubmitAssessmentAnswerRequest{
		UserID:     userID,
		QuestionID: q0.ID,
		ChosenKey:  "A",
	})
	r := httptest.NewRequest(http.MethodPost, "/gaming/assessment/sessions/"+sessionID+"/answer", bytes.NewReader(body))
	r = addChiParam(r, "sessionId", sessionID)
	r = r.WithContext(middleware.WithUserID(r.Context(), userID))
	rr := httptest.NewRecorder()

	h.SubmitAssessmentAnswer(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp model.SubmitAssessmentAnswerResponse
	decodeJSON(t, rr, &resp)

	if resp.NextQuestion == nil {
		t.Error("expected next_question for mid-session, got nil")
	}
	if resp.XPEarned != 0 {
		t.Errorf("expected xp_earned 0 for mid-session, got %d", resp.XPEarned)
	}
}

// TestSubmitAssessmentAnswer_HappyPathFinalAnswer verifies that the last answer
// triggers XP award logic and returns xp_earned with no next_question.
func TestSubmitAssessmentAnswer_HappyPathFinalAnswer(t *testing.T) {
	const (
		userID    = "user-abc"
		sessionID = "sess-006"
	)

	// Session is at the last question index (TotalCount - 1 = 11).
	lastIdx := assessment.TotalCount - 1
	activeSession := makeActiveSession(sessionID, userID)
	activeSession.CurrentIndex = lastIdx

	completedSession := makeCompletedSession(sessionID, userID)
	completedSession.XPEarned = 50

	store := &mockStore{
		getAssessmentSession: func(_ context.Context, _ string) (*model.AssessmentSession, error) {
			return activeSession, nil
		},
		recordAssessmentAnswer: func(_ context.Context, _, _, _, _ string) (*model.AssessmentSession, error) {
			return completedSession, nil
		},
		getXPAndLevel: func(_ context.Context, _ string) (int64, int, error) {
			return 100, 1, nil
		},
		upsertXP: func(_ context.Context, _ string, _ int64, _ int) error {
			return nil
		},
	}
	h := newTestHandler(store)

	lastQ := assessment.ByIndex(lastIdx)
	body, _ := json.Marshal(model.SubmitAssessmentAnswerRequest{
		UserID:     userID,
		QuestionID: lastQ.ID,
		ChosenKey:  "B",
	})
	r := httptest.NewRequest(http.MethodPost, "/gaming/assessment/sessions/"+sessionID+"/answer", bytes.NewReader(body))
	r = addChiParam(r, "sessionId", sessionID)
	r = r.WithContext(middleware.WithUserID(r.Context(), userID))
	rr := httptest.NewRecorder()

	h.SubmitAssessmentAnswer(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp model.SubmitAssessmentAnswerResponse
	decodeJSON(t, rr, &resp)

	if resp.NextQuestion != nil {
		t.Error("expected no next_question on final answer, got one")
	}
	if resp.XPEarned != 50 {
		t.Errorf("expected xp_earned 50, got %d", resp.XPEarned)
	}
}

// TestSubmitAssessmentAnswer_BadJSON verifies that malformed JSON returns 400.
func TestSubmitAssessmentAnswer_BadJSON(t *testing.T) {
	h := newTestHandler(&mockStore{})

	r := httptest.NewRequest(http.MethodPost, "/gaming/assessment/sessions/sess-007/answer", bytes.NewBufferString("{bad"))
	r = addChiParam(r, "sessionId", "sess-007")
	r = r.WithContext(middleware.WithUserID(r.Context(), "user-abc"))
	rr := httptest.NewRecorder()

	h.SubmitAssessmentAnswer(rr, r)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestSubmitAssessmentAnswer_MissingFields verifies that omitting question_id
// or chosen_key returns 400.
func TestSubmitAssessmentAnswer_MissingFields(t *testing.T) {
	const (
		userID    = "user-abc"
		sessionID = "sess-008"
	)

	h := newTestHandler(&mockStore{})

	body, _ := json.Marshal(model.SubmitAssessmentAnswerRequest{
		UserID:     userID,
		QuestionID: "", // missing
		ChosenKey:  "", // missing
	})
	r := httptest.NewRequest(http.MethodPost, "/gaming/assessment/sessions/"+sessionID+"/answer", bytes.NewReader(body))
	r = addChiParam(r, "sessionId", sessionID)
	r = r.WithContext(middleware.WithUserID(r.Context(), userID))
	rr := httptest.NewRecorder()

	h.SubmitAssessmentAnswer(rr, r)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestSubmitAssessmentAnswer_InvalidChosenKey verifies that a chosen_key other
// than "A" or "B" returns 400.
func TestSubmitAssessmentAnswer_InvalidChosenKey(t *testing.T) {
	const (
		userID    = "user-abc"
		sessionID = "sess-009"
	)

	h := newTestHandler(&mockStore{})

	body, _ := json.Marshal(model.SubmitAssessmentAnswerRequest{
		UserID:     userID,
		QuestionID: "ar-1",
		ChosenKey:  "C", // invalid
	})
	r := httptest.NewRequest(http.MethodPost, "/gaming/assessment/sessions/"+sessionID+"/answer", bytes.NewReader(body))
	r = addChiParam(r, "sessionId", sessionID)
	r = r.WithContext(middleware.WithUserID(r.Context(), userID))
	rr := httptest.NewRecorder()

	h.SubmitAssessmentAnswer(rr, r)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestSubmitAssessmentAnswer_SessionNotActive verifies that submitting to a
// completed session returns 409.
func TestSubmitAssessmentAnswer_SessionNotActive(t *testing.T) {
	const (
		userID    = "user-abc"
		sessionID = "sess-010"
	)

	store := &mockStore{
		getAssessmentSession: func(_ context.Context, sid string) (*model.AssessmentSession, error) {
			return makeCompletedSession(sid, userID), nil
		},
	}
	h := newTestHandler(store)

	body, _ := json.Marshal(model.SubmitAssessmentAnswerRequest{
		UserID:     userID,
		QuestionID: "ar-1",
		ChosenKey:  "A",
	})
	r := httptest.NewRequest(http.MethodPost, "/gaming/assessment/sessions/"+sessionID+"/answer", bytes.NewReader(body))
	r = addChiParam(r, "sessionId", sessionID)
	r = r.WithContext(middleware.WithUserID(r.Context(), userID))
	rr := httptest.NewRecorder()

	h.SubmitAssessmentAnswer(rr, r)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rr.Code)
	}
}

// TestSubmitAssessmentAnswer_QuestionIDMismatch verifies that supplying the
// wrong question_id for the current position returns 400.
func TestSubmitAssessmentAnswer_QuestionIDMismatch(t *testing.T) {
	const (
		userID    = "user-abc"
		sessionID = "sess-011"
	)

	// Session is at index 0 (ar-1), but request sends question id for index 1.
	activeSession := makeActiveSession(sessionID, userID)

	store := &mockStore{
		getAssessmentSession: func(_ context.Context, _ string) (*model.AssessmentSession, error) {
			return activeSession, nil
		},
	}
	h := newTestHandler(store)

	body, _ := json.Marshal(model.SubmitAssessmentAnswerRequest{
		UserID:     userID,
		QuestionID: "si-1", // index 1, not 0
		ChosenKey:  "A",
	})
	r := httptest.NewRequest(http.MethodPost, "/gaming/assessment/sessions/"+sessionID+"/answer", bytes.NewReader(body))
	r = addChiParam(r, "sessionId", sessionID)
	r = r.WithContext(middleware.WithUserID(r.Context(), userID))
	rr := httptest.NewRecorder()

	h.SubmitAssessmentAnswer(rr, r)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestSubmitAssessmentAnswer_Forbidden verifies that a caller cannot submit
// an answer on behalf of another user.
func TestSubmitAssessmentAnswer_Forbidden(t *testing.T) {
	const sessionID = "sess-012"

	h := newTestHandler(&mockStore{})

	body, _ := json.Marshal(model.SubmitAssessmentAnswerRequest{
		UserID:     "other-user",
		QuestionID: "ar-1",
		ChosenKey:  "A",
	})
	r := httptest.NewRequest(http.MethodPost, "/gaming/assessment/sessions/"+sessionID+"/answer", bytes.NewReader(body))
	r = addChiParam(r, "sessionId", sessionID)
	r = r.WithContext(middleware.WithUserID(r.Context(), "caller"))
	rr := httptest.NewRecorder()

	h.SubmitAssessmentAnswer(rr, r)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}
