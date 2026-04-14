// Package push delivers push notifications to registered device tokens via
// Firebase Cloud Messaging (FCM).
package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// sanitizeForLog strips CR/LF from user-controlled strings before they flow
// into log entries, preventing log-forging (CWE-117).
func sanitizeForLog(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

const defaultFCMEndpoint = "https://fcm.googleapis.com/fcm/send"

// Pusher delivers a push notification to a single device token.
// Implementations must be safe for concurrent use.
type Pusher interface {
	// Send delivers a push notification to the given device token.
	// Returns a non-nil error when delivery fails. Token-level errors
	// (e.g. InvalidRegistration) are surfaced so the caller can purge
	// the stale token from storage.
	Send(ctx context.Context, token, title, body string, data map[string]any) error
}

// FCMPusher sends push notifications via the Firebase Cloud Messaging
// legacy HTTP API (https://fcm.googleapis.com/fcm/send).
type FCMPusher struct {
	serverKey  string
	endpoint   string
	httpClient *http.Client
}

// WithEndpoint returns an option that overrides the FCM API URL.
// Intended for use in tests.
func WithEndpoint(url string) func(*FCMPusher) {
	return func(p *FCMPusher) { p.endpoint = url }
}

// NewFCMPusher returns an FCMPusher authenticated with the given FCM server key.
// Pass functional options such as WithEndpoint to customise behaviour.
func NewFCMPusher(serverKey string, opts ...func(*FCMPusher)) *FCMPusher {
	p := &FCMPusher{
		serverKey:  serverKey,
		endpoint:   defaultFCMEndpoint,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Send delivers a push notification to the given FCM device token.
// It serialises the notification as a FCM legacy message, authenticates
// with the server key, and checks both the HTTP status and the FCM-level
// failure count in the response body.
func (p *FCMPusher) Send(ctx context.Context, token, title, body string, data map[string]any) error {
	payload := map[string]any{
		"to": token,
		"notification": map[string]string{
			"title": title,
			"body":  body,
		},
	}
	if len(data) > 0 {
		payload["data"] = data
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("fcm: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("fcm: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "key="+p.serverKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fcm: http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort body drain

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fcm: unexpected HTTP status %d", resp.StatusCode)
	}

	var result struct {
		Failure int `json:"failure"`
		Results []struct {
			Error string `json:"error"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("fcm: decode response: %w", err)
	}
	if result.Failure > 0 && len(result.Results) > 0 {
		return fmt.Errorf("fcm: delivery failed: %s", result.Results[0].Error)
	}
	return nil
}

// LogPusher is a Pusher that discards notifications without delivering them.
// Use it when FCM credentials are absent (e.g. local development) so the
// rest of the stack exercises the same code path as production.
// It logs a warning on every Send call so operators know the delivery is a noop.
type LogPusher struct {
	Logger *zap.Logger
}

// Send logs a warning and returns nil without delivering the notification.
// The warn is intentional: operators need to know that FCM is not configured
// and every send is silently dropped, so the log is the signal.
func (p LogPusher) Send(_ context.Context, token, title, _ string, _ map[string]any) error {
	log := p.Logger
	if log == nil {
		log = zap.NewNop()
	}
	log.Warn("LogPusher.Send: FCM not configured — notification discarded (noop)",
		zap.String("token_prefix", tokenPrefix(token)),
		zap.String("title", sanitizeForLog(title)),
	)
	return nil
}

// tokenPrefix returns the first 8 characters of token for safe logging.
func tokenPrefix(token string) string {
	if len(token) <= 8 {
		return token
	}
	return token[:8]
}
