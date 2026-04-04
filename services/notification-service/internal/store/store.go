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

// Migrate creates the notifications table if it does not exist.
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
			WHERE read = false;`

	if _, err := db.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	return nil
}
