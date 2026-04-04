package battle

import (
	"testing"

	"github.com/teacherslounge/gaming-service/internal/model"
)

func TestCalculateDamage_CorrectAnswer(t *testing.T) {
	tests := []struct {
		name        string
		baseDmg     int
		bossDef     int
		powers      []model.ActivePowerUp
		wantDmg     int
	}{
		{
			name:    "basic damage minus defense",
			baseDmg: 20, bossDef: 5,
			wantDmg: 15,
		},
		{
			name:    "minimum 1 damage when defense exceeds base",
			baseDmg: 3, bossDef: 10,
			wantDmg: 1,
		},
		{
			name:    "zero base damage returns 0",
			baseDmg: 0, bossDef: 5,
			wantDmg: 0,
		},
		{
			name:    "double damage power-up",
			baseDmg: 20, bossDef: 5,
			powers:  []model.ActivePowerUp{{Type: model.PowerUpDoubleDamage, TurnsLeft: 2}},
			wantDmg: 30, // (20-5)*2
		},
		{
			name:    "critical power-up",
			baseDmg: 20, bossDef: 5,
			powers:  []model.ActivePowerUp{{Type: model.PowerUpCritical, TurnsLeft: 1}},
			wantDmg: 45, // (20-5)*3
		},
		{
			name:    "expired power-up ignored",
			baseDmg: 20, bossDef: 5,
			powers:  []model.ActivePowerUp{{Type: model.PowerUpDoubleDamage, TurnsLeft: 0}},
			wantDmg: 15,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDamage(tt.baseDmg, tt.bossDef, tt.powers)
			if got != tt.wantDmg {
				t.Errorf("CalculateDamage(%d, %d, ...) = %d, want %d", tt.baseDmg, tt.bossDef, got, tt.wantDmg)
			}
		})
	}
}

func TestCalculateBossAttack(t *testing.T) {
	tests := []struct {
		name      string
		bossAtk   int
		powers    []model.ActivePowerUp
		wantDmg   int
	}{
		{
			name: "no shield", bossAtk: 15,
			wantDmg: 15,
		},
		{
			name: "shield halves damage", bossAtk: 15,
			powers:  []model.ActivePowerUp{{Type: model.PowerUpShield, TurnsLeft: 2}},
			wantDmg: 7,
		},
		{
			name: "expired shield ignored", bossAtk: 15,
			powers:  []model.ActivePowerUp{{Type: model.PowerUpShield, TurnsLeft: 0}},
			wantDmg: 15,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateBossAttack(tt.bossAtk, tt.powers)
			if got != tt.wantDmg {
				t.Errorf("CalculateBossAttack(%d, ...) = %d, want %d", tt.bossAtk, got, tt.wantDmg)
			}
		})
	}
}

func TestTickPowerUps(t *testing.T) {
	powers := []model.ActivePowerUp{
		{Type: model.PowerUpDoubleDamage, TurnsLeft: 2},
		{Type: model.PowerUpShield, TurnsLeft: 1}, // should expire
	}
	result := TickPowerUps(powers)
	if len(result) != 1 {
		t.Fatalf("expected 1 remaining power-up, got %d", len(result))
	}
	if result[0].Type != model.PowerUpDoubleDamage || result[0].TurnsLeft != 1 {
		t.Errorf("unexpected power-up state: %+v", result[0])
	}
}

func TestApplyHeal(t *testing.T) {
	// Under max
	got := ApplyHeal(50, 100)
	if got != 80 { // 50 + 30
		t.Errorf("ApplyHeal(50, 100) = %d, want 80", got)
	}
	// Capped at max
	got = ApplyHeal(90, 100)
	if got != 100 {
		t.Errorf("ApplyHeal(90, 100) = %d, want 100", got)
	}
}

func TestBossCatalog_HasEntries(t *testing.T) {
	if len(BossCatalog) == 0 {
		t.Fatal("BossCatalog is empty")
	}
	for id, boss := range BossCatalog {
		if boss.MaxHP <= 0 {
			t.Errorf("boss %s has non-positive MaxHP: %d", id, boss.MaxHP)
		}
		if boss.XPReward <= 0 {
			t.Errorf("boss %s has non-positive XPReward: %d", id, boss.XPReward)
		}
	}
}
