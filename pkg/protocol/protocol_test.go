package protocol

import (
	"testing"
)

func TestSubstrateRefValidation(t *testing.T) {
	validRef := SubstrateRef{
		URI:         "sub://sess-101/parsed-ast",
		Version:     1,
		ContentType: "application/json",
		ByteSize:    512,
	}
	if err := validRef.Validate(); err != nil {
		t.Fatalf("expected valid ref, got error: %v", err)
	}

	invalidRef := SubstrateRef{
		URI:     "http://example.com/data",
		Version: 1,
	}
	if err := invalidRef.Validate(); err == nil {
		t.Fatal("expected error for non-sub:// URI, got nil")
	}

	invalidVersion := SubstrateRef{
		URI:     "sub://sess-101/item",
		Version: 0,
	}
	if err := invalidVersion.Validate(); err == nil {
		t.Fatal("expected error for version < 1, got nil")
	}
}

func TestParseSubstrateURI(t *testing.T) {
	sessionID, artifactID, err := ParseSubstrateURI("sub://session-999/artifacts/patch.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessionID != "session-999" {
		t.Errorf("expected session-999, got: %s", sessionID)
	}
	if artifactID != "artifacts/patch.yaml" {
		t.Errorf("expected artifacts/patch.yaml, got: %s", artifactID)
	}
}

func TestEnvelopeValidation(t *testing.T) {
	env := NewEnvelope("env-1", "trace-1", "sess-1", PlaneVertical, MsgTaskDispatch, "brain-core")
	env.TargetCapability = "telemetry-distiller"
	if err := env.Validate(); err != nil {
		t.Fatalf("expected valid envelope, got: %v", err)
	}

	env.SubstrateRefs = append(env.SubstrateRefs, SubstrateRef{
		URI:     "sub://sess-1/log-entry",
		Version: 1,
	})
	if err := env.Validate(); err != nil {
		t.Fatalf("expected valid envelope with substrate ref, got: %v", err)
	}

	// Negative hop count test
	env.HopCount = -1
	if err := env.Validate(); err == nil {
		t.Fatal("expected error for negative hop count")
	}
}

func TestArmManifest(t *testing.T) {
	m := ArmManifest{
		ArmID:          "arm-01",
		RuntimeAdapter: "go-native-fsm",
		RoleName:       "telemetry-distiller",
		CapabilityTags: []string{"log-parsing", "dedup"},
		Endpoint:       "local://arm-01",
		EscalationGates: DefaultEscalationGates(),
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("expected valid manifest, got: %v", err)
	}
	if !m.HasCapability("log-parsing") {
		t.Error("expected arm to have capability log-parsing")
	}
	if m.HasCapability("compiler") {
		t.Error("expected arm to not have capability compiler")
	}
}
