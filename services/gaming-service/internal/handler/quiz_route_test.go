package handler_test

// HTTP handler tests for quiz.go: StartQuiz, GetQuizSession, SubmitAnswer, GetHint.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/handler"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/store"
)

// ── quizStore stub ────────────────────────────────────────────────────────────

type quizStore struct {
	noopStore
	getRandomQuestionsFn func(ctx context.Context, topic string, n int) ([]*model.Question, error)
	createQuizSessionFn  func(ctx context.Context, userID string, topic, courseID *string, ids []string) (*model.QuizSession, error)
	getQuizSessionFn     func(ctx context.Context, sessionID string) (*model.QuizSession, error)
	getQuestionFn        func(ctx context.Context, id string) (*model.Question, error)
	recordAnswerFn       func(ctx context.Context, sessionID, userID, qID, chosen string, correct bool, hints, xpEarned int, timeMs *int) (*model.QuizSession, error)
	getHintIndexFn       func(ctx context.Context, sessionID, qID string) (int, error)
	incrHintIndexFn      func(ctx context.Context, sessionID, qID, userID string) (int, int, error)
}

func (s *quizStore) GetRandomQuestions(ctx context.Context, topic string, n int) ([]*model.Question, error) {
	if s.getRandomQuestionsFn != nil {
		return s.getRandomQuestionsFn(ctx, topic, n)
	}
	return []*model.Question{
		{ID: "q1", Topic: topic, Question: "Q1?", Options: []model.QuizOption{{Key: "A", Text: "a"}}, CorrectKey: "A", XPReward: 10},
	}, nil
}

func (s *quizStore) CreateQuizSession(ctx context.Context, userID string, topic, courseID *string, ids []string) (*model.QuizSession, error) {
	if s.createQuizSessionFn != nil {
		return s.createQuizSessionFn(ctx, userID, topic, courseID, ids)
	}
	return &model.QuizSession{ID: "sess-1", UserID: userID, Status: "active", QuestionIDs: ids}, nil
}

func (s *quizStore) GetQuizSession(ctx context.Context, sessionID string) (*model.QuizSession, error) {
	if s.getQuizSessionFn != nil {
		return s.getQuizSessionFn(ctx, sessionID)
	}
	return &model.QuizSession{ID: sessionID, UserID: "u1", Status: "active", QuestionIDs: []string{"q1"}, CurrentIndex: 0}, nil
}

func (s *quizStore) GetQuestion(ctx context.Context, id string) (*model.Question, error) {
	if s.getQuestionFn != nil {
		return s.getQuestionFn(ctx, id)
	}
	return &model.Question{
		ID: id, Question: "Q?",
		Options:    []model.QuizOption{{Key: "A", Text: "a"}, {Key: "B", Text: "b"}},
		CorrectKey: "A", Hints: []string{"hint1", "hint2"}, XPReward: 10,
	}, nil
}

func (s *quizStore) RecordAnswer(ctx context.Context, sessionID, userID, qID, chosen string, correct bool, hints, xpEarned int, timeMs *int) (*model.QuizSession, error) {
	if s.recordAnswerFn != nil {
		return s.recordAnswerFn(ctx, sessionID, userID, qID, chosen, correct, hints, xpEarned, timeMs)
	}
	return &model.QuizSession{ID: sessionID, UserID: userID, Status: "active", QuestionIDs: []string{"q1"}, CurrentIndex: 1}, nil
}

func (s *quizStore) GetHintIndex(ctx context.Context, sessionID, qID string) (int, error) {
	if s.getHintIndexFn != nil {
		return s.getHintIndexFn(ctx, sessionID, qID)
	}
	return 0, nil
}

func (s *quizStore) IncrHintIndex(ctx context.Context, sessionID, qID, userID string) (int, int, error) {
	if s.incrHintIndexFn != nil {
		return s.incrHintIndexFn(ctx, sessionID, qID, userID)
	}
	return 0, 9, nil
}

// ── StartQuiz tests ───────────────────────────────────────────────────────────

func TestStartQuiz_HappyPath(t *testing.T) {
	h := handler.New(&quizStore{}, nil, zap.NewNop())
	body, _ := json.Marshal(model.StartQuizRequest{UserID: "u1", Topic: "math", QuestionCount: 1})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quiz/start", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.StartQuiz(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body)
	}
	var resp model.StartQuizResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Session == nil || resp.Question == nil {
		t.Error("expected session and question in response")
	}
}

func TestStartQuiz_DefaultsQuestionCount(t *testing.T) {
	var capturedN int
	s := &quizStore{
		getRandomQuestionsFn: func(_ context.Context, _ string, n int) ([]*model.Question, error) {
			capturedN = n
			return []*model.Question{{ID: "q1", Question: "Q?", Options: []model.QuizOption{{Key: "A", Text: "a"}}, CorrectKey: "A"}}, nil
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	body, _ := json.Marshal(model.StartQuizRequest{UserID: "u1", Topic: "math"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quiz/start", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	httptest.NewRecorder()
	rr := httptest.NewRecorder()
	h.StartQuiz(rr, req)
	if capturedN != 5 {
		t.Errorf("expected default question_count=5, got %d", capturedN)
	}
}

func TestStartQuiz_InvalidBody(t *testing.T) {
	h := handler.New(&quizStore{}, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/gaming/quiz/start", bytes.NewBufferString("{bad"))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.StartQuiz(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestStartQuiz_MissingTopic(t *testing.T) {
	h := handler.New(&quizStore{}, nil, zap.NewNop())
	body, _ := json.Marshal(model.StartQuizRequest{UserID: "u1"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quiz/start", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.StartQuiz(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestStartQuiz_ForbiddenMismatch(t *testing.T) {
	h := handler.New(&quizStore{}, nil, zap.NewNop())
	body, _ := json.Marshal(model.StartQuizRequest{UserID: "other", Topic: "math"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quiz/start", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.StartQuiz(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestStartQuiz_NoQuestionsFound(t *testing.T) {
	s := &quizStore{
		getRandomQuestionsFn: func(_ context.Context, _ string, _ int) ([]*model.Question, error) {
			return []*model.Question{}, nil
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	body, _ := json.Marshal(model.StartQuizRequest{UserID: "u1", Topic: "obscure"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quiz/start", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.StartQuiz(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestStartQuiz_GetQuestionsError(t *testing.T) {
	s := &quizStore{
		getRandomQuestionsFn: func(_ context.Context, _ string, _ int) ([]*model.Question, error) {
			return nil, errors.New("db error")
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	body, _ := json.Marshal(model.StartQuizRequest{UserID: "u1", Topic: "math"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quiz/start", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.StartQuiz(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestStartQuiz_CreateSessionError(t *testing.T) {
	s := &quizStore{
		createQuizSessionFn: func(_ context.Context, _ string, _, _ *string, _ []string) (*model.QuizSession, error) {
			return nil, errors.New("session insert failed")
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	body, _ := json.Marshal(model.StartQuizRequest{UserID: "u1", Topic: "math"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quiz/start", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.StartQuiz(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// ── GetQuizSession tests ──────────────────────────────────────────────────────

func TestGetQuizSession_HappyPath(t *testing.T) {
	h := handler.New(&quizStore{}, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/gaming/quiz/sessions/sess-1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.GetQuizSession(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body)
	}
	var resp model.QuizSessionResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Session == nil {
		t.Error("expected session in response")
	}
}

func TestGetQuizSession_SessionNotFound(t *testing.T) {
	s := &quizStore{
		getQuizSessionFn: func(_ context.Context, _ string) (*model.QuizSession, error) {
			return nil, errors.New("not found")
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/gaming/quiz/sessions/sess-1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.GetQuizSession(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestGetQuizSession_ForbiddenCrossUser(t *testing.T) {
	s := &quizStore{
		getQuizSessionFn: func(_ context.Context, sessionID string) (*model.QuizSession, error) {
			return &model.QuizSession{ID: sessionID, UserID: "other", Status: "active"}, nil
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/gaming/quiz/sessions/sess-1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.GetQuizSession(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestGetQuizSession_ActiveIncludesQuestion(t *testing.T) {
	h := handler.New(&quizStore{}, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/gaming/quiz/sessions/sess-1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.GetQuizSession(rr, req)
	var resp model.QuizSessionResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Question == nil {
		t.Error("expected question for active session")
	}
}

// ── SubmitAnswer tests ────────────────────────────────────────────────────────

func TestSubmitAnswer_HappyPath(t *testing.T) {
	h := handler.New(&quizStore{}, nil, zap.NewNop())
	body, _ := json.Marshal(model.SubmitAnswerRequest{UserID: "u1", QuestionID: "q1", ChosenKey: "A"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quiz/sessions/sess-1/answer", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.SubmitAnswer(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body)
	}
	var resp model.SubmitAnswerResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if !resp.Correct {
		t.Error("expected correct=true for key A")
	}
}

func TestSubmitAnswer_WrongAnswerEarnsNoXP(t *testing.T) {
	h := handler.New(&quizStore{}, nil, zap.NewNop())
	body, _ := json.Marshal(model.SubmitAnswerRequest{UserID: "u1", QuestionID: "q1", ChosenKey: "B"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quiz/sessions/sess-1/answer", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.SubmitAnswer(rr, req)
	var resp model.SubmitAnswerResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.XPEarned != 0 {
		t.Errorf("expected 0 XP for wrong answer, got %d", resp.XPEarned)
	}
}

func TestSubmitAnswer_InvalidBody(t *testing.T) {
	h := handler.New(&quizStore{}, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/gaming/quiz/sessions/sess-1/answer", bytes.NewBufferString("bad"))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.SubmitAnswer(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestSubmitAnswer_MissingFields(t *testing.T) {
	h := handler.New(&quizStore{}, nil, zap.NewNop())
	body, _ := json.Marshal(model.SubmitAnswerRequest{UserID: "u1", QuestionID: "q1"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quiz/sessions/sess-1/answer", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.SubmitAnswer(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestSubmitAnswer_ForbiddenMismatch(t *testing.T) {
	h := handler.New(&quizStore{}, nil, zap.NewNop())
	body, _ := json.Marshal(model.SubmitAnswerRequest{UserID: "other", QuestionID: "q1", ChosenKey: "A"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quiz/sessions/sess-1/answer", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.SubmitAnswer(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestSubmitAnswer_SessionNotFound(t *testing.T) {
	s := &quizStore{
		getQuizSessionFn: func(_ context.Context, _ string) (*model.QuizSession, error) {
			return nil, errors.New("not found")
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	body, _ := json.Marshal(model.SubmitAnswerRequest{UserID: "u1", QuestionID: "q1", ChosenKey: "A"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quiz/sessions/sess-1/answer", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.SubmitAnswer(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestSubmitAnswer_QuestionIDMismatch(t *testing.T) {
	h := handler.New(&quizStore{}, nil, zap.NewNop())
	body, _ := json.Marshal(model.SubmitAnswerRequest{UserID: "u1", QuestionID: "wrong-q", ChosenKey: "A"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quiz/sessions/sess-1/answer", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.SubmitAnswer(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestSubmitAnswer_RecordAnswerError(t *testing.T) {
	s := &quizStore{
		recordAnswerFn: func(_ context.Context, _, _, _, _ string, _ bool, _, _ int, _ *int) (*model.QuizSession, error) {
			return nil, errors.New("write failed")
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	body, _ := json.Marshal(model.SubmitAnswerRequest{UserID: "u1", QuestionID: "q1", ChosenKey: "A"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quiz/sessions/sess-1/answer", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.SubmitAnswer(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// ── GetHint tests ─────────────────────────────────────────────────────────────

func TestGetHint_HappyPath(t *testing.T) {
	h := handler.New(&quizStore{}, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/gaming/quiz/sessions/sess-1/hint?question_id=q1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.GetHint(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body)
	}
	var resp model.HintResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.GemsSpent != 1 {
		t.Errorf("expected gems_spent=1, got %d", resp.GemsSpent)
	}
}

func TestGetHint_MissingQuestionID(t *testing.T) {
	h := handler.New(&quizStore{}, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/gaming/quiz/sessions/sess-1/hint", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.GetHint(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestGetHint_SessionNotFound(t *testing.T) {
	s := &quizStore{
		getQuizSessionFn: func(_ context.Context, _ string) (*model.QuizSession, error) {
			return nil, errors.New("not found")
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/gaming/quiz/sessions/sess-1/hint?question_id=q1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.GetHint(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestGetHint_SessionNotActive(t *testing.T) {
	s := &quizStore{
		getQuizSessionFn: func(_ context.Context, sessionID string) (*model.QuizSession, error) {
			return &model.QuizSession{ID: sessionID, UserID: "u1", Status: "completed"}, nil
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/gaming/quiz/sessions/sess-1/hint?question_id=q1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.GetHint(rr, req)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rr.Code)
	}
}

func TestGetHint_AllHintsExhausted(t *testing.T) {
	s := &quizStore{
		getHintIndexFn: func(_ context.Context, _, _ string) (int, error) {
			return 2, nil // at len(hints)=2
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/gaming/quiz/sessions/sess-1/hint?question_id=q1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.GetHint(rr, req)
	if rr.Code != http.StatusGone {
		t.Errorf("expected 410, got %d", rr.Code)
	}
}

func TestGetHint_NoHintsAvailable(t *testing.T) {
	s := &quizStore{
		getQuestionFn: func(_ context.Context, id string) (*model.Question, error) {
			return &model.Question{ID: id, Hints: nil}, nil
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/gaming/quiz/sessions/sess-1/hint?question_id=q1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.GetHint(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestGetHint_InsufficientGems(t *testing.T) {
	s := &quizStore{
		incrHintIndexFn: func(_ context.Context, _, _, _ string) (int, int, error) {
			return 0, 0, store.ErrNoGems
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/gaming/quiz/sessions/sess-1/hint?question_id=q1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.GetHint(rr, req)
	if rr.Code != http.StatusPaymentRequired {
		t.Errorf("expected 402, got %d", rr.Code)
	}
}

// TestSubmitAnswer_SessionNotActive verifies 409 when session is not active.
func TestSubmitAnswer_SessionNotActive(t *testing.T) {
	s := &quizStore{
		getQuizSessionFn: func(_ context.Context, sessionID string) (*model.QuizSession, error) {
			return &model.QuizSession{ID: sessionID, UserID: "u1", Status: "completed"}, nil
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	body, _ := json.Marshal(model.SubmitAnswerRequest{UserID: "u1", QuestionID: "q1", ChosenKey: "A"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quiz/sessions/sess-1/answer", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.SubmitAnswer(rr, req)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rr.Code)
	}
}

// TestSubmitAnswer_SessionCrossUser verifies 403 when caller != session owner (post-GetQuizSession check).
func TestSubmitAnswer_SessionCrossUser(t *testing.T) {
	s := &quizStore{
		getQuizSessionFn: func(_ context.Context, sessionID string) (*model.QuizSession, error) {
			return &model.QuizSession{ID: sessionID, UserID: "other", Status: "active", QuestionIDs: []string{"q1"}}, nil
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	body, _ := json.Marshal(model.SubmitAnswerRequest{UserID: "u1", QuestionID: "q1", ChosenKey: "A"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quiz/sessions/sess-1/answer", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.SubmitAnswer(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// TestGetHint_IncrHintGeneralError verifies 500 for a non-ErrNoGems IncrHintIndex error.
func TestGetHint_IncrHintGeneralError(t *testing.T) {
	s := &quizStore{
		incrHintIndexFn: func(_ context.Context, _, _, _ string) (int, int, error) {
			return 0, 0, errors.New("redis write failed")
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/gaming/quiz/sessions/sess-1/hint?question_id=q1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.GetHint(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// TestGetHint_GetQuestionError verifies 500 when GetQuestion fails in GetHint.
func TestGetHint_GetQuestionError(t *testing.T) {
	s := &quizStore{
		getQuestionFn: func(_ context.Context, _ string) (*model.Question, error) {
			return nil, errors.New("db error")
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/gaming/quiz/sessions/sess-1/hint?question_id=q1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.GetHint(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// TestGetHint_GetHintIndexError verifies 500 when GetHintIndex fails.
func TestGetHint_GetHintIndexError(t *testing.T) {
	s := &quizStore{
		getHintIndexFn: func(_ context.Context, _, _ string) (int, error) {
			return 0, errors.New("redis error")
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/gaming/quiz/sessions/sess-1/hint?question_id=q1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.GetHint(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}
