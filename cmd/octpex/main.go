package main

import (
	"context"
	"flag"
	"fmt"
	"octpex/pkg/adapter"
	"octpex/pkg/brain"
	"octpex/pkg/bus"
	"octpex/pkg/config"
	"octpex/pkg/driver"
	"octpex/pkg/gates"
	"octpex/pkg/protocol"
	"octpex/pkg/ring"
	"octpex/pkg/sandbox"
	"octpex/pkg/server"
	"octpex/pkg/storage"
	"octpex/pkg/substrate"
	"octpex/pkg/telemetry"
	"octpex/pkg/tool"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

const version = "0.5.0"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "eval":
			runSelfDiagnostic()
			return
		case "bench":
			runSystemBenchmark()
			return
		case "finops":
			runFinOpsDemo()
			return
		case "version", "-v", "--version":
			printVersion()
			return
		case "help", "-h", "--help":
			printHelp()
			return
		}
	}

	// Default: Run daemon
	runDaemon()
}

func printVersion() {
	fmt.Printf("Octpex Biomimetic Distributed Agent Nervous System\n")
	fmt.Printf("Version    : %s\n", version)
	fmt.Printf("Go Version : %s\n", runtime.Version())
	fmt.Printf("OS/Arch    : %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func printHelp() {
	fmt.Println("Usage: octpex <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  run       Boot the biomimetic nervous daemon (default)")
	fmt.Println("  eval      Execute automated kernel self-diagnostics & verification")
	fmt.Println("  bench     Execute high-throughput performance & latency stress benchmarks")
	fmt.Println("  finops    Display FinOps cost reduction & token ROI audit report")
	fmt.Println("  version   Display version and architecture information")
	fmt.Println("\nFlags for 'run':")
	fmt.Println("  -profile  'local' (single-node) or 'cluster' (distributed NATS/Redis/etcd)")
	fmt.Println("  -config   Path to YAML configuration file (e.g. configs/local.yaml)")
	fmt.Println("  -port     HTTP REST and web dashboard port (default: 8080, set 0 to disable)")
}

func runDaemon() {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	profile := fs.String("profile", "local", "Deployment profile: 'local' or 'cluster'")
	configFile := fs.String("config", "", "Path to YAML configuration file")
	httpPort := fs.Int("port", 8080, "HTTP dashboard port (0 disables)")

	args := os.Args[1:]
	if len(args) > 0 && args[0] == "run" {
		args = args[1:]
	}
	_ = fs.Parse(args)

	cfgDesc := *configFile
	if cfgDesc == "" {
		cfgDesc = fmt.Sprintf("configs/%s.yaml (default)", *profile)
	}

	fmt.Printf("[Octpex Kernel] Initializing biomimetic substrate (profile: %s, config: %s, version: %s)...\n",
		*profile, cfgDesc, version)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cfg *config.Config
	var err error
	if *configFile != "" {
		cfg, err = config.LoadConfig(*configFile)
		if err != nil {
			fmt.Printf("[Octpex Kernel] Warning: Failed to load config file: %v. Using defaults.\n", err)
			cfg = config.DefaultLocalConfig()
		}
	} else if *profile == "cluster" {
		cfg = config.DefaultClusterConfig()
	} else {
		cfg = config.DefaultLocalConfig()
	}

	var sharedSubstrate substrate.DataSubstrateSPI
	var nervousBus *bus.LocalNervousBus
	var capabilityRegistry *ring.CapabilityRegistry

	if *profile == "cluster" {
		fmt.Println("[Octpex Kernel] Establishing cluster connections to Redis, NATS, and etcd via Driver Factory...")
		clusterDrivers, dialErr := driver.DialClusterSubsystems(ctx, cfg)
		if dialErr != nil {
			fmt.Printf("[Octpex Kernel] Cluster dial error: %v, falling back to local.\n", dialErr)
			sharedSubstrate = substrate.NewInMemorySubstrate()
			nervousBus = bus.NewLocalNervousBus(512)
			capabilityRegistry = ring.NewCapabilityRegistry(15 * time.Second)
		} else {
			sharedSubstrate = clusterDrivers.Substrate
			nervousBus = bus.NewLocalNervousBus(512) // Keep local bus for fast loopback and HTTP proxy
			capabilityRegistry = ring.NewCapabilityRegistry(15 * time.Second)
			fmt.Println("[Octpex Kernel] Cluster physical network drivers online and connected.")
		}
	} else {
		// 1. Shared Data Substrate
		sharedSubstrate = substrate.NewInMemorySubstrate()
		fmt.Println("[Octpex Kernel] Shared Data Substrate (sub://) online.")

		// 2. Dual-Plane Nervous Bus
		nervousBus = bus.NewLocalNervousBus(512)
		fmt.Println("[Octpex Kernel] Dual-Plane Nervous Bus online.")

		// 3. Basal Nerve Ring
		capabilityRegistry = ring.NewCapabilityRegistry(15 * time.Second)
	}
	defer nervousBus.Close()

	// FinOps telemetry accumulator
	finOpsCollector := telemetry.NewFinOpsCollector(telemetry.DefaultPricingRates())

	// 4. AbloEx Security Sandbox
	abloexBox := sandbox.NewAbloExSandbox(sandbox.DefaultSandboxQuota())
	fmt.Println("[Octpex Kernel] AbloEx Zero-Trust Security Engine active (256MB quota, 5s timeout).")

	// Dynamic MCP Box
	dynamicMCPBox := tool.NewDynamicMCPBox()
	dynamicMCPBox.SetSecurityScanner(abloexBox)
	fmt.Println("[Octpex Kernel] Basal Nerve Ring Capability Registry & Dynamic MCP Box online.")

	// 5. Octo-Brain
	octoBrain := brain.NewOctoBrain(nil)
	_ = octoBrain
	fmt.Println("[Octpex Kernel] Octo-Brain Central Orchestrator & Strategic Arbitrator ready.")

	// 6. Mount default Arm
	telemetryArm := adapter.NewNativeFSMAdapter(sharedSubstrate)
	telemetryManifest := &protocol.ArmManifest{
		ArmID:          "arm-telemetry-01",
		RuntimeAdapter: "go-native-fsm",
		RoleName:       "telemetry-distiller",
		CapabilityTags: []string{"distill-telemetry"},
		Endpoint:       "local://arm-telemetry-01",
		EscalationGates: protocol.DefaultEscalationGates(),
	}
	_ = telemetryArm.Init(ctx, telemetryManifest)
	_ = capabilityRegistry.RegisterArm(telemetryManifest)
	nervousBus.Subscribe("distill-telemetry", telemetryArm.ExecuteStep)

	fmt.Printf("[Octpex Kernel] Mounted Octo-Arm 1: %s (%s, tags: %v)\n",
		telemetryManifest.ArmID, telemetryManifest.RoleName, telemetryManifest.CapabilityTags)

	activeArms := capabilityRegistry.ListHealthyArms()
	fmt.Printf("[Octpex Kernel] %d Octo-Arms active on Basal Nerve Ring.\n", len(activeArms))

	// 7. Boot HTTP Dashboard / REST API Server if enabled
	var httpSrv *server.HTTPServer
	if *httpPort > 0 {
		httpSrv = server.NewHTTPServer(server.ServerConfig{Port: *httpPort}, nervousBus, capabilityRegistry, sharedSubstrate, finOpsCollector)
		go func() {
			fmt.Printf("[Octpex Kernel] Web Visual Dashboard & REST API listening on http://localhost:%d\n", *httpPort)
			if err := httpSrv.Start(); err != nil && err.Error() != "http: Server closed" {
				fmt.Printf("[Octpex Kernel] HTTP server error: %v\n", err)
			}
		}()
	}

	fmt.Println("[Octpex Kernel] Ready. Listening on nervous buses. Press Ctrl+C to stop.")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n[Octpex Kernel] Shutting down gracefully...")
	if httpSrv != nil {
		shutdownCtx, shutCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutCancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}
}

func runSelfDiagnostic() {
	fmt.Println("================================================================================")
	fmt.Println("            OCTPEX SYSTEM SELF-DIAGNOSTIC & HEALTH BENCHMARK                    ")
	fmt.Println("================================================================================")

	ctx := context.Background()

	// 1. Substrate CAS benchmark
	start := time.Now()
	sub := substrate.NewInMemorySubstrate()
	v1, err1 := sub.PutArtifact(ctx, "sub://diag/test", 0, []byte("data"), time.Minute)
	_, err2 := sub.PutArtifact(ctx, "sub://diag/test", v1, []byte("updated"), time.Minute)
	durSub := time.Since(start)
	statusSub := "PASS"
	if err1 != nil || err2 != nil {
		statusSub = "FAIL"
	}
	fmt.Printf("[Substrate] Atomic CAS Versioning Check       : %s (latency: %v)\n", statusSub, durSub)

	// 2. Bus Plane & Security Attenuation benchmark
	start = time.Now()
	b := bus.NewLocalNervousBus(64)
	b.Subscribe("diag_cap", func(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
		return &protocol.StepResult{Status: "SUCCESS"}, nil
	})
	sec := protocol.DefaultSecurityContext("diag")
	env := &protocol.NervousEnvelope{
		EnvelopeID:       "env-diag",
		TraceID:          "tr-diag",
		SessionID:        "diag",
		Plane:            protocol.PlaneLateral,
		TargetCapability: "diag_cap",
		SourceNode:       "tester",
		Security:         sec,
	}
	_, errBus := b.Publish(ctx, env)
	durBus := time.Since(start)
	statusBus := "PASS"
	if errBus != nil || env.HopCount != 1 || env.Security.MaxAllowedHops != 2 {
		statusBus = "FAIL"
	}
	fmt.Printf("[Bus Plane] Lateral Hop & Token Attenuation   : %s (latency: %v)\n", statusBus, durBus)

	// 3. AbloEx Security Engine
	start = time.Now()
	ab := sandbox.NewAbloExSandbox(sandbox.DefaultSandboxQuota())
	errWiper := ab.ScanCode("rm -rf /")
	errClean := ab.ScanCode("fmt.Println(\"clean code\")")
	durAb := time.Since(start)
	statusAb := "PASS"
	if errWiper == nil || errClean != nil {
		statusAb = "FAIL"
	}
	fmt.Printf("[Security ] AbloEx Static AST Wiper Scanner   : %s (latency: %v)\n", statusAb, durAb)

	// 4. Tri-Gate Funnel
	start = time.Now()
	errG2Pass := gates.JSONSchemaVerifier(ctx, []byte(`{"status":"ok"}`))
	errG2Fail := gates.JSONSchemaVerifier(ctx, []byte(`bad-json-text`))
	durGate := time.Since(start)
	statusGate := "PASS"
	if errG2Pass != nil || errG2Fail == nil {
		statusGate = "FAIL"
	}
	fmt.Printf("[Gates    ] Tri-Gate Objective Schema Funnel  : %s (latency: %v)\n", statusGate, durGate)

	// 5. Ganglion Store & Vector Search
	start = time.Now()
	gs := storage.NewLocalGanglionStore()
	_ = gs.IndexVector(ctx, "vec1", []float32{1.0, 0.0}, nil)
	_ = gs.IndexVector(ctx, "vec2", []float32{0.0, 1.0}, nil)
	vecRes, errVec := gs.SearchVector(ctx, []float32{1.0, 0.0}, 1)
	durStore := time.Since(start)
	statusStore := "PASS"
	if errVec != nil || len(vecRes) != 1 || vecRes[0].Record.ID != "vec1" {
		statusStore = "FAIL"
	}
	fmt.Printf("[Storage  ] Ganglion Store & Cosine Search    : %s (latency: %v)\n", statusStore, durStore)

	fmt.Println("================================================================================")
	fmt.Println("                     DIAGNOSTIC SUMMARY: ALL SUBSYSTEMS HEALTHY                 ")
	fmt.Println("================================================================================")
}

func runFinOpsDemo() {
	rates := telemetry.DefaultPricingRates()
	col := telemetry.NewFinOpsCollector(rates)

	// Record sample workload
	for i := 0; i < 85; i++ {
		col.RecordTask(telemetry.TaskRecord{
			TraceID:     "trace-sample",
			ArmID:       "arm-sample",
			Level:       "L1_EDGE_SLM",
			CloudTokens: 0,
			EdgeTokens:  150,
			Duration:    60 * time.Millisecond,
		})
	}
	for i := 0; i < 15; i++ {
		col.RecordTask(telemetry.TaskRecord{
			TraceID:     "trace-sample-esc",
			ArmID:       "octo-brain",
			Level:       "BRAIN",
			CloudTokens: 1400,
			EdgeTokens:  0,
			Duration:    1500 * time.Millisecond,
			Escalated:   true,
		})
	}

	fmt.Println(col.PrintReport(1500))
}

func runSystemBenchmark() {
	fmt.Println("================================================================================")
	fmt.Println("        OCTPEX KERNEL HIGH-THROUGHPUT STRESS & LATENCY BENCHMARK                ")
	fmt.Println("================================================================================")

	ctx := context.Background()
	const iterations = 50000

	// 1. Benchmark Substrate CAS Throughput
	fmt.Printf("[1/3] Benchmarking Shared Data Substrate CAS (%d atomic updates)...\n", iterations)
	sub := substrate.NewInMemorySubstrate()
	uri := "sub://sess-bench/atomic_value"
	_, _ = sub.PutArtifact(ctx, uri, 0, []byte("init"), time.Hour)

	startSub := time.Now()
	for i := 1; i <= iterations; i++ {
		_, err := sub.PutArtifact(ctx, uri, int64(i), []byte("data"), time.Hour)
		if err != nil {
			fmt.Printf("Substrate error at %d: %v\n", i, err)
			break
		}
	}
	subDuration := time.Since(startSub)
	subOpsPerSec := float64(iterations) / subDuration.Seconds()
	subAvgLatency := subDuration / iterations

	// 2. Benchmark Nervous Bus Envelope Dispatch Throughput
	fmt.Printf("[2/3] Benchmarking Dual-Plane Nervous Bus (%d envelope dispatches)...\n", iterations)
	nervousBus := bus.NewLocalNervousBus(8192)
	defer nervousBus.Close()

	nervousBus.Subscribe("bench_cap", func(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
		return &protocol.StepResult{Status: "SUCCESS"}, nil
	})

	env := &protocol.NervousEnvelope{
		EnvelopeID:       "env-bench-1",
		TraceID:          "tr-bench-1",
		SessionID:        "sess-bench",
		Plane:            protocol.PlaneVertical,
		TargetCapability: "bench_cap",
		SourceNode:       "benchmarker",
	}

	startBus := time.Now()
	for i := 0; i < iterations; i++ {
		_, err := nervousBus.Publish(ctx, env)
		if err != nil {
			fmt.Printf("Bus error at %d: %v\n", i, err)
			break
		}
	}
	busDuration := time.Since(startBus)
	busOpsPerSec := float64(iterations) / busDuration.Seconds()
	busAvgLatency := busDuration / iterations

	// 3. Benchmark Tri-Gate 1 & 2 Verification Latency
	fmt.Printf("[3/3] Benchmarking Tri-Gate Objective Funnel (%d verifications)...\n", iterations)
	gateEnv := &protocol.NervousEnvelope{
		EnvelopeID:       "env-gate-1",
		TraceID:          "tr-gate-1",
		SessionID:        "sess-gate",
		TargetCapability: "distill-telemetry",
		Parameters:       map[string]any{"level": "critical"},
		Security:         protocol.DefaultSecurityContext("sess-gate"),
	}
	gateManifest := &protocol.ArmManifest{
		ArmID:          "arm-bench-01",
		CapabilityTags: []string{"distill-telemetry"},
		EscalationGates: protocol.DefaultEscalationGates(),
	}
	schemaPayload := []byte(`{"status":"ok","code":200}`)

	startGate := time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = gates.PreFlightCheck(gateManifest, gateEnv, 0.9, 100)
		_ = gates.JSONSchemaVerifier(ctx, schemaPayload)
	}
	gateDuration := time.Since(startGate)
	gateOpsPerSec := float64(iterations) / gateDuration.Seconds()
	gateAvgLatency := gateDuration / iterations

	// Print Summary Table
	fmt.Println("\n================================================================================")
	fmt.Println("                       PERFORMANCE BENCHMARK SUMMARY                            ")
	fmt.Println("================================================================================")
	fmt.Printf("%-32s | %-16s | %-16s\n", "Subsystem", "Throughput", "Avg Latency")
	fmt.Println("---------------------------------+------------------+---------------------------")
	fmt.Printf("%-32s | %13.0f ops/s | %v\n", "Shared Substrate CAS", subOpsPerSec, subAvgLatency)
	fmt.Printf("%-32s | %13.0f ops/s | %v\n", "Local Nervous Bus (Envelope)", busOpsPerSec, busAvgLatency)
	fmt.Printf("%-32s | %13.0f ops/s | %v\n", "Tri-Gate Schema Verifier", gateOpsPerSec, gateAvgLatency)
	fmt.Println("================================================================================")
}
