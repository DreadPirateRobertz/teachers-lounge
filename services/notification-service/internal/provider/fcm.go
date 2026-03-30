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

// FCMClient sends push notifications via Firebase Cloud Messaging v1 HTTP API.
type FCMClient struct {
	serverKey  string
	projectID  string
	httpClient *http.Client
}

// NewFCMClient creates an FCM client. serverKey is the legacy server key;
// projectID is the Firebase project ID.
func NewFCMClient(serverKey, projectID string) *FCMClient {
	return &FCMClient{
		serverKey: serverKey,
		projectID: projectID,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// fcmMessage is the legacy FCM HTTP v1 request body.
type fcmMessage struct {
	To           string            `json:"to"`
	Notification *fcmNotification  `json:"notification"`
	Data         map[string]string `json:"data,omitempty"`
}

type fcmNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type fcmResponse struct {
	Success int `json:"success"`
	Failure int `json:"failure"`
}

// Send sends a push notification to the given device token.
func (c *FCMClient) Send(ctx context.Context, deviceToken, title, body string, data map[string]string) error {
	msg := fcmMessage{
		To:           deviceToken,
		Notification: &fcmNotification{Title: title, Body: body},
		Data:         data,
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("fcm marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://fcm.googleapis.com/fcm/send", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("fcm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "key="+c.serverKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fcm send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("fcm status %d: %s", resp.StatusCode, string(respBody))
	}

	var result fcmResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("fcm decode response: %w", err)
	}
	if result.Failure > 0 {
		return fmt.Errorf("fcm delivery failed for token")
	}
	return nil
}

// Enabled returns true if the FCM client has valid credentials.
func (c *FCMClient) Enabled() bool {
	return c.serverKey != ""
}
