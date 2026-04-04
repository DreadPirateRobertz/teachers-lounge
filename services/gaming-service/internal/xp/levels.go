package xp

// levelThresholds maps level → minimum cumulative XP required.
// Level 1 is the starting level; no XP is required.
var levelThresholds = []int64{
	0,      // level 1  (index 0)
	500,    // level 2
	1200,   // level 3
	2200,   // level 4
	3500,   // level 5
	5000,   // level 6
	7000,   // level 7
	9500,   // level 8
	12500,  // level 9
	16000,  // level 10
	20000,  // level 11
	25000,  // level 12
	31000,  // level 13
	38000,  // level 14
	46000,  // level 15
	55500,  // level 16
	66500,  // level 17
	79000,  // level 18
	93500,  // level 19
	110000, // level 20
}

// MaxLevel is the highest achievable level.
const MaxLevel = 20

// LevelFor returns the level a player is at given their cumulative XP.
// Level numbers are 1-based.
func LevelFor(totalXP int64) int {
	level := 1
	for i, threshold := range levelThresholds {
		if totalXP >= threshold {
			level = i + 1
		} else {
			break
		}
	}
	return level
}

// ThresholdFor returns the minimum cumulative XP required to reach the given level.
// Returns 0 for level 1 or any level below 1. Returns the max threshold for levels
// above MaxLevel.
func ThresholdFor(level int) int64 {
	if level <= 1 {
		return 0
	}
	idx := level - 1
	if idx >= len(levelThresholds) {
		return levelThresholds[len(levelThresholds)-1]
	}
	return levelThresholds[idx]
}

// Apply adds amount XP to currentXP and returns the new total, the new level,
// and whether a level-up occurred.
func Apply(currentXP int64, currentLevel int, amount int64) (newXP int64, newLevel int, leveledUp bool) {
	newXP = currentXP + amount
	if newXP < 0 {
		newXP = 0
	}
	newLevel = LevelFor(newXP)
	leveledUp = newLevel > currentLevel
	return newXP, newLevel, leveledUp
}
