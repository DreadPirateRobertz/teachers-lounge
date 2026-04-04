package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/boss"
	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/store"
)

// StartBoss handles POST /gaming/boss/start.
//
// The caller selects a boss by ID (e.g. "the_atom"). The service looks up the
// boss in the catalog, fetches questions for its topic, initialises HP values
// scaled to the player's current level, and returns the first question.
func (h *Handler) StartBoss(w http.ResponseWriter, r *http.Request) {
	var req model.BossStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" || req.BossID == "" {
		writeError(w, http.StatusBadRequest, "user_id and boss_id required")
		return
	}

	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" || callerID != req.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	def := boss.ByID(req.BossID)
	if def == nil {
		writeError(w, http.StatusNotFound, "unknown boss_id")
		return
	}

	// Scale boss HP to the player's level for adaptive difficulty.
	_, playerLevel, err := h.store.GetXPAndLevel(r.Context(), req.UserID)
	if err != nil {
		h.logger.Error("get xp for boss start", zap.String("user_id", req.UserID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	bossHP := boss.BossHP(def.Tier, playerLevel)

	questions, err := h.store.GetRandomQuestions(r.Context(), def.Topic, def.MaxRounds)
	if err != nil {
		h.logger.Error("get boss questions", zap.String("topic", def.Topic), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if len(questions) == 0 {
		writeError(w, http.StatusNotFound, "no questions found for boss topic")
		return
	}

	ids := make([]string, len(questions))
	for i, q := range questions {
		ids[i] = q.ID
	}

	session, err := h.store.StartBossBattle(
		r.Context(),
		req.UserID, def.ID, def.Name, def.Topic,
		def.MaxRounds, bossHP, ids,
	)
	if err != nil {
		if errors.Is(err, store.ErrActiveBossBattle) {
			writeError(w, http.StatusConflict, "active boss battle already in progress")
			return
		}
		h.logger.Error("start boss battle", zap.String("user_id", req.UserID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, model.BossStartResponse{
		Session:  session,
		Question: stripAnswer(questions[0]),
	})
}

// GetBossSession handles GET /gaming/boss/sessions/{sessionId}.
func (h *Handler) GetBossSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	callerID := middleware.UserIDFromContext(r.Context())

	session, err := h.store.GetBossSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, store.ErrBossSessionNotFound) {
			writeError(w, http.StatusNotFound, "boss session not found")
			return
		}
		h.logger.Error("get boss session", zap.String("session_id", sessionID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if session.UserID != callerID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	resp := model.BossSessionResponse{Session: session}

	if session.Status == "active" && session.CurrentIndex < len(session.QuestionIDs) {
		q, err := h.store.GetQuestion(r.Context(), session.QuestionIDs[session.CurrentIndex])
		if err != nil {
			h.logger.Error("get boss current question", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		resp.Question = stripAnswer(q)
	}

	writeJSON(w, http.StatusOK, resp)
}

// SubmitBossAnswer handles POST /gaming/boss/sessions/{sessionId}/answer.
//
// Each call processes one round of the boss fight:
//   - Correct answer → deal damage to the boss; extend combo streak
//   - Wrong answer   → take damage from the boss; reset combo streak
//
// The battle ends when: boss HP ≤ 0 (victory), student HP ≤ 0 (defeat), or
// all rounds have been played. If rounds are exhausted with boss HP > 0, the
// student loses (they couldn't finish the boss in time).
//
// On battle completion, the result is persisted to Postgres and profile XP
// is updated; the full updated profile fields are included in the response.
func (h *Handler) SubmitBossAnswer(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")

	var req model.BossAnswerRequest
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

	session, err := h.store.GetBossSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, store.ErrBossSessionNotFound) {
			writeError(w, http.StatusNotFound, "boss session not found")
			return
		}
		h.logger.Error("get boss session for answer", zap.String("session_id", sessionID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if session.UserID != callerID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if session.Status != "active" {
		writeError(w, http.StatusConflict, "boss session is not active")
		return
	}
	if session.CurrentIndex >= len(session.QuestionIDs) || session.QuestionIDs[session.CurrentIndex] != req.QuestionID {
		writeError(w, http.StatusBadRequest, "question_id does not match current round question")
		return
	}

	question, err := h.store.GetQuestion(r.Context(), req.QuestionID)
	if err != nil {
		h.logger.Error("get boss question for answer", zap.String("question_id", req.QuestionID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	isCorrect := req.ChosenKey == question.CorrectKey

	// --- Damage calculation ---
	var (
		damageToBoss    int
		damageToStudent int
		xpEarned        int
		taunt           string
		multiplier      float64
	)

	if isCorrect {
		session.ComboStreak++
		multiplier = boss.ComboMultiplier(session.ComboStreak)
		damageToBoss = boss.DamageToBoss(question.XPReward, multiplier)
		xpEarned = question.XPReward
		session.BossHP -= damageToBoss
		if session.BossHP < 0 {
			session.BossHP = 0
		}
	} else {
		multiplier = 1.0
		session.ComboStreak = 0
		damageToStudent = boss.DamageToStudent(question.Difficulty)
		session.StudentHP -= damageToStudent
		if session.StudentHP < 0 {
			session.StudentHP = 0
		}
		def := boss.ByID(session.BossID)
		if def != nil {
			taunt = def.Taunt
		}
	}

	session.TotalXP += xpEarned
	session.CurrentIndex++
	session.Round = session.CurrentIndex + 1 // advance for display

	// --- Determine if battle is over ---
	battleOver := false
	victory := false

	switch {
	case session.BossHP <= 0:
		battleOver = true
		victory = true
		session.Status = "victory"
	case session.StudentHP <= 0:
		battleOver = true
		session.Status = "defeat"
	case session.CurrentIndex >= len(session.QuestionIDs):
		// All rounds played; boss survives → defeat.
		battleOver = true
		if session.BossHP <= 0 {
			victory = true
			session.Status = "victory"
		} else {
			session.Status = "defeat"
		}
	}

	resp := model.BossAnswerResponse{
		Correct:         isCorrect,
		CorrectKey:      question.CorrectKey,
		Explanation:     question.Explanation,
		DamageToBoss:    damageToBoss,
		DamageToStudent: damageToStudent,
		ComboStreak:     session.ComboStreak,
		ComboMultiplier: multiplier,
		NewStudentHP:    session.StudentHP,
		NewBossHP:       session.BossHP,
		XPEarned:        xpEarned,
		TotalXP:         session.TotalXP,
		BattleOver:      battleOver,
		Victory:         victory,
		Taunt:           taunt,
	}

	if battleOver {
		now := time.Now().UTC()
		session.CompletedAt = &now

		bonusXP := 0
		if victory {
			def := boss.ByID(session.BossID)
			if def != nil {
				bonusXP = def.VictoryXP
				resp.VictoryXP = bonusXP
			}
		}

		newXP, newLevel, leveledUp, bossesDefeated, completeErr := h.store.CompleteBossBattle(r.Context(), session, bonusXP)
		if completeErr != nil {
			h.logger.Error("complete boss battle", zap.String("session_id", sessionID), zap.Error(completeErr))
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		resp.NewXP = newXP
		resp.NewLevel = newLevel
		resp.LevelUp = leveledUp
		if victory {
			resp.BossesDefeated = bossesDefeated
		}
	} else {
		// Battle still active — persist updated state and queue next question.
		if err := h.store.SaveBossSession(r.Context(), session); err != nil {
			h.logger.Error("save boss session", zap.String("session_id", sessionID), zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		nextQID := session.QuestionIDs[session.CurrentIndex]
		nextQ, err := h.store.GetQuestion(r.Context(), nextQID)
		if err != nil {
			h.logger.Error("get next boss question", zap.String("question_id", nextQID), zap.Error(err))
		} else {
			resp.NextQuestion = stripAnswer(nextQ)
		}
	}

	resp.Session = session
	writeJSON(w, http.StatusOK, resp)
}
