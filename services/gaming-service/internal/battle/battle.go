package battle

import "github.com/teacherslounge/gaming-service/internal/model"

// Default player stats for boss battles.
const (
	DefaultPlayerHP = 100
)

// BossCatalog holds all available bosses keyed by ID.
var BossCatalog = map[model.BossID]model.Boss{
	"algebra_dragon": {
		ID: "algebra_dragon", Name: "Algebra Dragon",
		MaxHP: 200, Attack: 15, Defense: 5,
		XPReward: 500, GemReward: 10,
	},
	"grammar_golem": {
		ID: "grammar_golem", Name: "Grammar Golem",
		MaxHP: 150, Attack: 12, Defense: 3,
		XPReward: 350, GemReward: 7,
	},
	"history_hydra": {
		ID: "history_hydra", Name: "History Hydra",
		MaxHP: 300, Attack: 20, Defense: 8,
		XPReward: 800, GemReward: 15,
	},
	"science_sphinx": {
		ID: "science_sphinx", Name: "Science Sphinx",
		MaxHP: 250, Attack: 18, Defense: 6,
		XPReward: 650, GemReward: 12,
	},
}

// PowerUpCost is how many gems each power-up costs to activate.
var PowerUpCost = map[model.PowerUpType]int{
	model.PowerUpDoubleDamage: 3,
	model.PowerUpShield:       2,
	model.PowerUpHeal:         2,
	model.PowerUpCritical:     5,
}

// PowerUpDuration is how many turns each power-up lasts.
var PowerUpDuration = map[model.PowerUpType]int{
	model.PowerUpDoubleDamage: 2,
	model.PowerUpShield:       3,
	model.PowerUpHeal:         0, // instant
	model.PowerUpCritical:     1,
}

// HealAmount is how much HP the heal power-up restores.
const HealAmount = 30

// CalculateDamage computes the player's damage dealt to the boss for a given turn.
// Returns 0 if the answer was wrong.
func CalculateDamage(baseDamage, bossDefense int, activePowers []model.ActivePowerUp) int {
	if baseDamage <= 0 {
		return 0
	}

	dmg := baseDamage - bossDefense
	if dmg < 1 {
		dmg = 1 // minimum 1 damage on correct answer
	}

	for _, p := range activePowers {
		if p.TurnsLeft <= 0 {
			continue
		}
		switch p.Type {
		case model.PowerUpDoubleDamage:
			dmg *= 2
		case model.PowerUpCritical:
			dmg *= 3
		}
	}

	return dmg
}

// CalculateBossAttack computes the boss's damage to the player.
// Shield power-up halves incoming damage.
func CalculateBossAttack(bossAttack int, activePowers []model.ActivePowerUp) int {
	dmg := bossAttack
	for _, p := range activePowers {
		if p.TurnsLeft <= 0 {
			continue
		}
		if p.Type == model.PowerUpShield {
			dmg /= 2
			break
		}
	}
	if dmg < 0 {
		dmg = 0
	}
	return dmg
}

// TickPowerUps decrements turn counters and removes expired power-ups.
func TickPowerUps(powers []model.ActivePowerUp) []model.ActivePowerUp {
	result := make([]model.ActivePowerUp, 0, len(powers))
	for _, p := range powers {
		p.TurnsLeft--
		if p.TurnsLeft > 0 {
			result = append(result, p)
		}
	}
	return result
}

// ApplyHeal adds HP to the player, capped at max.
func ApplyHeal(currentHP, maxHP int) int {
	hp := currentHP + HealAmount
	if hp > maxHP {
		hp = maxHP
	}
	return hp
}
