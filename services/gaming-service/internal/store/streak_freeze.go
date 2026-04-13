package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	streakFreezeKeyPrefix = "streak:freeze:"
	streakFreezeTTL       = 24 * time.Hour
	// StreakFreezeCost is the gem cost to purchase one streak freeze.
	StreakFreezeCost = 50
)

// ErrInsufficientCoins is returned by CreateStreakFreeze when the user's gem
// balance is too low to cover the StreakFreezeCost.
var ErrInsufficientCoins = errors.New("insufficient coins")

// CreateStreakFreeze atomically deducts StreakFreezeCost gems from the user's
// profile and records an active streak freeze in Redis with a 24-hour TTL.
// Returns the updated gem balance after the deduction.
// Returns ErrInsufficientCoins when the user cannot afford the freeze.
func (s *Store) CreateStreakFreeze(ctx context.Context, userID string) (gemsLeft int, err error) {
	const q = `
		UPDATE gaming_profiles
		SET gems = gems - $2
		WHERE user_id = $1 AND gems >= $2
		RETURNING gems`

	err = s.db.QueryRow(ctx, q, userID, StreakFreezeCost).Scan(&gemsLeft)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrInsufficientCoins
		}
		return 0, fmt.Errorf("create streak freeze %s: %w", userID, err)
	}

	key := streakFreezeKeyPrefix + userID
	if err := s.rdb.Set(ctx, key, "1", streakFreezeTTL).Err(); err != nil {
		return 0, fmt.Errorf("set streak freeze key %s: %w", userID, err)
	}

	return gemsLeft, nil
}

// IsStreakFrozen returns true if the user has an active streak freeze recorded
// in Redis. A freeze expires automatically after 24 hours via the Redis TTL.
func (s *Store) IsStreakFrozen(ctx context.Context, userID string) (bool, error) {
	key := streakFreezeKeyPrefix + userID
	n, err := s.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("check streak freeze %s: %w", userID, err)
	}
	return n > 0, nil
}
