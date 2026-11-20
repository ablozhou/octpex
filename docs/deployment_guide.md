# Octpex Comprehensive Deployment and Operations Guide

This guide covers operational profiles, Kubernetes and containerized cluster prerequisites, and walkthroughs of the runnable production examples in Octpex.

---

## 1. Operational Deployment Profiles

Octpex supports two distinct operational modes via configuration files in `configs/`:

### 1.1 Standalone Developer Profile (`configs/local.yaml`)

- **Target Audience**: Single developer machines, CI/CD runners, and edge testing.
- **Topology**: Zero external cluster dependencies (in-memory bus, in-memory substrate, in-process capability registry).
- The default telemetry worker runs in Go. Applications embedding an SLM adapter can configure Ollama separately.
- Dynamic scripts require an explicitly configured isolated runner. The Docker example uses a 256 MiB memory cap and a five-second timeout.

#### 1.1.0 Building & Running octpex (Local Binary / Makefile / CLI)

From the repository root, you can build and run `octpex run` using:

1. **Build Native Binary (Recommended)**:
   ```bash
   make build
   ```
   Compiles a pure static binary (`CGO_ENABLED=0`) into `./bin/` (`bin/octpex.exe` on Windows, `bin/octpex` on Linux/macOS).
   If building directly with the `go` CLI on Linux, set `CGO_ENABLED=0` to avoid requiring local GCC or C standard library headers (`stdlib.h`):
   ```bash
   CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/octpex ./cmd/octpex
   ```

2. **Execute `octpex run`**:
   - **Using the compiled binary**:
     - Windows PowerShell:
       ```powershell
       .\bin\octpex.exe run -config configs/local.yaml
       ```
     - Linux / macOS / WSL:
       ```bash
       ./bin/octpex run -config configs/local.yaml
       ```
   - **Using Makefile (Parameterized CLI input)**:
     ```bash
     # 1. Run local standalone mode by default (PROFILE=local PORT=8080)
     make run

     # 2. Run distributed cluster mode (loads configs/cluster.yaml)
     make run PROFILE=cluster

     # 3. Custom HTTP port (e.g. port 9090 or 0 to disable dashboard)
     make run PORT=9090

     # 4. Custom config file or additional flags
     make run CONFIG=my_custom.yaml ARGS="-worker=false"
     ```
   - **Running directly with Go without compiling**:
     ```bash
     go run ./cmd/octpex run -config configs/local.yaml
     ```
   - **Global CLI access**:
     Add `./bin/` to your system `PATH` to invoke `octpex run`, `octpex eval`, or `octpex bench` globally from any terminal directory.

#### 1.1.1 WSL2 Optimization: Seamless Access to Host Windows Ollama

When developing on Windows workstations, developers commonly run Octpex inside WSL2 (Linux subsystem) while running Ollama directly on the Windows host to take full advantage of GPU drivers. Under default WSL2 NAT networking, network isolation often causes connection timeouts when accessing host Ollama.

> [!TIP]
> **Battle-Tested & Proven Solution: Enable WSL2 Mirrored Networking**
> 
> 1. **Configure `.wslconfig` on Windows Host**:
>    In `C:\Users\<username>\.wslconfig` (create if absent):
>    ```ini
>    [wsl2]
>    networkingMode=mirrored
>    ```
> 2. **Restart WSL2**:
>    Run in Windows host terminal:
>    ```powershell
>    wsl --shutdown
>    ```
> 3. **Ensure Ollama Binds to Global Interface**:
>    Set system environment variable `OLLAMA_HOST=0.0.0.0:11434` on Windows and restart Ollama.
> 4. **Connect Directly via `localhost` (Zero Gateway Hacks)**:
>    Inside WSL2, `http://localhost:11434` connects directly to Windows host Ollama:
>    ```bash
>    curl http://localhost:11434/api/tags
>    ```
>    In `configs/local.yaml`, `base_url` can simply remain `http://localhost:11434` without any dynamic IP resolution.

### 1.2 Enterprise Distributed Cluster Profile (`configs/cluster.yaml`)

- **Target Audience**: Kubernetes clusters, multi-node bare-metal servers, and edge server farms.
- **Nervous Bus**: NATS JetStream dual-plane bus (`octpex.vertical.dispatch.*` and `octpex.lateral.handoff.*`) with `arm-workers` queue groups.
- **Shared Data Substrate**: Redis / Valkey cluster with Lua-driven atomic Compare-And-Swap (CAS) version checks and distributed lease locking.
- **Capability Registry**: etcd cluster managing 5-second arm heartbeat leases with automatic fault eviction on pod termination.
- **Start Command**:
  ```bash
  (cd cluster && go run -mod=mod . -profile=cluster -config=../configs/cluster.yaml)
  ```

### 1.3 Web Visual Dashboard & REST API

The Octpex kernel embeds an HTTP server (default port `8080`) providing live visual topology and FinOps metrics:
- **Web UI**: Open `http://localhost:8080` in any web browser to view the real-time topology map, healthy Octo-Arms, and live FinOps metrics.
- **REST Endpoints**:
  - `GET /api/v1/topology`: Inspect registered arms and capability tags.
  - `GET /api/v1/finops`: Retrieve cost and token savings analytics.
  - `GET /api/v1/events`: Real-time Server-Sent Events (SSE) pulse stream of lateral handoffs and gate decisions.
  - `POST /api/v1/dispatch`: Dispatch execution envelopes directly via HTTP.
- **Custom Port**:
  ```bash
  go run cmd/octpex/main.go -port=9090
  ```
  Pass `-port=0` to disable the HTTP server if running in headless environments.

### 1.4 Container and Kubernetes Deployment

Octpex provides production-ready deployment configurations in `deploy/`:
- **Docker Compose (Full Cluster)**:
  ```bash
  docker compose -f deploy/docker-compose.yaml up -d
  ```
  Launches the Octpex kernel, NATS JetStream, Redis, etcd, and Ollama.

  Run `python cluster/test_workflow.py` for isolated five-container acceptance covering cross-node dispatch, Redis readback and worker loss.
- **Kubernetes Raw Manifests (All-in-One Multi-Agent Cluster)**:
  See [k8s_multi_agent_guide.md](k8s_multi_agent_guide.md) for full walkthrough:
  ```bash
  # Deploy Redis/NATS/etcd dependencies + Entry Cortex + 2 Edge Workers in one shot
  kubectl apply -f deploy/k8s/all-in-one.yaml
  ```
- **Kubernetes Production Standalone Manifests**:
  ```bash
  kubectl apply -f deploy/k8s/deployment.yaml
  ```
- **Enterprise Helm Chart (`deploy/helm/octpex`) & SLM Sidecar Auto-Injection**:
  ```bash
  helm install octpex deploy/helm/octpex -n octpex-system --create-namespace
  ```
  * Centralized declaration of Multi-LLM provider pool (`llm_providers`) and Multi-SLM catalog (`slm_catalog`) in `values.yaml`;
  * Arm workloads (`arms`) support assigning specialized SLM sidecars (e.g. `qwen2.5-coder-1.5b`), automatically falling back to global `default_slm` (`qwen2.5-1.5b`) when omitted;
  * Standalone demonstration manifest: `kubectl apply -f deploy/k8s/arm-sidecar-demo.yaml`, rendering dual-container Pods with `< 5ms` Localhost loopback inference.

---

## 2. Walkthrough of Runnable Production Examples

Octpex ships with four self-contained, fully functional examples in `examples/`:

### 2.1 Example 1: Local Quickstart (`examples/local_quickstart/`)

Demonstrates core biomimetic coordination (Vertical Dispatch -> Lateral Handoff -> Level 0 Rule Verification) with 0 cloud tokens:
- **Arm 1 (Telemetry Distiller)**: Filters noisy logs, stores clean data at `sub://sess/cleaned_log` (92% token savings), hands off laterally.
- **Arm 2 (Patch Synthesizer)**: Zero-copy reads the substrate pointer, synthesizes code fix at `sub://sess/patch.diff`, hands off laterally.
- **Arm 3 (Level 0 Native FSM Verifier)**: Verifies unit tests in pure Go (< 2ms, 0 tokens).
- **Run**:
  ```bash
  go run examples/local_quickstart/main.go
  ```

### 2.2 Example 2: AIOps Triage Mesh (`examples/aiops_triage_mesh/`)

Simulates a production incident remediation mesh across three specialized edge arms:
- **Arm 1 (Loki Telemetry Triage)**: Emits structured incident metadata into `sub://sess/incident.json`.
- **Arm 2 (K8s Config Patcher)**: Parses Kubernetes Deployment specifications and writes hotfix YAML.
- **Arm 3 (Prometheus Canary Verifier)**: Confirms P99 latency drops and SLA compliance.
- **Run**:
  ```bash
  go run examples/aiops_triage_mesh/main.go
  ```

### 2.3 Example 3: Self-Evolving Parser ("Brain Teaches Arm") (`examples/self_evolving_parser/`)

Demonstrates the cross-tier self-evolution distillation loop:
- The first task returns an escalation. The example registers a supplied Python parser.
- The next task runs the parser on its actual input in a disposable container and reports its output and measured latency.
- **Run**:
  ```bash
  docker pull python:3.12-alpine
  go run ./examples/self_evolving_parser
  ```

### 2.4 Example 4: Python Heterogeneous Agent Mesh (`examples/python_heterogeneous_mesh/`)

Demonstrates cross-framework lateral peer-to-peer collaboration without central cloud round-trips:
- **Arm 1 (Google ADK)**: Analyzes database pool alert and writes structured diagnosis to `sub://sess/diagnosis.json`.
- **Arm 2 (Microsoft AutoGen)**: Coder and Critic agents debate fix and generate code patch at `sub://sess/patch.diff`.
- **Arm 3 (LangGraph + Gate 2)**: Runs stateful regression unit tests, passes Gate 2 verification, and completes workflow with 0 Cloud Tokens.
- **Run**:
  ```bash
  python examples/python_heterogeneous_mesh/main.py
  ```

---

## 3. FinOps Telemetry & ROI Auditing

Octpex includes a built-in FinOps telemetry engine (`pkg/telemetry/finops.go`):
- Tracks cloud tokens, edge SLM tokens, latencies, and escalation rates.
- Computes dollars saved ($ USD) against full-frontier commercial baseline.
- Generates structured executive ROI audit reports.

## 4. Verify the default worker

```sh
go run ./cmd/octpex run -profile=local -port=8080
```

```sh
curl -s http://localhost:8080/api/v1/dispatch -H 'Content-Type: application/json' \
  -d '{"target_capability":"distill-telemetry","session_id":"demo","envelope_id":"e1","trace_id":"t1","parameters":{"log":"WARN retry\nWARN retry"}}'
```

Expect `SUCCESS`, one `WARN retry` line, a version-1 artifact and the worker ID. Omitting the log returns `ESCALATED` and an arbitration decision. FinOps and SSE are local to each process.
