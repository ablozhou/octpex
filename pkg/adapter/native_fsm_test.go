package adapter

import (
	"context"
	"octpex/pkg/protocol"
	"octpex/pkg/substrate"
	"testing"
	"time"
)

func TestNativeFSMAdapterExecution(t *testing.T) {
	ctx := context.Background()
	sub := substrate.NewInMemorySubstrate()
	arm := NewNativeFSMAdapter(sub)

	manifest := &protocol.ArmManifest{
		ArmID:          "arm-native-01",
		RuntimeAdapter: "go-native-fsm",
		RoleName:       "log-reducer",
		CapabilityTags: []string{"reduce-logs"},
		Endpoint:       "local://arm-native-01",
	}
	if err := arm.Init(ctx, manifest); err != nil {
		t.Fatalf("arm init failed: %v", err)
	}

	// Register deterministic handler
	arm.RegisterHandler("reduce-logs", func(ctx context.Context, env *protocol.NervousEnvelope, s substrate.DataSubstrateSPI) (*protocol.StepResult, error) {
		// Read input from sub://
		inputRef := env.SubstrateRefs[0]
		data, _, err := s.GetArtifact(ctx, inputRef.URI)
		if err != nil {
			return nil, err
		}

		// Deterministic reduction logic (e.g. string transform)
		reduced := append([]byte("REDUCED: "), data...)
		outURI := "sub://" + env.SessionID + "/reduced-log"
		ver, err := s.PutArtifact(ctx, outURI, 0, reduced, time.Minute)
		if err != nil {
			return nil, err
		}

		return &protocol.StepResult{
			Status: "SUCCESS",
			SubstrateRefs: []protocol.SubstrateRef{
				{
					URI:      outURI,
					Version:  ver,
					ByteSize: int64(len(reduced)),
				},
			},
		}, nil
	})

	// Pre-populate input in substrate
	inURI := "sub://sess-test/raw-log"
	v1, err := sub.PutArtifact(ctx, inURI, 0, []byte("FATAL: OOM Panic in container"), time.Minute)
	if err != nil {
		t.Fatalf("put raw log failed: %v", err)
	}

	// Prepare envelope
	env := protocol.NewEnvelope("env-1", "tr-1", "sess-test", protocol.PlaneLateral, protocol.MsgTaskDispatch, "test-runner")
	env.TargetCapability = "reduce-logs"
	env.SubstrateRefs = append(env.SubstrateRefs, protocol.SubstrateRef{
		URI:     inURI,
		Version: v1,
	})

	// Execute step
	res, err := arm.ExecuteStep(ctx, env)
	if err != nil {
		t.Fatalf("execute step failed: %v", err)
	}
	if res.Status != "SUCCESS" {
		t.Fatalf("expected status SUCCESS, got: %s", res.Status)
	}
	if len(res.SubstrateRefs) != 1 {
		t.Fatalf("expected 1 output substrate ref, got: %d", len(res.SubstrateRefs))
	}

	// Verify reduced artifact in substrate
	reducedData, ver, err := sub.GetArtifact(ctx, res.SubstrateRefs[0].URI)
	if err != nil {
		t.Fatalf("get reduced artifact failed: %v", err)
	}
	expected := "REDUCED: FATAL: OOM Panic in container"
	if string(reducedData) != expected || ver != 1 {
		t.Fatalf("unexpected reduced content: %s, v%d", string(reducedData), ver)
	}

	// Telemetry check
	telem, err := arm.Telemetry(ctx)
	if err != nil {
		t.Fatalf("telemetry check failed: %v", err)
	}
	if telem.CompletedTasks != 1 {
		t.Errorf("expected 1 completed task, got: %d", telem.CompletedTasks)
	}
}
