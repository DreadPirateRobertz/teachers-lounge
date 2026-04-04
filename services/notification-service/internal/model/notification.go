package model

import "time"

// Notification is an in-app notification persisted to Postgres.
type Notification struct {
	ID        string    `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Type      string    `json:"type" db:"type"`
	Message   string    `json:"message" db:"message"`
	Read      bool      `json:"read" db:"read"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// PushRequest is the payload for POST /notify/push.
type PushRequest struct {
	UserID string         `json:"user_id"`
	Title  string         `json:"title"`
	Body   string         `json:"body"`
	Data   map[string]any `json:"data,omitempty"`
}

// EmailRequest is the payload for POST /notify/email.
type EmailRequest struct {
	UserID   string         `json:"user_id"`
	Template string         `json:"template"`
	Vars     map[string]any `json:"vars,omitempty"`
}

// InAppRequest is the payload for POST /notify/in-app.
type InAppRequest struct {
	UserID  string `json:"user_id"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

// PushToken records a device push token registered by a user for FCM delivery.
type PushToken struct {
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	Platform  string    `json:"platform"`  // "android", "ios", or "web"
	UpdatedAt time.Time `json:"updated_at"`
}

// RegisterTokenRequest is the payload for POST /notify/push/token.
type RegisterTokenRequest struct {
	UserID   string `json:"user_id"`
	Token    string `json:"token"`
	Platform string `json:"platform,omitempty"` // defaults to "web" if empty
}
