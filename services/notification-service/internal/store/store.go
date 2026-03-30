package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/teacherslounge/notification-service/internal/model"
)

const (
	unreadCountKey = "notif:unread:"
)

// Store holds Postgres and Redis clients.
type Store struct {
	db  *pgxpool.Pool
	rdb redis.Cmdable
}

// New creates a Store.
func New(db *pgxpool.Pool, rdb redis.Cmdable) *Store {
	return &Store{db: db, rdb: rdb}
}

// CreateNotification inserts an in-app notification and increments the unread counter.
func (s *Store) CreateNotification(ctx context.Context, n *model.Notification) error {
	const q = `
		INSERT INTO notifications (id, user_id, channel, title, body, category, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := s.db.Exec(ctx, q, n.ID, n.UserID, n.Channel, n.Title, n.Body, n.Category, n.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}

	// Increment unread count in Redis
	s.rdb.Incr(ctx, unreadCountKey+n.UserID)
	return nil
}

// ListNotifications returns the most recent notifications for a user.
func (s *Store) ListNotifications(ctx context.Context, userID string, limit, offset int) ([]model.Notification, error) {
	const q = `
		SELECT id, user_id, channel, title, body, category, read_at, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := s.db.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	var notifications []model.Notification
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Channel, &n.Title, &n.Body, &n.Category, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		notifications = append(notifications, n)
	}
	if notifications == nil {
		notifications = []model.Notification{}
	}
	return notifications, nil
}

// UnreadCount returns the number of unread notifications for a user.
func (s *Store) UnreadCount(ctx context.Context, userID string) (int, error) {
	const q = `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL`
	var count int
	err := s.db.QueryRow(ctx, q, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("unread count: %w", err)
	}
	return count, nil
}

// MarkRead marks a single notification as read.
func (s *Store) MarkRead(ctx context.Context, userID, notificationID string) error {
	const q = `
		UPDATE notifications SET read_at = NOW()
		WHERE id = $1 AND user_id = $2 AND read_at IS NULL`

	tag, err := s.db.Exec(ctx, q, notificationID, userID)
	if err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	if tag.RowsAffected() > 0 {
		s.rdb.Decr(ctx, unreadCountKey+userID)
	}
	return nil
}

// MarkAllRead marks all unread notifications for a user as read.
func (s *Store) MarkAllRead(ctx context.Context, userID string) (int64, error) {
	const q = `UPDATE notifications SET read_at = NOW() WHERE user_id = $1 AND read_at IS NULL`

	tag, err := s.db.Exec(ctx, q, userID)
	if err != nil {
		return 0, fmt.Errorf("mark all read: %w", err)
	}

	count := tag.RowsAffected()
	if count > 0 {
		s.rdb.Set(ctx, unreadCountKey+userID, 0, 0)
	}
	return count, nil
}

// GetPreferences fetches notification preferences for a user. Returns defaults if not found.
func (s *Store) GetPreferences(ctx context.Context, userID string) (*model.Preferences, error) {
	const q = `
		SELECT user_id, push_enabled, email_enabled, in_app_enabled, category_overrides
		FROM notification_preferences
		WHERE user_id = $1`

	p := &model.Preferences{}
	var overridesRaw []byte
	err := s.db.QueryRow(ctx, q, userID).Scan(
		&p.UserID, &p.PushEnabled, &p.EmailEnabled, &p.InAppEnabled, &overridesRaw,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return model.DefaultPreferences(userID), nil
		}
		return nil, fmt.Errorf("get preferences: %w", err)
	}
	if overridesRaw != nil {
		if err := json.Unmarshal(overridesRaw, &p.CategoryOverrides); err != nil {
			return nil, fmt.Errorf("unmarshal overrides: %w", err)
		}
	}
	return p, nil
}

// UpsertPreferences creates or updates notification preferences.
func (s *Store) UpsertPreferences(ctx context.Context, p *model.Preferences) error {
	overridesJSON, err := json.Marshal(p.CategoryOverrides)
	if err != nil {
		return fmt.Errorf("marshal overrides: %w", err)
	}

	const q = `
		INSERT INTO notification_preferences (user_id, push_enabled, email_enabled, in_app_enabled, category_overrides)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE
		SET push_enabled = EXCLUDED.push_enabled,
		    email_enabled = EXCLUDED.email_enabled,
		    in_app_enabled = EXCLUDED.in_app_enabled,
		    category_overrides = EXCLUDED.category_overrides`

	_, err = s.db.Exec(ctx, q, p.UserID, p.PushEnabled, p.EmailEnabled, p.InAppEnabled, overridesJSON)
	if err != nil {
		return fmt.Errorf("upsert preferences: %w", err)
	}
	return nil
}

// SaveDeviceToken registers an FCM device token for a user.
func (s *Store) SaveDeviceToken(ctx context.Context, userID, token, platform string) error {
	const q = `
		INSERT INTO device_tokens (user_id, token, platform)
		VALUES ($1, $2, $3)
		ON CONFLICT (token) DO UPDATE
		SET user_id = EXCLUDED.user_id, platform = EXCLUDED.platform`

	_, err := s.db.Exec(ctx, q, userID, token, platform)
	if err != nil {
		return fmt.Errorf("save device token: %w", err)
	}
	return nil
}

// DeleteDeviceToken removes a device token.
func (s *Store) DeleteDeviceToken(ctx context.Context, userID, token string) error {
	const q = `DELETE FROM device_tokens WHERE user_id = $1 AND token = $2`
	_, err := s.db.Exec(ctx, q, userID, token)
	if err != nil {
		return fmt.Errorf("delete device token: %w", err)
	}
	return nil
}

// GetDeviceTokens returns all device tokens for a user.
func (s *Store) GetDeviceTokens(ctx context.Context, userID string) ([]model.DeviceToken, error) {
	const q = `SELECT user_id, token, platform, created_at FROM device_tokens WHERE user_id = $1`

	rows, err := s.db.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("get device tokens: %w", err)
	}
	defer rows.Close()

	var tokens []model.DeviceToken
	for rows.Next() {
		var dt model.DeviceToken
		if err := rows.Scan(&dt.UserID, &dt.Token, &dt.Platform, &dt.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan device token: %w", err)
		}
		tokens = append(tokens, dt)
	}
	return tokens, nil
}
