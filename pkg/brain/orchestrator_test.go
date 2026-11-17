package brain

import (
	"context"
	"octpex/pkg/protocol"
	"testing"
)

type mockFrontierLLM struct{}

func (m *mockFrontierLLM) GenerateDAG(ctx context.Context, goal string) (*PlanDAG, error) {
	return &PlanDAG{
		PlanID: "plan-mock-1",
		Goal:   goal,
		Steps: []PlanStep{
			{StepID: "step-1", TargetCapability: "distill-telemetry"},
			{StepID: "step-2", TargetCapability: "synthesize-patch", Dependencies: []string{"step-1"}},
		},
	}, nil
}

func (m *mockFrontierLLM) SolveEscalation(ctx context.Context, payload *protocol.EscalationPayload) (DecisionType, string, error) {
	return DecisionSynthesizeDynamicTool, "def fix(): return True", nil
}

func TestOctoBrainPlanning(t *testing.T) {
	ctx := context.Background()
	llm := &mockFrontierLLM{}
	brain := NewOctoBrain(llm)

	dag, err := brain.Plan(ctx, "Resolve pod crash", nil)
	if err != nil {
		t.Fatalf("brain plan failed: %v", err)
	}
	if len(dag.Steps) != 2 {
		t.Fatalf("expected 2 steps in DAG, got: %d", len(dag.Steps))
	}
}

func TestOctoBrainArbitrationAndHITL(t *testing.T) {
	ctx := context.Background()
	brain := NewOctoBrain(nil) // Fallback rule arbitrator

	// 1. High risk escalation -> triggers HITL suspension
	escEnv := protocol.NewEnvelope("env-danger", "tr-1", "sess-1", protocol.PlaneVertical, protocol.MsgEscalation, "arm-01")
	escPayload := &protocol.EscalationPayload{
		EscalationID:     "esc-1",
		OriginalEnvelope: *escEnv,
		TriggeredGate:    protocol.Gate1PreFlightPolicy,
		FailureReason:    "prohibited destructive system command",
	}

	dec, err := brain.Arbitrate(ctx, escPayload)
	if err != nil {
		t.Fatalf("arbitration failed: %v", err)
	}
	if dec.Decision != DecisionSuspendHITL {
		t.Fatalf("expected DecisionSuspendHITL, got: %s", dec.Decision)
	}
	if dec.HITLRequest == nil {
		t.Fatal("expected non-nil HITLRequest in decision")
	}

	reqID := dec.HITLRequest.RequestID

	// 2. Reject HITL
	_, err = brain.ResumeFromHITL(ctx, reqID, false, "Denied by SecOps")
	if err == nil {
		t.Fatal("expected error on rejected HITL, got nil")
	}

	// 3. Re-trigger and Approve HITL
	dec2, _ := brain.Arbitrate(ctx, escPayload)
	resumedEnv, err := brain.ResumeFromHITL(ctx, dec2.HITLRequest.RequestID, true, "Approved by SecOps Lead")
	if err != nil {
		t.Fatalf("resume failed: %v", err)
	}
	if resumedEnv.Parameters["hitl_approved"] != true {
		t.Error("expected hitl_approved = true in resumed envelope")
	}
}
