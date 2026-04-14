package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// StreakFreezeDuration is the window a purchased freeze protects against
// streak reset. Exposed for handler-level presentation (e.g., expires_at
// echo in the response body).
const StreakFreezeDuration = 24 * time.Hour

// ErrAlreadyFrozen is returned by CreateStreakFreeze when the caller
// already has an active freeze (streak_frozen_until > NOW()). Stacking
// freezes is not permitted — the caller should wait until the current
// one expires before purchasing another.
var ErrAlreadyFrozen = errors.New("streak already frozen")

// CreateStreakFreeze atomically deducts gemCost gems from the user's
// gaming profile and sets streak_frozen_until to NOW() + StreakFreezeDuration.
//
// Returns the post-deduction gem balance and the UTC timestamp at which
// the freeze expires.
//
// Error semantics:
//   - ErrNoGems          — the user does not have gemCost gems available.
//   - ErrAlreadyFrozen   — the user has an active freeze that has not expired.
//   - pgx.ErrNoRows      — never surfaces directly; distinguished via the
//     WHERE clause into the two sentinels above.
//
// The UPDATE is a single round-trip. We cannot distinguish "no gems" from
// "already frozen" from a single WHERE predicate, so when the first
// attempt returns zero rows we issue a narrow follow-up SELECT to
// classify which guard rejected the row. This keeps the happy path at
// one query and the failure path bounded at two.
func (s *Store) CreateStreakFreeze(ctx context.Context, userID string, gemCost int) (gemsLeft int, expiresAt time.Time, err error) {
	const q = `
		UPDATE gaming_profiles
		SET gems = gems - $2,
		    streak_frozen_until = NOW() + ($3 * INTERVAL '1 second')
		WHERE user_id = $1
		  AND gems >= $2
		  AND (streak_frozen_until IS NULL OR streak_frozen_until <= NOW())
		RETURNING gems, streak_frozen_until`

	secs := int(StreakFreezeDuration.Seconds())
	err = s.db.QueryRow(ctx, q, userID, gemCost, secs).Scan(&gemsLeft, &expiresAt)
	if err == nil {
		return gemsLeft, expiresAt, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, time.Time{}, fmt.Errorf("create streak freeze %s: %w", userID, err)
	}

	// Zero rows: classify the failure. Either the row doesn't exist,
	// the user can't afford it, or a freeze is still active.
	const classify = `
		SELECT gems, streak_frozen_until
		FROM gaming_profiles
		WHERE user_id = $1`
	var currentGems int
	var existing *time.Time
	if err := s.db.QueryRow(ctx, classify, userID).Scan(&currentGems, &existing); err != nil {
		return 0, time.Time{}, fmt.Errorf("classify streak freeze failure %s: %w", userID, err)
	}
	if existing != nil && existing.After(time.Now()) {
		return 0, time.Time{}, ErrAlreadyFrozen
	}
	if currentGems < gemCost {
		return 0, time.Time{}, ErrNoGems
	}
	// Neither guard tripped — shouldn't happen; surface as generic error.
	return 0, time.Time{}, fmt.Errorf("create streak freeze %s: update returned no rows but guards passed", userID)
}

// IsStreakFrozen reports whether the user currently has an active streak
// freeze (streak_frozen_until > NOW()). Returns false when no row exists
// for the user or when the column is NULL / already expired.
func (s *Store) IsStreakFrozen(ctx context.Context, userID string) (bool, error) {
	const q = `
		SELECT COALESCE(streak_frozen_until > NOW(), FALSE)
		FROM gaming_profiles
		WHERE user_id = $1`
	var frozen bool
	err := s.db.QueryRow(ctx, q, userID).Scan(&frozen)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("is streak frozen %s: %w", userID, err)
	}
	return frozen, nil
}
