package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const openRouterAPIURL = "https://openrouter.ai/api/v1/chat/completions"

// OpenRouterClient satisfies Completer against OpenRouter's
// OpenAI-compatible endpoint. Useful for testing against free-tier
// models without touching Anthropic API credit.
type OpenRouterClient struct {
	apiKey string
	http   *http.Client
}

var _ Completer = (*OpenRouterClient)(nil)

func NewOpenRouterClient(apiKey string) (*OpenRouterClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("llm: openrouter apiKey is required")
	}
	return &OpenRouterClient{apiKey: apiKey, http: &http.Client{}}, nil
}

// OpenAI-compatible chat completion shape — different from Anthropic's
// own shape (system is a separate top-level field there; here it's
// just another message with role "system").
type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterRequest struct {
	Model     string              `json:"model"`
	MaxTokens int                 `json:"max_tokens"`
	Messages  []openRouterMessage `json:"messages"`
}

type openRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *OpenRouterClient) Complete(model, system, userMsg string, maxTokens int) (string, error) {
	if maxTokens == 0 {
		maxTokens = 1024
	}

	var messages []openRouterMessage
	if system != "" {
		messages = append(messages, openRouterMessage{Role: "system", Content: system})
	}
	messages = append(messages, openRouterMessage{Role: "user", Content: userMsg})

	req := openRouterRequest{
		Model:     model,
		MaxTokens: maxTokens,
		Messages:  messages,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", openRouterAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	var r openRouterResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", fmt.Errorf("unmarshal response: %w (raw: %s)", err, raw)
	}
	if r.Error != nil {
		return "", fmt.Errorf("api error: %s", r.Error.Message)
	}
	if len(r.Choices) == 0 {
		return "", fmt.Errorf("empty response")
	}
	return r.Choices[0].Message.Content, nil
}
