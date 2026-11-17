package bus

import (
	"context"
	"errors"
	"octpex/pkg/protocol"
	"testing"
)

func TestNATSNervousBus_VerticalDispatch(t *testing.T) {
	mockNATS := NewMockNATSClient()
	bus := NewNATSNervousBus(mockNATS, "workers")

	ctx := context.Background()

	// Register capability handler
	err := bus.Subscribe("telemetry_cleaner", func(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
		return &protocol.StepResult{
			Status:        "SUCCESS",
			OutputMessage: "cleaned_telemetry_payload",
		}, nil
	})
	if err != nil {
		t.Fatalf("bus.Subscribe failed: %v", err)
	}

	env := &protocol.NervousEnvelope{
		EnvelopeID:       "env-nats-1",
		TraceID:          "trace-nats-1",
		SessionID:        "session-nats-1",
		Plane:            protocol.PlaneVertical,
		TargetCapability: "telemetry_cleaner",
		SourceNode:       "central-brain",
		Security:         protocol.DefaultSecurityContext("session-nats-1"),
	}

	res, err := bus.Publish(ctx, env)
	if err != nil {
		t.Fatalf("bus.Publish vertical failed: %v", err)
	}
	if res.Status != "SUCCESS" || res.OutputMessage != "cleaned_telemetry_payload" {
		t.Fatalf("unexpected step result: %+v", res)
	}
}

func TestNATSNervousBus_LateralHandoffAndAttenuation(t *testing.T) {
	mockNATS := NewMockNATSClient()
	bus := NewNATSNervousBus(mockNATS, "workers")

	ctx := context.Background()

	var receivedHops int
	var receivedAllowMutation bool

	err := bus.Subscribe("patch_synthesizer", func(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
		receivedHops = env.HopCount
		receivedAllowMutation = env.Security.AllowMutation
		return &protocol.StepResult{
			Status:        "SUCCESS",
			OutputMessage: "patch_applied",
		}, nil
	})
	if err != nil {
		t.Fatalf("bus.Subscribe failed: %v", err)
	}

	sec := protocol.DefaultSecurityContext("session-lateral-1")
	sec.MaxAllowedHops = 2
	sec.AllowMutation = true

	env := &protocol.NervousEnvelope{
		EnvelopeID:       "env-lateral-1",
		TraceID:          "trace-lateral-1",
		SessionID:        "session-lateral-1",
		Plane:            protocol.PlaneLateral,
		TargetCapability: "patch_synthesizer",
		SourceNode:       "arm-telemetry",
		HopCount:         0,
		Security:         sec,
	}

	res, err := bus.Publish(ctx, env)
	if err != nil {
		t.Fatalf("bus.Publish lateral failed: %v", err)
	}
	if res.Status != "SUCCESS" {
		t.Fatalf("expected success, got %+v", res)
	}
	if receivedHops != 1 {
		t.Fatalf("expected 1 hop, got %d", receivedHops)
	}
	// After publisher attenuation + receiver attenuation, hops decrement to <= 1, revoking AllowMutation
	if receivedAllowMutation {
		t.Fatalf("expected AllowMutation to be attenuated to false")
	}

	// Now test hop count exhaustion
	env.HopCount = 3
	_, err = bus.Publish(ctx, env)
	if err == nil || !errors.Is(err, ErrMaxHopsExceeded) {
		t.Fatalf("expected ErrMaxHopsExceeded, got %v", err)
	}
}
