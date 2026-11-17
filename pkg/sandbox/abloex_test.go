package sandbox

import (
	"context"
	"octpex/pkg/protocol"
	"testing"
	"time"
)

func TestAbloExSandboxSafeExecution(t *testing.T) {
	ctx := context.Background()
	sandbox := NewAbloExSandbox(DefaultSandboxQuota())

	safeCode := `
def calculate_metrics(data):
    return {"status": "ok", "count": len(data)}
`
	sec := protocol.DefaultSecurityContext("sess-1")
	res, err := sandbox.ExecuteDynamicScript(ctx, "calc-metrics", safeCode, nil, &sec, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success in AbloEx sandbox")
	}
}

func TestAbloExDangerousCodeRejection(t *testing.T) {
	ctx := context.Background()
	sandbox := NewAbloExSandbox(DefaultSandboxQuota())
	sec := protocol.DefaultSecurityContext("sess-danger")

	dangerousSnippets := []string{
		`rm -rf /var/data`,
		`echo "wiping" && mkfs /dev/sda1`,
		`:(){ :|:& };:`,
		`DROP TABLE users;`,
		`import socket; s = socket.socket()`,
	}

	for _, code := range dangerousSnippets {
		_, err := sandbox.ExecuteDynamicScript(ctx, "test-tool", code, nil, &sec, false)
		if err == nil {
			t.Fatalf("expected AbloEx to reject dangerous code: %s", code)
		}
	}
}

func TestAbloExTokenAttenuationEnforcement(t *testing.T) {
	ctx := context.Background()
	sandbox := NewAbloExSandbox(DefaultSandboxQuota())

	// 1. Initial SecurityContext has AllowMutation = true
	sec := protocol.DefaultSecurityContext("sess-attenuation")
	safeMutationCode := `UPDATE config SET memory = 2048`

	res, err := sandbox.ExecuteDynamicScript(ctx, "update-config", safeMutationCode, nil, &sec, true)
	if err != nil {
		t.Fatalf("expected initial mutation permitted, got: %v", err)
	}
	if !res.Success {
		t.Error("expected mutation success")
	}

	// 2. Envelope hops laterally across tentacles: privileges monotonically attenuate!
	sec.Attenuate("some-other-capability")
	sec.Attenuate("read-only-capability") // Hop 2: MaxAllowedHops <= 1 -> AllowMutation revoked!

	if sec.AllowMutation {
		t.Fatal("expected AllowMutation to be revoked after lateral attenuation")
	}

	// 3. Attenuated envelope attempts mutation: AbloEx blocks it!
	_, err = sandbox.ExecuteDynamicScript(ctx, "update-config", safeMutationCode, nil, &sec, true)
	if err != ErrPrivilegeAttenuated {
		t.Fatalf("expected ErrPrivilegeAttenuated, got: %v", err)
	}
}

func TestAbloExTimeoutQuota(t *testing.T) {
	quota := DefaultSandboxQuota()
	quota.MaxTimeout = 10 * time.Millisecond
	sandbox := NewAbloExSandbox(quota)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel context to simulate timeout

	sec := protocol.DefaultSecurityContext("sess-timeout")
	_, err := sandbox.ExecuteDynamicScript(ctx, "timeout-tool", "echo 'hello'", nil, &sec, false)
	if err != ErrSandboxTimeout {
		t.Fatalf("expected ErrSandboxTimeout, got: %v", err)
	}
}
