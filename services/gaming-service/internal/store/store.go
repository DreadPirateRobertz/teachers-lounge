package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/quest"
	"github.com/teacherslounge/gaming-service/internal/xp"
)

const (
	streakKeyPrefix   = "streak:"
	streakResetWindow = 24 * time.Hour

	questKeyPrefix = "quests:daily:"
	questTTL       = 24 * time.Hour
)

// DB is the subset of *pgxpool.Pool that the Store uses.
// Defined as an interface so tests can substitute a lightweight fake.
type DB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Store holds Postgres and Redis clients.
type Store struct {
	db  DB
	rdb redis.Cmdable
}

// New creates a Store with the given Postgres pool and Redis client.
// db must implement the DB interface; *pgxpool.Pool satisfies it.
func New(db DB, rdb redis.Cmdable) *Store {
	return &Store{db: db, rdb: rdb}
}

// GetProfile fetches the gaming profile for a user. Returns nil, nil if not found.
func (s *Store) GetProfile(ctx context.Context, userID string) (*model.Profile, error) {
	const q = `
		SELECT user_id, level, xp, current_streak, longest_streak,
		       bosses_defeated, gems, power_ups, last_study_date
		FROM gaming_profiles
		WHERE user_id = $1`

	p := &model.Profile{}
	var powerUpsRaw []byte
	err := s.db.QueryRow(ctx, q, userID).Scan(
		&p.UserID, &p.Level, &p.XP,
		&p.CurrentStreak, &p.LongestStreak,
		&p.BossesDefeated, &p.Gems,
		&powerUpsRaw, &p.LastStudyDate,
	)
	if err != nil {
		return nil, fmt.Errorf("get profile %s: %w", userID, err)
	}
	if powerUpsRaw != nil {
		p.PowerUps = json.RawMessage(powerUpsRaw)
	} else {
		p.PowerUps = json.RawMessage("{}")
	}
	return p, nil
}

// UpsertXP creates or updates a gaming profile's XP and level.
// Returns the updated xp and level.
func (s *Store) UpsertXP(ctx context.Context, userID string, newXP int64, newLevel int) error {
	const q = `
		INSERT INTO gaming_profiles (user_id, xp, level)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		SET xp = EXCLUDED.xp, level = EXCLUDED.level`

	_, err := s.db.Exec(ctx, q, userID, newXP, newLevel)
	if err != nil {
		return fmt.Errorf("upsert xp %s: %w", userID, err)
	}
	return nil
}

// GetXPAndLevel fetches current xp and level for a user. Returns 0, 1 if not found.
func (s *Store) GetXPAndLevel(ctx context.Context, userID string) (xp int64, level int, err error) {
	const q = `SELECT xp, level FROM gaming_profiles WHERE user_id = $1`
	err = s.db.QueryRow(ctx, q, userID).Scan(&xp, &level)
	if err != nil {
		// treat not found as fresh user
		return 0, 1, nil
	}
	return xp, level, nil
}

// StreakCheckin increments the streak counter in Redis, resets if > 24h gap,
// then updates longest_streak in Postgres if exceeded.
// Returns current streak, longest streak, and whether the streak was reset.
func (s *Store) StreakCheckin(ctx context.Context, userID string) (current, longest int, reset bool, err error) {
	key := streakKeyPrefix + userID

	// Fetch last check-in time from Redis hash
	vals, err := s.rdb.HMGet(ctx, key, "count", "last_ts").Result()
	if err != nil {
		return 0, 0, false, fmt.Errorf("streak hmget %s: %w", userID, err)
	}

	now := time.Now().UTC()
	nowUnix := now.Unix()

	var currentStreak int
	reset = false

	if vals[0] != nil && vals[1] != nil {
		// Parse existing streak
		var count int64
		var lastTS int64
		_, _ = fmt.Sscan(vals[0].(string), &count)
		_, _ = fmt.Sscan(vals[1].(string), &lastTS)

		lastTime := time.Unix(lastTS, 0)
		if now.Sub(lastTime) > streakResetWindow {
			// Gap > 24h: consume a streak freeze if one is active, otherwise reset.
			frozen, _ := s.IsStreakFrozen(ctx, userID)
			if frozen {
				currentStreak = int(count) // preserve streak
				// Consume the freeze so it can't be reused.
				_ = s.rdb.Del(ctx, streakFreezeKeyPrefix+userID)
			} else {
				currentStreak = 1
				reset = true
			}
		} else {
			currentStreak = int(count) + 1
		}
	} else {
		currentStreak = 1
	}

	// Store updated streak in Redis (no TTL — streak persists)
	if err := s.rdb.HMSet(ctx, key, map[string]any{
		"count":   currentStreak,
		"last_ts": nowUnix,
	}).Err(); err != nil {
		return 0, 0, false, fmt.Errorf("streak hmset %s: %w", userID, err)
	}

	// Update Postgres longest_streak and current_streak
	const q = `
		INSERT INTO gaming_profiles (user_id, current_streak, longest_streak, last_study_date)
		VALUES ($1, $2, $2, CURRENT_DATE)
		ON CONFLICT (user_id) DO UPDATE
		SET current_streak = $2,
		    longest_streak = GREATEST(gaming_profiles.longest_streak, $2),
		    last_study_date = CURRENT_DATE
		RETURNING longest_streak`

	var longestDB int
	err = s.db.QueryRow(ctx, q, userID, currentStreak).Scan(&longestDB)
	if err != nil {
		return 0, 0, false, fmt.Errorf("streak upsert %s: %w", userID, err)
	}

	return currentStreak, longestDB, reset, nil
}


// RandomQuote fetches a random row from scifi_quotes with no dedup or context filter.
// Kept for unauthenticated or fallback callers.
func (s *Store) RandomQuote(ctx context.Context) (*model.Quote, error) {
	const q = `
		SELECT id, quote, attribution, context
		FROM scifi_quotes
		ORDER BY RANDOM()
		LIMIT 1`

	quote := &model.Quote{}
	err := s.db.QueryRow(ctx, q).Scan(&quote.ID, &quote.Quote, &quote.Attribution, &quote.Context)
	if err != nil {
		return nil, fmt.Errorf("random quote: %w", err)
	}
	return quote, nil
}

// seenQuotesKey returns the Redis key for a user's set of seen quote IDs for today.
// Key format: quotes:seen:{userID}:{YYYY-MM-DD} — expires after 25 hours.
func seenQuotesKey(userID string) string {
	return fmt.Sprintf("quotes:seen:%s:%s", userID, time.Now().UTC().Format("2006-01-02"))
}

const seenQuotesTTL = 25 * time.Hour // slightly more than a day for timezone safety

// RandomQuoteForUser fetches a random quote for a specific user, applying a
// context filter (empty string means any context) and excluding quote IDs the
// user has already seen today, tracked in Redis with a 25-hour TTL.
//
// If all matching quotes have been seen today, the seen-list is cleared and
// the query runs without exclusions so the user always gets a quote.
func (s *Store) RandomQuoteForUser(ctx context.Context, userID, quotectx string) (*model.Quote, error) {
	key := seenQuotesKey(userID)

	seenIDs, err := s.rdb.SMembers(ctx, key).Result()
	if err != nil {
		seenIDs = nil
	}

	quote, err := s.fetchQuoteExcluding(ctx, quotectx, seenIDs)
	if err != nil {
		return nil, err
	}

	if quote == nil {
		if len(seenIDs) > 0 {
			_ = s.rdb.Del(ctx, key)
		}
		quote, err = s.fetchQuoteExcluding(ctx, quotectx, nil)
		if err != nil {
			return nil, err
		}
	}

	if quote == nil {
		return nil, fmt.Errorf("no quotes found for context %q", quotectx)
	}

	pipe := s.rdb.Pipeline()
	pipe.SAdd(ctx, key, fmt.Sprintf("%d", quote.ID))
	pipe.Expire(ctx, key, seenQuotesTTL)
	_, _ = pipe.Exec(ctx)

	return quote, nil
}

// fetchQuoteExcluding queries scifi_quotes with an optional context filter and
// an optional exclusion list of already-seen IDs. Returns nil, nil when no rows
// match (not an error — callers handle the empty case).
func (s *Store) fetchQuoteExcluding(ctx context.Context, quotectx string, excludeIDs []string) (*model.Quote, error) {
	const qWithContext = `
		SELECT id, quote, attribution, context
		FROM scifi_quotes
		WHERE ($1 = '' OR context = $1)
		  AND NOT (id::text = ANY($2))
		ORDER BY RANDOM()
		LIMIT 1`

	const qFull = `
		SELECT id, quote, attribution, context
		FROM scifi_quotes
		WHERE ($1 = '' OR context = $1)
		ORDER BY RANDOM()
		LIMIT 1`

	quote := &model.Quote{}
	var scanErr error

	switch {
	case len(excludeIDs) == 0 && quotectx == "":
		const q = `SELECT id, quote, attribution, context FROM scifi_quotes ORDER BY RANDOM() LIMIT 1`
		scanErr = s.db.QueryRow(ctx, q).Scan(&quote.ID, &quote.Quote, &quote.Attribution, &quote.Context)
	case len(excludeIDs) == 0:
		scanErr = s.db.QueryRow(ctx, qFull, quotectx).Scan(&quote.ID, &quote.Quote, &quote.Attribution, &quote.Context)
	default:
		scanErr = s.db.QueryRow(ctx, qWithContext, quotectx, excludeIDs).Scan(&quote.ID, &quote.Quote, &quote.Attribution, &quote.Context)
	}

	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("fetch quote: %w", scanErr)
	}
	return quote, nil
}

// GetDailyQuests returns the current daily quest states for a user from Redis.
// Quests without any recorded progress are returned with Progress=0.
func (s *Store) GetDailyQuests(ctx context.Context, userID string) ([]model.QuestState, error) {
	key := questKeyPrefix + userID

	fields := make([]string, 0, len(quest.Daily)*2)
	for _, def := range quest.Daily {
		fields = append(fields, def.ID+":progress", def.ID+":completed")
	}

	vals, err := s.rdb.HMGet(ctx, key, fields...).Result()
	if err != nil {
		return nil, fmt.Errorf("get daily quests %s: %w", userID, err)
	}

	states := make([]model.QuestState, len(quest.Daily))
	for i, def := range quest.Daily {
		var progress int
		completed := false

		if vals[i*2] != nil {
			_, _ = fmt.Sscan(vals[i*2].(string), &progress)
		}
		if vals[i*2+1] != nil {
			completed = vals[i*2+1].(string) == "1"
		}

		states[i] = model.QuestState{
			ID:          def.ID,
			Title:       def.Title,
			Description: def.Description,
			Progress:    progress,
			Target:      def.Target,
			Completed:   completed,
			XPReward:    def.XPReward,
			GemsReward:  def.GemsReward,
		}
	}
	return states, nil
}

// UpdateQuestProgress advances quest progress in Redis for the given action,
// detects completions, and returns the updated quest states plus total XP and
// gems earned from newly completed quests.
func (s *Store) UpdateQuestProgress(ctx context.Context, userID string, action string) ([]model.QuestState, int, int, error) {
	questIDs := quest.ForAction(action)
	key := questKeyPrefix + userID

	var totalXP, totalGems int

	for _, questID := range questIDs {
		def := quest.ByID(questID)
		if def == nil {
			continue
		}

		vals, err := s.rdb.HMGet(ctx, key, questID+":progress", questID+":completed").Result()
		if err != nil {
			return nil, 0, 0, fmt.Errorf("hmget quest %s for %s: %w", questID, userID, err)
		}

		// Skip quests already completed to prevent double-rewarding.
		if vals[1] != nil && vals[1].(string) == "1" {
			continue
		}

		var progress int
		if vals[0] != nil {
			_, _ = fmt.Sscan(vals[0].(string), &progress)
		}
		progress++

		updates := map[string]any{
			questID + ":progress": progress,
		}
		if progress >= def.Target {
			updates[questID+":completed"] = "1"
			totalXP += def.XPReward
			totalGems += def.GemsReward
		}

		pipe := s.rdb.Pipeline()
		pipe.HMSet(ctx, key, updates)
		pipe.Expire(ctx, key, questTTL)
		if _, err := pipe.Exec(ctx); err != nil {
			return nil, 0, 0, fmt.Errorf("update quest %s for %s: %w", questID, userID, err)
		}
	}

	states, err := s.GetDailyQuests(ctx, userID)
	if err != nil {
		return nil, 0, 0, err
	}
	return states, totalXP, totalGems, nil
}


// AwardQuestRewards adds xpDelta XP and gemsDelta gems to a user's profile,
// recomputes the level, and returns the updated totals.
func (s *Store) AwardQuestRewards(ctx context.Context, userID string, xpDelta, gemsDelta int) (newXP int64, newLevel int, leveledUp bool, newGems int, err error) {
	currentXP, currentLevel, err := s.GetXPAndLevel(ctx, userID)
	if err != nil {
		return 0, 0, false, 0, fmt.Errorf("get xp for reward %s: %w", userID, err)
	}

	newXP, newLevel, leveledUp = xp.Apply(currentXP, currentLevel, int64(xpDelta))

	const q = `
		INSERT INTO gaming_profiles (user_id, xp, level, gems)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE
		SET xp    = EXCLUDED.xp,
		    level = EXCLUDED.level,
		    gems  = gaming_profiles.gems + $4
		RETURNING gems`

	err = s.db.QueryRow(ctx, q, userID, newXP, newLevel, gemsDelta).Scan(&newGems)
	if err != nil {
		return 0, 0, false, 0, fmt.Errorf("award quest rewards %s: %w", userID, err)
	}
	return newXP, newLevel, leveledUp, newGems, nil
}

// SaveTaunt persists a generated taunt to the boss_taunts pool for the given
// boss and round. Multiple taunts per (boss_id, round) are allowed; callers
// use GetRandomTaunt to draw from the pool.
func (s *Store) SaveTaunt(ctx context.Context, bossID string, round int, taunt string) error {
	const q = `INSERT INTO boss_taunts (boss_id, round, taunt) VALUES ($1, $2, $3)`
	if _, err := s.db.Exec(ctx, q, bossID, round, taunt); err != nil {
		return fmt.Errorf("store: save taunt: %w", err)
	}
	return nil
}

// GetRandomTaunt returns a random cached taunt for the given boss and round.
// ok is false when no taunts have been stored for this (boss_id, round) pair yet.
func (s *Store) GetRandomTaunt(ctx context.Context, bossID string, round int) (taunt string, ok bool, err error) {
	const q = `
		SELECT taunt FROM boss_taunts
		WHERE boss_id = $1 AND round = $2
		ORDER BY RANDOM()
		LIMIT 1`
	err = s.db.QueryRow(ctx, q, bossID, round).Scan(&taunt)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("store: get random taunt: %w", err)
	}
	return taunt, true, nil
}
