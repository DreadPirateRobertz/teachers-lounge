// Package taunt generates contextual boss taunts via the LiteLLM AI gateway.
// Taunts are triggered when a student answers incorrectly during a boss battle.
package taunt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Generator generates a contextual boss taunt for a wrong answer.
// Implementations must be safe for concurrent use.
type Generator interface {
	// Generate returns a short taunt string for the given boss and round.
	// bossID is the machine ID (e.g. "algebra_dragon"), bossName is the display
	// name (e.g. "Algebra Dragon"), topic is the subject area, and round is the
	// current turn number (1-indexed).
	Generate(ctx context.Context, bossID, bossName, topic string, round int) (string, error)
}

// LiteLLMGenerator calls the LiteLLM AI gateway (OpenAI-compatible) to
// produce contextual taunts using Claude Haiku.
type LiteLLMGenerator struct {
	gatewayURL string
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewLiteLLMGenerator returns a LiteLLMGenerator targeting the given gateway.
// Functional options (WithModel, WithHTTPClient) override defaults.
func NewLiteLLMGenerator(gatewayURL, apiKey string, opts ...func(*LiteLLMGenerator)) *LiteLLMGenerator {
	g := &LiteLLMGenerator{
		gatewayURL: gatewayURL,
		apiKey:     apiKey,
		model:      "claude-haiku-4-5-20251001",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	for _, o := range opts {
		o(g)
	}
	return g
}

// WithModel overrides the default Claude Haiku model identifier.
func WithModel(model string) func(*LiteLLMGenerator) {
	return func(g *LiteLLMGenerator) { g.model = model }
}

// WithHTTPClient replaces the generator's HTTP client. Primarily used in tests
// to point at an httptest.Server.
func WithHTTPClient(c *http.Client) func(*LiteLLMGenerator) {
	return func(g *LiteLLMGenerator) { g.httpClient = c }
}

// Generate calls the AI gateway to produce a contextual taunt.
// The prompt instructs the model to stay in-character as the boss villain,
// reacting dramatically to the wrong answer while hinting at the correct topic.
func (g *LiteLLMGenerator) Generate(ctx context.Context, _, bossName, topic string, round int) (string, error) {
	prompt := fmt.Sprintf(
		"You are %s, a villain boss in an educational quiz game. "+
			"A student just answered a %s question incorrectly on round %d of our battle. "+
			"Give a short, dramatic villain taunt (1–2 sentences). "+
			"Be menacing but subtly educational — allude to why their answer was wrong. "+
			"No markdown. Output only the taunt text.",
		bossName, topic, round,
	)

	payload := map[string]any{
		"model": g.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":  80,
		"temperature": 0.9,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("taunt: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.gatewayURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("taunt: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("taunt: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("taunt: gateway returned %d", resp.StatusCode)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("taunt: decode response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("taunt: no choices in response")
	}
	return result.Choices[0].Message.Content, nil
}

// StaticGenerator always returns the same pre-configured taunt string.
// Used in tests and as a fallback when the AI gateway is not configured.
type StaticGenerator struct {
	Taunt string
}

// Generate returns the pre-configured static taunt, ignoring all parameters.
func (s StaticGenerator) Generate(_ context.Context, _, _, _ string, _ int) (string, error) {
	return s.Taunt, nil
}
