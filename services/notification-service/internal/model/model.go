package model

import "time"

// Channel represents a notification delivery channel.
type Channel string

const (
	ChannelPush  Channel = "push"
	ChannelEmail Channel = "email"
	ChannelInApp Channel = "in_app"
)

// Notification is a stored in-app notification.
type Notification struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Channel   Channel   `json:"channel"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Category  string    `json:"category"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Preferences stores per-user notification settings.
type Preferences struct {
	UserID       string `json:"user_id"`
	PushEnabled  bool   `json:"push_enabled"`
	EmailEnabled bool   `json:"email_enabled"`
	InAppEnabled bool   `json:"in_app_enabled"`
	// Category-level overrides: category -> channel -> enabled
	CategoryOverrides map[string]map[Channel]bool `json:"category_overrides,omitempty"`
}

// DefaultPreferences returns preferences with everything enabled.
func DefaultPreferences(userID string) *Preferences {
	return &Preferences{
		UserID:       userID,
		PushEnabled:  true,
		EmailEnabled: true,
		InAppEnabled: true,
	}
}

// SendRequest is the request body for POST /notifications/send.
type SendRequest struct {
	UserID   string    `json:"user_id"`
	Channels []Channel `json:"channels"`
	Title    string    `json:"title"`
	Body     string    `json:"body"`
	Category string    `json:"category"`
	// Email-specific fields
	EmailSubject string `json:"email_subject,omitempty"`
	// Push-specific fields
	Data map[string]string `json:"data,omitempty"`
}

// SendResponse is the response body for POST /notifications/send.
type SendResponse struct {
	Results []ChannelResult `json:"results"`
}

// ChannelResult reports success/failure for a single channel delivery.
type ChannelResult struct {
	Channel Channel `json:"channel"`
	Success bool    `json:"success"`
	Error   string  `json:"error,omitempty"`
}

// NotificationListResponse wraps a paginated list of notifications.
type NotificationListResponse struct {
	Notifications []Notification `json:"notifications"`
	UnreadCount   int            `json:"unread_count"`
}

// UpdatePreferencesRequest is the request body for PUT /notifications/preferences.
type UpdatePreferencesRequest struct {
	PushEnabled       *bool                       `json:"push_enabled,omitempty"`
	EmailEnabled      *bool                       `json:"email_enabled,omitempty"`
	InAppEnabled      *bool                       `json:"in_app_enabled,omitempty"`
	CategoryOverrides map[string]map[Channel]bool `json:"category_overrides,omitempty"`
}

// DeviceToken represents a registered FCM device token.
type DeviceToken struct {
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	Platform  string    `json:"platform"` // "ios", "android", "web"
	CreatedAt time.Time `json:"created_at"`
}

// RegisterTokenRequest is the request body for POST /notifications/devices.
type RegisterTokenRequest struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}
