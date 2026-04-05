package loot_test

import (
	"testing"

	"github.com/teacherslounge/gaming-service/internal/loot"
)

func TestForBoss_TheAtom(t *testing.T) {
	drop := loot.ForBoss("the_atom")

	if drop.BadgeType != "boss_the_atom" {
		t.Errorf("want BadgeType=boss_the_atom, got %s", drop.BadgeType)
	}
	if drop.BadgeName != "ATOM SMASHER" {
		t.Errorf("want BadgeName=ATOM SMASHER, got %s", drop.BadgeName)
	}
	if drop.CosmeticKey != "avatar_frame" {
		t.Errorf("want CosmeticKey=avatar_frame, got %s", drop.CosmeticKey)
	}
	if drop.CosmeticValue != "atomic_ring" {
		t.Errorf("want CosmeticValue=atomic_ring, got %s", drop.CosmeticValue)
	}
	if drop.Gems < 15 || drop.Gems > 50 {
		t.Errorf("Gems=%d out of expected range [15, 50]", drop.Gems)
	}
	if drop.Quote == "" {
		t.Error("Quote should not be empty")
	}
}

func TestForBoss_UnknownID(t *testing.T) {
	drop := loot.ForBoss("not_a_boss")

	if drop.BadgeType == "" {
		t.Error("unknown boss should still yield a badge type")
	}
	if drop.Gems < 15 || drop.Gems > 50 {
		t.Errorf("Gems=%d out of expected range [15, 50]", drop.Gems)
	}
}

func TestForBoss_AllCatalogBosses(t *testing.T) {
	bosses := []string{
		"the_atom", "bonding_brothers", "name_lord",
		"the_stereochemist", "the_reactor",
		"algebra_dragon", "grammar_golem", "history_hydra", "science_sphinx",
	}
	for _, id := range bosses {
		drop := loot.ForBoss(id)
		if drop.BadgeType == "" {
			t.Errorf("boss %s: empty BadgeType", id)
		}
		if drop.BadgeName == "" {
			t.Errorf("boss %s: empty BadgeName", id)
		}
		if drop.Gems < 15 || drop.Gems > 50 {
			t.Errorf("boss %s: Gems=%d out of [15, 50]", id, drop.Gems)
		}
	}
}

func TestGemMin_Tiers(t *testing.T) {
	cases := []struct {
		tier int
		want int
	}{
		{1, 15},
		{2, 20},
		{3, 25},
		{4, 35},
		{5, 40},
	}
	for _, tc := range cases {
		got := loot.GemMin(tc.tier)
		if got != tc.want {
			t.Errorf("GemMin(%d) = %d, want %d", tc.tier, got, tc.want)
		}
	}
}

func TestGemMin_Clamp(t *testing.T) {
	// Out-of-range tiers should be clamped.
	if loot.GemMin(0) != loot.GemMin(1) {
		t.Error("tier 0 should clamp to tier 1")
	}
	if loot.GemMin(99) != loot.GemMin(5) {
		t.Error("tier 99 should clamp to tier 5")
	}
}

func TestForBoss_HigherTierMoreGems(t *testing.T) {
	// Minimum gems for tier 5 > minimum gems for tier 1.
	if loot.GemMin(5) <= loot.GemMin(1) {
		t.Errorf("tier 5 min gems (%d) should exceed tier 1 (%d)", loot.GemMin(5), loot.GemMin(1))
	}
}
