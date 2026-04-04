package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/assessment"
	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/xp"
)

// questionForIndex converts the static bank entry at idx into the client-safe
// AssessmentQuestion type (no weights exposed).
func questionForIndex(idx int) *model.AssessmentQuestion {
	q := assessment.ByIndex(idx)
	if q == nil {
		return nil
	}
	opts := make([]model.AssessmentQuestionOpt, len(q.Options))
	for i, o := range q.Options {
		opts[i] = model.AssessmentQuestionOpt{Key: o.Key, Text: o.Text}
	}
	return &model.AssessmentQuestion{
		ID:        q.ID,
		Index:     idx,
		Total:     assessment.TotalCount,
		Dimension: q.Dimension,
		Stem:      q.Stem,
		Options:   opts,
	}
}

// StartAssessment handles POST /gaming/assessment/start.
func (h *Handler) StartAssessment(w http.ResponseWriter, r *http.Request) {
	var req model.StartAssessmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" || callerID != req.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	session, err := h.store.CreateAssessmentSession(r.Context(), req.UserID)
	if err != nil {
		h.logger.Error("create assessment session", zap.String("user_id", req.UserID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, model.StartAssessmentResponse{
		Session:  session,
		Question: questionForIndex(0),
	})
}

// GetAssessmentSession handles GET /gaming/assessment/sessions/{sessionId}.
func (h *Handler) GetAssessmentSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	callerID := middleware.UserIDFromContext(r.Context())

	session, err := h.store.GetAssessmentSession(r.Context(), sessionID)
	if err != nil {
		h.logger.Error("get assessment session", zap.String("session_id", sessionID), zap.Error(err))
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if session.UserID != callerID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	resp := model.AssessmentSessionResponse{Session: session}
	if session.Status == "active" {
		resp.Question = questionForIndex(session.CurrentIndex)
	}
	writeJSON(w, http.StatusOK, resp)
}

// SubmitAssessmentAnswer handles POST /gaming/assessment/sessions/{sessionId}/answer.
func (h *Handler) SubmitAssessmentAnswer(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")

	var req model.SubmitAssessmentAnswerRequest
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
	if req.ChosenKey != "A" && req.ChosenKey != "B" {
		writeError(w, http.StatusBadRequest, "chosen_key must be A or B")
		return
	}

	session, err := h.store.GetAssessmentSession(r.Context(), sessionID)
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

	// Verify the question ID matches the current position.
	currentQ := assessment.ByIndex(session.CurrentIndex)
	if currentQ == nil || currentQ.ID != req.QuestionID {
		writeError(w, http.StatusBadRequest, "question_id does not match current question")
		return
	}

	updatedSession, err := h.store.RecordAssessmentAnswer(
		r.Context(), sessionID, req.UserID, req.QuestionID, req.ChosenKey,
	)
	if err != nil {
		h.logger.Error("record assessment answer", zap.String("session_id", sessionID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := model.SubmitAssessmentAnswerResponse{Session: updatedSession}

	if updatedSession.Status == "completed" {
		// Award XP to the gaming profile.
		resp.XPEarned = updatedSession.XPEarned
		if updatedSession.XPEarned > 0 {
			currentXP, currentLevel, xerr := h.store.GetXPAndLevel(r.Context(), req.UserID)
			if xerr == nil {
				newXP, newLevel, _ := xp.Apply(currentXP, currentLevel, int64(updatedSession.XPEarned))
				_ = h.store.UpsertXP(r.Context(), req.UserID, newXP, newLevel)
			}
		}
	} else {
		resp.NextQuestion = questionForIndex(updatedSession.CurrentIndex)
	}

	writeJSON(w, http.StatusOK, resp)
}
