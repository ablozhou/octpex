package brain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFrontierClient_Complete(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-api-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		resp := ChatCompletionResponse{
			ID: "chatcmpl-12345",
		}
		resp.Choices = append(resp.Choices, struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			Index: 0,
			Message: struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{
				Role:    "assistant",
				Content: `{"task_dag":[{"task_id":"sub-1","assigned_arm":"arm-code"}]}`,
			},
			FinishReason: "stop",
		})
		resp.Usage.TotalTokens = 128

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	client := NewFrontierClient(FrontierClientConfig{
		BaseURL: mockServer.URL,
		APIKey:  "test-api-key",
		Model:   "gemini-1.5-pro",
	})

	ctx := context.Background()
	content, tokens, err := client.Complete(ctx, "You are Central Brain", "Plan DAG", 0.2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tokens != 128 {
		t.Fatalf("expected 128 tokens, got %d", tokens)
	}
	if content != `{"task_dag":[{"task_id":"sub-1","assigned_arm":"arm-code"}]}` {
		t.Fatalf("unexpected completion: %s", content)
	}
}
