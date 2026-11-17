package ring

import (
	"octpex/pkg/protocol"
	"testing"
	"time"
)

func TestCapabilityRegistryLifecycle(t *testing.T) {
	reg := NewCapabilityRegistry(2 * time.Second)

	arm1 := &protocol.ArmManifest{
		ArmID:          "arm-01",
		RuntimeAdapter: "go-native-fsm",
		RoleName:       "telemetry-distiller",
		CapabilityTags: []string{"distill-logs", "metric-filter"},
		Endpoint:       "local://arm-01",
		Telemetry: protocol.ArmTelemetry{
			Status:            "HEALTHY",
			ActiveQueueDepth:  5,
			LastHeartbeatUnix: time.Now().Unix(),
		},
	}

	arm2 := &protocol.ArmManifest{
		ArmID:          "arm-02",
		RuntimeAdapter: "go-native-fsm",
		RoleName:       "telemetry-distiller-replica",
		CapabilityTags: []string{"distill-logs"},
		Endpoint:       "local://arm-02",
		Telemetry: protocol.ArmTelemetry{
			Status:            "HEALTHY",
			ActiveQueueDepth:  1, // Lower queue depth!
			LastHeartbeatUnix: time.Now().Unix(),
		},
	}

	// 1. Register arms
	if err := reg.RegisterArm(arm1); err != nil {
		t.Fatalf("register arm1 failed: %v", err)
	}
	if err := reg.RegisterArm(arm2); err != nil {
		t.Fatalf("register arm2 failed: %v", err)
	}

	// Duplicate registration should fail
	if err := reg.RegisterArm(arm1); err != ErrArmAlreadyRegistered {
		t.Fatalf("expected ErrArmAlreadyRegistered, got: %v", err)
	}

	// 2. Resolve worker with RouteLowestQueue (Arm2 has depth 1 vs Arm1 depth 5)
	resolved, err := reg.ResolveWorker("distill-logs", RouteLowestQueue)
	if err != nil {
		t.Fatalf("resolve worker failed: %v", err)
	}
	if resolved.ArmID != "arm-02" {
		t.Errorf("expected arm-02 (lowest queue), got: %s", resolved.ArmID)
	}

	// 3. Heartbeat update on arm1 to reduce queue depth to 0
	if err := reg.Heartbeat("arm-01", 0, "HEALTHY"); err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}
	resolved, err = reg.ResolveWorker("distill-logs", RouteLowestQueue)
	if err != nil {
		t.Fatalf("resolve worker failed: %v", err)
	}
	if resolved.ArmID != "arm-01" {
		t.Errorf("expected arm-01 (lowest queue after heartbeat), got: %s", resolved.ArmID)
	}

	// 4. Resolve non-existent capability
	_, err = reg.ResolveWorker("non-existent-capability", RouteLowestQueue)
	if err == nil {
		t.Fatal("expected error for non-existent capability, got nil")
	}

	// 5. Deregister
	if err := reg.DeregisterArm("arm-01"); err != nil {
		t.Fatalf("deregister failed: %v", err)
	}
	resolved, err = reg.ResolveWorker("distill-logs", RouteLowestQueue)
	if err != nil {
		t.Fatalf("resolve worker failed after deregistration: %v", err)
	}
	if resolved.ArmID != "arm-02" {
		t.Errorf("expected arm-02 after arm-01 deregistered, got: %s", resolved.ArmID)
	}
}
