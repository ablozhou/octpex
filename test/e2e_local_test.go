package test

import (
	"context"
	"encoding/json"
	"octpex/pkg/adapter"
	"octpex/pkg/bus"
	"octpex/pkg/gates"
	"octpex/pkg/protocol"
	"octpex/pkg/substrate"
	"testing"
	"time"
)

func TestSingleNodeBiomimeticKernelE2E(t *testing.T) {
	ctx := context.Background()

	// 1. Initialize Shared Data Substrate & Dual-Plane Nervous Bus
	sub := substrate.NewInMemorySubstrate()
	nerveBus := bus.NewLocalNervousBus(32)
	defer nerveBus.Close()

	// 2. Setup Arm 1: Telemetry Distiller (Level 0 Native FSM)
	arm1 := adapter.NewNativeFSMAdapter(sub)
	arm1Manifest := &protocol.ArmManifest{
		ArmID:          "arm-telemetry-01",
		RuntimeAdapter: "go-native-fsm",
		RoleName:       "telemetry-distiller",
		CapabilityTags: []string{"distill-telemetry"},
		Endpoint:       "local://arm-telemetry-01",
	}
	if err := arm1.Init(ctx, arm1Manifest); err != nil {
		t.Fatalf("arm1 init failed: %v", err)
	}

	arm1.RegisterHandler("distill-telemetry", func(ctx context.Context, env *protocol.NervousEnvelope, s substrate.DataSubstrateSPI) (*protocol.StepResult, error) {
		// Read input from sub://
		rawBytes, _, err := s.GetArtifact(ctx, env.SubstrateRefs[0].URI)
		if err != nil {
			return nil, err
		}

		// Distill raw log into clean structured JSON
		distilled := map[string]string{
			"error_type": "OutOfMemory",
			"component":  "payments-service",
			"raw_sample": string(rawBytes),
		}
		distilledBytes, _ := json.Marshal(distilled)

		outURI := "sub://" + env.SessionID + "/distilled-telemetry"
		ver, err := s.PutArtifact(ctx, outURI, 0, distilledBytes, time.Minute)
		if err != nil {
			return nil, err
		}

		return &protocol.StepResult{
			Status: "SUCCESS",
			SubstrateRefs: []protocol.SubstrateRef{
				{
					URI:         outURI,
					Version:     ver,
					ContentType: "application/json",
					ByteSize:    int64(len(distilledBytes)),
				},
			},
		}, nil
	})

	// 3. Setup Arm 2: Patch Synthesizer (Level 0 Native FSM)
	arm2 := adapter.NewNativeFSMAdapter(sub)
	arm2Manifest := &protocol.ArmManifest{
		ArmID:          "arm-patch-02",
		RuntimeAdapter: "go-native-fsm",
		RoleName:       "patch-synthesizer",
		CapabilityTags: []string{"synthesize-patch"},
		Endpoint:       "local://arm-patch-02",
	}
	if err := arm2.Init(ctx, arm2Manifest); err != nil {
		t.Fatalf("arm2 init failed: %v", err)
	}

	arm2.RegisterHandler("synthesize-patch", func(ctx context.Context, env *protocol.NervousEnvelope, s substrate.DataSubstrateSPI) (*protocol.StepResult, error) {
		// Read distilled JSON from substrate using sub:// reference
		ref := env.SubstrateRefs[0]
		data, _, err := s.GetArtifact(ctx, ref.URI)
		if err != nil {
			return nil, err
		}

		// Objective Gate 2 validation: Verify input is valid JSON schema
		if err := gates.JSONSchemaVerifier(ctx, data); err != nil {
			return nil, err
		}

		// Synthesize resource limit patch
		patch := map[string]any{
			"action":    "scale_memory",
			"target":    "payments-service",
			"memory_mb": 2048,
		}
		patchBytes, _ := json.Marshal(patch)

		patchURI := "sub://" + env.SessionID + "/k8s-patch"
		ver, err := s.PutArtifact(ctx, patchURI, 0, patchBytes, time.Minute)
		if err != nil {
			return nil, err
		}

		return &protocol.StepResult{
			Status: "SUCCESS",
			SubstrateRefs: []protocol.SubstrateRef{
				{
					URI:         patchURI,
					Version:     ver,
					ContentType: "application/json",
					ByteSize:    int64(len(patchBytes)),
				},
			},
		}, nil
	})

	// 4. Register both arms onto the Basal Nerve Ring bus
	nerveBus.Subscribe("distill-telemetry", arm1.ExecuteStep)
	nerveBus.Subscribe("synthesize-patch", arm2.ExecuteStep)

	// 5. Pre-seed raw error log in Shared Data Substrate
	sessionID := "sess-e2e-001"
	rawLogURI := "sub://" + sessionID + "/raw-log"
	v1, err := sub.PutArtifact(ctx, rawLogURI, 0, []byte("CRITICAL OOM: Process killed with exit 137"), time.Minute)
	if err != nil {
		t.Fatalf("failed to seed raw log: %v", err)
	}

	// 6. Step 1: Vertical dispatch from Brain -> Arm 1
	env1 := protocol.NewEnvelope("env-step-1", "trace-e2e", sessionID, protocol.PlaneVertical, protocol.MsgTaskDispatch, "octo-brain")
	env1.TargetCapability = "distill-telemetry"
	env1.SubstrateRefs = append(env1.SubstrateRefs, protocol.SubstrateRef{
		URI:     rawLogURI,
		Version: v1,
	})

	res1, err := nerveBus.Publish(ctx, env1)
	if err != nil {
		t.Fatalf("step 1 failed: %v", err)
	}
	if res1.Status != "SUCCESS" || len(res1.SubstrateRefs) != 1 {
		t.Fatalf("unexpected step 1 result: %+v", res1)
	}

	// 7. Step 2: Lateral handoff along tentacles: Arm 1 -> Arm 2
	env2 := protocol.NewEnvelope("env-step-2", "trace-e2e", sessionID, protocol.PlaneLateral, protocol.MsgHandoffRequest, "arm-telemetry-01")
	env2.TargetCapability = "synthesize-patch"
	env2.SubstrateRefs = append(env2.SubstrateRefs, res1.SubstrateRefs[0])

	res2, err := nerveBus.Publish(ctx, env2)
	if err != nil {
		t.Fatalf("step 2 lateral handoff failed: %v", err)
	}
	if res2.Status != "SUCCESS" {
		t.Fatalf("step 2 status not SUCCESS: %s", res2.Status)
	}
	if env2.HopCount != 1 {
		t.Errorf("expected hop_count = 1 after lateral handoff, got: %d", env2.HopCount)
	}

	// 8. Verify final patch in Shared Data Substrate
	finalData, ver, err := sub.GetArtifact(ctx, res2.SubstrateRefs[0].URI)
	if err != nil {
		t.Fatalf("failed to get final patch: %v", err)
	}
	if ver != 1 {
		t.Errorf("expected v1 patch, got v%d", ver)
	}

	var parsedPatch map[string]any
	if err := json.Unmarshal(finalData, &parsedPatch); err != nil {
		t.Fatalf("final patch JSON unmarshal failed: %v", err)
	}
	if parsedPatch["action"] != "scale_memory" || parsedPatch["memory_mb"].(float64) != 2048 {
		t.Fatalf("unexpected patch content: %+v", parsedPatch)
	}

	t.Logf("E2E Test completed successfully: Biomimetic Kernel pipeline passed in zero-cloud-token environment!")
}
