package event_test

import (
	"testing"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/event"
)

func TestContent_StreakAtRisk(t *testing.T) {
	p, e := event.Content(event.StreakAtRisk, nil)
	if p.Title == "" {
		t.Error("push title should not be empty for streak_at_risk")
	}
	if p.Body == "" {
		t.Error("push body should not be empty for streak_at_risk")
	}
	if e.Subject == "" {
		t.Error("email subject should not be empty for streak_at_risk")
	}
}

func TestContent_RivalPassed(t *testing.T) {
	p, e := event.Content(event.RivalPassed, map[string]any{"rival_name": "MoleMaster"})
	if p.Title == "" {
		t.Error("push title should not be empty for rival_passed")
	}
	if e.Subject == "" {
		t.Error("email subject should not be empty for rival_passed")
	}
}

func TestContent_BossUnlock(t *testing.T) {
	p, e := event.Content(event.BossUnlock, map[string]any{"boss_name": "Algebra Dragon"})
	if p.Title == "" {
		t.Error("push title empty for boss_unlock")
	}
	if e.Subject == "" {
		t.Error("email subject empty for boss_unlock")
	}
}

func TestContent_QuizCountdown(t *testing.T) {
	p, _ := event.Content(event.QuizCountdown, nil)
	if p.Title == "" {
		t.Error("push title empty for quiz_countdown")
	}
}

func TestContent_AchievementNearMiss(t *testing.T) {
	p, _ := event.Content(event.AchievementNearMiss, nil)
	if p.Title == "" {
		t.Error("push title empty for achievement_near_miss")
	}
}

func TestContent_AllTypesReturnNonEmpty(t *testing.T) {
	types := []event.Type{
		event.StreakAtRisk,
		event.RivalPassed,
		event.BossUnlock,
		event.QuizCountdown,
		event.AchievementNearMiss,
	}
	for _, typ := range types {
		p, e := event.Content(typ, nil)
		if p.Title == "" || p.Body == "" {
			t.Errorf("event %q: push content incomplete — title=%q body=%q", typ, p.Title, p.Body)
		}
		if e.Subject == "" {
			t.Errorf("event %q: email subject empty", typ)
		}
	}
}

func TestContent_UnknownType_DoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Content panicked on unknown type: %v", r)
		}
	}()
	p, _ := event.Content("totally_unknown_type", nil)
	// Should return some fallback, not panic.
	_ = p
}
