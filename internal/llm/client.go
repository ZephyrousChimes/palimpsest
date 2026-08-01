package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	ModelHaiku  = "claude-haiku-4-5-20251001"
	ModelSonnet = "claude-sonnet-4-6"

	apiURL = "https://api.anthropic.com/v1/messages"
)

type Client struct {
	apiKey string
	http   *http.Client
}

// NewClient takes the API key explicitly — it never reaches into the
// environment itself. Whoever constructs a Client is responsible for
// having already loaded config (see internal/config). This keeps
// every dependency visible in the call signature instead of hidden
// inside the package.
func NewClient(apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("llm: apiKey is required")
	}
	return &Client{apiKey: apiKey, http: &http.Client{}}, nil
}

// ... rest of the file (message, request, response types, Complete method)
// is unchanged from before

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []message `json:"messages"`
}

type response struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) Complete(model, system, userMsg string, maxTokens int) (string, error) {
	if maxTokens == 0 {
		maxTokens = 1024
	}

	req := request{
		Model:     model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  []message{{Role: "user", Content: userMsg}},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	var r response
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if r.Error != nil {
		return "", fmt.Errorf("api error: %s", r.Error.Message)
	}
	if len(r.Content) == 0 {
		return "", fmt.Errorf("empty response")
	}
	return r.Content[0].Text, nil
}
