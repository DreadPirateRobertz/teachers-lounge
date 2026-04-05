package model

import "time"

// BossID identifies a boss encounter. Each boss has fixed stats.
type BossID string

// Boss defines the template for a boss encounter.
type Boss struct {
	ID       BossID `json:"id"`
	Name     string `json:"name"`
	MaxHP    int    `json:"max_hp"`
	Attack   int    `json:"attack"`   // base damage per hit on player
	Defense  int    `json:"defense"`  // damage reduction from player attacks
	XPReward int64  `json:"xp_reward"`
	GemReward int   `json:"gem_reward"`
}

// BattlePhase tracks the lifecycle of a boss battle.
type BattlePhase string

const (
	PhaseIntro   BattlePhase = "intro"
	PhaseActive  BattlePhase = "active"
	PhaseVictory BattlePhase = "victory"
	PhaseDefeat  BattlePhase = "defeat"
)

// PowerUpType identifies what kind of power-up is being used.
type PowerUpType string

const (
	PowerUpDoubleDamage PowerUpType = "double_damage"
	PowerUpShield       PowerUpType = "shield"
	PowerUpHeal         PowerUpType = "heal"
	PowerUpCritical     PowerUpType = "critical"
)

// ActivePowerUp represents a power-up currently in effect during a battle.
type ActivePowerUp struct {
	Type       PowerUpType `json:"type"`
	TurnsLeft  int         `json:"turns_left"`
}

// BattleSession is the in-memory state of an ongoing boss battle.
type BattleSession struct {
	SessionID    string         `json:"session_id"`
	UserID       string         `json:"user_id"`
	BossID       BossID         `json:"boss_id"`
	Phase        BattlePhase    `json:"phase"`
	PlayerHP     int            `json:"player_hp"`
	PlayerMaxHP  int            `json:"player_max_hp"`
	BossHP       int            `json:"boss_hp"`
	BossMaxHP    int            `json:"boss_max_hp"`
	BossAttack   int            `json:"boss_attack"`
	BossDefense  int            `json:"boss_defense"`
	Turn         int            `json:"turn"`
	ActivePowers []ActivePowerUp `json:"active_powers"`
	XPReward     int64          `json:"xp_reward"`
	GemReward    int            `json:"gem_reward"`
	StartedAt    time.Time      `json:"started_at"`
	ExpiresAt    time.Time      `json:"expires_at"`
}

// StartBattleRequest is the request body for POST /gaming/boss/start.
type StartBattleRequest struct {
	UserID string `json:"user_id"`
	BossID BossID `json:"boss_id"`
}

// StartBattleResponse is returned when a battle session is created.
type StartBattleResponse struct {
	Session BattleSession `json:"session"`
}

// AttackRequest is the request body for POST /gaming/boss/attack.
type AttackRequest struct {
	SessionID     string `json:"session_id"`
	AnswerCorrect bool   `json:"answer_correct"`
	// BaseDamage is the raw damage before modifiers (e.g., based on question difficulty).
	BaseDamage int `json:"base_damage"`
}

// AttackResponse is the response body for POST /gaming/boss/attack.
type AttackResponse struct {
	PlayerDamageDealt int         `json:"player_damage_dealt"`
	BossDamageDealt   int         `json:"boss_damage_dealt"`
	BossHP            int         `json:"boss_hp"`
	PlayerHP          int         `json:"player_hp"`
	Phase             BattlePhase `json:"phase"`
	Turn              int         `json:"turn"`
	// Taunt is an AI-generated boss taunt shown on a wrong answer. Empty on
	// correct answers and when taunt generation is unavailable.
	Taunt string `json:"taunt,omitempty"`
	// Set when the battle ends.
	Result *BattleResult `json:"result,omitempty"`
}

// PowerUpRequest is the request body for POST /gaming/boss/powerup.
type PowerUpRequest struct {
	SessionID string      `json:"session_id"`
	PowerUp   PowerUpType `json:"power_up"`
}

// PowerUpResponse is the response body for POST /gaming/boss/powerup.
type PowerUpResponse struct {
	Applied      bool           `json:"applied"`
	ActivePowers []ActivePowerUp `json:"active_powers"`
	GemsLeft     int            `json:"gems_left"`
}

// BattleResult is the outcome recorded when a battle ends.
type BattleResult struct {
	SessionID  string      `json:"session_id"`
	UserID     string      `json:"user_id"`
	BossID     BossID      `json:"boss_id"`
	Won        bool        `json:"won"`
	TurnsUsed  int         `json:"turns_used"`
	XPEarned   int64       `json:"xp_earned"`
	GemsEarned int         `json:"gems_earned"`
	FinishedAt time.Time   `json:"finished_at"`
}
