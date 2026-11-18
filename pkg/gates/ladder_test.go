package gates

import (
	"context"
	"octpex/pkg/protocol"
	"strings"
	"testing"
)

// mockDecisionEngine implements protocol.DecisionEngineSPI for unit tests.
type mockDecisionEngine struct {
	decideFunc        func(ctx context.Context, stateText string, questions []protocol.DecisionQuestion) ([]protocol.DecisionResult, error)
	reverseOptionFunc func(ctx context.Context, stateText string, question protocol.DecisionQuestion) (*protocol.DecisionResult, bool, error)
}

func (m *mockDecisionEngine) Decide(ctx context.Context, stateText string, questions []protocol.DecisionQuestion, images [][]byte) ([]protocol.DecisionResult, error) {
	if m.decideFunc != nil {
		return m.decideFunc(ctx, stateText, questions)
	}
	return []protocol.DecisionResult{
		{
			QID:        questions[0].QID,
			TopOption:  questions[0].Options[0],
			Confidence: 0.85,
			EngineUsed: "mock-engine",
			LatencyMs:  5,
		},
	}, nil
}

func (m *mockDecisionEngine) ReverseOptionCheck(ctx context.Context, stateText string, question protocol.DecisionQuestion) (*protocol.DecisionResult, bool, error) {
	if m.reverseOptionFunc != nil {
		return m.reverseOptionFunc(ctx, stateText, question)
	}
	return &protocol.DecisionResult{
		QID:        question.QID,
		TopOption:  question.Options[0],
		Confidence: 0.60,
		EngineUsed: "mock-engine",
		LatencyMs:  8,
	}, true, nil
}

func (m *mockDecisionEngine) ActiveProfile() string {
	return "mock-profile"
}

func TestLadder_Tier1DirectResolve(t *testing.T) {
	manifest := &protocol.ArmManifest{
		ArmID: "arm-fast",
		SLMProfile: protocol.SLMProfile{
			SafeContextLimit: 4096,
		},
		EscalationGates: protocol.EscalationGates{
			MinCapabilitySimilarity: 0.65,
			MinSystem1Confidence:    0.75,
			UncertaintyBandLow:      0.50,
		},
	}
	env := protocol.NewEnvelope("env-1", "tr-1", "sess-1", protocol.PlaneLateral, protocol.MsgTaskDispatch, "arm-fast")

	mockEngine := &mockDecisionEngine{
		decideFunc: func(ctx context.Context, stateText string, questions []protocol.DecisionQuestion) ([]protocol.DecisionResult, error) {
			return []protocol.DecisionResult{
				{
					QID:        questions[0].QID,
					TopOption:  "EXECUTE_DIRECT",
					Confidence: 0.92,
					EngineUsed: "laya-onnx-int8",
					LatencyMs:  11,
				},
			}, nil
		},
	}

	q := &protocol.DecisionQuestion{
		QID:     "q1",
		QType:   "choice",
		Prompt:  "Can this task be completed without generative reasoning?",
		Options: []string{"EXECUTE_DIRECT", "NEEDS_GENERATION"},
	}

	res := EvaluateTier1Synaptic(context.Background(), manifest, env, mockEngine, q, 0.80, 500)
	if res.Resolution != ResolutionSystem1Direct {
		t.Fatalf("expected ResolutionSystem1Direct, got %s (reason: %s)", res.Resolution, res.Reason)
	}
	if res.System1Metrics == nil || res.System1Metrics.System1Confidence < 0.90 {
		t.Fatalf("expected confidence >= 0.90, got: %+v", res.System1Metrics)
	}
	if res.DecisionResult.TopOption != "EXECUTE_DIRECT" {
		t.Fatalf("unexpected top option: %s", res.DecisionResult.TopOption)
	}
}

func TestLadder_Tier1UncertaintyAndFlexDM(t *testing.T) {
	manifest := &protocol.ArmManifest{
		ArmID: "arm-edge",
		SLMProfile: protocol.SLMProfile{
			SafeContextLimit: 4096,
		},
		EscalationGates: protocol.EscalationGates{
			MinCapabilitySimilarity: 0.65,
			MinSystem1Confidence:    0.75,
			UncertaintyBandLow:      0.50,
		},
	}
	env := protocol.NewEnvelope("env-2", "tr-2", "sess-2", protocol.PlaneLateral, protocol.MsgTaskDispatch, "arm-edge")

	q := &protocol.DecisionQuestion{
		QID:     "q2",
		QType:   "choice",
		Prompt:  "Categorize intent",
		Options: []string{"A", "B"},
	}

	// 1. Extreme uncertainty (< 0.50) -> immediate escalation to brain
	mockSevereUncertainty := &mockDecisionEngine{
		decideFunc: func(ctx context.Context, stateText string, questions []protocol.DecisionQuestion) ([]protocol.DecisionResult, error) {
			return []protocol.DecisionResult{
				{
					QID:        questions[0].QID,
					TopOption:  "A",
					Confidence: 0.35,
					EngineUsed: "laya-onnx-int8",
					LatencyMs:  9,
				},
			}, nil
		},
	}
	res1 := EvaluateTier1Synaptic(context.Background(), manifest, env, mockSevereUncertainty, q, 0.80, 500)
	if res1.Resolution != ResolutionEscalatedToBrain {
		t.Fatalf("expected ResolutionEscalatedToBrain on severe uncertainty, got %s", res1.Resolution)
	}
	if res1.Escalation == nil || res1.Escalation.System1Metrics == nil {
		t.Fatalf("expected escalation with System1Metrics, got: %+v", res1.Escalation)
	}

	// 2. Uncertainty band (0.60) + FlexDM reverse calibration FAILURE -> escalation
	mockFlexDMFail := &mockDecisionEngine{
		decideFunc: func(ctx context.Context, stateText string, questions []protocol.DecisionQuestion) ([]protocol.DecisionResult, error) {
			return []protocol.DecisionResult{
				{
					QID:        questions[0].QID,
					TopOption:  "A",
					Confidence: 0.60,
					EngineUsed: "laya-onnx-int8",
					LatencyMs:  10,
				},
			}, nil
		},
		reverseOptionFunc: func(ctx context.Context, stateText string, question protocol.DecisionQuestion) (*protocol.DecisionResult, bool, error) {
			return &protocol.DecisionResult{
				QID:        question.QID,
				TopOption:  "B", // Inconsistent!
				Confidence: 0.58,
				EngineUsed: "laya-onnx-int8",
				LatencyMs:  10,
			}, false, nil
		},
	}
	res2 := EvaluateTier1Synaptic(context.Background(), manifest, env, mockFlexDMFail, q, 0.80, 500)
	if res2.Resolution != ResolutionEscalatedToBrain {
		t.Fatalf("expected ResolutionEscalatedToBrain on reverse calibration failure, got %s", res2.Resolution)
	}
	if res2.System1Metrics.ReverseOptionConsistent {
		t.Fatal("expected ReverseOptionConsistent to be false")
	}

	// 3. Uncertainty band (0.62) + FlexDM reverse calibration SUCCESS -> transition to Tier 2 Generative
	mockFlexDMSuccess := &mockDecisionEngine{
		decideFunc: func(ctx context.Context, stateText string, questions []protocol.DecisionQuestion) ([]protocol.DecisionResult, error) {
			return []protocol.DecisionResult{
				{
					QID:        questions[0].QID,
					TopOption:  "A",
					Confidence: 0.62,
					EngineUsed: "laya-onnx-int8",
					LatencyMs:  10,
				},
			}, nil
		},
		reverseOptionFunc: func(ctx context.Context, stateText string, question protocol.DecisionQuestion) (*protocol.DecisionResult, bool, error) {
			return &protocol.DecisionResult{
				QID:        question.QID,
				TopOption:  "A", // Consistent!
				Confidence: 0.64,
				EngineUsed: "laya-onnx-int8",
				LatencyMs:  10,
			}, true, nil
		},
	}
	res3 := EvaluateTier1Synaptic(context.Background(), manifest, env, mockFlexDMSuccess, q, 0.80, 500)
	if res3.Resolution != ResolutionSystem2Generative {
		t.Fatalf("expected ResolutionSystem2Generative on calibrated uncertainty, got %s", res3.Resolution)
	}
	if !res3.System1Metrics.ReverseOptionConsistent {
		t.Fatal("expected ReverseOptionConsistent to be true")
	}
}

func TestLadder_Tier3Critic(t *testing.T) {
	manifest := &protocol.ArmManifest{
		ArmID: "arm-critic",
		EscalationGates: protocol.EscalationGates{
			MinTokenLogprob:      -0.51,
			MaxLateralHops:       3,
			MinSystem1Confidence: 0.75,
		},
	}
	env := protocol.NewEnvelope("env-3", "tr-3", "sess-3", protocol.PlaneLateral, protocol.MsgTaskDispatch, "arm-critic")
	env.HopCount = 1

	// 1. Output passes critic check
	validEngine := &mockDecisionEngine{
		decideFunc: func(ctx context.Context, stateText string, questions []protocol.DecisionQuestion) ([]protocol.DecisionResult, error) {
			return []protocol.DecisionResult{
				{
					QID:        questions[0].QID,
					TopOption:  "VALID",
					Confidence: 0.95,
					EngineUsed: "laya-onnx-int8",
					LatencyMs:  8,
				},
			}, nil
		},
	}

	passed, reason, metrics := EvaluateTier3Critic(context.Background(), manifest, env, validEngine, "def add(a, b): return a + b", -0.15)
	if !passed {
		t.Fatalf("expected passed, got: %s", reason)
	}
	if metrics == nil || metrics.System1Confidence < 0.90 {
		t.Fatalf("expected valid metrics, got: %+v", metrics)
	}

	// 2. Output flagged as ANOMALY with high confidence by System-1
	anomalyEngine := &mockDecisionEngine{
		decideFunc: func(ctx context.Context, stateText string, questions []protocol.DecisionQuestion) ([]protocol.DecisionResult, error) {
			return []protocol.DecisionResult{
				{
					QID:        questions[0].QID,
					TopOption:  "ANOMALY",
					Confidence: 0.88,
					EngineUsed: "laya-onnx-int8",
					LatencyMs:  9,
				},
			}, nil
		},
	}

	passed, reason, _ = EvaluateTier3Critic(context.Background(), manifest, env, anomalyEngine, "infinite recursion detected", -0.15)
	if passed {
		t.Fatal("expected failure when critic flags ANOMALY")
	}
	if !strings.Contains(reason, "ANOMALY") {
		t.Fatalf("expected reason to mention ANOMALY, got: %s", reason)
	}
}
