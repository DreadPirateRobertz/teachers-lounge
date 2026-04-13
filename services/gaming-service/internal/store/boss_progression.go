package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
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

// GetChapterMastery returns the user's average mastery_score across every
// concept whose ltree path matches any of the supplied lquery patterns.
//
// The score is in [0.0, 1.0]. When the user has no recorded mastery for any
// matching concept (new user, empty chapter, or all-lqueries match zero rows)
// the result is 0.0, not an error.
//
// Example: paths = []string{"chemistry.bonding.*"} returns the mean mastery
// across every concept under the bonding chapter for that user.
func (s *Store) GetChapterMastery(ctx context.Context, userID string, paths []string) (float64, error) {
	if len(paths) == 0 {
		return 0.0, nil
	}

	const q = `
		SELECT COALESCE(AVG(scm.mastery_score), 0.0)
		FROM student_concept_mastery scm
		JOIN concepts c ON c.id = scm.concept_id
		WHERE scm.user_id = $1
		  AND c.path ? $2::lquery[]`

	var mastery float64
	err := s.db.QueryRow(ctx, q, userID, paths).Scan(&mastery)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0.0, nil
		}
		return 0.0, fmt.Errorf("get chapter mastery %s: %w", userID, err)
	}
	return mastery, nil
}
