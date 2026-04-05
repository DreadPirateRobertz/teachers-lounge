// Package loot defines boss-specific loot drops awarded on victory.
// It maps boss IDs to achievement badges and cosmetic items,
// and computes randomised gem rewards seeded by boss tier.
package loot

import (
	"math/rand"
	"time"
)

// Drop is the reward package granted when a player defeats a boss.
type Drop struct {
	// Gems earned — random in [GemMin(tier), 50], higher tier → higher floor.
	Gems int
	// BadgeType is the achievement_type written to the achievements table.
	BadgeType string
	// BadgeName is the human-readable display name for the badge.
	BadgeName string
	// CosmeticKey is the JSONB key under gaming_profiles.cosmetics; empty if none.
	CosmeticKey string
	// CosmeticValue is the string value stored at CosmeticKey; empty if none.
	CosmeticValue string
	// Quote is a sci-fi quote displayed during the loot reveal animation.
	Quote string
}

// bossDef is the static definition for a single boss's loot.
type bossDef struct {
	badgeType     string
	badgeName     string
	cosmeticKey   string
	cosmeticValue string
	tier          int
}

// bossCatalog maps boss_id → static loot definition.
var bossCatalog = map[string]bossDef{
	// Phase 4 chemistry bosses (internal/boss/catalog.go)
	"the_atom": {
		badgeType:     "boss_the_atom",
		badgeName:     "ATOM SMASHER",
		cosmeticKey:   "avatar_frame",
		cosmeticValue: "atomic_ring",
		tier:          1,
	},
	"bonding_brothers": {
		badgeType:     "boss_bonding_brothers",
		badgeName:     "BOND BREAKER",
		cosmeticKey:   "color_palette",
		cosmeticValue: "molecule_teal",
		tier:          2,
	},
	"name_lord": {
		badgeType:     "boss_name_lord",
		badgeName:     "NOMENCLATURE KNIGHT",
		cosmeticKey:   "title",
		cosmeticValue: "The Named",
		tier:          3,
	},
	"the_stereochemist": {
		badgeType:     "boss_the_stereochemist",
		badgeName:     "MIRROR BREAKER",
		cosmeticKey:   "avatar_frame",
		cosmeticValue: "stereo_lens",
		tier:          4,
	},
	"the_reactor": {
		badgeType:     "boss_the_reactor",
		badgeName:     "REACTION MASTER",
		cosmeticKey:   "title",
		cosmeticValue: "Reaction Master",
		tier:          5,
	},
	// Legacy boss catalog (internal/battle/battle.go)
	"algebra_dragon": {
		badgeType:     "boss_algebra_dragon",
		badgeName:     "DRAGON SLAYER",
		cosmeticKey:   "avatar_frame",
		cosmeticValue: "dragon_scales",
		tier:          2,
	},
	"grammar_golem": {
		badgeType:     "boss_grammar_golem",
		badgeName:     "GOLEM CRUSHER",
		cosmeticKey:   "color_palette",
		cosmeticValue: "golem_grey",
		tier:          1,
	},
	"history_hydra": {
		badgeType:     "boss_history_hydra",
		badgeName:     "HYDRA HUNTER",
		cosmeticKey:   "title",
		cosmeticValue: "Hydra Hunter",
		tier:          3,
	},
	"science_sphinx": {
		badgeType:     "boss_science_sphinx",
		badgeName:     "SPHINX RIDDLER",
		cosmeticKey:   "avatar_frame",
		cosmeticValue: "sphinx_crown",
		tier:          2,
	},
}

// victoryQuotes are displayed during the loot reveal animation.
var victoryQuotes = []string{
	"\"Space is big. You just won't believe how vastly, hugely, mind-bogglingly big it is.\" — Douglas Adams",
	"\"The universe is under no obligation to make sense to you.\" — Neil deGrasse Tyson",
	"\"We are all made of star-stuff.\" — Carl Sagan",
	"\"Look up at the stars and not down at your feet.\" — Stephen Hawking",
	"\"The most beautiful thing we can experience is the mysterious.\" — Albert Einstein",
	"\"Somewhere, something incredible is waiting to be known.\" — Carl Sagan",
	"\"We shall not cease from exploration.\" — T.S. Eliot",
	"\"Science and everyday life cannot and should not be separated.\" — Rosalind Franklin",
}

// GemMin returns the minimum gem reward for a given boss tier (1–5).
// Higher tiers yield a higher gem floor so harder victories feel more valuable.
func GemMin(tier int) int {
	if tier < 1 {
		tier = 1
	}
	// tier 1→15, tier 2→20, tier 3→25, tier 4→35, tier 5→40
	floors := [6]int{0, 15, 20, 25, 35, 40}
	if tier > 5 {
		tier = 5
	}
	return floors[tier]
}

// ForBoss computes the loot Drop for defeating the given boss. The gem count is
// randomised in [GemMin(tier), 50]. Unknown boss IDs receive a generic reward.
func ForBoss(bossID string) Drop {
	def, ok := bossCatalog[bossID]
	if !ok {
		def = bossDef{
			badgeType:     "boss_slayer",
			badgeName:     "BOSS SLAYER",
			cosmeticKey:   "",
			cosmeticValue: "",
			tier:          1,
		}
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	minGems := GemMin(def.tier)
	spread := 50 - minGems
	if spread < 0 {
		spread = 0
	}
	gems := minGems + rng.Intn(spread+1)
	quote := victoryQuotes[rng.Intn(len(victoryQuotes))]

	return Drop{
		Gems:          gems,
		BadgeType:     def.badgeType,
		BadgeName:     def.badgeName,
		CosmeticKey:   def.cosmeticKey,
		CosmeticValue: def.cosmeticValue,
		Quote:         quote,
	}
}
