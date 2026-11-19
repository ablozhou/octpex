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
- **Start Command**:
  ```bash
  go run cmd/octpex/main.go -profile=local
  ```

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
- **Enterprise Helm Chart (`deploy/helm/octpex`)**:
  ```bash
  helm install octpex deploy/helm/octpex -n octpex-system --create-namespace
  ```
  Deploys the Octpex cluster daemon with configurable replicas, Horizontal Pod Autoscaling (HPA), and readiness/liveness health probes.

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
