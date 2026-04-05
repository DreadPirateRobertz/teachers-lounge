package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/teacherslounge/gaming-service/internal/model"
)

// BuyPowerUp atomically deducts gemCost gems from the user's profile and
// increments the power-up count in the power_ups JSONB inventory column.
// Returns the updated gem balance and inventory count for the purchased type.
// Returns ErrNoGems if the user's balance is insufficient (the WHERE clause
// filters out the row, causing pgx.ErrNoRows). Any other error is unexpected.
func (s *Store) BuyPowerUp(ctx context.Context, userID string, pu model.PowerUpType, gemCost int) (gemsLeft, newCount int, err error) {
	const q = `
		UPDATE gaming_profiles
		SET gems = gems - $2,
		    power_ups = jsonb_set(
		        power_ups,
		        ARRAY[$3],
		        to_jsonb(COALESCE((power_ups->>$3)::int, 0) + 1)
		    )
		WHERE user_id = $1 AND gems >= $2
		RETURNING gems, power_ups`

	var rawPU []byte
	err = s.db.QueryRow(ctx, q, userID, gemCost, string(pu)).Scan(&gemsLeft, &rawPU)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, ErrNoGems
		}
		return 0, 0, fmt.Errorf("buy power-up %s for %s (cost=%d): %w", pu, userID, gemCost, err)
	}

	var inv map[string]int
	if err2 := json.Unmarshal(rawPU, &inv); err2 != nil {
		return gemsLeft, 0, fmt.Errorf("unmarshal power_ups after buy: %w", err2)
	}
	newCount = inv[string(pu)]
	return gemsLeft, newCount, nil
}
