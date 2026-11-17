package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOllamaClient_Generate(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			http.NotFound(w, r)
			return
		}
		var req OllamaGenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Model != "qwen2.5-coder:7b" {
			http.Error(w, "invalid model", http.StatusBadRequest)
			return
		}

		resp := OllamaGenerateResponse{
			Model:     req.Model,
			CreatedAt: time.Now(),
			Response:  `{"status":"ok","latency_ms":12}`,
			Done:      true,
			EvalCount: 42,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	client := NewOllamaClient(OllamaClientConfig{
		BaseURL: mockServer.URL,
		Model:   "qwen2.5-coder:7b",
	})

	ctx := context.Background()
	resp, err := client.Generate(ctx, "Analyze code", "You are an octopus arm", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Response != `{"status":"ok","latency_ms":12}` {
		t.Fatalf("unexpected response content: %s", resp.Response)
	}
	if resp.EvalCount != 42 {
		t.Fatalf("expected 42 tokens, got %d", resp.EvalCount)
	}
}

func TestOllamaClient_ErrorHandling(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not found", http.StatusNotFound)
	}))
	defer mockServer.Close()

	client := NewOllamaClient(OllamaClientConfig{
		BaseURL: mockServer.URL,
		Model:   "unknown-model",
	})

	ctx := context.Background()
	_, err := client.Generate(ctx, "test", "", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
