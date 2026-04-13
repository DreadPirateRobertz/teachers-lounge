package boss_test

import (
	"strings"
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

func TestByID_TheBonder(t *testing.T) {
	def := boss.ByID("the_bonder")
	if def == nil {
		t.Fatal("expected to find the_bonder boss")
	}
	if def.Name != "THE BONDER" {
		t.Errorf("want Name=THE BONDER, got %s", def.Name)
	}
	if def.Tier != 2 {
		t.Errorf("want Tier=2, got %d", def.Tier)
	}
}

func TestByID_FinalBoss(t *testing.T) {
	def := boss.ByID("final_boss")
	if def == nil {
		t.Fatal("expected to find final_boss")
	}
	if def.Name != "FINAL BOSS" {
		t.Errorf("want Name=FINAL BOSS, got %s", def.Name)
	}
	if def.Tier != 6 {
		t.Errorf("want Tier=6, got %d", def.Tier)
	}
	if def.MaxRounds != 10 {
		t.Errorf("final boss should have 10 rounds, got %d", def.MaxRounds)
	}
}

func TestByID_Unknown(t *testing.T) {
	def := boss.ByID("not_a_boss")
	if def != nil {
		t.Errorf("expected nil for unknown boss, got %+v", def)
	}
}

func TestCatalog_HasSixBosses(t *testing.T) {
	if len(boss.Catalog) != 6 {
		t.Errorf("expected 6 bosses in catalog, got %d", len(boss.Catalog))
	}
}

func TestCatalog_OrderedByTier(t *testing.T) {
	for i, def := range boss.Catalog {
		if def.Tier != i+1 {
			t.Errorf("boss at index %d has Tier=%d, want %d", i, def.Tier, i+1)
		}
	}
}

func TestBossHP_ScalesWithTierAndLevel(t *testing.T) {
	// Higher tier → higher HP at same level.
	hpTier1 := boss.BossHP(1, 5)
	hpTier6 := boss.BossHP(6, 5)
	if hpTier6 <= hpTier1 {
		t.Errorf("tier 6 boss HP (%d) should exceed tier 1 (%d)", hpTier6, hpTier1)
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
		if def.Tier < 1 || def.Tier > 6 {
			t.Errorf("boss %s Tier=%d out of range 1-6", def.ID, def.Tier)
		}
		if def.MaxRounds < 5 {
			t.Errorf("boss %s MaxRounds=%d should be >= 5", def.ID, def.MaxRounds)
		}
		if def.VictoryXP <= 0 {
			t.Errorf("boss %s VictoryXP=%d should be positive", def.ID, def.VictoryXP)
		}
		if def.Taunt == "" {
			t.Errorf("boss %s missing Taunt", def.ID)
		}
	}
}

func TestAllBossesHaveVisualConfig(t *testing.T) {
	for _, def := range boss.Catalog {
		if def.Visual.PrimaryColor == "" {
			t.Errorf("boss %s missing Visual.PrimaryColor", def.ID)
		}
		if !strings.HasPrefix(def.Visual.PrimaryColor, "#") {
			t.Errorf("boss %s PrimaryColor %q should be a hex color", def.ID, def.Visual.PrimaryColor)
		}
		if def.Visual.Geometry == "" {
			t.Errorf("boss %s missing Visual.Geometry", def.ID)
		}
		if len(def.Visual.TauntPool) < 3 {
			t.Errorf("boss %s should have at least 3 taunts, has %d", def.ID, len(def.Visual.TauntPool))
		}
		if def.Visual.AttackDescription == "" {
			t.Errorf("boss %s missing Visual.AttackDescription", def.ID)
		}
		if def.Visual.IdleDescription == "" {
			t.Errorf("boss %s missing Visual.IdleDescription", def.ID)
		}
	}
}

func TestAllBossesHaveChapterConceptPaths(t *testing.T) {
	for _, def := range boss.Catalog {
		if len(def.ChapterConceptPaths) == 0 {
			t.Errorf("boss %s missing ChapterConceptPaths — mastery gate needs at least one ltree lquery", def.ID)
		}
		for _, p := range def.ChapterConceptPaths {
			if p == "" {
				t.Errorf("boss %s has empty ChapterConceptPaths entry", def.ID)
			}
		}
	}
}

func TestCatalog_ExactlyOneFirstBoss(t *testing.T) {
	firsts := 0
	var firstID string
	for _, def := range boss.Catalog {
		if def.IsFirstBoss {
			firsts++
			firstID = def.ID
		}
	}
	if firsts != 1 {
		t.Errorf("expected exactly 1 boss with IsFirstBoss=true, got %d", firsts)
	}
	// The onboarding boss must be tier 1 — spec requires the first available
	// boss to sit at the bottom of the progression trail.
	if first := boss.ByID(firstID); first != nil && first.Tier != 1 {
		t.Errorf("IsFirstBoss should be tier 1, got tier %d on %s", first.Tier, firstID)
	}
}

func TestMasteryUnlockThreshold_Is60Percent(t *testing.T) {
	// Spec: "Completing a mastery threshold (60%+ on a chapter's concepts)
	// unlocks the chapter boss." Guard against silent drift.
	if boss.MasteryUnlockThreshold != 0.60 {
		t.Errorf("MasteryUnlockThreshold = %.2f, want 0.60", boss.MasteryUnlockThreshold)
	}
}

func TestAllBossGeometriesAreUnique(t *testing.T) {
	seen := map[string]string{}
	for _, def := range boss.Catalog {
		if prev, ok := seen[def.Visual.Geometry]; ok {
			t.Errorf("boss %s reuses geometry %q already used by %s", def.ID, def.Visual.Geometry, prev)
		}
		seen[def.Visual.Geometry] = def.ID
	}
}
