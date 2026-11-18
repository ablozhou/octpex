package brain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// FrontierClientConfig defines configuration for frontier LLMs (Claude, GPT, Gemini, Qwen, DeepSeek).
type FrontierClientConfig struct {
	BaseURL    string        `json:"base_url"` // e.g. "https://api.openai.com/v1" or compatible endpoint
	APIKey     string        `json:"api_key"`
	Model      string        `json:"model"`    // e.g. "claude", "gpt", "gemini", "qwen", "deepseek"
	Timeout    time.Duration `json:"timeout"`
	HTTPClient *http.Client  `json:"-"`
}

// ChatMessage represents a single message in a chat completion conversation.
type ChatMessage struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// ChatCompletionRequest is the standard OpenAI-compatible API request.
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// ChatCompletionResponse is the standard OpenAI-compatible API response.
type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// FrontierClient interacts with cloud frontier models for central arbitration and code synthesis.
type FrontierClient struct {
	config FrontierClientConfig
	client *http.Client
}

// NewFrontierClient creates a new frontier API client.
func NewFrontierClient(cfg FrontierClientConfig) *FrontierClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: cfg.Timeout}
	}
	return &FrontierClient{
		config: cfg,
		client: client,
	}
}

// Complete executes a chat completion request to the frontier model.
func (c *FrontierClient) Complete(ctx context.Context, systemPrompt, userPrompt string, temperature float64) (string, int, error) {
	reqBody := ChatCompletionRequest{
		Model: c.config.Model,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: temperature,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal chat completion request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", c.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.APIKey))
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("frontier model request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("frontier model returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", 0, fmt.Errorf("failed to decode chat completion response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", 0, fmt.Errorf("frontier model returned no completion choices")
	}

	return chatResp.Choices[0].Message.Content, chatResp.Usage.TotalTokens, nil
}
