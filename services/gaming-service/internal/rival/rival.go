// Package rival defines the simulated competitor profiles that populate the
// leaderboard around real users to provide always-on competition.
package rival

import "strings"

// Rival is a simulated competitor that lives on the leaderboard.
// Its ID always starts with the "rival:" prefix so the store layer can
// distinguish it from real user IDs.
type Rival struct {
	// ID is the leaderboard member key, e.g. "rival:molemaster".
	ID string
	// DisplayName is the human-readable name shown in the UI.
	DisplayName string
	// BaseXP is the score injected on first seed (ZAddNX — never overwritten).
	BaseXP int
	// DailyGainMin is the minimum XP increment applied on each tick.
	DailyGainMin int
	// DailyGainMax is the maximum XP increment applied on each tick.
	DailyGainMax int
}

// Roster is the fixed set of simulated rivals seeded into every leaderboard.
// Rivals are spread across a range of XP values so they bracket typical users
// at multiple skill levels and always provide a meaningful target to chase or
// a score to defend against.
var Roster = []Rival{
	{
		ID:           "rival:molemaster",
		DisplayName:  "MoleMaster",
		BaseXP:       4800,
		DailyGainMin: 30,
		DailyGainMax: 80,
	},
	{
		ID:           "rival:bondbreaker",
		DisplayName:  "BondBreaker",
		BaseXP:       2050,
		DailyGainMin: 20,
		DailyGainMax: 60,
	},
	{
		ID:           "rival:novastar",
		DisplayName:  "NovaStar",
		BaseXP:       1900,
		DailyGainMin: 15,
		DailyGainMax: 50,
	},
	{
		ID:           "rival:reactking",
		DisplayName:  "ReactKing",
		BaseXP:       1750,
		DailyGainMin: 10,
		DailyGainMax: 45,
	},
}

// IsRival reports whether the given user ID belongs to a simulated rival.
// All rival IDs use the "rival:" prefix, which is never assigned to real users.
func IsRival(userID string) bool {
	return strings.HasPrefix(userID, "rival:")
}
