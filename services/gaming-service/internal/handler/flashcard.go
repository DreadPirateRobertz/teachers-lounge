package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/flashcard"
	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
)

// GenerateFlashcards handles POST /gaming/flashcards/generate.
//
// It auto-creates flashcards from every question in a completed quiz session.
// Existing cards for the session are skipped so the endpoint is idempotent.
// The caller must be the owner of the session (callerID == req.UserID).
func (h *Handler) GenerateFlashcards(w http.ResponseWriter, r *http.Request) {
	var req model.GenerateFlashcardsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" || req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "user_id and session_id required")
		return
	}

	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" || callerID != req.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	// Fetch and validate the quiz session.
	session, err := h.store.GetQuizSession(r.Context(), req.SessionID)
	if err != nil {
		h.logger.Error("generate flashcards: get session", zap.String("session_id", req.SessionID), zap.Error(err))
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if session.UserID != callerID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if session.Status != "completed" {
		writeError(w, http.StatusConflict, "session must be completed to generate flashcards")
		return
	}

	// Build a set of question IDs that already have cards for this session.
	existing, err := h.store.FlashcardsForSession(r.Context(), req.SessionID)
	if err != nil {
		h.logger.Error("generate flashcards: existing cards", zap.String("session_id", req.SessionID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	existingByQuestion := make(map[string]bool, len(existing))
	for _, c := range existing {
		if c.QuestionID != nil {
			existingByQuestion[*c.QuestionID] = true
		}
	}

	sessionID := req.SessionID
	var created []*model.Flashcard

	for _, qID := range session.QuestionIDs {
		if existingByQuestion[qID] {
			continue
		}

		q, err := h.store.GetQuestion(r.Context(), qID)
		if err != nil {
			h.logger.Warn("generate flashcards: skip missing question",
				zap.String("question_id", qID), zap.Error(err))
			continue
		}

		// Build the back of the card: correct answer text + explanation.
		back := correctOptionText(q) + "\n\n" + q.Explanation

		qIDCopy := qID
		topicVal := q.Topic

		card := &model.Flashcard{
			UserID:       req.UserID,
			QuestionID:   &qIDCopy,
			SessionID:    &sessionID,
			Front:        q.Question,
			Back:         back,
			Source:       "quiz",
			Topic:        &topicVal,
			CourseID:     session.CourseID,
			EaseFactor:   2.5,
			IntervalDays: 1,
			Repetitions:  0,
			NextReviewAt: time.Now().UTC(),
		}

		saved, err := h.store.CreateFlashcard(r.Context(), card)
		if err != nil {
			h.logger.Error("generate flashcards: create", zap.String("question_id", qID), zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		created = append(created, saved)
	}

	writeJSON(w, http.StatusCreated, model.GenerateFlashcardsResponse{
		Created: len(created),
		Cards:   created,
	})
}

// ListFlashcards handles GET /gaming/flashcards.
//
// Returns all flashcards for the authenticated user together with the current
// due count and total count.
func (h *Handler) ListFlashcards(w http.ResponseWriter, r *http.Request) {
	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	cards, err := h.store.ListFlashcards(r.Context(), callerID)
	if err != nil {
		h.logger.Error("list flashcards", zap.String("user_id", callerID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	now := time.Now().UTC()
	dueCount := 0
	for _, c := range cards {
		if !c.NextReviewAt.After(now) {
			dueCount++
		}
	}

	writeJSON(w, http.StatusOK, model.ListFlashcardsResponse{
		Cards:    cards,
		DueCount: dueCount,
		Total:    len(cards),
	})
}

// DueFlashcards handles GET /gaming/flashcards/due.
//
// Returns only the flashcards currently due for review (next_review_at <= now).
func (h *Handler) DueFlashcards(w http.ResponseWriter, r *http.Request) {
	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	cards, err := h.store.DueFlashcards(r.Context(), callerID)
	if err != nil {
		h.logger.Error("due flashcards", zap.String("user_id", callerID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, model.ListFlashcardsResponse{
		Cards:    cards,
		DueCount: len(cards),
		Total:    len(cards),
	})
}

// ReviewFlashcard handles POST /gaming/flashcards/{cardId}/review.
//
// Applies one SM-2 review cycle to the specified flashcard and records the
// review event. The caller must be the owner of the card.
func (h *Handler) ReviewFlashcard(w http.ResponseWriter, r *http.Request) {
	cardID := chi.URLParam(r, "cardId")
	if cardID == "" {
		writeError(w, http.StatusBadRequest, "cardId required")
		return
	}

	var req model.ReviewFlashcardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id required")
		return
	}
	if req.Quality < 0 || req.Quality > 5 {
		writeError(w, http.StatusBadRequest, "quality must be between 0 and 5")
		return
	}

	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" || callerID != req.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	// Verify ownership before applying the review.
	card, err := h.store.GetFlashcard(r.Context(), cardID)
	if err != nil {
		h.logger.Error("review flashcard: get card", zap.String("card_id", cardID), zap.Error(err))
		writeError(w, http.StatusNotFound, "flashcard not found")
		return
	}
	if card.UserID != callerID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	updated, err := h.store.ReviewFlashcard(r.Context(), cardID, req.UserID, req.Quality)
	if err != nil {
		h.logger.Error("review flashcard", zap.String("card_id", cardID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, model.ReviewFlashcardResponse{
		Card:         updated,
		NextReviewAt: updated.NextReviewAt,
		IntervalDays: updated.IntervalDays,
	})
}

// ExportAnki handles GET /gaming/flashcards/export/anki.
//
// Fetches all flashcards for the authenticated user and returns a binary
// Anki 2.1 .apkg file as an attachment download.
func (h *Handler) ExportAnki(w http.ResponseWriter, r *http.Request) {
	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	cards, err := h.store.AllFlashcardsForExport(r.Context(), callerID)
	if err != nil {
		h.logger.Error("export anki: fetch cards", zap.String("user_id", callerID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Convert store models to AnkiCard export DTOs.
	ankiCards := make([]flashcard.AnkiCard, len(cards))
	for i, c := range cards {
		topic := ""
		if c.Topic != nil {
			topic = *c.Topic
		}
		ankiCards[i] = flashcard.AnkiCard{
			ID:    c.ID,
			Front: c.Front,
			Back:  c.Back,
			Topic: topic,
		}
	}

	apkgBytes, err := flashcard.BuildAPKG("TeachersLounge", ankiCards)
	if err != nil {
		h.logger.Error("export anki: build apkg", zap.String("user_id", callerID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="teacherslounge-flashcards.apkg"`)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(apkgBytes)))
	w.WriteHeader(http.StatusOK)
	w.Write(apkgBytes)
}

// correctOptionText finds the text of the correct answer option in a question.
// Returns an empty string if the correct key does not match any option.
func correctOptionText(q *model.Question) string {
	for _, opt := range q.Options {
		if opt.Key == q.CorrectKey {
			return opt.Text
		}
	}
	return ""
}
