package adapter

import (
	"context"
	"fmt"
	"octpex/pkg/gates"
	"octpex/pkg/protocol"
	"octpex/pkg/substrate"
	"strings"
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
// Incorporates the System-1 Synaptic Engine (Laya/Karpo) for sub-15ms 0-token direct resolution.
type SLMGanglionAdapter struct {
	manifest       *protocol.ArmManifest
	substrate      substrate.DataSubstrateSPI
	inference      SLMInferenceSPI
	decisionEngine protocol.DecisionEngineSPI
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

// SetDecisionEngine registers a non-autoregressive System-1 decision engine.
func (a *SLMGanglionAdapter) SetDecisionEngine(engine protocol.DecisionEngineSPI) {
	a.decisionEngine = engine
}

// NewDecisionEngineFromProfile instantiates the appropriate DecisionEngineSPI based on manifest profile.
func NewDecisionEngineFromProfile(profile protocol.DecisionEngineProfile) protocol.DecisionEngineSPI {
	if strings.HasPrefix(profile.PrimaryEngine, "karpo") {
		endpoint := profile.Endpoint
		if endpoint == "" {
			endpoint = "http://127.0.0.1:8091/v1/systemone"
		}
		return NewKarpoAdapter(endpoint)
	}
	return NewLayaAdapter()
}

func (a *SLMGanglionAdapter) Init(ctx context.Context, manifest *protocol.ArmManifest) error {
	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("slm_adapter: invalid manifest: %w", err)
	}
	a.manifest = manifest
	if a.decisionEngine == nil && manifest.DecisionEngineProfile.PrimaryEngine != "" && manifest.DecisionEngineProfile.PrimaryEngine != "none" {
		a.decisionEngine = NewDecisionEngineFromProfile(manifest.DecisionEngineProfile)
	}
	return nil
}

func (a *SLMGanglionAdapter) ExecuteStep(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
	atomic.AddInt64(&a.activeQueue, 1)
	defer atomic.AddInt64(&a.activeQueue, -1)

	// 1. Gate 1: Cognitive Ladder Tier-1 Synaptic Pre-Flight & Discriminative Evaluation
	promptTokens := 256 // baseline estimate
	decisionQ := extractDecisionQuestion(env)

	ladderRes := gates.EvaluateTier1Synaptic(ctx, a.manifest, env, a.decisionEngine, decisionQ, 0.90, promptTokens)
	if ladderRes.Resolution == gates.ResolutionEscalatedToBrain {
		atomic.AddInt64(&a.escalatedTasks, 1)
		return &protocol.StepResult{
			Status:        "ESCALATED",
			OutputMessage: fmt.Sprintf("gate 1 escalation: %s", ladderRes.Reason),
			Metadata: map[string]any{
				"escalation": ladderRes.Escalation,
			},
		}, nil
	}

	// Direct resolution via System-1 Synaptic Engine (0 Token, <15ms)
	if ladderRes.Resolution == gates.ResolutionSystem1Direct && ladderRes.DecisionResult != nil {
		atomic.AddInt64(&a.completedTasks, 1)
		return &protocol.StepResult{
			Status:        "SUCCESS",
			OutputMessage: ladderRes.DecisionResult.TopOption,
			Metadata: map[string]any{
				"resolution_tier": "TIER_1_SYNAPTIC",
				"tokens_consumed": 0,
				"engine_used":     ladderRes.DecisionResult.EngineUsed,
				"confidence":      ladderRes.DecisionResult.Confidence,
				"latency_ms":      ladderRes.DecisionResult.LatencyMs,
			},
		}, nil
	}

	// 2. Fetch input data from Shared Data Substrate if present (Tier-2 SLM generation)
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

	// 4. Gate 3: Mathematical Logprob & System-1 Post-Critic Quality Check
	g3Passed, g3Reason, sys1Critic := gates.EvaluateTier3Critic(ctx, a.manifest, env, a.decisionEngine, finalResp.Content, finalResp.AvgLogprob)
	if !g3Passed {
		atomic.AddInt64(&a.escalatedTasks, 1)
		return &protocol.StepResult{
			Status:        "ESCALATED",
			OutputMessage: fmt.Sprintf("gate 3 critic escalation: %s", g3Reason),
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
	resMeta := map[string]any{
		"resolution_tier": "TIER_2_GENERATIVE_SLM",
		"prompt_tokens":   finalResp.PromptTokens,
		"output_tokens":   finalResp.OutputTokens,
	}
	if sys1Critic != nil {
		resMeta["system1_critic"] = sys1Critic
	}

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
		Metadata: resMeta,
	}, nil
}

func extractDecisionQuestion(env *protocol.NervousEnvelope) *protocol.DecisionQuestion {
	if env.Parameters == nil {
		return nil
	}
	if dqVal, ok := env.Parameters["decision_question"]; ok {
		if dq, ok := dqVal.(*protocol.DecisionQuestion); ok {
			return dq
		}
		if dqMap, ok := dqVal.(map[string]any); ok {
			return parseDecisionQuestionFromMap(dqMap)
		}
	}
	if opts, ok := env.Parameters["options"].([]string); ok && len(opts) > 0 {
		prompt, _ := env.Parameters["prompt"].(string)
		if prompt == "" {
			prompt = env.TargetCapability
		}
		return &protocol.DecisionQuestion{
			QID:     env.EnvelopeID + "-dq",
			QType:   "choice",
			Prompt:  prompt,
			Options: opts,
		}
	}
	return nil
}

func parseDecisionQuestionFromMap(m map[string]any) *protocol.DecisionQuestion {
	q := &protocol.DecisionQuestion{}
	if qid, ok := m["qid"].(string); ok {
		q.QID = qid
	}
	if qtype, ok := m["qtype"].(string); ok {
		q.QType = qtype
	}
	if prompt, ok := m["prompt"].(string); ok {
		q.Prompt = prompt
	}
	if opts, ok := m["options"].([]string); ok {
		q.Options = opts
	} else if optsI, ok := m["options"].([]any); ok {
		for _, o := range optsI {
			if s, ok := o.(string); ok {
				q.Options = append(q.Options, s)
			}
		}
	}
	return q
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
