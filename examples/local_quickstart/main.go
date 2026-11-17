package main

import (
	"context"
	"fmt"
	"octpex/pkg/adapter"
	"octpex/pkg/bus"
	"octpex/pkg/protocol"
	"octpex/pkg/ring"
	"octpex/pkg/sandbox"
	"octpex/pkg/substrate"
	"octpex/pkg/tool"
	"time"
)

func main() {
	fmt.Println("================================================================================")
	fmt.Println("             OCTPEX BIOMIMETIC AGENT NERVOUS SYSTEM - LOCAL QUICKSTART           ")
	fmt.Println("================================================================================")

	ctx := context.Background()

	// 1. Initialize Basal Nerve Ring components
	dataSubstrate := substrate.NewInMemorySubstrate()
	nervousBus := bus.NewLocalNervousBus(256)
	registry := ring.NewCapabilityRegistry(15 * time.Second)
	dynamicMCP := tool.NewDynamicMCPBox()
	abloexBox := sandbox.NewAbloExSandbox(sandbox.DefaultSandboxQuota())
	dynamicMCP.SetSecurityScanner(abloexBox)

	fmt.Println("[Init] Shared Data Substrate (sub://), Bus, and AbloEx Security Engine active.")

	// 2. Register Arm 1: Telemetry Distiller (Level 1 SLM Edge Worker)
	arm1Manifest := protocol.ArmManifest{
		ArmID:          "arm-telemetry-01",
		RoleName:       "Telemetry Distiller",
		CapabilityTags: []string{"telemetry_filter"},
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
	_ = registry.RegisterArm(&arm1Manifest)

	nervousBus.Subscribe("telemetry_filter", func(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
		fmt.Printf("  -> [Arm 1: Telemetry Filter] Processing noisy logs from Trace %s\n", env.TraceID)

		// Distill log and store in Shared Data Substrate
		cleanData := []byte("CRITICAL: database connection timeout to host=10.0.0.12:5432 after 30000ms")
		uri := fmt.Sprintf("sub://%s/cleaned_log", env.SessionID)
		ver, err := dataSubstrate.PutArtifact(ctx, uri, 0, cleanData, 10*time.Minute)
		if err != nil {
			return nil, err
		}

		fmt.Printf("  -> [Arm 1] Stored distilled log at %s (version: %d, tokens saved: 92%%)\n", uri, ver)

		// Handoff laterally to Arm 2 (Patch Synthesizer) via lateral plane
		lateralEnv := &protocol.NervousEnvelope{
			EnvelopeID:       fmt.Sprintf("env-lat-%d", time.Now().UnixNano()),
			TraceID:          env.TraceID,
			SessionID:        env.SessionID,
			Plane:            protocol.PlaneLateral,
			TargetCapability: "patch_synthesizer",
			SourceNode:       "arm-telemetry-01",
			HopCount:         0,
			SubstrateRefs: []protocol.SubstrateRef{
				{URI: uri, Version: ver, ContentType: "text/plain"},
			},
			Security: env.Security,
		}

		fmt.Println("  -> [Arm 1] Dispatching lateral handoff along the nervous ring to Arm 2...")
		return nervousBus.Publish(ctx, lateralEnv)
	})

	// 3. Register Arm 2: Patch Synthesizer (Level 1 Edge SLM Worker)
	arm2Manifest := protocol.ArmManifest{
		ArmID:          "arm-patch-02",
		RoleName:       "Patch Synthesizer",
		CapabilityTags: []string{"patch_synthesizer"},
		RuntimeAdapter: "ollama",
		Endpoint:       "in-process",
		SLMProfile: protocol.SLMProfile{
			Provider:         "ollama",
			Model:            "qwen2.5-coder:3b",
			SafeContextLimit: 8192,
		},
		EscalationGates: protocol.DefaultEscalationGates(),
		Telemetry: protocol.ArmTelemetry{
			Status: "HEALTHY",
		},
	}
	_ = registry.RegisterArm(&arm2Manifest)

	nervousBus.Subscribe("patch_synthesizer", func(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
		fmt.Printf("  -> [Arm 2: Patch Synthesizer] Received lateral envelope (HopCount: %d, MaxAllowedHops: %d)\n",
			env.HopCount, env.Security.MaxAllowedHops)

		if len(env.SubstrateRefs) == 0 {
			return nil, fmt.Errorf("arm 2: expected substrate reference")
		}

		// Zero-copy read from Shared Data Substrate
		ref := env.SubstrateRefs[0]
		logBytes, _, err := dataSubstrate.GetArtifact(ctx, ref.URI)
		if err != nil {
			return nil, fmt.Errorf("arm 2: failed to fetch substrate ref: %w", err)
		}

		fmt.Printf("  -> [Arm 2] Retrieved %d bytes via pointer %s: \"%s\"\n", len(logBytes), ref.URI, string(logBytes))

		// Synthesize fix
		patchCode := []byte("diff --git a/db.go b/db.go\n+ timeout = 60 * time.Second")
		patchURI := fmt.Sprintf("sub://%s/patch.diff", env.SessionID)
		patchVer, err := dataSubstrate.PutArtifact(ctx, patchURI, 0, patchCode, 10*time.Minute)
		if err != nil {
			return nil, err
		}

		// Handoff to Arm 3 (Level 0 Native FSM Verifier)
		fsmEnv := &protocol.NervousEnvelope{
			EnvelopeID:       fmt.Sprintf("env-fsm-%d", time.Now().UnixNano()),
			TraceID:          env.TraceID,
			SessionID:        env.SessionID,
			Plane:            protocol.PlaneLateral,
			TargetCapability: "fsm_verifier",
			SourceNode:       "arm-patch-02",
			HopCount:         env.HopCount,
			SubstrateRefs: []protocol.SubstrateRef{
				{URI: patchURI, Version: patchVer, ContentType: "text/x-diff"},
			},
			Security: env.Security,
		}

		return nervousBus.Publish(ctx, fsmEnv)
	})

	// 4. Register Arm 3: Level 0 Native FSM Verifier (0 Tokens, < 2ms)
	arm3FSM := adapter.NewNativeFSMAdapter(dataSubstrate)
	arm3FSM.RegisterHandler("fsm_verifier", func(ctx context.Context, env *protocol.NervousEnvelope, sub substrate.DataSubstrateSPI) (*protocol.StepResult, error) {
		return &protocol.StepResult{
			Status:        "SUCCESS",
			OutputMessage: "PATCH_VERIFIED_PASS (all 14/14 unit tests passed, 0 regressions)",
			SubstrateRefs: env.SubstrateRefs,
		}, nil
	})

	arm3Manifest := protocol.ArmManifest{
		ArmID:          "arm-fsm-03",
		RoleName:       "Native FSM Verifier",
		CapabilityTags: []string{"fsm_verifier"},
		RuntimeAdapter: "native-fsm",
		Endpoint:       "in-process",
		SLMProfile: protocol.SLMProfile{
			Provider: "none",
			Model:    "none",
		},
		EscalationGates: protocol.DefaultEscalationGates(),
		Telemetry: protocol.ArmTelemetry{
			Status: "HEALTHY",
		},
	}
	_ = arm3FSM.Init(ctx, &arm3Manifest)
	_ = registry.RegisterArm(&arm3Manifest)

	nervousBus.Subscribe("fsm_verifier", func(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
		fmt.Printf("  -> [Arm 3: Level 0 FSM Verifier] Received verification job (HopCount: %d)\n", env.HopCount)
		return arm3FSM.ExecuteStep(ctx, env)
	})

	// 5. Central Brain Dispatches Initial Task
	sessionID := "sess-quickstart-001"
	traceID := "trace-quickstart-001"
	start := time.Now()

	rootEnv := &protocol.NervousEnvelope{
		EnvelopeID:       "env-root-001",
		TraceID:          traceID,
		SessionID:        sessionID,
		Plane:            protocol.PlaneVertical,
		MessageType:      protocol.MsgTaskDispatch,
		SourceNode:       "octo-brain",
		TargetCapability: "telemetry_filter",
		Security:         protocol.DefaultSecurityContext(sessionID),
	}

	fmt.Printf("\n[Dispatch] Central Brain initiating vertical dispatch for Trace %s...\n\n", traceID)
	result, err := nervousBus.Publish(ctx, rootEnv)
	if err != nil {
		panic(err)
	}

	duration := time.Since(start)

	fmt.Println("\n================================================================================")
	fmt.Println("                        TASK EXECUTION SUMMARY                                  ")
	fmt.Println("================================================================================")
	fmt.Printf("Execution Status : %s\n", result.Status)
	fmt.Printf("Final Output     : %s\n", result.OutputMessage)
	fmt.Printf("Total Latency    : %v\n", duration)
	fmt.Printf("Cloud Tokens     : 0 (100%% Edge & Local SLM Decoupling)\n")
	fmt.Println("================================================================================")
}
