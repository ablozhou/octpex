# AIOps Log Drain: Three-Stage Autonomous Closed Loop

This example demonstrates the complete biomimetic self-healing pipeline for high-frequency microservice logs:

```
[Business Container]
       │
       ▼ (writes to shared volume /var/log/mock-app)
┌─────────────────────────────────────────────────────────────┐
│ Stage 1: Edge Autonomy (Sidecar + Octo-Arm Ganglia)         │
│ • Harvester Sidecar: streaming window batches (10s)         │
│ • Octo-Arm Ganglia (0 Cloud Tokens):                        │
│   - Stream deduplication (reduces 90%+ redundant ERRORs)    │
│   - Root-cause classification & incident context extraction │
│   - CAS persistence: writes snapshot to sub://incident.json │
│   - Local reflex: resolves transient jitters on-site        │
└──────────────────────────────┬──────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────┐
│ Stage 2: Objective Gate Gating (Gate 1 / Gate 2 Hard Checks)│
│ • Gate 1: Safety & domain similarity check                  │
│ • Gate 2: Local retry quota exhaustion (retries > 3)        │
│ • Decides: LOCAL_RESOLVE vs ESCALATE to Octo-Brain          │
└──────────────────────────────┬──────────────────────────────┘
                               │ (Only lightweight pointer + evidence)
                               ▼
┌─────────────────────────────────────────────────────────────┐
│ Stage 3: Octo-Brain Strategic Arbitration (Frontier LLM)    │
│ • Reviews structured incident context without reading logs  │
│ • High-level Strategic Decision:                            │
│   ├─ [Auto-Recovery]: Restart stateless Pod (rollout restart│
│   ├─ [HITL Alarm]: Escalate to SRE on-call for DB/financial │
│   └─ [Skill Distillation]: Synthesizes dynamic MCP watchdog │
└─────────────────────────────────────────────────────────────┘
```

## Running the Three-Stage Closed Loop Demo Locally

Run the complete end-to-end simulation showing Case 1 (0-Token Edge Resolve) and Case 2 (Gate Escalation to Cloud Brain):

```bash
python examples/aiops_log_drain/main.py
```

## Running on Kubernetes Cluster

Deploy the shared-volume pod into the `octpex-system` namespace:

```bash
kubectl apply -f examples/aiops_log_drain/k8s-deployment.yaml
```

View sidecar logs capturing and dispatching to Octpex:

```bash
kubectl logs -n octpex-system deploy/mock-app-log-generator -c octpex-log-sidecar --tail=10
```

Check deduplicated results in Redis CAS:

```bash
kubectl exec -n octpex-system deploy/redis-cluster -- redis-cli KEYS "octpex:sub:sub://sess-k8s-autolog/*"
```

Clean up:

```bash
kubectl delete -f examples/aiops_log_drain/k8s-deployment.yaml
```
