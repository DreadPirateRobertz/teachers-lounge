package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SendGridClient sends email notifications via the SendGrid v3 API.
type SendGridClient struct {
	apiKey     string
	fromEmail  string
	fromName   string
	httpClient *http.Client
}

// NewSendGridClient creates a SendGrid client.
func NewSendGridClient(apiKey, fromEmail, fromName string) *SendGridClient {
	return &SendGridClient{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		fromName:  fromName,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type sgMailRequest struct {
	Personalizations []sgPersonalization `json:"personalizations"`
	From             sgAddress           `json:"from"`
	Subject          string              `json:"subject"`
	Content          []sgContent         `json:"content"`
}

type sgPersonalization struct {
	To []sgAddress `json:"to"`
}

type sgAddress struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type sgContent struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Send sends an email via SendGrid.
func (c *SendGridClient) Send(ctx context.Context, toEmail, subject, htmlBody string) error {
	msg := sgMailRequest{
		Personalizations: []sgPersonalization{
			{To: []sgAddress{{Email: toEmail}}},
		},
		From:    sgAddress{Email: c.fromEmail, Name: c.fromName},
		Subject: subject,
		Content: []sgContent{{Type: "text/html", Value: htmlBody}},
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("sendgrid marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.sendgrid.com/v3/mail/send", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("sendgrid request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sendgrid send: %w", err)
	}
	defer resp.Body.Close()

	// SendGrid returns 202 on success
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sendgrid status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// Enabled returns true if the SendGrid client has valid credentials.
func (c *SendGridClient) Enabled() bool {
	return c.apiKey != ""
}
