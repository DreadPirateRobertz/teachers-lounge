package store

import (
	"context"
	"fmt"

	"github.com/teacherslounge/gaming-service/internal/model"
)

// GrantAchievement inserts a new achievement for the user. The insert is
// ignored if the achievement already exists (UNIQUE on user_id, achievement_type).
// Returns the achievement record and whether it was newly earned (true) or a
// duplicate (false).
func (s *Store) GrantAchievement(ctx context.Context, userID, achievementType, badgeName string) (*model.Achievement, bool, error) {
	const q = `
		INSERT INTO achievements (user_id, achievement_type, badge_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, achievement_type) DO NOTHING
		RETURNING id, user_id, achievement_type, badge_name, earned_at`

	a := &model.Achievement{}
	err := s.db.QueryRow(ctx, q, userID, achievementType, badgeName).Scan(
		&a.ID, &a.UserID, &a.AchievementType, &a.BadgeName, &a.EarnedAt,
	)
	if err != nil {
		// ON CONFLICT DO NOTHING returns no rows on duplicate — fetch the existing row.
		if err.Error() == "no rows in result set" {
			return s.getAchievement(ctx, userID, achievementType)
		}
		return nil, false, fmt.Errorf("grant achievement %s for %s: %w", achievementType, userID, err)
	}
	return a, true, nil
}

// getAchievement fetches a single achievement by user_id + achievement_type.
func (s *Store) getAchievement(ctx context.Context, userID, achievementType string) (*model.Achievement, bool, error) {
	const q = `
		SELECT id, user_id, achievement_type, badge_name, earned_at
		FROM achievements
		WHERE user_id = $1 AND achievement_type = $2`

	a := &model.Achievement{}
	err := s.db.QueryRow(ctx, q, userID, achievementType).Scan(
		&a.ID, &a.UserID, &a.AchievementType, &a.BadgeName, &a.EarnedAt,
	)
	if err != nil {
		return nil, false, fmt.Errorf("get achievement %s for %s: %w", achievementType, userID, err)
	}
	return a, false, nil
}

// GetAchievements returns all achievements for a user, ordered by earned_at DESC.
func (s *Store) GetAchievements(ctx context.Context, userID string) ([]model.Achievement, error) {
	const q = `
		SELECT id, user_id, achievement_type, badge_name, earned_at
		FROM achievements
		WHERE user_id = $1
		ORDER BY earned_at DESC`

	rows, err := s.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("get achievements for %s: %w", userID, err)
	}
	defer rows.Close()

	var achievements []model.Achievement
	for rows.Next() {
		var a model.Achievement
		if err := rows.Scan(&a.ID, &a.UserID, &a.AchievementType, &a.BadgeName, &a.EarnedAt); err != nil {
			return nil, fmt.Errorf("scan achievement: %w", err)
		}
		achievements = append(achievements, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error for achievements %s: %w", userID, err)
	}
	if achievements == nil {
		achievements = []model.Achievement{}
	}
	return achievements, nil
}

// AddCosmeticItem merges a cosmetic key/value pair into the user's
// gaming_profiles.cosmetics JSONB column. If the user has no profile row yet,
// one is upserted with the cosmetic set.
func (s *Store) AddCosmeticItem(ctx context.Context, userID, key, value string) error {
	const q = `
		INSERT INTO gaming_profiles (user_id, cosmetics)
		VALUES ($1, jsonb_build_object($2::text, $3::text))
		ON CONFLICT (user_id) DO UPDATE
		SET cosmetics = gaming_profiles.cosmetics || jsonb_build_object($2::text, $3::text)`

	if _, err := s.db.Exec(ctx, q, userID, key, value); err != nil {
		return fmt.Errorf("add cosmetic %s=%s for %s: %w", key, value, userID, err)
	}
	return nil
}
