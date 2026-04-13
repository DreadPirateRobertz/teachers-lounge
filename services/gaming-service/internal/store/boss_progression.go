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

// GetChapterMasteryBatch returns per-boss average mastery_score in a single
// round-trip. pathsByBossID maps each boss_id to its ChapterConceptPaths.
//
// The returned map contains one entry per input boss_id — bosses with no
// matching concepts (fresh user, unmapped chapter) appear as 0.0 rather than
// being omitted. A nil or empty input returns an empty map and does not
// query the DB.
//
// Replaces the N+1 GetChapterMastery loop in the progression handler so a
// full boss-trail render costs one DB round-trip regardless of catalog size
// (tl-7wv follow-on).
func (s *Store) GetChapterMasteryBatch(ctx context.Context, userID string, pathsByBossID map[string][]string) (map[string]float64, error) {
	out := make(map[string]float64, len(pathsByBossID))
	if len(pathsByBossID) == 0 {
		return out, nil
	}

	// Flatten into parallel arrays: one row per (boss_id, lquery). Bosses
	// with multiple paths repeat their boss_id across multiple rows; the
	// GROUP BY reassembles them into a single AVG per boss.
	var bossIDs, paths []string
	for bossID, bossPaths := range pathsByBossID {
		out[bossID] = 0.0 // seed so zero-result bosses still appear in the map
		for _, p := range bossPaths {
			bossIDs = append(bossIDs, bossID)
			paths = append(paths, p)
		}
	}

	// A boss with zero paths never contributes to the SELECT — the seeded
	// 0.0 above keeps the returned map complete for the caller.
	if len(bossIDs) == 0 {
		return out, nil
	}

	const q = `
		SELECT bp.boss_id, COALESCE(AVG(scm.mastery_score), 0.0) AS mastery
		FROM unnest($2::text[], $3::text[]) AS bp(boss_id, path)
		LEFT JOIN concepts c ON c.path ~ bp.path::lquery
		LEFT JOIN student_concept_mastery scm
		  ON scm.concept_id = c.id AND scm.user_id = $1
		GROUP BY bp.boss_id`

	rows, err := s.db.Query(ctx, q, userID, bossIDs, paths)
	if err != nil {
		return nil, fmt.Errorf("batch chapter mastery %s: %w", userID, err)
	}
	defer rows.Close()

	for rows.Next() {
		var bossID string
		var mastery float64
		if err := rows.Scan(&bossID, &mastery); err != nil {
			return nil, fmt.Errorf("scan batch mastery: %w", err)
		}
		out[bossID] = mastery
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows batch mastery: %w", err)
	}
	return out, nil
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
