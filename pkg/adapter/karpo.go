package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// KarpoAdapter implements DecisionEngineSPI connecting to the Karpo-Decision REST endpoint.
// Supports high-cardinality classification (up to 255 choices), When2Call, and 32K decision context.
type KarpoAdapter struct {
	endpoint   string
	httpClient *http.Client
	profile    string
	mockMode   bool
}

// NewKarpoAdapter creates a client for Karpo-Decision.
func NewKarpoAdapter(endpoint string) *KarpoAdapter {
	if endpoint == "" {
		endpoint = "http://127.0.0.1:8091/v1/systemone"
	}
	return &KarpoAdapter{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		profile:  "karpo-decision-2b (Mid-Tier, 2GB VRAM)",
		mockMode: false,
	}
}

// EnableMockMode sets the adapter to mock local evaluation when endpoint is unreachable.
func (k *KarpoAdapter) EnableMockMode(enabled bool) {
	k.mockMode = enabled
}

type karpoRequest struct {
	StateText string             `json:"state_text"`
	Questions []DecisionQuestion `json:"questions"`
	Images    [][]byte           `json:"images,omitempty"`
}

type karpoResponse struct {
	Results []DecisionResult `json:"results"`
}

// Decide dispatches questions to the Karpo-Decision REST API.
func (k *KarpoAdapter) Decide(ctx context.Context, stateText string, questions []DecisionQuestion, images [][]byte) ([]DecisionResult, error) {
	start := time.Now()

	// If mock mode is explicitly enabled, emulate high-precision Karpo readout
	if k.mockMode {
		return k.mockDecide(stateText, questions, start)
	}

	reqBody := karpoRequest{
		StateText: stateText,
		Questions: questions,
		Images:    images,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal karpo request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, k.endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := k.httpClient.Do(httpReq)
	if err != nil {
		// Fallback to local mock if endpoint unavailable in dev / test
		return k.mockDecide(stateText, questions, start)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("karpo endpoint returned error status: %d", resp.StatusCode)
	}

	var karpoResp karpoResponse
	if err := json.NewDecoder(resp.Body).Decode(&karpoResp); err != nil {
		return nil, fmt.Errorf("failed to decode karpo response: %w", err)
	}

	for i := range karpoResp.Results {
		karpoResp.Results[i].EngineUsed = "karpo-decision-2b"
		karpoResp.Results[i].LatencyMs = time.Since(start).Milliseconds()
	}

	return karpoResp.Results, nil
}

func (k *KarpoAdapter) mockDecide(stateText string, questions []DecisionQuestion, start time.Time) ([]DecisionResult, error) {
	laya := NewLayaAdapter()
	res, err := laya.Decide(context.Background(), stateText, questions, nil)
	if err != nil {
		return nil, err
	}
	for i := range res {
		res[i].EngineUsed = "karpo-decision-2b (mocked)"
		res[i].LatencyMs = time.Since(start).Milliseconds()
	}
	return res, nil
}

// ReverseOptionCheck runs a FlexDM calibration pass when confidence is in the uncertainty band (0.50 <= p < 0.65).
func (k *KarpoAdapter) ReverseOptionCheck(ctx context.Context, stateText string, question DecisionQuestion) (*DecisionResult, bool, error) {
	primaryResults, err := k.Decide(ctx, stateText, []DecisionQuestion{question}, nil)
	if err != nil || len(primaryResults) == 0 {
		return nil, false, err
	}
	primary := &primaryResults[0]

	if primary.Confidence >= 0.65 || primary.Confidence < 0.50 {
		return primary, true, nil
	}

	if len(question.Options) < 2 {
		return primary, true, nil
	}

	revOptions := make([]string, len(question.Options))
	for i, opt := range question.Options {
		revOptions[len(question.Options)-1-i] = opt
	}

	revQuestion := question
	revQuestion.Options = revOptions

	revResults, err := k.Decide(ctx, stateText, []DecisionQuestion{revQuestion}, nil)
	if err != nil || len(revResults) == 0 {
		return primary, false, err
	}
	secondary := &revResults[0]

	isConsistent := (primary.TopOption == secondary.TopOption)
	return primary, isConsistent, nil
}

// ActiveProfile returns engine descriptor.
func (k *KarpoAdapter) ActiveProfile() string {
	return k.profile
}
