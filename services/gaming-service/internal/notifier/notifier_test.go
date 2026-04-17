package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNoopNotifier_AllMethodsReturnNil(t *testing.T) {
	var n Notifier = NoopNotifier{}
	if err := n.NotifyLevelUp(context.Background(), "u1", 5); err != nil {
		t.Fatalf("NotifyLevelUp: %v", err)
	}
	if err := n.NotifyQuestComplete(context.Background(), "u1", "Daily Grind", 100); err != nil {
		t.Fatalf("NotifyQuestComplete: %v", err)
	}
}

func TestHTTPNotifier_NotifyLevelUp_PostsExpectedPayload(t *testing.T) {
	var (
		gotPath   string
		gotMethod string
		gotBody   map[string]any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	n := NewHTTP(srv.URL, WithHTTPClient(srv.Client()))
	if err := n.NotifyLevelUp(context.Background(), "u42", 7); err != nil {
		t.Fatalf("NotifyLevelUp: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/internal/push/level-up" {
		t.Errorf("path = %s, want /internal/push/level-up", gotPath)
	}
	if gotBody["user_id"] != "u42" {
		t.Errorf("user_id = %v, want u42", gotBody["user_id"])
	}
	// JSON numbers decode as float64
	if gotBody["new_level"].(float64) != 7 {
		t.Errorf("new_level = %v, want 7", gotBody["new_level"])
	}
}

func TestHTTPNotifier_NotifyQuestComplete_PostsExpectedPayload(t *testing.T) {
	var gotBody map[string]any
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	n := NewHTTP(srv.URL, WithHTTPClient(srv.Client()))
	if err := n.NotifyQuestComplete(context.Background(), "u9", "Daily Grind", 150); err != nil {
		t.Fatalf("NotifyQuestComplete: %v", err)
	}

	if gotPath != "/internal/push/quest-complete" {
		t.Errorf("path = %s, want /internal/push/quest-complete", gotPath)
	}
	if gotBody["user_id"] != "u9" {
		t.Errorf("user_id = %v, want u9", gotBody["user_id"])
	}
	if gotBody["quest_title"] != "Daily Grind" {
		t.Errorf("quest_title = %v, want 'Daily Grind'", gotBody["quest_title"])
	}
	if gotBody["xp_reward"].(float64) != 150 {
		t.Errorf("xp_reward = %v, want 150", gotBody["xp_reward"])
	}
}

func TestHTTPNotifier_Non2xx_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := NewHTTP(srv.URL, WithHTTPClient(srv.Client()))
	err := n.NotifyLevelUp(context.Background(), "u1", 2)
	if err == nil {
		t.Fatal("expected error on 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status code, got %v", err)
	}
}

func TestHTTPNotifier_NetworkError_ReturnsError(t *testing.T) {
	// Point at a closed server so Do() returns a connection error.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close() // closed — dialing will fail

	n := NewHTTP(srv.URL, WithHTTPClient(&http.Client{}))
	err := n.NotifyQuestComplete(context.Background(), "u1", "x", 1)
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
	if !strings.Contains(err.Error(), "notifier: http") {
		t.Errorf("error should be wrapped with notifier prefix, got %v", err)
	}
}

func TestHTTPNotifier_InvalidURL_ReturnsError(t *testing.T) {
	n := NewHTTP("://not-a-valid-url")
	err := n.NotifyLevelUp(context.Background(), "u1", 2)
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestHTTPNotifier_ContextCancelled_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call

	n := NewHTTP(srv.URL, WithHTTPClient(srv.Client()))
	err := n.NotifyLevelUp(ctx, "u1", 2)
	if err == nil {
		t.Fatal("expected context-cancelled error, got nil")
	}
}
