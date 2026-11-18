package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLayaAdapter_DecideChoiceAndNoul(t *testing.T) {
	laya := NewLayaAdapter()
	ctx := context.Background()

	questions := []DecisionQuestion{
		{
			QID:     "q1",
			QType:   "choice",
			Prompt:  "Which action should be taken?",
			Options: []string{"scale_replicas", "restart_pod", "ignore_alert"},
		},
		{
			QID:    "q2",
			QType:  "noul",
			Prompt: "Is the operation safe to execute?",
		},
		{
			QID:    "q3",
			QType:  "score",
			Prompt: "Rate the severity of the incident",
		},
	}

	stateText := "CRITICAL alert: memory limit exceeded, scale_replicas recommended immediately. Validation check failed."
	results, err := laya.Decide(ctx, stateText, questions, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Q1: choice -> scale_replicas
	if results[0].TopOption != "scale_replicas" {
		t.Fatalf("expected scale_replicas, got %s", results[0].TopOption)
	}
	if results[0].Confidence <= 0.0 {
		t.Fatalf("expected positive confidence, got %f", results[0].Confidence)
	}

	// Q2: noul -> contains "failed" -> should be "no"
	if results[1].TopOption != "no" {
		t.Fatalf("expected 'no', got %s", results[1].TopOption)
	}

	// Q3: score
	if results[2].ScoreValue <= 0.0 {
		t.Fatalf("expected valid score value, got %f", results[2].ScoreValue)
	}
}

func TestLayaAdapter_ReverseOptionCheck(t *testing.T) {
	laya := NewLayaAdapter()
	ctx := context.Background()

	q := DecisionQuestion{
		QID:     "q_calib",
		QType:   "choice",
		Prompt:  "Select cluster namespace",
		Options: []string{"staging", "production"},
	}

	// With explicit clear signal, high confidence > 0.65 -> consistent
	stateText := "Target cluster is definitely production environment with live traffic."
	res, consistent, err := laya.ReverseOptionCheck(ctx, stateText, q)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !consistent {
		t.Fatalf("expected consistent decision on clear signal")
	}
	if res.TopOption != "production" {
		t.Fatalf("expected production, got %s", res.TopOption)
	}
}

func TestKarpoAdapter_HTTPDecide(t *testing.T) {
	// Mock HTTP server responding with valid Karpo output
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := karpoResponse{
			Results: []DecisionResult{
				{
					QID:        "q1",
					TopOption:  "dispatch_to_arm_2",
					Confidence: 0.94,
					Probabilities: map[string]float64{
						"dispatch_to_arm_2": 0.94,
						"escalate_brain":    0.06,
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	karpo := NewKarpoAdapter(srv.URL)
	ctx := context.Background()

	results, err := karpo.Decide(ctx, "sample state", []DecisionQuestion{
		{
			QID:     "q1",
			QType:   "choice",
			Options: []string{"dispatch_to_arm_2", "escalate_brain"},
		},
	}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].TopOption != "dispatch_to_arm_2" {
		t.Fatalf("expected dispatch_to_arm_2, got %s", results[0].TopOption)
	}
	if results[0].EngineUsed != "karpo-decision-2b" {
		t.Fatalf("expected engine karpo-decision-2b, got %s", results[0].EngineUsed)
	}
}
