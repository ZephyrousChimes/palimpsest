package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const anthropicAPIURL = "https://api.anthropic.com/v1/messages"

// AnthropicClient satisfies Completer against Anthropic's native API.
type AnthropicClient struct {
	apiKey string
	http   *http.Client
}

// compile-time check: AnthropicClient must satisfy Completer.
var _ Completer = (*AnthropicClient)(nil)

func NewAnthropicClient(apiKey string) (*AnthropicClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("llm: anthropic apiKey is required")
	}
	return &AnthropicClient{apiKey: apiKey, http: &http.Client{}}, nil
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *AnthropicClient) Complete(model, system, userMsg string, maxTokens int) (string, error) {
	if maxTokens == 0 {
		maxTokens = 1024
	}

	req := anthropicRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  []anthropicMessage{{Role: "user", Content: userMsg}},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", anthropicAPIURL, bytes.NewReader(body))
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

	var r anthropicResponse
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
