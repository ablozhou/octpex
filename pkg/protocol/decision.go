package protocol

import (
	"context"
)

// DecisionQuestion defines a single question evaluated by the non-autoregressive System-1 engine.
type DecisionQuestion struct {
	QID     string   `json:"qid"`
	QType   string   `json:"qtype"`   // "choice" | "noul" | "score"
	Prompt  string   `json:"prompt"`
	Options []string `json:"options"` // Up to 255 candidate options for "choice"
}

// DecisionResult represents the single-pass inference readout for a question.
type DecisionResult struct {
	QID           string             `json:"qid"`
	Probabilities map[string]float64 `json:"probabilities"`
	TopOption     string             `json:"top_option"`
	Confidence    float64            `json:"confidence"`
	ScoreValue    float64            `json:"score_value,omitempty"`
	EngineUsed    string             `json:"engine_used"` // e.g. "laya-onnx-int8" or "karpo-decision-2b"
	LatencyMs     int64              `json:"latency_ms"`
}

// DecisionEngineSPI defines the non-autoregressive System-1 Synaptic Engine contract.
// Executes single-forward-pass evaluations over typed questions with 0 generative tokens.
type DecisionEngineSPI interface {
	// Decide evaluates a batch of typed questions in a single forward pass.
	Decide(ctx context.Context, stateText string, questions []DecisionQuestion, images [][]byte) ([]DecisionResult, error)

	// ReverseOptionCheck runs a FlexDM calibration pass when confidence is in the uncertainty band (0.50 <= p < 0.65).
	// Returns the primary result and whether the reverse-order distribution agrees.
	ReverseOptionCheck(ctx context.Context, stateText string, question DecisionQuestion) (primary *DecisionResult, isConsistent bool, err error)

	// ActiveProfile reports the detected hardware tier and active engine descriptor.
	ActiveProfile() string
}
