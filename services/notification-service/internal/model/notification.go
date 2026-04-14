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
	To       string         `json:"to"`               // recipient email address
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

// TriggerRequest is the payload for POST /notify/trigger.
// Called by internal services (gaming, tutoring) to fire event-based
// push + email + in-app notifications for a specific user.
type TriggerRequest struct {
	EventType string         `json:"event_type"` // one of the event.Type constants
	UserID    string         `json:"user_id"`
	ToEmail   string         `json:"to_email,omitempty"` // skip email when empty
	Payload   map[string]any `json:"payload,omitempty"`  // dynamic vars, e.g. rival_name
}

// TriggerResponse reports how many notification channels were reached.
type TriggerResponse struct {
	PushSent  int  `json:"push_sent"`   // number of device tokens dispatched
	EmailSent bool `json:"email_sent"`  // true when email was attempted without error
	InAppSent bool `json:"in_app_sent"` // true when in-app notification was persisted
}

// UserAtRisk identifies a user whose active streak is about to expire.
// Emitted by GetUsersAtRiskOfStreakLoss for consumption by the streak-reminder
// cron: users with a live streak who have not studied for a configurable
// window (default ≥20h, <24h) and who do not hold an active freeze.
type UserAtRisk struct {
	UserID        string `json:"user_id"`
	CurrentStreak int    `json:"current_streak"`
}

// StreakReminderResponse is the response body for POST /internal/notify/streak-reminder.
// Counts are best-effort — individual FCM failures are recorded under Failed
// without failing the request so the cron can keep iterating other users.
type StreakReminderResponse struct {
	AtRisk int `json:"at_risk"` // users returned by the at-risk query
	Sent   int `json:"sent"`    // device-token deliveries that succeeded
	Failed int `json:"failed"`  // device-token deliveries that returned an error
}
