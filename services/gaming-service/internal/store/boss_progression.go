package store

import (
	"context"
	"fmt"
)

// GetDefeatedBossIDs returns the unique set of boss IDs that the given user has
// beaten at least once (result = 'victory' in boss_history).
//
// The returned slice contains canonical boss IDs (e.g. "the_atom", "the_bonder")
// in the order they first appear in the result set — callers should not depend on
// ordering; use the boss catalog tier field for ordering.
func (s *Store) GetDefeatedBossIDs(ctx context.Context, userID string) ([]string, error) {
	const q = `
		SELECT DISTINCT boss_name
		FROM boss_history
		WHERE user_id = $1 AND result = 'victory'`

	rows, err := s.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("get defeated bosses %s: %w", userID, err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan defeated boss: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows defeated bosses: %w", err)
	}
	return ids, nil
}
