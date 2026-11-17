package test

import (
	"context"
	"octpex/pkg/brain"
	"octpex/pkg/gates"
	"octpex/pkg/protocol"
	"octpex/pkg/ring"
	"octpex/pkg/sandbox"
	"octpex/pkg/substrate"
	"octpex/pkg/tool"
	"testing"
	"time"
)

func TestSelfEvolutionAndDynamicMCPDistillationLoop(t *testing.T) {
	ctx := context.Background()

	// 1. Initialize Substrate, Dynamic MCP Box with AbloEx Sandbox, Capability Registry, and Octo-Brain
	sub := substrate.NewInMemorySubstrate()
	mcpBox := tool.NewDynamicMCPBox()
	abloex := sandbox.NewAbloExSandbox(sandbox.DefaultSandboxQuota())
	mcpBox.SetSecurityScanner(abloex)
	registry := ring.NewCapabilityRegistry(10 * time.Second)
	octoBrain := brain.NewOctoBrain(nil) // Rule-based arbitrator

	// 2. Setup Arm 1: Specialized worker that initially lacks a tool
	arm1Manifest := &protocol.ArmManifest{
		ArmID:          "arm-worker-01",
		RuntimeAdapter: "go-native-fsm",
		RoleName:       "database-migrator",
		CapabilityTags: []string{"migrate-db"},
		Endpoint:       "local://arm-worker-01",
		EscalationGates: protocol.EscalationGates{
			MaxReflexRetries: 3,
		},
		Telemetry: protocol.ArmTelemetry{
			Status: "HEALTHY",
		},
	}
	if err := registry.RegisterArm(arm1Manifest); err != nil {
		t.Fatalf("register arm failed: %v", err)
	}

	// 3. Task 1 Arrives: Fails because tool is missing -> Triggers Escalation
	env1 := protocol.NewEnvelope("env-task-1", "tr-101", "sess-101", protocol.PlaneVertical, protocol.MsgTaskDispatch, "octo-brain")
	env1.TargetCapability = "migrate-db"

	// Worker fails Gate 2 and creates an EscalationPayload
	escPayload := gates.BuildEscalation(
		env1,
		protocol.Gate2VerifierFailed,
		"tool 'dynamic-db-patcher' not found in local ganglion",
		3,
		[]string{"attempt 1: missing tool", "attempt 2: missing tool", "attempt 3: missing tool"},
	)

	// 4. Octo-Brain arbitrates the escalation and synthesizes a Dynamic MCP tool
	decision, err := octoBrain.Arbitrate(ctx, escPayload)
	if err != nil {
		t.Fatalf("brain arbitration failed: %v", err)
	}
	if decision.Decision != brain.DecisionSynthesizeDynamicTool {
		t.Fatalf("expected DecisionSynthesizeDynamicTool, got: %s", decision.Decision)
	}
	if decision.SynthesizedToolCode == "" {
		t.Fatal("expected synthesized tool code in decision")
	}

	// 5. Basal Nerve Ring: Distill & Register synthesized script into Dynamic MCP Box (Guarded by AbloEx Sandbox)
	if err := mcpBox.RegisterDynamicScript(
		decision.DynamicToolName,
		"Runtime synthesized database migration tool",
		decision.SynthesizedToolCode,
	); err != nil {
		t.Fatalf("register dynamic script failed: %v", err)
	}

	// Verify dynamic tool is in the box
	dynToolSchema, err := mcpBox.InspectTool(ctx, decision.DynamicToolName)
	if err != nil {
		t.Fatalf("inspect tool failed: %v", err)
	}
	if !dynToolSchema.IsDynamic {
		t.Error("expected tool to be marked as dynamic")
	}

	// 6. Future Task 2 Arrives: Resolved 100% locally via the newly distilled Dynamic Tool!
	toolResult, err := mcpBox.InvokeTool(ctx, decision.DynamicToolName, map[string]any{"dry_run": true})
	if err != nil {
		t.Fatalf("invoke distilled tool failed: %v", err)
	}
	if !toolResult.Success {
		t.Errorf("expected dynamic tool execution success")
	}

	// Write successful execution result to Shared Data Substrate
	outURI := "sub://sess-101/migration-complete"
	v1, err := sub.PutArtifact(ctx, outURI, 0, []byte(toolResult.Output), time.Minute)
	if err != nil {
		t.Fatalf("put artifact failed: %v", err)
	}
	if v1 != 1 {
		t.Errorf("expected version 1, got: %d", v1)
	}

	t.Logf("Self-Evolution Loop passed: Capability gap escalated to Brain -> Dynamic MCP tool synthesized -> Task resolved locally without cloud LLM!")
}
