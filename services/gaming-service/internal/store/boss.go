package store

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/xp"
)

const (
	bossSessionKeyFmt = "boss:session:%s"    // boss:session:{sessionID}
	bossActiveKeyFmt  = "boss:active:%s"     // boss:active:{userID} → sessionID
	bossBattleTTL     = time.Hour
)

// ErrActiveBossBattle is returned when the user already has an active boss session.
var ErrActiveBossBattle = errors.New("active boss battle already in progress")

// ErrBossSessionNotFound is returned when a requested boss session does not exist.
var ErrBossSessionNotFound = errors.New("boss session not found")

// StartBossBattle creates a new boss session in Redis and records the active-session
// pointer. Returns ErrActiveBossBattle if the user is already mid-fight.
func (s *Store) StartBossBattle(
	ctx context.Context,
	userID, bossID, bossName, topic string,
	maxRounds, bossHP int,
	questionIDs []string,
) (*model.BossSession, error) {
	// Reject if the user already has an active session.
	activeKey := fmt.Sprintf(bossActiveKeyFmt, userID)
	existingID, err := s.rdb.Get(ctx, activeKey).Result()
	if err == nil && existingID != "" {
		// Key exists — check if the session itself is still active.
		existing, sessErr := s.GetBossSession(ctx, existingID)
		if sessErr == nil && existing.Status == "active" {
			return nil, ErrActiveBossBattle
		}
		// Session expired or completed — fall through and start fresh.
	}

	id := newBossUUID()
	session := &model.BossSession{
		ID:          id,
		UserID:      userID,
		BossID:      bossID,
		BossName:    bossName,
		Topic:       topic,
		StudentHP:   100,
		BossHP:      bossHP,
		MaxBossHP:   bossHP,
		Round:       1,
		MaxRounds:   maxRounds,
		ComboStreak: 0,
		QuestionIDs: questionIDs,
		Status:      "active",
		StartedAt:   time.Now().UTC(),
	}

	if err := s.saveBossSession(ctx, session); err != nil {
		return nil, err
	}

	// Record active-session pointer with same TTL.
	if err := s.rdb.Set(ctx, activeKey, id, bossBattleTTL).Err(); err != nil {
		return nil, fmt.Errorf("set boss active key %s: %w", userID, err)
	}

	return session, nil
}

// GetBossSession loads a boss session from Redis.
func (s *Store) GetBossSession(ctx context.Context, sessionID string) (*model.BossSession, error) {
	key := fmt.Sprintf(bossSessionKeyFmt, sessionID)
	raw, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, ErrBossSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get boss session %s: %w", sessionID, err)
	}

	var session model.BossSession
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return nil, fmt.Errorf("unmarshal boss session %s: %w", sessionID, err)
	}
	return &session, nil
}

// SaveBossSession writes an updated session back to Redis, refreshing the TTL.
func (s *Store) SaveBossSession(ctx context.Context, session *model.BossSession) error {
	return s.saveBossSession(ctx, session)
}

// CompleteBossBattle persists the battle result to Postgres (boss_history,
// gaming_profiles), handles achievements, and clears the active-session pointer.
// It returns the player's updated XP/level and total bosses defeated.
func (s *Store) CompleteBossBattle(
	ctx context.Context,
	session *model.BossSession,
	bonusXP int,
) (newXP int64, newLevel int, leveledUp bool, bossesDefeated int, err error) {
	result := "defeat"
	if session.Status == "victory" {
		result = "victory"
	}

	totalXPEarned := session.TotalXP + bonusXP

	tx, txErr := s.db.Begin(ctx)
	if txErr != nil {
		return 0, 0, false, 0, fmt.Errorf("begin complete boss tx: %w", txErr)
	}
	defer tx.Rollback(ctx)

	// Record in boss_history.
	const insertHistory = `
		INSERT INTO boss_history (id, user_id, boss_name, topic, rounds, result, xp_earned)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5::boss_result, $6)`
	if _, err = tx.Exec(ctx, insertHistory,
		session.UserID, session.BossName, session.Topic,
		session.CurrentIndex, result, totalXPEarned,
	); err != nil {
		return 0, 0, false, 0, fmt.Errorf("insert boss history: %w", err)
	}

	// Update gaming_profiles: apply XP and, on victory, increment bosses_defeated.
	currentXP, currentLevel, err := s.GetXPAndLevel(ctx, session.UserID)
	if err != nil {
		return 0, 0, false, 0, fmt.Errorf("get xp for boss complete %s: %w", session.UserID, err)
	}
	newXP, newLevel, leveledUp = xp.Apply(currentXP, currentLevel, int64(totalXPEarned))

	var updateProfile string
	var profileArgs []any
	if result == "victory" {
		updateProfile = `
			INSERT INTO gaming_profiles (user_id, xp, level, bosses_defeated)
			VALUES ($1, $2, $3, 1)
			ON CONFLICT (user_id) DO UPDATE
			SET xp              = EXCLUDED.xp,
			    level           = EXCLUDED.level,
			    bosses_defeated = gaming_profiles.bosses_defeated + 1
			RETURNING bosses_defeated`
		profileArgs = []any{session.UserID, newXP, newLevel}
	} else {
		updateProfile = `
			INSERT INTO gaming_profiles (user_id, xp, level, bosses_defeated)
			VALUES ($1, $2, $3, 0)
			ON CONFLICT (user_id) DO UPDATE
			SET xp    = EXCLUDED.xp,
			    level = EXCLUDED.level
			RETURNING bosses_defeated`
		profileArgs = []any{session.UserID, newXP, newLevel}
	}
	if err = tx.QueryRow(ctx, updateProfile, profileArgs...).Scan(&bossesDefeated); err != nil {
		return 0, 0, false, 0, fmt.Errorf("update profile on boss complete: %w", err)
	}

	// Award first-boss achievement if this is the player's first victory.
	if result == "victory" {
		const upsertAchievement = `
			INSERT INTO achievements (id, user_id, achievement_type, earned_at)
			VALUES (gen_random_uuid(), $1, $2, NOW())
			ON CONFLICT (user_id, achievement_type) DO NOTHING`
		if _, err = tx.Exec(ctx, upsertAchievement, session.UserID, "first_boss"); err != nil {
			return 0, 0, false, 0, fmt.Errorf("upsert first_boss achievement: %w", err)
		}
		if bossesDefeated >= 5 {
			if _, err = tx.Exec(ctx, upsertAchievement, session.UserID, "boss_slayer"); err != nil {
				return 0, 0, false, 0, fmt.Errorf("upsert boss_slayer achievement: %w", err)
			}
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, 0, false, 0, fmt.Errorf("commit boss complete tx: %w", err)
	}

	// Clear the active-session pointer.
	_ = s.rdb.Del(ctx, fmt.Sprintf(bossActiveKeyFmt, session.UserID))

	return newXP, newLevel, leveledUp, bossesDefeated, nil
}

// --- helpers ---

func (s *Store) saveBossSession(ctx context.Context, session *model.BossSession) error {
	raw, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal boss session %s: %w", session.ID, err)
	}
	key := fmt.Sprintf(bossSessionKeyFmt, session.ID)
	if err := s.rdb.Set(ctx, key, raw, bossBattleTTL).Err(); err != nil {
		return fmt.Errorf("save boss session %s: %w", session.ID, err)
	}
	return nil
}

// newBossUUID generates a random UUID v4.
func newBossUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant RFC4122
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
