package store

import (
	"context"
	"fmt"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/model"
)

// streakRiskWindowQuery selects users with a live streak whose last study
// session landed in the reminder window (last_study_date between NOW-maxAge
// and NOW-minAge) and who do not currently hold an active streak freeze.
// Users who already received a reminder within the window (last_streak_reminder_at
// within the past minAgeHours) are excluded to prevent duplicate notifications
// when the cron fires multiple times per day.
//
// The 20h/24h window intentionally excludes users who already lost their
// streak (last_study_date older than 24h): the reminder is pre-lapse only.
const streakRiskWindowQuery = `
	SELECT user_id, current_streak
	FROM gaming_profiles
	WHERE current_streak > 0
	  AND last_study_date IS NOT NULL
	  AND last_study_date <= NOW() - make_interval(hours => $1)
	  AND last_study_date >  NOW() - make_interval(hours => $2)
	  AND (streak_frozen_until IS NULL OR streak_frozen_until <= NOW())
	  AND (last_streak_reminder_at IS NULL
	       OR last_streak_reminder_at < NOW() - make_interval(hours => $1))`

// GetUsersAtRiskOfStreakLoss returns users whose active streaks are in the
// reminder window (≥minAgeHours, <maxAgeHours since last_study_date),
// whose freeze is not currently active, and who have not already been sent
// a reminder within the current window (guarded by last_streak_reminder_at).
//
// The query reads gaming-service's gaming_profiles table, which shares the
// same Postgres cluster as notification-service. Callers typically pass
// minAgeHours=20, maxAgeHours=24 to target the four-hour pre-lapse window.
func (s *Store) GetUsersAtRiskOfStreakLoss(ctx context.Context, minAgeHours, maxAgeHours int) ([]model.UserAtRisk, error) {
	rows, err := s.db.Query(ctx, streakRiskWindowQuery, minAgeHours, maxAgeHours)
	if err != nil {
		return nil, fmt.Errorf("store: at-risk query: %w", err)
	}
	defer rows.Close()

	out := []model.UserAtRisk{}
	for rows.Next() {
		var u model.UserAtRisk
		if err := rows.Scan(&u.UserID, &u.CurrentStreak); err != nil {
			return nil, fmt.Errorf("store: scan at-risk row: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UpdateLastStreakReminderAt stamps gaming_profiles.last_streak_reminder_at
// to NOW() for the given user. Called after successfully fanning out push
// reminders so subsequent cron runs within the same window are skipped.
// A missing row is silently ignored — the cron should not fail if the user's
// profile disappeared between the SELECT and the UPDATE.
func (s *Store) UpdateLastStreakReminderAt(ctx context.Context, userID string) error {
	const q = `
		UPDATE gaming_profiles
		SET last_streak_reminder_at = NOW()
		WHERE user_id = $1`
	if _, err := s.db.Exec(ctx, q, userID); err != nil {
		return fmt.Errorf("store: update last_streak_reminder_at: %w", err)
	}
	return nil
}
