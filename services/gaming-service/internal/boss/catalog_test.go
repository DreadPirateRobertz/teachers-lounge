package boss_test

import (
	"testing"

	"github.com/teacherslounge/gaming-service/internal/boss"
)

func TestByID_KnownBoss(t *testing.T) {
	def := boss.ByID("the_atom")
	if def == nil {
		t.Fatal("expected to find the_atom boss")
	}
	if def.Name != "THE ATOM" {
		t.Errorf("want Name=THE ATOM, got %s", def.Name)
	}
	if def.Tier != 1 {
		t.Errorf("want Tier=1, got %d", def.Tier)
	}
}

func TestByID_Unknown(t *testing.T) {
	def := boss.ByID("not_a_boss")
	if def != nil {
		t.Errorf("expected nil for unknown boss, got %+v", def)
	}
}

func TestBossHP_ScalesWithTierAndLevel(t *testing.T) {
	// Higher tier → higher HP at same level.
	hpTier1 := boss.BossHP(1, 5)
	hpTier5 := boss.BossHP(5, 5)
	if hpTier5 <= hpTier1 {
		t.Errorf("tier 5 boss HP (%d) should exceed tier 1 (%d)", hpTier5, hpTier1)
	}

	// Higher player level → higher boss HP at same tier.
	hpLevel1 := boss.BossHP(1, 1)
	hpLevel10 := boss.BossHP(1, 10)
	if hpLevel10 <= hpLevel1 {
		t.Errorf("level 10 boss HP (%d) should exceed level 1 (%d)", hpLevel10, hpLevel1)
	}
}

func TestComboMultiplier(t *testing.T) {
	cases := []struct {
		streak int
		want   float64
	}{
		{0, 1.0},
		{1, 1.0},
		{2, 1.0},
		{3, 1.5},
		{4, 1.5},
		{5, 2.0},
		{10, 2.0},
	}
	for _, tc := range cases {
		got := boss.ComboMultiplier(tc.streak)
		if got != tc.want {
			t.Errorf("ComboMultiplier(%d) = %.1f, want %.1f", tc.streak, got, tc.want)
		}
	}
}

func TestDamageToBoss_MinimumTen(t *testing.T) {
	// Even low-XP questions deal at least 10 damage.
	dmg := boss.DamageToBoss(0, 1.0)
	if dmg < 10 {
		t.Errorf("minimum damage should be 10, got %d", dmg)
	}
}

func TestDamageToBoss_ComboScales(t *testing.T) {
	base := boss.DamageToBoss(40, 1.0)
	withCombo := boss.DamageToBoss(40, 1.5)
	if withCombo <= base {
		t.Errorf("combo damage (%d) should exceed base (%d)", withCombo, base)
	}
}

func TestDamageToStudent_ScalesWithDifficulty(t *testing.T) {
	easy := boss.DamageToStudent(1)
	hard := boss.DamageToStudent(5)
	if hard <= easy {
		t.Errorf("hard damage (%d) should exceed easy (%d)", hard, easy)
	}
}

func TestAllBossesHaveRequiredFields(t *testing.T) {
	for _, def := range boss.Catalog {
		if def.ID == "" {
			t.Errorf("boss missing ID: %+v", def)
		}
		if def.Name == "" {
			t.Errorf("boss %s missing Name", def.ID)
		}
		if def.Topic == "" {
			t.Errorf("boss %s missing Topic", def.ID)
		}
		if def.Tier < 1 || def.Tier > 5 {
			t.Errorf("boss %s Tier=%d out of range 1-5", def.ID, def.Tier)
		}
		if def.MaxRounds < 5 || def.MaxRounds > 7 {
			t.Errorf("boss %s MaxRounds=%d should be 5-7", def.ID, def.MaxRounds)
		}
		if def.VictoryXP <= 0 {
			t.Errorf("boss %s VictoryXP=%d should be positive", def.ID, def.VictoryXP)
		}
		if def.Taunt == "" {
			t.Errorf("boss %s missing Taunt", def.ID)
		}
	}
}
