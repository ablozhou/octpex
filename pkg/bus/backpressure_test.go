package bus

import (
	"context"
	"errors"
	"octpex/pkg/protocol"
	"testing"
)

type mockRouter struct {
	sibling *protocol.ArmManifest
}

func (m *mockRouter) RouteLowestQueue(capabilityTag string) (*protocol.ArmManifest, error) {
	if m.sibling == nil {
		return nil, errors.New("no sibling available")
	}
	return m.sibling, nil
}

func TestWorkStealingController_BackpressureAndStealing(t *testing.T) {
	localBus := NewLocalNervousBus(256)
	ctx := context.Background()

	// Register capability handler on bus
	localBus.Subscribe("metric_compressor", func(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
		return &protocol.StepResult{
			Status:        "SUCCESS",
			OutputMessage: "compressed",
		}, nil
	})

	siblingArm := &protocol.ArmManifest{
		ArmID:          "arm-worker-02",
		CapabilityTags: []string{"metric_compressor"},
		Telemetry: protocol.ArmTelemetry{
			ActiveQueueDepth: 0,
		},
	}

	router := &mockRouter{sibling: siblingArm}
	controller := NewWorkStealingController(router, localBus)

	// Arm 1 with max capacity of 2
	mon1 := controller.RegisterArm("arm-worker-01", "metric_compressor", 2)

	env := &protocol.NervousEnvelope{
		EnvelopeID:       "env-bp-1",
		TraceID:          "trace-bp-1",
		SessionID:        "session-bp-1",
		Plane:            protocol.PlaneVertical,
		TargetCapability: "metric_compressor",
		SourceNode:       "octo-brain",
		Security:         protocol.DefaultSecurityContext("session-bp-1"),
	}

	// 1. Normal execution
	res, execArm, err := controller.DispatchWithBackpressure(ctx, "arm-worker-01", env)
	if err != nil || execArm != "arm-worker-01" || res.Status != "SUCCESS" {
		t.Fatalf("expected arm-worker-01 execution, got %s, err: %v", execArm, err)
	}

	// 2. Artificially fill Arm 1 queue to capacity (2/2)
	_ = mon1.Enqueue()
	_ = mon1.Enqueue()

	// 3. Next dispatch to Arm 1 must trigger work stealing to sibling Arm 2
	res2, execArm2, err2 := controller.DispatchWithBackpressure(ctx, "arm-worker-01", env)
	if err2 != nil {
		t.Fatalf("work stealing dispatch failed: %v", err2)
	}
	if execArm2 != "arm-worker-02" {
		t.Fatalf("expected task to be stolen by arm-worker-02, got %s", execArm2)
	}
	if res2.Status != "SUCCESS" {
		t.Fatalf("unexpected step result: %+v", res2)
	}

	// 4. Test when no sibling available and queue is full -> throttled
	router.sibling = nil
	_, _, err3 := controller.DispatchWithBackpressure(ctx, "arm-worker-01", env)
	if err3 == nil || !errors.Is(err3, ErrBackpressureThrottled) {
		t.Fatalf("expected ErrBackpressureThrottled, got %v", err3)
	}
}
