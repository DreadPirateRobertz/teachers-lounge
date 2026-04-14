package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/model"
)

// Store handles all Postgres operations for the notification service.
type Store struct {
	db *pgxpool.Pool
}

// New returns a Store connected to the given Postgres pool.
func New(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// CreateNotification inserts a new in-app notification and returns it with its generated ID.
func (s *Store) CreateNotification(ctx context.Context, n *model.Notification) (*model.Notification, error) {
	const q = `
		INSERT INTO notifications (user_id, type, message, read, created_at)
		VALUES ($1, $2, $3, false, NOW())
		RETURNING id, user_id, type, message, read, created_at`

	row := s.db.QueryRow(ctx, q, n.UserID, n.Type, n.Message)
	var out model.Notification
	if err := row.Scan(&out.ID, &out.UserID, &out.Type, &out.Message, &out.Read, &out.CreatedAt); err != nil {
		return nil, fmt.Errorf("store: create notification: %w", err)
	}
	return &out, nil
}

// ListUnread returns all unread notifications for a user, newest first.
func (s *Store) ListUnread(ctx context.Context, userID string) ([]model.Notification, error) {
	const q = `
		SELECT id, user_id, type, message, read, created_at
		FROM notifications
		WHERE user_id = $1 AND read = false
		ORDER BY created_at DESC`

	rows, err := s.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("store: list unread: %w", err)
	}
	defer rows.Close()

	var out []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Message, &n.Read, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: scan notification: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// SavePushToken upserts a device push token for a user. If the (user_id, token)
// pair already exists, only the platform and updated_at fields are refreshed.
func (s *Store) SavePushToken(ctx context.Context, userID, token, platform string) error {
	const q = `
		INSERT INTO push_tokens (user_id, token, platform, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id, token) DO UPDATE
		SET platform = EXCLUDED.platform, updated_at = NOW()`
	if _, err := s.db.Exec(ctx, q, userID, token, platform); err != nil {
		return fmt.Errorf("store: save push token: %w", err)
	}
	return nil
}

// DeletePushToken removes a single FCM device token for a user.
// Called when FCM returns InvalidRegistration or NotRegistered to purge the
// stale token so it is not retried on future cron runs. A missing row is not
// an error — the token may have already been purged by a concurrent runner.
func (s *Store) DeletePushToken(ctx context.Context, userID, token string) error {
	const q = `DELETE FROM push_tokens WHERE user_id = $1 AND token = $2`
	if _, err := s.db.Exec(ctx, q, userID, token); err != nil {
		return fmt.Errorf("store: delete push token: %w", err)
	}
	return nil
}

// GetPushTokens returns all FCM device tokens registered for a user.
// Returns an empty (non-nil) slice when the user has no tokens.
func (s *Store) GetPushTokens(ctx context.Context, userID string) ([]string, error) {
	const q = `SELECT token FROM push_tokens WHERE user_id = $1`
	rows, err := s.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("store: get push tokens: %w", err)
	}
	defer rows.Close()

	tokens := []string{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("store: scan push token: %w", err)
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// Migrate creates the notifications and push_tokens tables if they do not exist,
// and applies any additive DDL migrations needed by the notification service.
// Called once on startup before the server accepts traffic.
func Migrate(ctx context.Context, db *pgxpool.Pool) error {
	const ddl = `
		CREATE TABLE IF NOT EXISTS notifications (
			id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id    TEXT        NOT NULL,
			type       TEXT        NOT NULL,
			message    TEXT        NOT NULL,
			read       BOOLEAN     NOT NULL DEFAULT false,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS notifications_user_unread
			ON notifications (user_id, read)
			WHERE read = false;

		CREATE TABLE IF NOT EXISTS push_tokens (
			user_id    TEXT        NOT NULL,
			token      TEXT        NOT NULL,
			platform   TEXT        NOT NULL DEFAULT 'web',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (user_id, token)
		);

		-- Deduplication guard for streak-reminder cron (tl-wti).
		-- gaming_profiles is owned by gaming-service but both services share the
		-- same Postgres cluster. Adding the column here is idempotent (IF NOT
		-- EXISTS) and does not require gaming-service to know about it.
		ALTER TABLE IF EXISTS gaming_profiles
			ADD COLUMN IF NOT EXISTS last_streak_reminder_at TIMESTAMPTZ;`

	if _, err := db.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	return nil
}
