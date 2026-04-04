package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/store"
	"github.com/teacherslounge/gaming-service/internal/xp"
)

// xpForAnswer returns XP earned for an answer given how many hints were used.
// Wrong answers always earn 0 XP. Each hint tier halves the bonus.
func xpForAnswer(baseXP, hintsUsed int, correct bool) int {
	if !correct {
		return 0
	}
	switch {
	case hintsUsed == 0:
		return baseXP
	case hintsUsed == 1:
		return baseXP * 3 / 4 // 75%
	case hintsUsed == 2:
		return baseXP / 2 // 50%
	default:
		return baseXP / 4 // 25%
	}
}

// stripAnswer returns a copy of the question safe to send to the client:
// correct_key and hints are never included; explanation is withheld until after answering.
func stripAnswer(q *model.Question) *model.Question {
	return &model.Question{
		ID:         q.ID,
		CourseID:   q.CourseID,
		Topic:      q.Topic,
		Difficulty: q.Difficulty,
		Question:   q.Question,
		Options:    q.Options,
		XPReward:   q.XPReward,
	}
}

// StartQuiz handles POST /gaming/quiz/start.
func (h *Handler) StartQuiz(w http.ResponseWriter, r *http.Request) {
	var req model.StartQuizRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" || callerID != req.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if req.Topic == "" {
		writeError(w, http.StatusBadRequest, "topic required")
		return
	}
	if req.QuestionCount <= 0 {
		req.QuestionCount = 5
	}

	questions, err := h.store.GetRandomQuestions(r.Context(), req.Topic, req.QuestionCount)
	if err != nil {
		h.logger.Error("get random questions", zap.String("topic", req.Topic), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if len(questions) == 0 {
		writeError(w, http.StatusNotFound, "no questions found for topic")
		return
	}

	ids := make([]string, len(questions))
	for i, q := range questions {
		ids[i] = q.ID
	}

	session, err := h.store.CreateQuizSession(r.Context(), req.UserID, &req.Topic, req.CourseID, ids)
	if err != nil {
		h.logger.Error("create quiz session", zap.String("user_id", req.UserID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, model.StartQuizResponse{
		Session:  session,
		Question: stripAnswer(questions[0]),
	})
}

// GetQuizSession handles GET /gaming/quiz/sessions/{sessionId}.
func (h *Handler) GetQuizSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	callerID := middleware.UserIDFromContext(r.Context())

	session, err := h.store.GetQuizSession(r.Context(), sessionID)
	if err != nil {
		h.logger.Error("get quiz session", zap.String("session_id", sessionID), zap.Error(err))
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if session.UserID != callerID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	resp := model.QuizSessionResponse{Session: session}

	if session.Status == "active" && session.CurrentIndex < len(session.QuestionIDs) {
		qID := session.QuestionIDs[session.CurrentIndex]
		q, err := h.store.GetQuestion(r.Context(), qID)
		if err != nil {
			h.logger.Error("get question", zap.String("question_id", qID), zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		resp.Question = stripAnswer(q)
	}

	writeJSON(w, http.StatusOK, resp)
}

// SubmitAnswer handles POST /gaming/quiz/sessions/{sessionId}/answer.
func (h *Handler) SubmitAnswer(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")

	var req model.SubmitAnswerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" || callerID != req.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if req.QuestionID == "" || req.ChosenKey == "" {
		writeError(w, http.StatusBadRequest, "question_id and chosen_key required")
		return
	}

	session, err := h.store.GetQuizSession(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if session.UserID != callerID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if session.Status != "active" {
		writeError(w, http.StatusConflict, "session is not active")
		return
	}
	if session.CurrentIndex >= len(session.QuestionIDs) || session.QuestionIDs[session.CurrentIndex] != req.QuestionID {
		writeError(w, http.StatusBadRequest, "question_id does not match current question")
		return
	}

	question, err := h.store.GetQuestion(r.Context(), req.QuestionID)
	if err != nil {
		h.logger.Error("get question for answer", zap.String("question_id", req.QuestionID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	hintsUsed, err := h.store.GetHintIndex(r.Context(), sessionID, req.QuestionID)
	if err != nil {
		h.logger.Warn("get hint index failed, defaulting to 0", zap.Error(err))
		hintsUsed = 0
	}

	isCorrect := req.ChosenKey == question.CorrectKey
	xpEarned := xpForAnswer(question.XPReward, hintsUsed, isCorrect)

	updatedSession, err := h.store.RecordAnswer(
		r.Context(), sessionID, req.UserID, req.QuestionID,
		req.ChosenKey, isCorrect, hintsUsed, xpEarned, req.TimeMs,
	)
	if err != nil {
		h.logger.Error("record answer", zap.String("session_id", sessionID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Credit XP to the gaming profile.
	if xpEarned > 0 {
		currentXP, currentLevel, xerr := h.store.GetXPAndLevel(r.Context(), req.UserID)
		if xerr == nil {
			newXP, newLevel, _ := xp.Apply(currentXP, currentLevel, int64(xpEarned))
			_ = h.store.UpsertXP(r.Context(), req.UserID, newXP, newLevel)
		}
	}

	resp := model.SubmitAnswerResponse{
		Correct:     isCorrect,
		CorrectKey:  question.CorrectKey,
		Explanation: question.Explanation,
		XPEarned:    xpEarned,
		Session:     updatedSession,
	}

	if updatedSession.Status == "active" && updatedSession.CurrentIndex < len(updatedSession.QuestionIDs) {
		nextQID := updatedSession.QuestionIDs[updatedSession.CurrentIndex]
		nextQ, err := h.store.GetQuestion(r.Context(), nextQID)
		if err != nil {
			h.logger.Error("get next question", zap.String("question_id", nextQID), zap.Error(err))
		} else {
			resp.NextQuestion = stripAnswer(nextQ)
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetHint handles GET /gaming/quiz/sessions/{sessionId}/hint?question_id=<uuid>.
// Each hint costs 1 gem and reveals the next unrevealed hint for the question.
func (h *Handler) GetHint(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	questionID := r.URL.Query().Get("question_id")
	if questionID == "" {
		writeError(w, http.StatusBadRequest, "question_id query param required")
		return
	}

	callerID := middleware.UserIDFromContext(r.Context())

	session, err := h.store.GetQuizSession(r.Context(), sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if session.UserID != callerID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if session.Status != "active" {
		writeError(w, http.StatusConflict, "session is not active")
		return
	}

	question, err := h.store.GetQuestion(r.Context(), questionID)
	if err != nil {
		h.logger.Error("get question for hint", zap.String("question_id", questionID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if len(question.Hints) == 0 {
		writeError(w, http.StatusNotFound, "no hints available for this question")
		return
	}

	currentIdx, err := h.store.GetHintIndex(r.Context(), sessionID, questionID)
	if err != nil {
		h.logger.Error("get hint index", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if currentIdx >= len(question.Hints) {
		writeError(w, http.StatusGone, "all hints already revealed")
		return
	}

	newIdx, gemsRemaining, err := h.store.IncrHintIndex(r.Context(), sessionID, questionID, callerID)
	if err != nil {
		if errors.Is(err, store.ErrNoGems) {
			writeError(w, http.StatusPaymentRequired, "insufficient gems")
			return
		}
		h.logger.Error("incr hint index", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, model.HintResponse{
		HintIndex:     newIdx,
		Hint:          question.Hints[newIdx],
		GemsSpent:     1,
		GemsRemaining: gemsRemaining,
		HasMore:       newIdx+1 < len(question.Hints),
	})
}

