package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/battle"
	"github.com/teacherslounge/gaming-service/internal/loot"
	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/xp"
)

// StartBattle handles POST /gaming/boss/start
func (h *Handler) StartBattle(w http.ResponseWriter, r *http.Request) {
	var req model.StartBattleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" || callerID != req.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	boss, ok := battle.BossCatalog[req.BossID]
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown boss_id")
		return
	}

	now := time.Now().UTC()
	session := &model.BattleSession{
		SessionID:    uuid.New().String(),
		UserID:       req.UserID,
		BossID:       req.BossID,
		Phase:        model.PhaseActive,
		PlayerHP:     battle.DefaultPlayerHP,
		PlayerMaxHP:  battle.DefaultPlayerHP,
		BossHP:       boss.MaxHP,
		BossMaxHP:    boss.MaxHP,
		BossAttack:   boss.Attack,
		BossDefense:  boss.Defense,
		Turn:         0,
		ActivePowers: []model.ActivePowerUp{},
		XPReward:     boss.XPReward,
		GemReward:    boss.GemReward,
		StartedAt:    now,
		ExpiresAt:    now.Add(30 * time.Minute),
	}

	if err := h.store.SaveBattleSession(r.Context(), session); err != nil {
		h.logger.Error("save battle session", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, model.StartBattleResponse{Session: *session})
}

// GetBattleSession handles GET /gaming/boss/session/{sessionId}
func (h *Handler) GetBattleSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	callerID := middleware.UserIDFromContext(r.Context())

	session, err := h.store.GetBattleSession(r.Context(), sessionID)
	if err != nil {
		h.logger.Error("get battle session", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if session == nil {
		writeError(w, http.StatusNotFound, "session not found or expired")
		return
	}
	if session.UserID != callerID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	writeJSON(w, http.StatusOK, session)
}

// Attack handles POST /gaming/boss/attack
func (h *Handler) Attack(w http.ResponseWriter, r *http.Request) {
	var req model.AttackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	callerID := middleware.UserIDFromContext(r.Context())

	session, err := h.store.GetBattleSession(r.Context(), req.SessionID)
	if err != nil {
		h.logger.Error("get battle session", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if session == nil {
		writeError(w, http.StatusNotFound, "session not found or expired")
		return
	}
	if session.UserID != callerID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if session.Phase != model.PhaseActive {
		writeError(w, http.StatusConflict, "battle is not active")
		return
	}

	session.Turn++

	// Player attacks boss (only on correct answer).
	var playerDmg int
	if req.AnswerCorrect {
		playerDmg = battle.CalculateDamage(req.BaseDamage, session.BossDefense, session.ActivePowers)
		session.BossHP -= playerDmg
		if session.BossHP < 0 {
			session.BossHP = 0
		}
	}

	// Boss attacks player (always, unless boss is dead).
	var bossDmg int
	if session.BossHP > 0 {
		bossDmg = battle.CalculateBossAttack(session.BossAttack, session.ActivePowers)
		session.PlayerHP -= bossDmg
		if session.PlayerHP < 0 {
			session.PlayerHP = 0
		}
	}

	// Tick power-up durations.
	session.ActivePowers = battle.TickPowerUps(session.ActivePowers)

	// Check win/lose conditions.
	resp := model.AttackResponse{
		PlayerDamageDealt: playerDmg,
		BossDamageDealt:   bossDmg,
		BossHP:            session.BossHP,
		PlayerHP:          session.PlayerHP,
		Phase:             model.PhaseActive,
		Turn:              session.Turn,
	}

	if session.BossHP <= 0 {
		session.Phase = model.PhaseVictory
		resp.Phase = model.PhaseVictory
		result := h.finishBattle(r, session, true)
		resp.Result = result
	} else if session.PlayerHP <= 0 {
		session.Phase = model.PhaseDefeat
		resp.Phase = model.PhaseDefeat
		result := h.finishBattle(r, session, false)
		resp.Result = result
	} else {
		// Save updated session.
		if err := h.store.SaveBattleSession(r.Context(), session); err != nil {
			h.logger.Error("save battle session", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// ActivatePowerUp handles POST /gaming/boss/powerup
func (h *Handler) ActivatePowerUp(w http.ResponseWriter, r *http.Request) {
	var req model.PowerUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	callerID := middleware.UserIDFromContext(r.Context())

	session, err := h.store.GetBattleSession(r.Context(), req.SessionID)
	if err != nil {
		h.logger.Error("get battle session", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if session == nil {
		writeError(w, http.StatusNotFound, "session not found or expired")
		return
	}
	if session.UserID != callerID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if session.Phase != model.PhaseActive {
		writeError(w, http.StatusConflict, "battle is not active")
		return
	}

	cost, ok := battle.PowerUpCost[req.PowerUp]
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown power_up type")
		return
	}

	// Deduct gems.
	gemsLeft, err := h.store.DeductGems(r.Context(), callerID, cost)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "not enough gems")
		return
	}

	// Apply the power-up.
	if req.PowerUp == model.PowerUpHeal {
		session.PlayerHP = battle.ApplyHeal(session.PlayerHP, session.PlayerMaxHP)
	} else {
		duration := battle.PowerUpDuration[req.PowerUp]
		session.ActivePowers = append(session.ActivePowers, model.ActivePowerUp{
			Type:      req.PowerUp,
			TurnsLeft: duration,
		})
	}

	if err := h.store.SaveBattleSession(r.Context(), session); err != nil {
		h.logger.Error("save battle session after powerup", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, model.PowerUpResponse{
		Applied:      true,
		ActivePowers: session.ActivePowers,
		GemsLeft:     gemsLeft,
	})
}

// ForfeitBattle handles POST /gaming/boss/forfeit
func (h *Handler) ForfeitBattle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	callerID := middleware.UserIDFromContext(r.Context())

	session, err := h.store.GetBattleSession(r.Context(), req.SessionID)
	if err != nil {
		h.logger.Error("get battle session", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if session == nil {
		writeError(w, http.StatusNotFound, "session not found or expired")
		return
	}
	if session.UserID != callerID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if session.Phase != model.PhaseActive {
		writeError(w, http.StatusConflict, "battle is not active")
		return
	}

	session.Phase = model.PhaseDefeat
	result := h.finishBattle(r, session, false)

	writeJSON(w, http.StatusOK, result)
}

// finishBattle records the result, awards XP and loot on victory, and cleans
// up the Redis session. On victory it computes the boss-specific loot drop
// (gems, achievement badge, cosmetic), persists each reward, and attaches the
// full LootDrop to the returned BattleResult for the UI to consume.
func (h *Handler) finishBattle(r *http.Request, session *model.BattleSession, won bool) *model.BattleResult {
	ctx := r.Context()

	var xpEarned int64
	var drop *loot.Drop
	if won {
		xpEarned = session.XPReward
		d := loot.ForBoss(string(session.BossID))
		drop = &d
	} else {
		// Consolation XP: 10% of reward.
		xpEarned = session.XPReward / 10
	}

	gemsEarned := 0
	if drop != nil {
		gemsEarned = drop.Gems
	}

	result := &model.BattleResult{
		SessionID:  session.SessionID,
		UserID:     session.UserID,
		BossID:     session.BossID,
		Won:        won,
		TurnsUsed:  session.Turn,
		XPEarned:   xpEarned,
		GemsEarned: gemsEarned,
		FinishedAt: time.Now().UTC(),
	}

	if err := h.store.RecordBattleResult(ctx, result); err != nil {
		h.logger.Error("record battle result", zap.Error(err))
	}

	// Award XP through the standard path.
	if xpEarned > 0 {
		currentXP, currentLevel, err := h.store.GetXPAndLevel(ctx, session.UserID)
		if err != nil {
			h.logger.Error("get xp for battle reward", zap.Error(err))
		} else {
			newXP, newLevel, _ := xp.Apply(currentXP, currentLevel, xpEarned)
			if err := h.store.UpsertXP(ctx, session.UserID, newXP, newLevel); err != nil {
				h.logger.Error("upsert xp for battle reward", zap.Error(err))
			}
		}
	}

	// Persist loot on victory and build the LootDrop for the response.
	if drop != nil {
		lootDrop := &model.LootDrop{
			XPEarned:   xpEarned,
			GemsEarned: gemsEarned,
			Quote:      drop.Quote,
		}

		// Grant achievement badge.
		if drop.BadgeType != "" {
			achievement, isNew, err := h.store.GrantAchievement(ctx, session.UserID, drop.BadgeType, drop.BadgeName)
			if err != nil {
				h.logger.Error("grant achievement", zap.String("type", drop.BadgeType), zap.Error(err))
			} else {
				lootDrop.Achievement = achievement
				lootDrop.NewBadge = isNew
			}
		}

		// Persist cosmetic item.
		if drop.CosmeticKey != "" {
			if err := h.store.AddCosmeticItem(ctx, session.UserID, drop.CosmeticKey, drop.CosmeticValue); err != nil {
				h.logger.Error("add cosmetic", zap.String("key", drop.CosmeticKey), zap.Error(err))
			} else {
				lootDrop.Cosmetic = &model.Cosmetic{
					Key:   drop.CosmeticKey,
					Value: drop.CosmeticValue,
				}
			}
		}

		result.LootDrop = lootDrop
	}

	// Clean up Redis session.
	if err := h.store.DeleteBattleSession(ctx, session.SessionID); err != nil {
		h.logger.Error("delete battle session", zap.Error(err))
	}

	return result
}
