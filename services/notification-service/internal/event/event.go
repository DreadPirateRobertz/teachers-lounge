// Package event defines the notification event types and maps each type to
// the push and email content delivered to the student.
package event

import "fmt"

// Type identifies a game event that triggers a notification.
type Type string

const (
	// StreakAtRisk fires at 8 pm local time when the student has not studied
	// that day and risks losing their streak.
	StreakAtRisk Type = "streak_at_risk"

	// RivalPassed fires when a simulated rival (or real peer) overtakes the
	// student on the leaderboard.
	RivalPassed Type = "rival_passed"

	// BossUnlock fires when the student's level progression unlocks a new boss.
	BossUnlock Type = "boss_unlock"

	// QuizCountdown fires when an in-progress quiz session is approaching its
	// expiry and the student has not finished.
	QuizCountdown Type = "quiz_countdown"

	// AchievementNearMiss fires when the student is within 10% of earning an
	// achievement, prompting them to close the gap.
	AchievementNearMiss Type = "achievement_near_miss"
)

// PushContent is the title and body for a mobile/web push notification.
type PushContent struct {
	Title string
	Body  string
}

// EmailContent is the subject line and HTML body for a transactional email.
type EmailContent struct {
	Subject  string
	HTMLBody string
}

// Content returns the push and email content for the given event type.
// The payload map provides optional dynamic values (e.g. rival_name, boss_name).
// Unknown event types return safe fallback content rather than panicking.
func Content(t Type, payload map[string]any) (PushContent, EmailContent) {
	str := func(key, fallback string) string {
		if payload == nil {
			return fallback
		}
		if v, ok := payload[key].(string); ok && v != "" {
			return v
		}
		return fallback
	}

	switch t {
	case StreakAtRisk:
		return PushContent{
				Title: "Your streak is at risk! 🔥",
				Body:  "Study today to keep your streak alive. Don't let it slip away!",
			}, EmailContent{
				Subject:  "Don't break your streak — study today!",
				HTMLBody: streakAtRiskEmail(),
			}

	case RivalPassed:
		rival := str("rival_name", "a rival")
		return PushContent{
				Title: "You've been passed! ⚔️",
				Body:  fmt.Sprintf("%s just overtook you on the leaderboard. Fight back!", rival),
			}, EmailContent{
				Subject:  fmt.Sprintf("%s just passed you — take back your rank!", rival),
				HTMLBody: rivalPassedEmail(rival),
			}

	case BossUnlock:
		boss := str("boss_name", "a new boss")
		return PushContent{
				Title: "New boss unlocked! 👾",
				Body:  fmt.Sprintf("%s is waiting. Do you have what it takes?", boss),
			}, EmailContent{
				Subject:  fmt.Sprintf("Challenge unlocked: %s awaits!", boss),
				HTMLBody: bossUnlockEmail(boss),
			}

	case QuizCountdown:
		return PushContent{
				Title: "Your quiz is expiring soon! ⏱",
				Body:  "You have an unfinished quiz session. Complete it before time runs out.",
			}, EmailContent{
				Subject:  "Quiz expiring — finish it now!",
				HTMLBody: quizCountdownEmail(),
			}

	case AchievementNearMiss:
		achievement := str("achievement_name", "an achievement")
		return PushContent{
				Title: "So close! 🏆",
				Body:  fmt.Sprintf("You're almost there — just a bit more to unlock %s.", achievement),
			}, EmailContent{
				Subject:  "You're this close to unlocking an achievement!",
				HTMLBody: achievementNearMissEmail(achievement),
			}

	default:
		return PushContent{
				Title: "New activity on TeachersLounge",
				Body:  "Check the app to see what's new.",
			}, EmailContent{
				Subject:  "Activity update — TeachersLounge",
				HTMLBody: "<p>You have a new notification. Open the app to see more.</p>",
			}
	}
}

// ── Email HTML templates ──────────────────────────────────────────────────────
// Minimal inline HTML so the service has no file-system dependency.

func streakAtRiskEmail() string {
	return `<h2>Your streak is at risk 🔥</h2>
<p>You haven't studied today. Log in and answer a few questions to keep your streak alive!</p>
<p><a href="https://teacherslounge.app">Study now →</a></p>`
}

func rivalPassedEmail(rival string) string {
	return fmt.Sprintf(`<h2>%s just passed you ⚔️</h2>
<p>Don't let them stay ahead. Answer some questions and climb back up the leaderboard.</p>
<p><a href="https://teacherslounge.app/leaderboard">See the leaderboard →</a></p>`, rival)
}

func bossUnlockEmail(boss string) string {
	return fmt.Sprintf(`<h2>New challenge unlocked: %s 👾</h2>
<p>You've levelled up enough to face a new boss. Are you ready?</p>
<p><a href="https://teacherslounge.app/boss">Start the battle →</a></p>`, boss)
}

func quizCountdownEmail() string {
	return `<h2>Your quiz session is expiring ⏱</h2>
<p>You have an unfinished quiz. Complete it now before the session expires.</p>
<p><a href="https://teacherslounge.app">Finish your quiz →</a></p>`
}

func achievementNearMissEmail(achievement string) string {
	return fmt.Sprintf(`<h2>Almost there! 🏆</h2>
<p>You're within reach of unlocking <strong>%s</strong>. A little more effort and it's yours.</p>
<p><a href="https://teacherslounge.app">Keep going →</a></p>`, achievement)
}
