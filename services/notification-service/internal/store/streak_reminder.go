package store

import (
	"context"
	"fmt"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/model"
)

// streakRiskWindowQuery selects users with a live streak whose last study
// session landed in the reminder window (last_study_date between NOW-maxAge
// and NOW-minAge) and who do not currently hold an active streak freeze.
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
	  AND (streak_frozen_until IS NULL OR streak_frozen_until <= NOW())`

// GetUsersAtRiskOfStreakLoss returns users whose active streaks are in the
// reminder window (≥minAgeHours, <maxAgeHours since last_study_date) and
// whose freeze is not currently active.
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
