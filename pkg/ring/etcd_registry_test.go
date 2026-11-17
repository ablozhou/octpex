package ring

import (
	"context"
	"octpex/pkg/protocol"
	"testing"
	"time"
)

func TestETCDCapabilityRegistry_LifecycleAndAutoEviction(t *testing.T) {
	mockETCD := NewMockETCDClient()
	reg, err := NewETCDCapabilityRegistry(mockETCD, 2)
	if err != nil {
		t.Fatalf("failed to create etcd registry: %v", err)
	}
	defer reg.Close()

	ctx := context.Background()

	arm1 := &protocol.ArmManifest{
		ArmID:          "arm-worker-01",
		RuntimeAdapter: "go-native-fsm",
		RoleName:       "Worker 1",
		CapabilityTags: []string{"metric_reducer"},
		Endpoint:       "http://10.0.0.1:8080",
		Telemetry: protocol.ArmTelemetry{
			Status:           "HEALTHY",
			ActiveQueueDepth: 5,
		},
	}

	arm2 := &protocol.ArmManifest{
		ArmID:          "arm-worker-02",
		RuntimeAdapter: "go-native-fsm",
		RoleName:       "Worker 2",
		CapabilityTags: []string{"metric_reducer"},
		Endpoint:       "http://10.0.0.2:8080",
		Telemetry: protocol.ArmTelemetry{
			Status:           "HEALTHY",
			ActiveQueueDepth: 1, // Lower queue
		},
	}

	// 1. Register arm 1 and arm 2
	if err := reg.RegisterArm(ctx, arm1); err != nil {
		t.Fatalf("failed to register arm1: %v", err)
	}
	if err := reg.RegisterArm(ctx, arm2); err != nil {
		t.Fatalf("failed to register arm2: %v", err)
	}

	// 2. Check active arms count
	active := reg.ListHealthyArms()
	if len(active) != 2 {
		t.Fatalf("expected 2 active arms, got %d", len(active))
	}

	// 3. Routing must pick Arm 2 (active queue depth 1 < 5)
	chosen, err := reg.RouteLowestQueue("metric_reducer")
	if err != nil {
		t.Fatalf("RouteLowestQueue failed: %v", err)
	}
	if chosen.ArmID != "arm-worker-02" {
		t.Fatalf("expected arm-worker-02, got %s", chosen.ArmID)
	}

	// 4. Simulate pod crash for Arm 2 (etcd key deleted)
	mockETCD.SimulateCrash("/octpex/ring/arms/arm-worker-02")
	time.Sleep(10 * time.Millisecond) // Let watcher process

	// 5. Arm 2 must be purged, only Arm 1 remains
	activeAfterCrash := reg.ListHealthyArms()
	if len(activeAfterCrash) != 1 || activeAfterCrash[0].ArmID != "arm-worker-01" {
		t.Fatalf("expected only arm-worker-01 after crash, got %d arms", len(activeAfterCrash))
	}

	// 6. Graceful unregister
	if err := reg.UnregisterArm(ctx, "arm-worker-01"); err != nil {
		t.Fatalf("failed to unregister arm: %v", err)
	}
	if len(reg.ListHealthyArms()) != 0 {
		t.Fatalf("expected 0 arms after unregister")
	}
}
