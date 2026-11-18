package gates

import (
	"context"
	"fmt"
	"octpex/pkg/protocol"
)

// CognitiveLadderResolution defines the resolution tier selected by the cognitive ladder.
type CognitiveLadderResolution string

const (
	ResolutionSystem1Direct     CognitiveLadderResolution = "SYSTEM_1_DIRECT"
	ResolutionSystem2Generative CognitiveLadderResolution = "SYSTEM_2_GENERATIVE"
	ResolutionEscalatedToBrain  CognitiveLadderResolution = "ESCALATED_TO_BRAIN"
)

// LadderEvaluationResult encapsulates the decision outcome of Tier-1 Synaptic Engine routing.
type LadderEvaluationResult struct {
	Resolution     CognitiveLadderResolution
	DecisionResult *protocol.DecisionResult
	System1Metrics *protocol.System1Metrics
	Escalation     *protocol.EscalationPayload
	Reason         string
}

// EvaluateTier1Synaptic evaluates pre-flight policy and discriminative questions through the
// non-autoregressive System-1 synaptic engine.
// Flow:
// 1. Basic Gate 1 PreFlightCheck (Safe context limit, OOD similarity, destructive policy).
// 2. If decisionEngine or question is nil: bypass to Tier 2 Generative SLM.
// 3. Evaluate question via System-1:
//    - If confidence >= MinSystem1Confidence (default 0.75): Resolve directly at Tier 1 with 0 tokens.
//    - If confidence < UncertaintyBandLow (default 0.50): Escalate directly to Octo-Brain (Gate 1).
//    - If 0.50 <= confidence < 0.75: Run FlexDM ReverseOptionCheck.
//      - If reverse calibration inconsistent: Escalate to Octo-Brain (Gate 1).
//      - If consistent: Transition to Tier 2 Generative SLM.
func EvaluateTier1Synaptic(
	ctx context.Context,
	manifest *protocol.ArmManifest,
	env *protocol.NervousEnvelope,
	engine protocol.DecisionEngineSPI,
	question *protocol.DecisionQuestion,
	capabilityDistance float32,
	promptTokenEstimate int,
) *LadderEvaluationResult {
	// Step 1: Pre-flight check
	passed, reason := PreFlightCheck(manifest, env, capabilityDistance, promptTokenEstimate)
	if !passed {
		return &LadderEvaluationResult{
			Resolution: ResolutionEscalatedToBrain,
			Escalation: BuildEscalation(env, protocol.Gate1PreFlightPolicy, reason, 1, nil),
			Reason:     reason,
		}
	}

	// Step 2: If no System-1 engine or question provided, proceed to Tier-2 Generative SLM
	if engine == nil || question == nil {
		return &LadderEvaluationResult{
			Resolution: ResolutionSystem2Generative,
			Reason:     "no system-1 engine or discriminative question specified; proceeding to tier-2 slm",
		}
	}

	minConf := manifest.EscalationGates.MinSystem1Confidence
	if minConf <= 0 {
		minConf = 0.75
	}
	uncBand := manifest.EscalationGates.UncertaintyBandLow
	if uncBand <= 0 {
		uncBand = 0.50
	}

	stateText := env.TargetCapability
	if stateText == "" {
		if p, ok := env.Parameters["prompt"].(string); ok {
			stateText = p
		} else {
			stateText = env.EnvelopeID
		}
	}

	// Single forward pass via Decide
	results, err := engine.Decide(ctx, stateText, []protocol.DecisionQuestion{*question}, nil)
	if err != nil {
		reason := fmt.Sprintf("gate 1: system-1 inference failure: %v", err)
		return &LadderEvaluationResult{
			Resolution: ResolutionEscalatedToBrain,
			Escalation: BuildEscalation(env, protocol.Gate1PreFlightPolicy, reason, 1, nil),
			Reason:     reason,
		}
	}

	if len(results) == 0 {
		return &LadderEvaluationResult{
			Resolution: ResolutionSystem2Generative,
			Reason:     "empty result from system-1 engine; fallback to tier-2",
		}
	}

	res := results[0]
	sys1Metrics := &protocol.System1Metrics{
		System1Engine:            res.EngineUsed,
		System1Confidence:        res.Confidence,
		ReverseOptionConsistent: true,
		DecisionLatencyMs:        res.LatencyMs,
	}

	// Case A: High confidence -> Direct 0-token resolution
	if res.Confidence >= float64(minConf) {
		return &LadderEvaluationResult{
			Resolution:     ResolutionSystem1Direct,
			DecisionResult: &res,
			System1Metrics: sys1Metrics,
			Reason:         fmt.Sprintf("system-1 direct resolution: confidence %.2f >= threshold %.2f", res.Confidence, minConf),
		}
	}

	// Case B: Severe uncertainty (OOD) -> Immediate escalation
	if res.Confidence < float64(uncBand) {
		reason := fmt.Sprintf("gate 1: system-1 confidence (%.2f) below uncertainty floor (%.2f)", res.Confidence, uncBand)
		return &LadderEvaluationResult{
			Resolution:     ResolutionEscalatedToBrain,
			DecisionResult: &res,
			System1Metrics: sys1Metrics,
			Escalation:     BuildEscalationWithSystem1(env, protocol.Gate1PreFlightPolicy, reason, 1, nil, sys1Metrics),
			Reason:         reason,
		}
	}

	// Case C: In uncertainty band (0.50 <= p < 0.75) -> FlexDM Reverse Option Calibration
	_, isConsistent, err := engine.ReverseOptionCheck(ctx, stateText, *question)
	sys1Metrics.ReverseOptionConsistent = isConsistent
	if err != nil || !isConsistent {
		reason := fmt.Sprintf("gate 1: flexdm reverse-option calibration failure (confidence %.2f)", res.Confidence)
		return &LadderEvaluationResult{
			Resolution:     ResolutionEscalatedToBrain,
			DecisionResult: &res,
			System1Metrics: sys1Metrics,
			Escalation:     BuildEscalationWithSystem1(env, protocol.Gate1PreFlightPolicy, reason, 1, nil, sys1Metrics),
			Reason:         reason,
		}
	}

	// Calibration passed -> Transition to Tier-2 Generative SLM
	return &LadderEvaluationResult{
		Resolution:     ResolutionSystem2Generative,
		DecisionResult: &res,
		System1Metrics: sys1Metrics,
		Reason:         fmt.Sprintf("system-1 calibrated (confidence %.2f); proceeding to tier-2 generative slm", res.Confidence),
	}
}

// EvaluateTier3Critic evaluates Gate 3 post-generation quality and behavioral constraints.
// It verifies token logprob, lateral hops, and runs an optional System-1 critic check on the output text.
func EvaluateTier3Critic(
	ctx context.Context,
	manifest *protocol.ArmManifest,
	env *protocol.NervousEnvelope,
	engine protocol.DecisionEngineSPI,
	outputText string,
	avgLogprob float32,
) (passed bool, reason string, metrics *protocol.System1Metrics) {
	// 1. Evaluate standard Gate 3 behavioral constraints (hop count and logprob floor)
	if passed, reason := BehavioralCheck(manifest, env, avgLogprob); !passed {
		return false, reason, nil
	}

	// 2. If System-1 engine is available, execute a non-autoregressive critic evaluation
	if engine != nil && outputText != "" {
		criticQ := protocol.DecisionQuestion{
			QID:     "gate3-critic",
			QType:   "choice",
			Prompt:  fmt.Sprintf("Critique the generation output: %s", outputText),
			Options: []string{"VALID", "INVALID", "ANOMALY"},
		}

		results, err := engine.Decide(ctx, outputText, []protocol.DecisionQuestion{criticQ}, nil)
		if err == nil && len(results) > 0 {
			res := results[0]
			metrics = &protocol.System1Metrics{
				System1Engine:            res.EngineUsed,
				System1Confidence:        res.Confidence,
				ReverseOptionConsistent: true,
				DecisionLatencyMs:        res.LatencyMs,
			}

			// If the critic identifies an anomaly with high confidence
			minConf := manifest.EscalationGates.MinSystem1Confidence
			if minConf <= 0 {
				minConf = 0.75
			}
			if (res.TopOption == "INVALID" || res.TopOption == "ANOMALY") && res.Confidence >= float64(minConf) {
				return false, fmt.Sprintf("gate 3 critic: output flagged as %s by %s (confidence %.2f)",
					res.TopOption, res.EngineUsed, res.Confidence), metrics
			}
		}
	}

	return true, "", metrics
}
