package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/teacherslounge/gaming-service/internal/battle"
	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/store"
	"go.uber.org/zap"
)

// shopCatalog is the ordered list of items sold in the gem shop.
var shopCatalog = model.ShopCatalog{
	Items: []model.ShopItem{
		{
			Type:        model.PowerUpShield,
			Label:       "Shield",
			Icon:        "🛡️",
			Description: "Blocks one wrong answer's damage for 3 turns.",
			GemCost:     battle.PowerUpCost[model.PowerUpShield],
		},
		{
			Type:        model.PowerUpDoubleDamage,
			Label:       "Double Damage",
			Icon:        "⚔️",
			Description: "Doubles damage dealt on correct answers for 2 turns.",
			GemCost:     battle.PowerUpCost[model.PowerUpDoubleDamage],
		},
		{
			Type:        model.PowerUpHeal,
			Label:       "Heal",
			Icon:        "💊",
			Description: "Instantly restores 30 HP.",
			GemCost:     battle.PowerUpCost[model.PowerUpHeal],
		},
		{
			Type:        model.PowerUpCritical,
			Label:       "Critical Hit",
			Icon:        "💥",
			Description: "Guarantees a critical hit on your next correct answer.",
			GemCost:     battle.PowerUpCost[model.PowerUpCritical],
		},
	},
}

// GetShopCatalog handles GET /gaming/shop/catalog.
// Returns the full list of purchasable power-ups with gem costs.
func (h *Handler) GetShopCatalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, shopCatalog)
}

// BuyPowerUp handles POST /gaming/shop/buy.
// Deducts gems and adds the power-up to the caller's inventory.
func (h *Handler) BuyPowerUp(w http.ResponseWriter, r *http.Request) {
	var req model.BuyPowerUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" || callerID != req.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	cost, ok := battle.PowerUpCost[req.PowerUp]
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown power_up type")
		return
	}

	gemsLeft, newCount, err := h.store.BuyPowerUp(r.Context(), callerID, req.PowerUp, cost)
	if err != nil {
		if errors.Is(err, store.ErrNoGems) {
			writeError(w, http.StatusUnprocessableEntity, "not enough gems")
			return
		}
		h.logger.Error("buy power-up", zap.String("user_id", callerID), zap.String("power_up", string(req.PowerUp)), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, model.BuyPowerUpResponse{
		PowerUp:  req.PowerUp,
		NewCount: newCount,
		GemsLeft: gemsLeft,
	})
}
