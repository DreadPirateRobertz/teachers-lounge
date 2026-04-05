package boss

// Def defines a boss encounter that guards a topic.
type Def struct {
	ID          string // machine-friendly ID, e.g. "the_atom"
	Name        string // display name, e.g. "THE ATOM"
	Topic       string // maps to question_bank.topic
	Tier        int    // 1–6; drives boss HP scaling and victory XP
	MaxRounds   int    // number of questions (rounds) in the fight
	Taunt       string // shown to the student on a wrong answer
	VictoryXP   int    // bonus XP awarded on defeating this boss
	Visual      VisualConfig
}

// VisualConfig holds all client-side rendering metadata for a boss.
// The frontend Three.js scene reads this to build procedural geometry
// and drive animations.
type VisualConfig struct {
	// PrimaryColor is the boss's dominant neon hex color.
	PrimaryColor string
	// SecondaryColor is used for secondary geometry and glow effects.
	SecondaryColor string
	// Geometry identifies which procedural Three.js builder to use.
	Geometry string
	// TauntPool is a set of taunts shown during the fight.
	TauntPool []string
	// AttackDescription describes the visual attack animation.
	AttackDescription string
	// IdleDescription describes the visual idle animation.
	IdleDescription string
}

// Catalog is the ordered sequence of boss encounters in chapter order.
// The final boss (tier 6) is unlocked only after all chapter bosses are defeated.
var Catalog = []*Def{
	{
		ID:        "the_atom",
		Name:      "THE ATOM",
		Topic:     "general_chemistry",
		Tier:      1,
		MaxRounds: 5,
		Taunt:     "Your electrons are all over the place! Back to basics.",
		VictoryXP: 150,
		Visual: VisualConfig{
			PrimaryColor:      "#00aaff",
			SecondaryColor:    "#00ff88",
			Geometry:          "atom",
			AttackDescription: "Fires electron beams from orbiting shells",
			IdleDescription:   "Nucleus pulses; electron rings orbit at varying speeds",
			TauntPool: []string{
				"Your electrons are all over the place!",
				"Electron configuration: WRONG.",
				"Did you forget about quantum numbers?",
				"That answer had zero valence electrons of correctness.",
				"Back to the periodic table with you!",
			},
		},
	},
	{
		ID:        "the_bonder",
		Name:      "THE BONDER",
		Topic:     "molecular_bonding",
		Tier:      2,
		MaxRounds: 6,
		Taunt:     "Weak bonds break — just like that answer!",
		VictoryXP: 200,
		Visual: VisualConfig{
			PrimaryColor:      "#00ff88",
			SecondaryColor:    "#ffdc00",
			Geometry:          "bonder",
			AttackDescription: "Dual heads split apart then reform, firing bond energy",
			IdleDescription:   "Two spheres connected by oscillating bond cylinders",
			TauntPool: []string{
				"Weak bonds break — just like that answer!",
				"That's not how covalent bonding works.",
				"Ionic or covalent? You clearly don't know.",
				"Your bond angles are catastrophically wrong.",
				"VSEPR theory weeps at your answer.",
			},
		},
	},
	{
		ID:        "name_lord",
		Name:      "NAME LORD",
		Topic:     "nomenclature",
		Tier:      3,
		MaxRounds: 6,
		Taunt:     "You dare mislabel compounds in MY presence?!",
		VictoryXP: 250,
		Visual: VisualConfig{
			PrimaryColor:      "#ff00aa",
			SecondaryColor:    "#ffdc00",
			Geometry:          "name_lord",
			AttackDescription: "Rapid-fire shape shifts with cascading name labels",
			IdleDescription:   "Icosahedron slowly morphing between molecular shapes",
			TauntPool: []string{
				"You dare mislabel compounds in MY presence?!",
				"That name is so wrong it broke the IUPAC handbook.",
				"Did you just call that a... never mind. WRONG.",
				"Names have POWER. Yours has none.",
				"I have been named. You have named nothing correctly.",
			},
		},
	},
	{
		ID:        "the_stereochemist",
		Name:      "THE STEREOCHEMIST",
		Topic:     "stereochemistry",
		Tier:      4,
		MaxRounds: 7,
		Taunt:     "Mirror, mirror — and your knowledge is shattered!",
		VictoryXP: 300,
		Visual: VisualConfig{
			PrimaryColor:      "#cc44ff",
			SecondaryColor:    "#00aaff",
			Geometry:          "stereochemist",
			AttackDescription: "Mirror image clone advances while chirality flips",
			IdleDescription:   "Paired mirrored tetrahedra orbiting a central axis",
			TauntPool: []string{
				"Mirror, mirror — and your knowledge is shattered!",
				"R or S? You chose... poorly.",
				"Enantiomers are not the same. Neither are your answers.",
				"That stereocenter is crying right now.",
				"Chiral. Unlike your reasoning, which has no handedness.",
			},
		},
	},
	{
		ID:        "the_reactor",
		Name:      "THE REACTOR",
		Topic:     "organic_reactions",
		Tier:      5,
		MaxRounds: 7,
		Taunt:     "Your reaction mechanism is a catastrophic failure!",
		VictoryXP: 400,
		Visual: VisualConfig{
			PrimaryColor:      "#ff6600",
			SecondaryColor:    "#ffdc00",
			Geometry:          "reactor",
			AttackDescription: "Cascading chain reaction: particles burst outward in sequence",
			IdleDescription:   "Churning torus vessel with swirling reaction particles",
			TauntPool: []string{
				"Your reaction mechanism is a catastrophic failure!",
				"SN1? SN2? You clearly don't know the difference.",
				"That's not a nucleophile, that's an embarrassment.",
				"Leaving groups are leaving. Just like your GPA.",
				"The reaction is exothermic. Your knowledge is endothermic.",
			},
		},
	},
	{
		ID:        "final_boss",
		Name:      "FINAL BOSS",
		Topic:     "course_final",
		Tier:      6,
		MaxRounds: 10,
		Taunt:     "All my predecessors warned me. You are nothing.",
		VictoryXP: 1000,
		Visual: VisualConfig{
			PrimaryColor:      "#ff00aa",
			SecondaryColor:    "#00aaff",
			Geometry:          "final_boss",
			AttackDescription: "Multi-phase: cycles through all chapter boss attacks",
			IdleDescription:   "Composite entity: orbiting electrons, bonded arms, shifting shape, mirrored core, reactive aura",
			TauntPool: []string{
				"All my predecessors warned me. You are nothing.",
				"You've come so far only to fail now.",
				"I contain all of chemistry. You know a fraction.",
				"Every mistake you made brought you here.",
				"This is where the learning ends.",
				"I am every concept. I am every exam.",
				"They all fell. You will too.",
				"The course final claims another victim.",
			},
		},
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
//
//	tier 1 → 40 + 20 + 15 = 75
//	tier 6 → 40 + 120 + 15 = 175
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
