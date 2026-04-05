package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/teacherslounge/gaming-service/internal/model"
)

const (
	battleKeyPrefix = "battle:session:"
	battleTTL       = 30 * time.Minute
)

func battleKey(sessionID string) string {
	return battleKeyPrefix + sessionID
}

// SaveBattleSession stores or updates a battle session in Redis.
func (s *Store) SaveBattleSession(ctx context.Context, session *model.BattleSession) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal battle session: %w", err)
	}
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		ttl = battleTTL
	}
	if err := s.rdb.Set(ctx, battleKey(session.SessionID), data, ttl).Err(); err != nil {
		return fmt.Errorf("save battle session %s: %w", session.SessionID, err)
	}
	return nil
}

// GetBattleSession retrieves a battle session from Redis. Returns nil, nil if not found.
func (s *Store) GetBattleSession(ctx context.Context, sessionID string) (*model.BattleSession, error) {
	data, err := s.rdb.Get(ctx, battleKey(sessionID)).Bytes()
	if err != nil {
		if err.Error() == "redis: nil" {
			return nil, nil
		}
		return nil, fmt.Errorf("get battle session %s: %w", sessionID, err)
	}
	var session model.BattleSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal battle session %s: %w", sessionID, err)
	}
	return &session, nil
}

// DeleteBattleSession removes a battle session from Redis.
func (s *Store) DeleteBattleSession(ctx context.Context, sessionID string) error {
	if err := s.rdb.Del(ctx, battleKey(sessionID)).Err(); err != nil {
		return fmt.Errorf("delete battle session %s: %w", sessionID, err)
	}
	return nil
}

// RecordBattleResult persists the outcome of a boss battle to Postgres
// and updates the gaming_profiles counters.
func (s *Store) RecordBattleResult(ctx context.Context, result *model.BattleResult) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Map to existing boss_history table schema.
	bossResult := "defeat"
	if result.Won {
		bossResult = "victory"
	}
	const insertResult = `
		INSERT INTO boss_history (user_id, boss_name, topic, rounds, result, xp_earned, fought_at)
		VALUES ($1, $2, $3, $4, $5::boss_result, $6, $7)`

	_, err = tx.Exec(ctx, insertResult,
		result.UserID, string(result.BossID), string(result.BossID),
		result.TurnsUsed, bossResult, result.XPEarned, result.FinishedAt,
	)
	if err != nil {
		return fmt.Errorf("insert battle result: %w", err)
	}

	if result.Won {
		const updateProfile = `
			UPDATE gaming_profiles
			SET bosses_defeated = bosses_defeated + 1,
			    gems = gems + $2
			WHERE user_id = $1`
		_, err = tx.Exec(ctx, updateProfile, result.UserID, result.GemsEarned)
		if err != nil {
			return fmt.Errorf("update profile bosses_defeated: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit battle result: %w", err)
	}
	return nil
}

// DeductGems removes gems from a user's profile. Returns the new gem count.
// Returns an error if the user doesn't have enough gems.
func (s *Store) DeductGems(ctx context.Context, userID string, amount int) (int, error) {
	const q = `
		UPDATE gaming_profiles
		SET gems = gems - $2
		WHERE user_id = $1 AND gems >= $2
		RETURNING gems`

	var remaining int
	err := s.db.QueryRow(ctx, q, userID, amount).Scan(&remaining)
	if err != nil {
		return 0, fmt.Errorf("deduct gems %s (amount=%d): %w", userID, amount, err)
	}
	return remaining, nil
}
