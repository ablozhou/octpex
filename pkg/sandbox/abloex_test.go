package sandbox

import (
	"context"
	"octpex/pkg/protocol"
	"octpex/pkg/tool"
	"os"
	"strings"
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
	if err != tool.ErrNoScriptRunner || res != nil {
		t.Fatalf("unexpected result without runner: %+v, %v", res, err)
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
	if err != tool.ErrNoScriptRunner || res != nil {
		t.Fatalf("admitted tool requires a runner: %v", err)
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

func TestDockerSandboxIntegration(t *testing.T) {
	image := os.Getenv("OCTPEX_TEST_SANDBOX_IMAGE")
	if image == "" {
		t.Skip("set OCTPEX_TEST_SANDBOX_IMAGE to a locally available Python image")
	}
	runner := NewDockerSandbox(DefaultSandboxQuota(), image)
	box := tool.NewDynamicMCPBox()
	box.SetSecurityScanner(runner)
	box.SetScriptRunner(runner)
	err := box.RegisterDynamicScript("echo-data", "Return input", `import json,sys; print(json.load(sys.stdin)["name"])`)
	if err != nil {
		t.Fatal(err)
	}
	res, err := box.InvokeTool(context.Background(), "echo-data", map[string]any{"name": "sample"})
	if err != nil || !res.Success || strings.TrimSpace(res.Output) != "sample" {
		t.Fatalf("%+v %v", res, err)
	}
	if _, err := runner.Run(context.Background(), "fail", "raise ValueError('failed')", nil); err == nil {
		t.Fatal("nonzero exit reported success")
	}
}
