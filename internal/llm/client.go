// Package llm is a small OpenAI-compatible chat client.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrNoKey means the client has no API key.
var ErrNoKey = errors.New("llm api key is empty")

// Client calls an OpenAI-compatible chat completions endpoint.
type Client struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

// New returns a client with sensible defaults.
func New(baseURL, apiKey, model string) *Client {
	if baseURL == "" {
		baseURL = "https://routerai.ru/api/v1"
	}
	if model == "" {
		model = "deepseek/deepseek-v4.1-flash"
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		HTTP:    &http.Client{Timeout: 20 * time.Second},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// Complete sends one system and one user message and returns the answer.
func (c *Client) Complete(ctx context.Context, system, user string) (string, error) {
	if c == nil || c.APIKey == "" {
		return "", ErrNoKey
	}

	body, err := json.Marshal(chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0,
		MaxTokens:   600,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("llm status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out chatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("llm decode: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", errors.New("llm returned no choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}
