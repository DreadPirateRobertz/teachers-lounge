package boss

// Def defines a boss encounter that guards a topic.
type Def struct {
	ID        string // machine-friendly ID, e.g. "the_atom"
	Name      string // display name, e.g. "THE ATOM"
	Topic     string // maps to question_bank.topic
	Tier      int    // 1–5; drives boss HP scaling and victory XP
	MaxRounds int    // number of questions (rounds) in the fight
	Taunt     string // shown to the student on a wrong answer
	VictoryXP int    // bonus XP awarded on defeating this boss
}

// Catalog is the ordered sequence of boss encounters.
var Catalog = []*Def{
	{
		ID:        "the_atom",
		Name:      "THE ATOM",
		Topic:     "general_chemistry",
		Tier:      1,
		MaxRounds: 5,
		Taunt:     "Your electrons are all over the place! Back to basics.",
		VictoryXP: 150,
	},
	{
		ID:        "bonding_brothers",
		Name:      "BONDING BROTHERS",
		Topic:     "molecular_bonding",
		Tier:      2,
		MaxRounds: 6,
		Taunt:     "Weak bonds break — just like that answer!",
		VictoryXP: 200,
	},
	{
		ID:        "name_lord",
		Name:      "NAME LORD",
		Topic:     "nomenclature",
		Tier:      3,
		MaxRounds: 6,
		Taunt:     "You dare mislabel compounds in MY presence?!",
		VictoryXP: 250,
	},
	{
		ID:        "the_stereochemist",
		Name:      "THE STEREOCHEMIST",
		Topic:     "stereochemistry",
		Tier:      4,
		MaxRounds: 7,
		Taunt:     "Mirror, mirror — and your knowledge is shattered!",
		VictoryXP: 300,
	},
	{
		ID:        "the_reactor",
		Name:      "THE REACTOR",
		Topic:     "organic_reactions",
		Tier:      5,
		MaxRounds: 7,
		Taunt:     "Your reaction mechanism is a catastrophic failure!",
		VictoryXP: 400,
	},
}

// ByID returns the Def for the given boss ID, or nil if not found.
func ByID(id string) *Def {
	for _, d := range Catalog {
		if d.ID == id {
			return d
		}
	}
	return nil
}

// BossHP returns the starting HP for a boss given the player's current level.
// HP scales with both the boss's tier (harder bosses) and the player's level
// (adaptive difficulty) to keep fights challenging across progression.
//
// Formula: 40 + (tier * 20) + (playerLevel * 3)
// Examples at level 5:
//   tier 1 → 40 + 20 + 15 = 75
//   tier 5 → 40 + 100 + 15 = 155
func BossHP(tier, playerLevel int) int {
	return 40 + (tier * 20) + (playerLevel * 3)
}

// DamageToBoss returns the HP damage dealt to the boss for a correct answer.
// Base damage is derived from the question's XP reward, then multiplied by the
// student's current combo multiplier.
func DamageToBoss(xpReward int, multiplier float64) int {
	base := xpReward / 4
	if base < 10 {
		base = 10
	}
	d := int(float64(base) * multiplier)
	if d < 1 {
		d = 1
	}
	return d
}

// DamageToStudent returns the HP damage dealt to the student for a wrong answer.
// Harder questions hit harder (difficulty 1–5 → 5–25 damage).
func DamageToStudent(difficulty int) int {
	d := difficulty * 5
	if d < 5 {
		d = 5
	}
	return d
}

// ComboMultiplier returns the damage multiplier for the given consecutive-correct
// streak. Matches the design spec: 3+ → 1.5x, 5+ → 2x.
func ComboMultiplier(streak int) float64 {
	switch {
	case streak >= 5:
		return 2.0
	case streak >= 3:
		return 1.5
	default:
		return 1.0
	}
}
