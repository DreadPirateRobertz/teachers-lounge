package rival_test

import (
	"strings"
	"testing"

	"github.com/teacherslounge/gaming-service/internal/rival"
)

func TestIsRival_MatchesPrefix(t *testing.T) {
	ids := []string{
		"rival:molemaster",
		"rival:bondbreaker",
		"rival:novastar",
		"rival:",
	}
	for _, id := range ids {
		if !rival.IsRival(id) {
			t.Errorf("IsRival(%q) = false, want true", id)
		}
	}
}

func TestIsRival_NoMatchRegularUser(t *testing.T) {
	ids := []string{
		"user-123",
		"alice",
		"",
		"arival:foo",
		"RIVAL:upper",
	}
	for _, id := range ids {
		if rival.IsRival(id) {
			t.Errorf("IsRival(%q) = true, want false", id)
		}
	}
}

func TestRoster_HasEntries(t *testing.T) {
	if len(rival.Roster) == 0 {
		t.Fatal("Roster is empty; at least one rival is required")
	}
}

func TestRoster_AllValidFields(t *testing.T) {
	for _, r := range rival.Roster {
		if !strings.HasPrefix(r.ID, "rival:") {
			t.Errorf("rival %q: ID must start with 'rival:'", r.ID)
		}
		if r.DisplayName == "" {
			t.Errorf("rival %q: DisplayName must not be empty", r.ID)
		}
		if r.BaseXP <= 0 {
			t.Errorf("rival %q: BaseXP must be positive, got %d", r.ID, r.BaseXP)
		}
		if r.DailyGainMin <= 0 {
			t.Errorf("rival %q: DailyGainMin must be positive, got %d", r.ID, r.DailyGainMin)
		}
		if r.DailyGainMax < r.DailyGainMin {
			t.Errorf("rival %q: DailyGainMax (%d) must be >= DailyGainMin (%d)",
				r.ID, r.DailyGainMax, r.DailyGainMin)
		}
	}
}

func TestRoster_UniqueIDs(t *testing.T) {
	seen := make(map[string]bool, len(rival.Roster))
	for _, r := range rival.Roster {
		if seen[r.ID] {
			t.Errorf("duplicate rival ID %q in Roster", r.ID)
		}
		seen[r.ID] = true
	}
}
