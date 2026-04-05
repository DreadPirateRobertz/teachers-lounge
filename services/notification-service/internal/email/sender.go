// Package email delivers transactional email via the SendGrid v3 API.
package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const sendgridEndpoint = "https://api.sendgrid.com/v3/mail/send"

// Sender delivers a single transactional email.
// Implementations must be safe for concurrent use.
type Sender interface {
	// Send delivers an HTML email to the given address.
	// subject is the email subject line; htmlBody is the full HTML content.
	// Returns a non-nil error when delivery fails.
	Send(ctx context.Context, to, subject, htmlBody string) error
}

// SendGridSender delivers email via the SendGrid v3 Mail Send API.
type SendGridSender struct {
	apiKey      string
	fromAddress string
	endpoint    string
	httpClient  *http.Client
}

// NewSendGridSender returns a SendGridSender configured with the given API key
// and sender address. Use WithEndpoint and WithHTTPClient for test overrides.
func NewSendGridSender(apiKey, fromAddress string, opts ...func(*SendGridSender)) *SendGridSender {
	s := &SendGridSender{
		apiKey:      apiKey,
		fromAddress: fromAddress,
		endpoint:    sendgridEndpoint,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// WithEndpoint overrides the SendGrid API endpoint. Used in tests to point at
// an httptest.Server.
func WithEndpoint(url string) func(*SendGridSender) {
	return func(s *SendGridSender) { s.endpoint = url }
}

// WithHTTPClient replaces the HTTP client used for SendGrid calls.
func WithHTTPClient(c *http.Client) func(*SendGridSender) {
	return func(s *SendGridSender) { s.httpClient = c }
}

// Send delivers an HTML email via the SendGrid v3 API.
// Returns an error when the HTTP status is outside the 2xx range or when the
// request cannot be constructed or executed.
func (s *SendGridSender) Send(ctx context.Context, to, subject, htmlBody string) error {
	payload := map[string]any{
		"personalizations": []map[string]any{
			{"to": []map[string]string{{"email": to}}},
		},
		"from":    map[string]string{"email": s.fromAddress},
		"subject": subject,
		"content": []map[string]string{
			{"type": "text/html", "value": htmlBody},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("email: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("email: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("email: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("email: sendgrid returned %d", resp.StatusCode)
	}
	return nil
}

// LogSender is a no-op Sender that discards all emails.
// Used in local development and tests when SendGrid credentials are not set.
type LogSender struct{}

// Send discards the email and returns nil.
func (LogSender) Send(_ context.Context, _, _, _ string) error {
	return nil
}
