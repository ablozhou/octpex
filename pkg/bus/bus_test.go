package bus

import (
	"context"
	"octpex/pkg/protocol"
	"testing"
)

func TestLocalNervousBusPublishSubscribe(t *testing.T) {
	ctx := context.Background()
	b := NewLocalNervousBus(16)
	defer b.Close()

	// Register subscriber for "patch-synthesizer"
	b.Subscribe("patch-synthesizer", func(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
		if env.HopCount != 1 {
			t.Errorf("expected hop_count = 1 for lateral plane, got: %d", env.HopCount)
		}
		return &protocol.StepResult{
			Status:        "SUCCESS",
			OutputMessage: "patch generated",
		}, nil
	})

	// Publish lateral envelope
	env := protocol.NewEnvelope("env-1", "tr-1", "sess-1", protocol.PlaneLateral, protocol.MsgHandoffRequest, "arm-telemetry")
	env.TargetCapability = "patch-synthesizer"

	res, err := b.Publish(ctx, env)
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if res.Status != "SUCCESS" {
		t.Errorf("expected SUCCESS, got: %s", res.Status)
	}

	// Publish to unrouted capability
	env.TargetCapability = "unknown-worker"
	_, err = b.Publish(ctx, env)
	if err == nil {
		t.Fatal("expected error for unrouted capability, got nil")
	}
}
