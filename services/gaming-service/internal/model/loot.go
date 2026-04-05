package model

import "time"

// Achievement is a badge earned by a user on completing a milestone.
type Achievement struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	AchievementType string    `json:"achievement_type"`
	BadgeName       string    `json:"badge_name"`
	EarnedAt        time.Time `json:"earned_at"`
}

// Cosmetic is a visual customisation item unlocked by the player.
type Cosmetic struct {
	// Key identifies the cosmetic category: "avatar_frame", "color_palette", "title".
	Key string `json:"key"`
	// Value is the item identifier within that category.
	Value string `json:"value"`
}

// LootDrop is the reward package shown on boss defeat — it drives the loot reveal UI.
type LootDrop struct {
	XPEarned    int64        `json:"xp_earned"`
	GemsEarned  int          `json:"gems_earned"`
	Achievement *Achievement `json:"achievement,omitempty"`
	Cosmetic    *Cosmetic    `json:"cosmetic,omitempty"`
	// Quote is a sci-fi quote displayed during the loot reveal animation.
	Quote string `json:"quote"`
	// NewBadge is true when the achievement was just earned (not a duplicate).
	NewBadge bool `json:"new_badge"`
}

// AchievementsResponse is the response body for GET /gaming/achievements/{userId}.
type AchievementsResponse struct {
	Achievements []Achievement `json:"achievements"`
}
