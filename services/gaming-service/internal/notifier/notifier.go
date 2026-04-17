// Package notifier dispatches gaming-service achievement events to the
// notification-service, which owns FCM fan-out and per-user dedup windows.
//
// The Notifier interface lets handlers fire level-up and quest-complete
// events without knowing whether a real HTTP client is wired up — in tests
// and unconfigured environments a NoopNotifier is substituted.
package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Notifier fires push notifications for gaming-service achievement events.
// All methods must be safe for concurrent use. Implementations should treat
// transient errors as recoverable — the caller fires-and-forgets.
type Notifier interface {
	// NotifyLevelUp posts a level-up event for the given user.
	NotifyLevelUp(ctx context.Context, userID string, newLevel int) error
	// NotifyQuestComplete posts a quest-completion event for the given user.
	NotifyQuestComplete(ctx context.Context, userID, questTitle string, xpReward int) error
}

// NoopNotifier satisfies Notifier but performs no work. Used when the
// notification-service URL is not configured or in tests that don't exercise
// push dispatch.
type NoopNotifier struct{}

// NotifyLevelUp is a no-op.
func (NoopNotifier) NotifyLevelUp(_ context.Context, _ string, _ int) error {
	return nil
}

// NotifyQuestComplete is a no-op.
func (NoopNotifier) NotifyQuestComplete(_ context.Context, _, _ string, _ int) error {
	return nil
}

// HTTPNotifier posts achievement events to the notification-service internal
// push endpoints. Dedup and FCM fan-out are owned by notification-service —
// this client's job is to deliver the event JSON and return a terminal status.
type HTTPNotifier struct {
	baseURL    string
	httpClient *http.Client
}

// HTTPOption configures an HTTPNotifier.
type HTTPOption func(*HTTPNotifier)

// WithHTTPClient overrides the default HTTP client (primarily for tests that
// point at an httptest.Server).
func WithHTTPClient(c *http.Client) HTTPOption {
	return func(n *HTTPNotifier) { n.httpClient = c }
}

// NewHTTP returns an HTTPNotifier posting to baseURL (e.g. "http://notification-service:9000").
// A sensible default HTTP timeout of 5s is applied; override with WithHTTPClient.
func NewHTTP(baseURL string, opts ...HTTPOption) *HTTPNotifier {
	n := &HTTPNotifier{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
	for _, o := range opts {
		o(n)
	}
	return n
}

// NotifyLevelUp POSTs {user_id, new_level} to baseURL + "/internal/push/level-up".
// Any non-2xx response is returned as an error.
func (n *HTTPNotifier) NotifyLevelUp(ctx context.Context, userID string, newLevel int) error {
	payload := map[string]any{"user_id": userID, "new_level": newLevel}
	return n.post(ctx, "/internal/push/level-up", payload)
}

// NotifyQuestComplete POSTs {user_id, quest_title, xp_reward} to
// baseURL + "/internal/push/quest-complete".
func (n *HTTPNotifier) NotifyQuestComplete(ctx context.Context, userID, questTitle string, xpReward int) error {
	payload := map[string]any{
		"user_id":     userID,
		"quest_title": questTitle,
		"xp_reward":   xpReward,
	}
	return n.post(ctx, "/internal/push/quest-complete", payload)
}

func (n *HTTPNotifier) post(ctx context.Context, path string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notifier: marshal %s: %w", path, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notifier: build request %s: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("notifier: http %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("notifier: %s returned %d", path, resp.StatusCode)
	}
	return nil
}
