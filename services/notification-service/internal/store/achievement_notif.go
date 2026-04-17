package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// achievementNotifColumns enumerates the per-event dedup columns on
// gaming_profiles. Adding a new event type requires (1) a column here,
// (2) an entry in the Migrate DDL, and (3) a constant below.
const (
	colLastLevelUpNotifAt       = "last_level_up_notif_at"
	colLastQuestCompleteNotifAt = "last_quest_complete_notif_at"
)

// DedupTTLLevelUp is the guard window for level-up pushes.
// A user levelling up twice within this window receives a single push.
const DedupTTLLevelUp = 5 * time.Minute

// DedupTTLQuestComplete is the guard window for quest-completion pushes.
// Prevents duplicate pushes when gaming-service retries the trigger call.
const DedupTTLQuestComplete = 1 * time.Hour

// MarkLevelUpNotified atomically stamps gaming_profiles.last_level_up_notif_at
// to NOW() iff the user has not been notified within dedupWindow. Returns
// true when the stamp was new (caller should proceed with FCM fan-out) or
// false when the call was suppressed as a duplicate.
//
// The UPDATE is a single round-trip with a WHERE-guard on the existing
// timestamp, so two concurrent retries cannot both stamp and both fan out.
func (s *Store) MarkLevelUpNotified(ctx context.Context, userID string, dedupWindow time.Duration) (bool, error) {
	return s.markEventNotified(ctx, colLastLevelUpNotifAt, userID, dedupWindow)
}

// MarkQuestCompleteNotified atomically stamps
// gaming_profiles.last_quest_complete_notif_at to NOW() iff the user has
// not been notified within dedupWindow. Mirrors MarkLevelUpNotified.
func (s *Store) MarkQuestCompleteNotified(ctx context.Context, userID string, dedupWindow time.Duration) (bool, error) {
	return s.markEventNotified(ctx, colLastQuestCompleteNotifAt, userID, dedupWindow)
}

// markEventNotified is the shared dedup stamp helper. column must be a
// trusted identifier — never a value from untrusted input.
func (s *Store) markEventNotified(ctx context.Context, column, userID string, dedupWindow time.Duration) (bool, error) {
	// column is hard-coded above — this fmt.Sprintf carries no injection risk.
	q := fmt.Sprintf(`
		UPDATE gaming_profiles
		SET %s = NOW()
		WHERE user_id = $1
		  AND (%s IS NULL
		       OR %s < NOW() - make_interval(secs => $2))
		RETURNING 1`, column, column, column)

	var one int
	err := s.db.QueryRow(ctx, q, userID, dedupWindow.Seconds()).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		// No row updated: either the user has no profile row, or the stamp
		// is still inside the dedup window. Either way, skip the push.
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("store: mark %s: %w", column, err)
	}
	return true, nil
}
