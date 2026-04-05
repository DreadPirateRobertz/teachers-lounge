package email_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/email"
)

// ── SendGridSender tests ──────────────────────────────────────────────────────

func TestSendGridSender_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted) // SendGrid returns 202
	}))
	defer srv.Close()

	s := email.NewSendGridSender("test-key", "from@example.com",
		email.WithEndpoint(srv.URL),
		email.WithHTTPClient(srv.Client()),
	)
	err := s.Send(context.Background(), "to@example.com", "Subject", "<p>Hello</p>")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestSendGridSender_SetsAuthorizationHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	s := email.NewSendGridSender("my-api-key", "from@example.com",
		email.WithEndpoint(srv.URL),
		email.WithHTTPClient(srv.Client()),
	)
	_ = s.Send(context.Background(), "to@example.com", "Hi", "<p>body</p>")

	if gotAuth != "Bearer my-api-key" {
		t.Fatalf("expected Authorization %q, got %q", "Bearer my-api-key", gotAuth)
	}
}

func TestSendGridSender_RequestBodyContainsToAndSubject(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	s := email.NewSendGridSender("key", "sender@tl.dev",
		email.WithEndpoint(srv.URL),
		email.WithHTTPClient(srv.Client()),
	)
	_ = s.Send(context.Background(), "student@example.com", "Your streak is at risk!", "<p>Study now</p>")

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("body not valid JSON: %v — body: %s", err, body)
	}
	raw, _ := json.Marshal(payload)
	bodyStr := string(raw)
	if !strings.Contains(bodyStr, "student@example.com") {
		t.Errorf("to address missing from request body: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, "Your streak is at risk!") {
		t.Errorf("subject missing from request body: %s", bodyStr)
	}
}

func TestSendGridSender_Non2xxReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	s := email.NewSendGridSender("key", "from@example.com",
		email.WithEndpoint(srv.URL),
		email.WithHTTPClient(srv.Client()),
	)
	err := s.Send(context.Background(), "to@example.com", "Subject", "<p>body</p>")
	if err == nil {
		t.Fatal("expected error on 400 response, got nil")
	}
}

func TestSendGridSender_ServerError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := email.NewSendGridSender("key", "from@example.com",
		email.WithEndpoint(srv.URL),
		email.WithHTTPClient(srv.Client()),
	)
	err := s.Send(context.Background(), "to@example.com", "Subject", "<p>body</p>")
	if err == nil {
		t.Fatal("expected error on 500 response, got nil")
	}
}

// ── LogSender tests ───────────────────────────────────────────────────────────

func TestLogSender_ReturnsNil(t *testing.T) {
	s := email.LogSender{}
	err := s.Send(context.Background(), "to@example.com", "Subject", "<p>body</p>")
	if err != nil {
		t.Fatalf("LogSender should never error, got %v", err)
	}
}

func TestLogSender_ZeroValue_ReturnsNil(t *testing.T) {
	var s email.LogSender
	if err := s.Send(context.Background(), "", "", ""); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
