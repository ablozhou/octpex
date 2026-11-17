package main

import (
	"context"
	"fmt"
	"octpex/pkg/brain"
	"octpex/pkg/bus"
	"octpex/pkg/protocol"
	"octpex/pkg/ring"
	"octpex/pkg/sandbox"
	"octpex/pkg/substrate"
	"octpex/pkg/tool"
	"strings"
	"time"
)

func main() {
	fmt.Println("================================================================================")
	fmt.Println("       OCTPEX BIOMIMETIC AGENT NERVOUS SYSTEM - SELF-EVOLVING PARSER DEMO       ")
	fmt.Println("================================================================================")

	ctx := context.Background()

	// 1. Initialize Basal Nerve Ring components
	sharedSubstrate := substrate.NewInMemorySubstrate()
	nervousBus := bus.NewLocalNervousBus(256)
	registry := ring.NewCapabilityRegistry(15 * time.Second)
	dynamicMCP := tool.NewDynamicMCPBox()
	abloexBox := sandbox.NewAbloExSandbox(sandbox.DefaultSandboxQuota())
	dynamicMCP.SetSecurityScanner(abloexBox)

	// 2. Initialize Octo-Brain
	octoBrain := brain.NewOctoBrain(nil)
	_ = sharedSubstrate
	_ = octoBrain

	fmt.Println("[Init] Nervous Ring, AbloEx Sandbox, and Octo-Brain successfully online.")

	// 3. Register Arm 1: Edge Parser (Level 1 Edge SLM Worker)
	armManifest := protocol.ArmManifest{
		ArmID:          "arm-parser-01",
		RoleName:       "Proprietary Log Parser",
		CapabilityTags: []string{"proprietary_parser"},
		RuntimeAdapter: "ollama",
		Endpoint:       "in-process",
		SLMProfile: protocol.SLMProfile{
			Provider:         "ollama",
			Model:            "qwen2.5-coder:0.5b",
			SafeContextLimit: 4096,
		},
		EscalationGates: protocol.DefaultEscalationGates(),
		Telemetry: protocol.ArmTelemetry{
			Status: "HEALTHY",
		},
	}
	_ = registry.RegisterArm(&armManifest)

	// Simulated Arm Execution state: Initially does not have the parser tool
	nervousBus.Subscribe("proprietary_parser", func(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
		rawInput := env.Parameters["raw_log"].(string)

		// Check if DynamicMCPBox has the specialized tool installed
		toolSchema, err := dynamicMCP.InspectTool(ctx, "parse_octo_v2")
		if err == nil && toolSchema != nil {
			// Fast path: Edge arm uses distilled tool locally!
			fmt.Println("  -> [Arm 1] MATCH FOUND: Tool 'parse_octo_v2' found in local DynamicMCPBox!")
			res, err := dynamicMCP.InvokeTool(ctx, "parse_octo_v2", map[string]any{"raw": rawInput})
			if err != nil {
				return nil, err
			}
			return &protocol.StepResult{
				Status:        "SUCCESS",
				OutputMessage: res.Output,
			}, nil
		}

		// Edge Arm attempts local parsing without tool
		fmt.Println("  -> [Arm 1] Encountered unseen format: 'OCTO-V2|SYS_ALERT|ERR_CODE:0x7F'")
		fmt.Println("  -> [Arm 1] Gate 2 Schema Validation FAILED: Unknown binary header format.")
		fmt.Println("  -> [Arm 1] Gate 3 Confidence Logprob (-0.89) below threshold (-0.51). Retries exhausted.")
		fmt.Println("  -> [Arm 1] TRIPPING VERTICAL ESCALATION TO OCTO-BRAIN...")

		return &protocol.StepResult{
			Status:        "ESCALATED",
			OutputMessage: "Schema mismatch: Unrecognized protocol header OCTO-V2",
			Metadata: map[string]any{
				"raw_sample":     rawInput,
				"escalation_gate": "Gate 2 Schema Verification Failure",
			},
		}, nil
	})

	// ============================================================================
	// PHASE 1: FIRST REQUEST WITH NOVEL PROTOCOL FORMAT (Triggers Self-Evolution)
	// ============================================================================
	sessionID := "session-evo-001"
	novelLog := "OCTO-V2|SYS_ALERT|ERR_CODE:0x7F|LATENCY:142ms|MSG:Cache cluster degraded"

	fmt.Printf("\n[Step 1] Central Brain dispatches unknown protocol log to Arm 1...\n")
	env1 := &protocol.NervousEnvelope{
		EnvelopeID:       "env-evo-001",
		TraceID:          "trace-evo-001",
		SessionID:        sessionID,
		Plane:            protocol.PlaneVertical,
		MessageType:      protocol.MsgTaskDispatch,
		SourceNode:       "octo-brain",
		TargetCapability: "proprietary_parser",
		Parameters: map[string]any{
			"raw_log": novelLog,
		},
		Security: protocol.DefaultSecurityContext(sessionID),
	}

	res1, _ := nervousBus.Publish(ctx, env1)
	fmt.Printf("[Result 1] Arm 1 returned status: %s (Reason: %s)\n", res1.Status, res1.OutputMessage)

	// Central Brain Arbitrates Escalation
	fmt.Println("\n[Step 2] Central Brain receives escalation. Activating Self-Evolution Loop...")
	fmt.Println("  -> [Brain] Synthesizing dedicated Go/Python tool for protocol 'OCTO-V2'...")

	// Synthesize tool code
	synthesizedCode := `
package main
func ParseOctoV2(raw string) map[string]string {
    // Extracted parser logic for OCTO-V2 header
    return map[string]string{"protocol": "OCTO-V2", "level": "SYS_ALERT", "code": "0x7F", "status": "PARSED"}
}
`
	fmt.Println("  -> [Brain] Code synthesized. Submitting to AbloEx Pre-Admission Security Scanner...")

	// Pass through AbloEx Static AST Pre-Admission Scan
	err := dynamicMCP.RegisterDynamicScript(
		"parse_octo_v2",
		"Synthesized parser for OCTO-V2 proprietary protocol logs",
		synthesizedCode,
	)
	if err != nil {
		panic(fmt.Sprintf("AbloEx admission rejected: %v", err))
	}

	// Register invocation handler for the newly admitted tool
	dynamicMCP.RegisterTool(&tool.ToolSchema{
		Name:        "parse_octo_v2",
		Description: "Parses OCTO-V2 logs",
		IsDynamic:   true,
	}, func(ctx context.Context, params map[string]any) (*tool.ToolResult, error) {
		raw := params["raw"].(string)
		parts := strings.Split(raw, "|")
		out := fmt.Sprintf(`{"protocol":"%s","alert_type":"%s","error_code":"%s","status":"SUCCESS_PARSED"}`,
			parts[0], parts[1], parts[2])
		return &tool.ToolResult{
			ToolName: "parse_octo_v2",
			Success:  true,
			Output:   out,
		}, nil
	})

	fmt.Println("  -> [AbloEx] Static AST Scan: CLEAN (0 wiper calls, memory <= 256MB quota).")
	fmt.Println("  -> [Brain] Tool 'parse_octo_v2' officially admitted and broadcasted to Basal Nerve Ring!")

	// Brain retries task with newly synthesized tool
	res1Retry, err := nervousBus.Publish(ctx, env1)
	if err != nil {
		panic(err)
	}
	fmt.Printf("  -> [Brain Arbitration Resolution] Task successfully resolved: %s\n", res1Retry.OutputMessage)

	// ============================================================================
	// PHASE 2: SECOND REQUEST WITH NOVEL PROTOCOL FORMAT (Zero Cloud Tokens!)
	// ============================================================================
	fmt.Println("\n================================================================================")
	fmt.Println(" PHASE 2: SUBSEQUENT NOVEL REQUEST ARRIVES (Autonomous Edge Execution)         ")
	fmt.Println("================================================================================")

	secondLog := "OCTO-V2|SYS_ALERT|ERR_CODE:0x3A|LATENCY:89ms|MSG:Disk IO throttle"
	env2 := &protocol.NervousEnvelope{
		EnvelopeID:       "env-evo-002",
		TraceID:          "trace-evo-002",
		SessionID:        "session-evo-002",
		Plane:            protocol.PlaneVertical,
		MessageType:      protocol.MsgTaskDispatch,
		SourceNode:       "octo-brain",
		TargetCapability: "proprietary_parser",
		Parameters: map[string]any{
			"raw_log": secondLog,
		},
		Security: protocol.DefaultSecurityContext("session-evo-002"),
	}

	start := time.Now()
	res2, err := nervousBus.Publish(ctx, env2)
	if err != nil {
		panic(err)
	}
	elapsed := time.Since(start)

	fmt.Printf("\n[Result 2] Arm 1 Execution Output : %s\n", res2.OutputMessage)
	fmt.Printf("[Result 2] Execution Status      : %s\n", res2.Status)
	fmt.Printf("[Result 2] Local Latency         : %v\n", elapsed)
	fmt.Printf("[Result 2] Central Brain LLM Cost: 0 Tokens (Self-Evolution Loop Closed!)\n")
	fmt.Println("================================================================================")
}
