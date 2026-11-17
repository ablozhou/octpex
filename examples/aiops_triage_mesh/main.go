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
	fmt.Println("         OCTPEX BIOMIMETIC AGENT NERVOUS SYSTEM - AIOPS TRIAGE MESH             ")
	fmt.Println("================================================================================")

	ctx := context.Background()

	// 1. Initialize Basal Nerve Ring components
	sharedSubstrate := substrate.NewInMemorySubstrate()
	nervousBus := bus.NewLocalNervousBus(512)
	registry := ring.NewCapabilityRegistry(15 * time.Second)
	dynamicMCP := tool.NewDynamicMCPBox()
	abloexBox := sandbox.NewAbloExSandbox(sandbox.DefaultSandboxQuota())
	dynamicMCP.SetSecurityScanner(abloexBox)

	fmt.Println("[Init] Basal Nerve Ring, Substrate, and AbloEx Security Engine ready.")

	// 2. Register Arm 1: Telemetry Triage Worker (Level 1 Edge SLM)
	arm1Manifest := protocol.ArmManifest{
		ArmID:          "arm-aiops-triage-01",
		RoleName:       "Loki Telemetry Triage",
		CapabilityTags: []string{"aiops_triage"},
		RuntimeAdapter: "ollama",
		Endpoint:       "in-process",
		SLMProfile: protocol.SLMProfile{
			Provider:         "ollama",
			Model:            "qwen2.5-coder:0.5b",
			SafeContextLimit: 4096,
		},
		EscalationGates: protocol.DefaultEscalationGates(),
		Telemetry: protocol.ArmTelemetry{
			Status:           "HEALTHY",
			ActiveQueueDepth: 1,
		},
	}
	_ = registry.RegisterArm(&arm1Manifest)

	nervousBus.Subscribe("aiops_triage", func(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
		fmt.Printf("  -> [Arm 1: Telemetry Triage] Triaging alert from %s\n", env.TraceID)

		incidentJSON := []byte(`{"incident_id":"INC-9042","service":"payment-gateway","error":"P99 latency > 2500ms, DB connection pool exhausted"}`)
		uri := fmt.Sprintf("sub://%s/incident.json", env.SessionID)
		ver, err := sharedSubstrate.PutArtifact(ctx, uri, 0, incidentJSON, 10*time.Minute)
		if err != nil {
			return nil, err
		}

		fmt.Printf("  -> [Arm 1] Generated Incident Report at %s (Version: %d)\n", uri, ver)

		// Lateral handoff to Arm 2 (Config Patcher)
		lateralEnv := &protocol.NervousEnvelope{
			EnvelopeID:       fmt.Sprintf("env-lat-%d", time.Now().UnixNano()),
			TraceID:          env.TraceID,
			SessionID:        env.SessionID,
			Plane:            protocol.PlaneLateral,
			TargetCapability: "config_patcher",
			SourceNode:       "arm-aiops-triage-01",
			HopCount:         0,
			SubstrateRefs: []protocol.SubstrateRef{
				{URI: uri, Version: ver, ContentType: "application/json"},
			},
			Security: env.Security,
		}

		return nervousBus.Publish(ctx, lateralEnv)
	})

	// 3. Register Arm 2: K8s Config Patcher (Level 1 Edge SLM)
	arm2Manifest := protocol.ArmManifest{
		ArmID:          "arm-aiops-patcher-02",
		RoleName:       "K8s AST Config Patcher",
		CapabilityTags: []string{"config_patcher"},
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

	nervousBus.Subscribe("config_patcher", func(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
		fmt.Printf("  -> [Arm 2: Config Patcher] Lateral hop %d (MaxAllowed: %d). Reading incident...\n",
			env.HopCount, env.Security.MaxAllowedHops)

		ref := env.SubstrateRefs[0]
		incidentData, _, err := sharedSubstrate.GetArtifact(ctx, ref.URI)
		if err != nil {
			return nil, err
		}

		fmt.Printf("  -> [Arm 2] Ingested incident data (%d bytes). Synthesizing K8s hotfix config...\n", len(incidentData))

		k8sPatch := []byte("apiVersion: apps/v1\nkind: Deployment\nspec:\n  replicas: 8\n  template:\n    spec:\n      containers:\n      - env:\n        - name: DB_POOL_SIZE\n          value: \"64\"")
		patchURI := fmt.Sprintf("sub://%s/k8s_patch.yaml", env.SessionID)
		patchVer, err := sharedSubstrate.PutArtifact(ctx, patchURI, 0, k8sPatch, 10*time.Minute)
		if err != nil {
			return nil, err
		}

		// Lateral handoff to Arm 3 (Canary Verifier)
		canaryEnv := &protocol.NervousEnvelope{
			EnvelopeID:       fmt.Sprintf("env-canary-%d", time.Now().UnixNano()),
			TraceID:          env.TraceID,
			SessionID:        env.SessionID,
			Plane:            protocol.PlaneLateral,
			TargetCapability: "canary_verifier",
			SourceNode:       "arm-aiops-patcher-02",
			HopCount:         env.HopCount,
			SubstrateRefs: []protocol.SubstrateRef{
				{URI: patchURI, Version: patchVer, ContentType: "application/x-yaml"},
			},
			Security: env.Security,
		}

		return nervousBus.Publish(ctx, canaryEnv)
	})

	// 4. Register Arm 3: Canary Verifier (Level 0 Native FSM)
	arm3FSM := adapter.NewNativeFSMAdapter(sharedSubstrate)
	arm3FSM.RegisterHandler("canary_verifier", func(ctx context.Context, env *protocol.NervousEnvelope, sub substrate.DataSubstrateSPI) (*protocol.StepResult, error) {
		return &protocol.StepResult{
			Status:        "SUCCESS",
			OutputMessage: "CANARY_DEPLOYMENT_HEALTHY: P99 latency dropped to 48ms, ErrorRate=0.00%",
			SubstrateRefs: env.SubstrateRefs,
		}, nil
	})

	arm3Manifest := protocol.ArmManifest{
		ArmID:          "arm-aiops-canary-03",
		RoleName:       "Prometheus Canary Verifier",
		CapabilityTags: []string{"canary_verifier"},
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

	nervousBus.Subscribe("canary_verifier", func(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
		fmt.Printf("  -> [Arm 3: Canary Verifier] Running canary Prometheus SLA gate (Hop %d)...\n", env.HopCount)
		return arm3FSM.ExecuteStep(ctx, env)
	})

	// 5. Central Brain Dispatches Incident Alert
	sessionID := "sess-incident-9042"
	traceID := "trace-aiops-mesh"

	rootEnv := &protocol.NervousEnvelope{
		EnvelopeID:       "env-aiops-root",
		TraceID:          traceID,
		SessionID:        sessionID,
		Plane:            protocol.PlaneVertical,
		MessageType:      protocol.MsgTaskDispatch,
		SourceNode:       "octo-brain",
		TargetCapability: "aiops_triage",
		Security:         protocol.DefaultSecurityContext(sessionID),
	}

	fmt.Printf("\n[Dispatch] Central Brain dispatching alert to AIOps Triage Mesh...\n\n")
	res, err := nervousBus.Publish(ctx, rootEnv)
	if err != nil {
		panic(err)
	}

	fmt.Println("\n================================================================================")
	fmt.Println("                        TRIAGE PIPELINE RESULTS                                 ")
	fmt.Println("================================================================================")
	fmt.Printf("Resolution Status : %s\n", res.Status)
	fmt.Printf("Canary Result     : %s\n", res.OutputMessage)
	fmt.Printf("Cloud Tokens Used : 0 Tokens (100%% Resolved On-Premise via Edge Mesh)\n")
	fmt.Println("================================================================================")
}
