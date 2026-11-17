package adapter

import (
	"context"
	"fmt"
	"octpex/pkg/gates"
	"octpex/pkg/protocol"
	"octpex/pkg/substrate"
	"sync/atomic"
	"time"
)

// InferenceRequest defines the input to a local SLM.
type InferenceRequest struct {
	Prompt      string   `json:"prompt"`
	System      string   `json:"system,omitempty"`
	Critiques   []string `json:"critiques,omitempty"`
	Temperature float32  `json:"temperature"`
}

// InferenceResponse contains the text output and mathematical token logprobs.
type InferenceResponse struct {
	Content      string  `json:"content"`
	AvgLogprob   float32 `json:"avg_logprob"`
	PromptTokens int     `json:"prompt_tokens"`
	OutputTokens int     `json:"output_tokens"`
}

// SLMInferenceSPI abstracts the local inference engine (e.g. Ollama, llama.cpp, vLLM, mock).
type SLMInferenceSPI interface {
	Generate(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error)
	ContextCapacity() int
}

// SLMGanglionAdapter implements a Level 1 Edge Reflex Arm.
// It executes the biomimetic reflex loop: Act -> Inspect (Gate 2) -> Retry with Critique -> Gate 3 -> Resolve or Escalate.
type SLMGanglionAdapter struct {
	manifest       *protocol.ArmManifest
	substrate      substrate.DataSubstrateSPI
	inference      SLMInferenceSPI
	verifier       gates.VerifierFunc
	activeQueue    int64
	completedTasks int64
	escalatedTasks int64
}

// NewSLMGanglionAdapter creates an initialized Level 1 SLM Arm.
func NewSLMGanglionAdapter(sub substrate.DataSubstrateSPI, engine SLMInferenceSPI, verifier gates.VerifierFunc) *SLMGanglionAdapter {
	if verifier == nil {
		verifier = gates.JSONSchemaVerifier
	}
	return &SLMGanglionAdapter{
		substrate: sub,
		inference: engine,
		verifier:  verifier,
	}
}

func (a *SLMGanglionAdapter) Init(ctx context.Context, manifest *protocol.ArmManifest) error {
	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("slm_adapter: invalid manifest: %w", err)
	}
	a.manifest = manifest
	return nil
}

func (a *SLMGanglionAdapter) ExecuteStep(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
	atomic.AddInt64(&a.activeQueue, 1)
	defer atomic.AddInt64(&a.activeQueue, -1)

	// 1. Gate 1: Pre-Flight Check (Capacity & Policy)
	promptTokens := 256 // baseline estimate
	passed, reason := gates.PreFlightCheck(a.manifest, env, 0.90, promptTokens)
	if !passed {
		atomic.AddInt64(&a.escalatedTasks, 1)
		return &protocol.StepResult{
			Status:        "ESCALATED",
			OutputMessage: fmt.Sprintf("gate 1 preflight escalation: %s", reason),
		}, nil
	}

	// 2. Fetch input data from Shared Data Substrate if present
	var inputPayload string
	if len(env.SubstrateRefs) > 0 {
		data, _, err := a.substrate.GetArtifact(ctx, env.SubstrateRefs[0].URI)
		if err != nil {
			return nil, fmt.Errorf("slm_adapter: failed to read input ref: %w", err)
		}
		inputPayload = string(data)
	} else if p, ok := env.Parameters["prompt"].(string); ok {
		inputPayload = p
	}

	// 3. Local Reflex Loop: Act -> Inspect (Gate 2) -> Retry
	maxRetries := a.manifest.EscalationGates.MaxReflexRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	critiques := make([]string, 0)
	var finalResp *InferenceResponse

	for attempt := 1; attempt <= maxRetries; attempt++ {
		req := &InferenceRequest{
			Prompt:      inputPayload,
			Critiques:   critiques,
			Temperature: a.manifest.SLMProfile.Temperature,
		}

		resp, err := a.inference.Generate(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("slm_adapter: inference engine error: %w", err)
		}
		finalResp = resp

		// Gate 2: Deterministic Verifier (e.g. JSON schema / AST syntax)
		vErr := a.verifier(ctx, []byte(resp.Content))
		if vErr == nil {
			// Gate 2 Passed! Proceed to Gate 3
			break
		}

		// Gate 2 Failed: Inject critique and retry
		critique := fmt.Sprintf("attempt %d validation error: %v", attempt, vErr)
		critiques = append(critiques, critique)

		if attempt == maxRetries {
			// Retries exhausted -> Escalate to Octo-Brain
			atomic.AddInt64(&a.escalatedTasks, 1)
			return &protocol.StepResult{
				Status:        "ESCALATED",
				OutputMessage: fmt.Sprintf("gate 2 verifier failed after %d attempts: %v", maxRetries, vErr),
			}, nil
		}
	}

	// 4. Gate 3: Mathematical Logprob & Behavioral Check
	g3Passed, g3Reason := gates.BehavioralCheck(a.manifest, env, finalResp.AvgLogprob)
	if !g3Passed {
		atomic.AddInt64(&a.escalatedTasks, 1)
		return &protocol.StepResult{
			Status:        "ESCALATED",
			OutputMessage: fmt.Sprintf("gate 3 behavioral escalation: %s", g3Reason),
		}, nil
	}

	// 5. Success! Write output artifact to Shared Data Substrate
	outURI := fmt.Sprintf("sub://%s/slm-result-%d", env.SessionID, time.Now().UnixNano())
	outData := []byte(finalResp.Content)
	ver, err := a.substrate.PutArtifact(ctx, outURI, 0, outData, time.Minute)
	if err != nil {
		return nil, fmt.Errorf("slm_adapter: failed to persist output artifact: %w", err)
	}

	atomic.AddInt64(&a.completedTasks, 1)
	return &protocol.StepResult{
		Status:        "SUCCESS",
		OutputMessage: "SLM reflex loop passed all 3 gates",
		SubstrateRefs: []protocol.SubstrateRef{
			{
				URI:         outURI,
				Version:     ver,
				ContentType: "application/json",
				ByteSize:    int64(len(outData)),
			},
		},
	}, nil
}

func (a *SLMGanglionAdapter) Telemetry(ctx context.Context) (*protocol.ArmTelemetry, error) {
	return &protocol.ArmTelemetry{
		ActiveQueueDepth:  int(atomic.LoadInt64(&a.activeQueue)),
		Status:            "HEALTHY",
		LastHeartbeatUnix: time.Now().Unix(),
		CompletedTasks:    atomic.LoadInt64(&a.completedTasks),
		EscalatedTasks:    atomic.LoadInt64(&a.escalatedTasks),
	}, nil
}
