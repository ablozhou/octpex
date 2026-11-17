package test

import (
	"context"
	"fmt"
	"octpex/pkg/bus"
	"octpex/pkg/gates"
	"octpex/pkg/protocol"
	"octpex/pkg/substrate"
	"testing"
	"time"
)

func BenchmarkSubstrateCAS(b *testing.B) {
	ctx := context.Background()
	sub := substrate.NewInMemorySubstrate()
	uri := "sub://sess-bench/data"
	_, _ = sub.PutArtifact(ctx, uri, 0, []byte("init"), time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ver := int64(i + 1)
		_, err := sub.PutArtifact(ctx, uri, ver, []byte("data"), time.Hour)
		if err != nil {
			b.Fatalf("CAS put failed at iteration %d: %v", i, err)
		}
	}
}

func BenchmarkLocalNervousBus(b *testing.B) {
	ctx := context.Background()
	localBus := bus.NewLocalNervousBus(4096)
	defer localBus.Close()

	localBus.Subscribe("benchmark_task", func(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
		return &protocol.StepResult{Status: "SUCCESS"}, nil
	})

	env := &protocol.NervousEnvelope{
		EnvelopeID:       "env-bench",
		TraceID:          "tr-bench",
		SessionID:        "sess-bench",
		Plane:            protocol.PlaneVertical,
		TargetCapability: "benchmark_task",
		SourceNode:       "benchmarker",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := localBus.Publish(ctx, env)
		if err != nil {
			b.Fatalf("Publish failed: %v", err)
		}
	}
}

func BenchmarkTriGateVerification(b *testing.B) {
	ctx := context.Background()
	env := &protocol.NervousEnvelope{
		EnvelopeID:       "env-gate-bench",
		TraceID:          "tr-gate-bench",
		SessionID:        "sess-gate-bench",
		TargetCapability: "code_patch",
		Parameters:       map[string]any{"target": "app.go"},
		Security:         protocol.DefaultSecurityContext("sess-gate-bench"),
	}

	manifest := &protocol.ArmManifest{
		ArmID:          "arm-bench",
		CapabilityTags: []string{"code_patch"},
		EscalationGates: protocol.DefaultEscalationGates(),
	}

	payload := []byte(`{"status":"ok","patch":"fmt.Println(1)"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Gate 1 check
		passed, reason := gates.PreFlightCheck(manifest, env, 0.9, 50)
		if !passed {
			b.Fatalf("Gate 1 failed: %s", reason)
		}
		// Gate 2 check
		if err := gates.JSONSchemaVerifier(ctx, payload); err != nil {
			b.Fatalf("Gate 2 failed: %v", err)
		}
	}
}

func TestBenchmarkRunnerQuickSanity(t *testing.T) {
	// Ensures benchmark helper logic executes correctly
	ctx := context.Background()
	sub := substrate.NewInMemorySubstrate()
	v, err := sub.PutArtifact(ctx, "sub://test/item", 0, []byte("ok"), time.Minute)
	if err != nil || v != 1 {
		t.Fatalf("sanity check failed: %v", err)
	}
	fmt.Printf("[Benchmark Quick Sanity] Substrate CAS verified: version %d\n", v)
}
