package taunt_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/teacherslounge/gaming-service/internal/taunt"
)

// completionsResponse builds a minimal OpenAI-compatible chat completions body.
func completionsResponse(content string) string {
	b, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{
			{"message": map[string]string{"content": content}},
		},
	})
	return string(b)
}

// ── LiteLLMGenerator tests ────────────────────────────────────────────────────

func TestLiteLLMGenerator_ReturnsGeneratedTaunt(t *testing.T) {
	want := "Your algebra is as broken as your confidence!"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, completionsResponse(want))
	}))
	defer srv.Close()

	g := taunt.NewLiteLLMGenerator(srv.URL, "test-key",
		taunt.WithHTTPClient(srv.Client()),
	)
	got, err := g.Generate(context.Background(), "algebra_dragon", "Algebra Dragon", "algebra", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLiteLLMGenerator_PromptContainsBossNameAndRound(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, completionsResponse("taunt"))
	}))
	defer srv.Close()

	g := taunt.NewLiteLLMGenerator(srv.URL, "test-key",
		taunt.WithHTTPClient(srv.Client()),
	)
	_, _ = g.Generate(context.Background(), "grammar_golem", "Grammar Golem", "grammar", 5)

	body := string(capturedBody)
	if !strings.Contains(body, "Grammar Golem") {
		t.Errorf("prompt missing boss name: %s", body)
	}
	if !strings.Contains(body, "5") {
		t.Errorf("prompt missing round number: %s", body)
	}
}

func TestLiteLLMGenerator_SetsAuthorizationHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, completionsResponse("taunt"))
	}))
	defer srv.Close()

	g := taunt.NewLiteLLMGenerator(srv.URL, "secret-key",
		taunt.WithHTTPClient(srv.Client()),
	)
	_, _ = g.Generate(context.Background(), "algebra_dragon", "Algebra Dragon", "algebra", 1)

	if gotAuth != "Bearer secret-key" {
		t.Errorf("got Authorization %q, want %q", gotAuth, "Bearer secret-key")
	}
}

func TestLiteLLMGenerator_GatewayError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	g := taunt.NewLiteLLMGenerator(srv.URL, "test-key",
		taunt.WithHTTPClient(srv.Client()),
	)
	_, err := g.Generate(context.Background(), "algebra_dragon", "Algebra Dragon", "algebra", 1)
	if err == nil {
		t.Fatal("expected error on 500 response, got nil")
	}
}

func TestLiteLLMGenerator_EmptyChoices_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[]}`)
	}))
	defer srv.Close()

	g := taunt.NewLiteLLMGenerator(srv.URL, "test-key",
		taunt.WithHTTPClient(srv.Client()),
	)
	_, err := g.Generate(context.Background(), "algebra_dragon", "Algebra Dragon", "algebra", 2)
	if err == nil {
		t.Fatal("expected error on empty choices, got nil")
	}
}

// ── StaticGenerator tests ─────────────────────────────────────────────────────

func TestStaticGenerator_ReturnsConfiguredTaunt(t *testing.T) {
	want := "You shall not pass this exam!"
	g := taunt.StaticGenerator{Taunt: want}
	got, err := g.Generate(context.Background(), "any_boss", "Any Boss", "any_topic", 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestStaticGenerator_ZeroValue_ReturnsEmpty(t *testing.T) {
	var g taunt.StaticGenerator
	got, err := g.Generate(context.Background(), "", "", "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}
