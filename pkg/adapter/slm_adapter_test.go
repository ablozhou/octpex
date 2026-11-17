package adapter

import (
	"context"
	"octpex/pkg/protocol"
	"octpex/pkg/substrate"
	"testing"
)

type mockSLMEngine struct {
	responses []*InferenceResponse
	callCount int
}

func (m *mockSLMEngine) Generate(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error) {
	idx := m.callCount
	m.callCount++
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return m.responses[len(m.responses)-1], nil
}

func (m *mockSLMEngine) ContextCapacity() int {
	return 4096
}

func TestSLMGanglionReflexLoopSelfHealing(t *testing.T) {
	ctx := context.Background()
	sub := substrate.NewInMemorySubstrate()

	// Mock engine: attempt 1 produces broken JSON, attempt 2 produces valid JSON!
	mockEngine := &mockSLMEngine{
		responses: []*InferenceResponse{
			{
				Content:    `{broken_json: unquoted`,
				AvgLogprob: -0.20,
			},
			{
				Content:    `{"status":"recovered","fixed_by":"reflex_loop"}`,
				AvgLogprob: -0.15,
			},
		},
	}

	arm := NewSLMGanglionAdapter(sub, mockEngine, nil)
	manifest := &protocol.ArmManifest{
		ArmID:          "arm-slm-01",
		RuntimeAdapter: "ollama",
		RoleName:       "patch-synthesizer",
		CapabilityTags: []string{"fix-code"},
		Endpoint:       "local://arm-slm-01",
		SLMProfile: protocol.SLMProfile{
			SafeContextLimit: 4096,
		},
		EscalationGates: protocol.DefaultEscalationGates(),
	}
	if err := arm.Init(ctx, manifest); err != nil {
		t.Fatalf("arm init failed: %v", err)
	}

	env := protocol.NewEnvelope("env-1", "tr-1", "sess-slm", protocol.PlaneVertical, protocol.MsgTaskDispatch, "brain")
	env.Parameters["prompt"] = "generate json patch"

	res, err := arm.ExecuteStep(ctx, env)
	if err != nil {
		t.Fatalf("execute step failed: %v", err)
	}
	if res.Status != "SUCCESS" {
		t.Fatalf("expected self-healing SUCCESS, got: %s (msg: %s)", res.Status, res.OutputMessage)
	}
	if mockEngine.callCount != 2 {
		t.Errorf("expected exactly 2 calls (attempt 1 failed, attempt 2 healed), got: %d", mockEngine.callCount)
	}
}

func TestSLMGanglionReflexRetriesExhausted(t *testing.T) {
	ctx := context.Background()
	sub := substrate.NewInMemorySubstrate()

	// Mock engine: persistently invalid output
	mockEngine := &mockSLMEngine{
		responses: []*InferenceResponse{
			{Content: `invalid_1`, AvgLogprob: -0.20},
			{Content: `invalid_2`, AvgLogprob: -0.20},
			{Content: `invalid_3`, AvgLogprob: -0.20},
		},
	}

	arm := NewSLMGanglionAdapter(sub, mockEngine, nil)
	manifest := &protocol.ArmManifest{
		ArmID:          "arm-slm-02",
		RuntimeAdapter: "ollama",
		RoleName:       "failing-worker",
		CapabilityTags: []string{"broken"},
		Endpoint:       "local://arm-slm-02",
		EscalationGates: protocol.EscalationGates{
			MaxReflexRetries: 3,
			MinTokenLogprob:  -0.51,
		},
	}
	arm.Init(ctx, manifest)

	env := protocol.NewEnvelope("env-2", "tr-2", "sess-fail", protocol.PlaneVertical, protocol.MsgTaskDispatch, "brain")
	res, err := arm.ExecuteStep(ctx, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != "ESCALATED" {
		t.Fatalf("expected status ESCALATED after 3 failed retries, got: %s", res.Status)
	}
}
