package quest

// Definition is a daily quest template.
type Definition struct {
	ID          string
	Title       string
	Description string
	Target      int
	XPReward    int
	GemsReward  int
}

// Daily is the fixed set of 3 daily quests.
var Daily = []Definition{
	{
		ID:          "questions_answered",
		Title:       "Question Seeker",
		Description: "Answer 5 questions today",
		Target:      5,
		XPReward:    25,
		GemsReward:  5,
	},
	{
		ID:          "keep_streak_alive",
		Title:       "Streak Keeper",
		Description: "Keep your learning streak alive",
		Target:      1,
		XPReward:    35,
		GemsReward:  10,
	},
	{
		ID:          "master_new_concept",
		Title:       "Concept Pioneer",
		Description: "Master a new concept",
		Target:      1,
		XPReward:    75,
		GemsReward:  20,
	},
}

// ByID returns the quest definition for the given ID, or nil if not found.
func ByID(id string) *Definition {
	for i := range Daily {
		if Daily[i].ID == id {
			return &Daily[i]
		}
	}
	return nil
}

// ForAction returns the quest IDs advanced by the given action string.
// Returns nil for unknown actions.
func ForAction(action string) []string {
	switch action {
	case "question_answered":
		return []string{"questions_answered"}
	case "streak_checkin":
		return []string{"keep_streak_alive"}
	case "concept_mastered":
		return []string{"master_new_concept"}
	default:
		return nil
	}
}
