package gates

import (
	"context"
	"octpex/pkg/protocol"
	"testing"
)

func TestGate1PreFlightCheck(t *testing.T) {
	manifest := &protocol.ArmManifest{
		ArmID: "arm-01",
		SLMProfile: protocol.SLMProfile{
			SafeContextLimit: 4096,
		},
		EscalationGates: protocol.EscalationGates{
			MinCapabilitySimilarity: 0.65,
		},
	}
	env := protocol.NewEnvelope("env-1", "tr-1", "sess-1", protocol.PlaneLateral, protocol.MsgTaskDispatch, "node-1")

	// 1. Valid case
	passed, reason := PreFlightCheck(manifest, env, 0.85, 1024)
	if !passed {
		t.Fatalf("expected passed, got failure: %s", reason)
	}

	// 2. Token overflow
	passed, reason = PreFlightCheck(manifest, env, 0.85, 5000)
	if passed {
		t.Fatal("expected failure on token overflow")
	}

	// 3. OOD similarity below 0.65
	passed, reason = PreFlightCheck(manifest, env, 0.45, 1024)
	if passed {
		t.Fatal("expected failure on OOD similarity")
	}

	// 4. Destructive command
	env.Parameters["command"] = "rm -rf /"
	passed, reason = PreFlightCheck(manifest, env, 0.85, 1024)
	if passed {
		t.Fatal("expected failure on destructive command")
	}
}

func TestGate2JSONVerifier(t *testing.T) {
	ctx := context.Background()
	validJSON := []byte(`{"status":"OK","code":200}`)
	if err := JSONSchemaVerifier(ctx, validJSON); err != nil {
		t.Fatalf("expected valid JSON, got: %v", err)
	}

	invalidJSON := []byte(`{status: OK, unquoted`)
	if err := JSONSchemaVerifier(ctx, invalidJSON); err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}

func TestGate3BehavioralCheck(t *testing.T) {
	manifest := &protocol.ArmManifest{
		ArmID: "arm-01",
		EscalationGates: protocol.EscalationGates{
			MinTokenLogprob: -0.51,
			MaxLateralHops:  3,
		},
	}
	env := protocol.NewEnvelope("env-1", "tr-1", "sess-1", protocol.PlaneLateral, protocol.MsgTaskDispatch, "node-1")
	env.HopCount = 1

	// 1. Valid case
	passed, reason := BehavioralCheck(manifest, env, -0.20)
	if !passed {
		t.Fatalf("expected passed, got: %s", reason)
	}

	// 2. Hop count exhausted
	env.HopCount = 3
	passed, reason = BehavioralCheck(manifest, env, -0.20)
	if passed {
		t.Fatal("expected failure on hop count limit reached")
	}

	// 3. Low confidence logprob
	env.HopCount = 1
	passed, reason = BehavioralCheck(manifest, env, -0.85)
	if passed {
		t.Fatal("expected failure on low token logprob")
	}
}
