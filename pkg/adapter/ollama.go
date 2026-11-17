package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaClientConfig defines connection parameters for local Ollama daemon.
type OllamaClientConfig struct {
	BaseURL    string        `json:"base_url"`    // Default: "http://localhost:11434"
	Model      string        `json:"model"`       // e.g. "qwen2.5-coder:7b" or "phi-4"
	Timeout    time.Duration `json:"timeout"`     // Request timeout
	HTTPClient *http.Client  `json:"-"`           // Optional custom HTTP client (for testing/mocking)
}

// OllamaGenerateRequest represents the JSON request sent to /api/generate.
type OllamaGenerateRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	System  string                 `json:"system,omitempty"`
	Stream  bool                   `json:"stream"`
	Format  string                 `json:"format,omitempty"` // "json" or empty
	Options map[string]interface{} `json:"options,omitempty"`
}

// OllamaGenerateResponse represents the JSON response returned by /api/generate.
type OllamaGenerateResponse struct {
	Model              string    `json:"model"`
	CreatedAt          time.Time `json:"created_at"`
	Response           string    `json:"response"`
	Done               bool      `json:"done"`
	TotalDuration      int64     `json:"total_duration"`
	PromptEvalCount    int       `json:"prompt_eval_count"`
	EvalCount          int       `json:"eval_count"`
	Context            []int     `json:"context,omitempty"`
}

// OllamaClient provides structured execution for local edge SLMs running on Ollama.
type OllamaClient struct {
	config OllamaClientConfig
	client *http.Client
}

// NewOllamaClient creates a new Ollama API client.
func NewOllamaClient(cfg OllamaClientConfig) *OllamaClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:11434"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: cfg.Timeout}
	}
	return &OllamaClient{
		config: cfg,
		client: client,
	}
}

// Generate invokes the Ollama generation endpoint.
func (c *OllamaClient) Generate(ctx context.Context, prompt, systemPrompt string, jsonMode bool) (*OllamaGenerateResponse, error) {
	reqBody := OllamaGenerateRequest{
		Model:   c.config.Model,
		Prompt:  prompt,
		System:  systemPrompt,
		Stream:  false,
		Options: map[string]interface{}{
			"temperature": 0.1,
		},
	}
	if jsonMode {
		reqBody.Format = "json"
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ollama request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", c.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama generation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var genResp OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return nil, fmt.Errorf("failed to decode ollama response: %w", err)
	}

	return &genResp, nil
}
