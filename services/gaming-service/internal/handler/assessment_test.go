package handler_test

// Tests for assessment.go: StartAssessment, GetAssessmentSession,
// SubmitAssessmentAnswer, and the unexported questionForIndex helper (exercised
// indirectly through the handler calls).

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/assessment"
	"github.com/teacherslounge/gaming-service/internal/handler"
	"github.com/teacherslounge/gaming-service/internal/model"
)

// ── assessmentStore stub ──────────────────────────────────────────────────────

type assessmentStore struct {
	noopStore
	createSessionFn func(ctx context.Context, userID string) (*model.AssessmentSession, error)
	getSessionFn    func(ctx context.Context, sessionID string) (*model.AssessmentSession, error)
	recordAnswerFn  func(ctx context.Context, sessionID, userID, questionID, chosenKey string) (*model.AssessmentSession, error)
}

func (s *assessmentStore) CreateAssessmentSession(ctx context.Context, userID string) (*model.AssessmentSession, error) {
	if s.createSessionFn != nil {
		return s.createSessionFn(ctx, userID)
	}
	return &model.AssessmentSession{
		ID:             "sess-1",
		UserID:         userID,
		Status:         "active",
		CurrentIndex:   0,
		TotalQuestions: assessment.TotalCount,
	}, nil
}

func (s *assessmentStore) GetAssessmentSession(ctx context.Context, sessionID string) (*model.AssessmentSession, error) {
	if s.getSessionFn != nil {
		return s.getSessionFn(ctx, sessionID)
	}
	return &model.AssessmentSession{
		ID:             sessionID,
		UserID:         "u1",
		Status:         "active",
		CurrentIndex:   0,
		TotalQuestions: assessment.TotalCount,
	}, nil
}

func (s *assessmentStore) RecordAssessmentAnswer(ctx context.Context, sessionID, userID, questionID, chosenKey string) (*model.AssessmentSession, error) {
	if s.recordAnswerFn != nil {
		return s.recordAnswerFn(ctx, sessionID, userID, questionID, chosenKey)
	}
	return &model.AssessmentSession{
		ID:           sessionID,
		UserID:       userID,
		Status:       "active",
		CurrentIndex: 1,
	}, nil
}

// ── StartAssessment tests ─────────────────────────────────────────────────────

// TestStartAssessment_HappyPath verifies 201 with session + first question.
func TestStartAssessment_HappyPath(t *testing.T) {
	h := handler.New(&assessmentStore{}, nil, zap.NewNop())
	body, _ := json.Marshal(model.StartAssessmentRequest{UserID: "u1"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/assessment/start", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()

	h.StartAssessment(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body)
	}
	var resp model.StartAssessmentResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Session == nil {
		t.Error("expected session in response")
	}
	if resp.Question == nil {
		t.Error("expected question in response")
	}
	if resp.Question.Index != 0 {
		t.Errorf("expected first question (index=0), got %d", resp.Question.Index)
	}
}

// TestStartAssessment_InvalidBody verifies 400 for malformed JSON.
func TestStartAssessment_InvalidBody(t *testing.T) {
	h := handler.New(&assessmentStore{}, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/gaming/assessment/start", bytes.NewBufferString("bad"))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.StartAssessment(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestStartAssessment_ForbiddenMismatch verifies 403 when caller != user_id.
func TestStartAssessment_ForbiddenMismatch(t *testing.T) {
	h := handler.New(&assessmentStore{}, nil, zap.NewNop())
	body, _ := json.Marshal(model.StartAssessmentRequest{UserID: "other"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/assessment/start", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.StartAssessment(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// TestStartAssessment_StoreError verifies 500 when CreateAssessmentSession fails.
func TestStartAssessment_StoreError(t *testing.T) {
	s := &assessmentStore{
		createSessionFn: func(_ context.Context, _ string) (*model.AssessmentSession, error) {
			return nil, errors.New("db error")
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	body, _ := json.Marshal(model.StartAssessmentRequest{UserID: "u1"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/assessment/start", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.StartAssessment(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// ── GetAssessmentSession tests ────────────────────────────────────────────────

// TestGetAssessmentSession_HappyPath verifies 200 with session and current question.
func TestGetAssessmentSession_HappyPath(t *testing.T) {
	h := handler.New(&assessmentStore{}, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/gaming/assessment/sessions/sess-1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()

	h.GetAssessmentSession(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body)
	}
	var resp model.AssessmentSessionResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Session == nil {
		t.Error("expected session in response")
	}
	// Active session should include the next question.
	if resp.Question == nil {
		t.Error("expected question for active session")
	}
}

// TestGetAssessmentSession_CompletedOmitsQuestion verifies no question is returned for completed sessions.
func TestGetAssessmentSession_CompletedOmitsQuestion(t *testing.T) {
	s := &assessmentStore{
		getSessionFn: func(_ context.Context, sessionID string) (*model.AssessmentSession, error) {
			return &model.AssessmentSession{
				ID:     sessionID,
				UserID: "u1",
				Status: "completed",
			}, nil
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/gaming/assessment/sessions/sess-1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()

	h.GetAssessmentSession(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp model.AssessmentSessionResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Question != nil {
		t.Error("expected no question for completed session")
	}
}

// TestGetAssessmentSession_StoreError verifies 404 when store fails.
func TestGetAssessmentSession_StoreError(t *testing.T) {
	s := &assessmentStore{
		getSessionFn: func(_ context.Context, _ string) (*model.AssessmentSession, error) {
			return nil, errors.New("not found")
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/gaming/assessment/sessions/sess-1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.GetAssessmentSession(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// TestGetAssessmentSession_ForbiddenCrossUser verifies 403 when caller != session owner.
func TestGetAssessmentSession_ForbiddenCrossUser(t *testing.T) {
	s := &assessmentStore{
		getSessionFn: func(_ context.Context, sessionID string) (*model.AssessmentSession, error) {
			return &model.AssessmentSession{
				ID:     sessionID,
				UserID: "other-user",
				Status: "active",
			}, nil
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodGet, "/gaming/assessment/sessions/sess-1", nil)
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.GetAssessmentSession(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// ── SubmitAssessmentAnswer tests ──────────────────────────────────────────────

// firstQuestion returns the ID and key for question 0 in the static bank.
func firstQuestion() (id, key string) {
	q := assessment.ByIndex(0)
	return q.ID, q.Options[0].Key
}

// TestSubmitAssessmentAnswer_HappyPath verifies 200 when the answer is valid.
func TestSubmitAssessmentAnswer_HappyPath(t *testing.T) {
	qID, chosenKey := firstQuestion()
	s := &assessmentStore{
		getSessionFn: func(_ context.Context, sessionID string) (*model.AssessmentSession, error) {
			return &model.AssessmentSession{
				ID:           sessionID,
				UserID:       "u1",
				Status:       "active",
				CurrentIndex: 0,
			}, nil
		},
	}
	h := handler.New(s, nil, zap.NewNop())

	body, _ := json.Marshal(model.SubmitAssessmentAnswerRequest{
		UserID:     "u1",
		QuestionID: qID,
		ChosenKey:  chosenKey,
	})
	req := httptest.NewRequest(http.MethodPost, "/gaming/assessment/sessions/sess-1/answer", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()

	h.SubmitAssessmentAnswer(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body)
	}
}

// TestSubmitAssessmentAnswer_InvalidBody verifies 400 for malformed JSON.
func TestSubmitAssessmentAnswer_InvalidBody(t *testing.T) {
	h := handler.New(&assessmentStore{}, nil, zap.NewNop())
	req := httptest.NewRequest(http.MethodPost, "/gaming/assessment/sessions/sess-1/answer", bytes.NewBufferString("bad"))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.SubmitAssessmentAnswer(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestSubmitAssessmentAnswer_InvalidChosenKey verifies 400 for a key other than A or B.
func TestSubmitAssessmentAnswer_InvalidChosenKey(t *testing.T) {
	qID, _ := firstQuestion()
	h := handler.New(&assessmentStore{}, nil, zap.NewNop())
	body, _ := json.Marshal(model.SubmitAssessmentAnswerRequest{
		UserID:     "u1",
		QuestionID: qID,
		ChosenKey:  "C",
	})
	req := httptest.NewRequest(http.MethodPost, "/gaming/assessment/sessions/sess-1/answer", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.SubmitAssessmentAnswer(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestSubmitAssessmentAnswer_ForbiddenMismatch verifies 403 when caller != user_id.
func TestSubmitAssessmentAnswer_ForbiddenMismatch(t *testing.T) {
	qID, chosenKey := firstQuestion()
	h := handler.New(&assessmentStore{}, nil, zap.NewNop())
	body, _ := json.Marshal(model.SubmitAssessmentAnswerRequest{
		UserID:     "other",
		QuestionID: qID,
		ChosenKey:  chosenKey,
	})
	req := httptest.NewRequest(http.MethodPost, "/gaming/assessment/sessions/sess-1/answer", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.SubmitAssessmentAnswer(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// TestSubmitAssessmentAnswer_SessionNotFound verifies 404 when session is missing.
func TestSubmitAssessmentAnswer_SessionNotFound(t *testing.T) {
	qID, chosenKey := firstQuestion()
	s := &assessmentStore{
		getSessionFn: func(_ context.Context, _ string) (*model.AssessmentSession, error) {
			return nil, errors.New("not found")
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	body, _ := json.Marshal(model.SubmitAssessmentAnswerRequest{
		UserID:     "u1",
		QuestionID: qID,
		ChosenKey:  chosenKey,
	})
	req := httptest.NewRequest(http.MethodPost, "/gaming/assessment/sessions/sess-1/answer", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.SubmitAssessmentAnswer(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// TestSubmitAssessmentAnswer_SessionNotActive verifies 409 when session is completed.
func TestSubmitAssessmentAnswer_SessionNotActive(t *testing.T) {
	qID, chosenKey := firstQuestion()
	s := &assessmentStore{
		getSessionFn: func(_ context.Context, sessionID string) (*model.AssessmentSession, error) {
			return &model.AssessmentSession{
				ID:     sessionID,
				UserID: "u1",
				Status: "completed",
			}, nil
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	body, _ := json.Marshal(model.SubmitAssessmentAnswerRequest{
		UserID:     "u1",
		QuestionID: qID,
		ChosenKey:  chosenKey,
	})
	req := httptest.NewRequest(http.MethodPost, "/gaming/assessment/sessions/sess-1/answer", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.SubmitAssessmentAnswer(rr, req)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rr.Code)
	}
}

// TestSubmitAssessmentAnswer_QuestionIDMismatch verifies 400 when question_id is wrong.
func TestSubmitAssessmentAnswer_QuestionIDMismatch(t *testing.T) {
	_, chosenKey := firstQuestion()
	s := &assessmentStore{
		getSessionFn: func(_ context.Context, sessionID string) (*model.AssessmentSession, error) {
			return &model.AssessmentSession{
				ID:           sessionID,
				UserID:       "u1",
				Status:       "active",
				CurrentIndex: 0,
			}, nil
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	body, _ := json.Marshal(model.SubmitAssessmentAnswerRequest{
		UserID:     "u1",
		QuestionID: "wrong-id",
		ChosenKey:  chosenKey,
	})
	req := httptest.NewRequest(http.MethodPost, "/gaming/assessment/sessions/sess-1/answer", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.SubmitAssessmentAnswer(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestSubmitAssessmentAnswer_RecordAnswerError verifies 500 when RecordAssessmentAnswer fails.
func TestSubmitAssessmentAnswer_RecordAnswerError(t *testing.T) {
	qID, chosenKey := firstQuestion()
	s := &assessmentStore{
		getSessionFn: func(_ context.Context, sessionID string) (*model.AssessmentSession, error) {
			return &model.AssessmentSession{
				ID:           sessionID,
				UserID:       "u1",
				Status:       "active",
				CurrentIndex: 0,
			}, nil
		},
		recordAnswerFn: func(_ context.Context, _, _, _, _ string) (*model.AssessmentSession, error) {
			return nil, errors.New("db write failed")
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	body, _ := json.Marshal(model.SubmitAssessmentAnswerRequest{
		UserID:     "u1",
		QuestionID: qID,
		ChosenKey:  chosenKey,
	})
	req := httptest.NewRequest(http.MethodPost, "/gaming/assessment/sessions/sess-1/answer", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.SubmitAssessmentAnswer(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// TestSubmitAssessmentAnswer_CompletedWithXP verifies the XP award path fires when session completes.
func TestSubmitAssessmentAnswer_CompletedWithXP(t *testing.T) {
	qID, chosenKey := firstQuestion()
	s := &assessmentStore{
		getSessionFn: func(_ context.Context, sessionID string) (*model.AssessmentSession, error) {
			return &model.AssessmentSession{
				ID:           sessionID,
				UserID:       "u1",
				Status:       "active",
				CurrentIndex: 0,
			}, nil
		},
		recordAnswerFn: func(_ context.Context, _, _, _, _ string) (*model.AssessmentSession, error) {
			return &model.AssessmentSession{
				ID:      "sess-1",
				UserID:  "u1",
				Status:  "completed",
				XPEarned: 50, // triggers XP award path
			}, nil
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	body, _ := json.Marshal(model.SubmitAssessmentAnswerRequest{
		UserID:     "u1",
		QuestionID: qID,
		ChosenKey:  chosenKey,
	})
	req := httptest.NewRequest(http.MethodPost, "/gaming/assessment/sessions/sess-1/answer", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.SubmitAssessmentAnswer(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body)
	}
	var resp model.SubmitAssessmentAnswerResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.XPEarned != 50 {
		t.Errorf("expected xp_earned=50 in response, got %d", resp.XPEarned)
	}
}

// TestSubmitAssessmentAnswer_SessionCrossUser verifies 403 when caller != session owner.
func TestSubmitAssessmentAnswer_SessionCrossUser(t *testing.T) {
	qID, chosenKey := firstQuestion()
	s := &assessmentStore{
		getSessionFn: func(_ context.Context, sessionID string) (*model.AssessmentSession, error) {
			return &model.AssessmentSession{
				ID:           sessionID,
				UserID:       "other", // different from caller
				Status:       "active",
				CurrentIndex: 0,
			}, nil
		},
	}
	h := handler.New(s, nil, zap.NewNop())
	body, _ := json.Marshal(model.SubmitAssessmentAnswerRequest{
		UserID:     "u1",
		QuestionID: qID,
		ChosenKey:  chosenKey,
	})
	req := httptest.NewRequest(http.MethodPost, "/gaming/assessment/sessions/sess-1/answer", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	req = withURLParam(req, "sessionId", "sess-1")
	rr := httptest.NewRecorder()
	h.SubmitAssessmentAnswer(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}
