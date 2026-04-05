package push_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/push"
)

// ── FCMPusher ────────────────────────────────────────────────────────────────

func TestFCMPusher_Send_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "key=test-key" {
			t.Errorf("Authorization: want %q, got %q", "key=test-key", got)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json content-type")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["to"] != "device-tok" {
			t.Errorf("to: want device-tok, got %v", body["to"])
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": 1, "failure": 0,
			"results": []map[string]string{{"message_id": "msg1"}},
		})
	}))
	defer srv.Close()

	p := push.NewFCMPusher("test-key", push.WithEndpoint(srv.URL))
	if err := p.Send(context.Background(), "device-tok", "Hello", "World", nil); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestFCMPusher_Send_IncludesDataWhenNonEmpty(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": 1, "failure": 0,
			"results": []map[string]string{{"message_id": "x"}},
		})
	}))
	defer srv.Close()

	p := push.NewFCMPusher("key", push.WithEndpoint(srv.URL))
	if err := p.Send(context.Background(), "tok", "Title", "Body",
		map[string]any{"rival": "molemaster"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got["data"] == nil {
		t.Error("expected data field in FCM payload")
	}
}

func TestFCMPusher_Send_OmitsDataWhenNil(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": 1, "failure": 0,
			"results": []map[string]string{{"message_id": "x"}},
		})
	}))
	defer srv.Close()

	p := push.NewFCMPusher("key", push.WithEndpoint(srv.URL))
	if err := p.Send(context.Background(), "tok", "T", "B", nil); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if _, present := got["data"]; present {
		t.Error("data field should be absent when no data is provided")
	}
}

func TestFCMPusher_Send_FCMDeliveryFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": 0, "failure": 1,
			"results": []map[string]string{{"error": "InvalidRegistration"}},
		})
	}))
	defer srv.Close()

	p := push.NewFCMPusher("key", push.WithEndpoint(srv.URL))
	err := p.Send(context.Background(), "bad-token", "Hi", "Bye", nil)
	if err == nil {
		t.Fatal("expected error for FCM delivery failure, got nil")
	}
}

func TestFCMPusher_Send_HTTPUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := push.NewFCMPusher("wrong-key", push.WithEndpoint(srv.URL))
	err := p.Send(context.Background(), "tok", "Hi", "Bye", nil)
	if err == nil {
		t.Fatal("expected error for HTTP 401, got nil")
	}
}

// ── LogPusher ────────────────────────────────────────────────────────────────

func TestLogPusher_Send_AlwaysSucceeds(t *testing.T) {
	var p push.LogPusher
	err := p.Send(context.Background(), "any-token", "title", "body",
		map[string]any{"k": "v"})
	if err != nil {
		t.Fatalf("LogPusher.Send: %v", err)
	}
}

func TestLogPusher_Send_NilDataSucceeds(t *testing.T) {
	var p push.LogPusher
	if err := p.Send(context.Background(), "tok", "t", "b", nil); err != nil {
		t.Fatalf("LogPusher.Send nil data: %v", err)
	}
}
