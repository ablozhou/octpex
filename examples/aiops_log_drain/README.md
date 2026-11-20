# AIOps Log Drain: Three-Stage Autonomous Closed Loop

[Chinese Version](README_CN.md)

This example demonstrates the complete biomimetic self-healing pipeline for high-frequency microservice logs:

```mermaid
%%{init: {
  "themeVariables": {
    "background": "transparent",
    "mainBkg": "transparent",
    "subGraphBkg": "transparent",
    "clusterBkg": "transparent",
    "edgeLabelBackground": "transparent"
  },
  "themeCSS": ".edgeLabel, .edgeLabel span, .edgeLabel div { background-color: transparent !important; } .edgeLabel rect { fill: transparent !important; opacity: 0 !important; }"
}}%%
flowchart TD
    classDef default fill:none;

    APP["Microservice Container (Order / Payment)"] -->|"Writes to app.log"| VOL[("Kubernetes Shared Volume")]

    subgraph STAGE1["Stage 1: Edge Autonomic Triage (Sidecar + Octo-Arm Ganglia)"]
        direction TB
        SIDECAR["Harvester Sidecar (10s Window Batching)"]
        ARM["Octo-Arm Ganglia (Local SLM / FSM, 0 Cloud Tokens)<br/>• Stream deduplication: reduces 90%+ redundant error spam<br/>• Root-cause classification: writes snapshot to sub://<br/>• Local reflex: resolves transient jitters on-site"]
        SIDECAR --> ARM
    end

    VOL --> SIDECAR

    subgraph STAGE2["Stage 2: Objective Deterministic Gate Gating"]
        direction TB
        GATE{"Gate 1 / Gate 2 Hard Checks"}
        PASS["Known Transient Jitter -> Local 0-Token Resolve"]
        ESC["Critical Anomaly / Retries Exceeded -> Escalate"]
        GATE -->|"Pass"| PASS
        GATE -->|"Escalate Triggered"| ESC
    end

    ARM --> GATE

    subgraph STAGE3["Stage 3: Octo-Brain Strategic Arbitration (Frontier LLM)"]
        direction TB
        BRAIN["Octo-Brain Strategic Cortex<br/>(Reviews structured incident context without raw logs)"]
        ACT_AUTO["Auto-Recovery: Rollout restart stateless Pod"]
        ACT_HITL["Safety Alarm: Escalate to On-Call SRE"]
        ACT_DISTILL["Skill Distill: Synthesize dynamic watchdog script"]
        BRAIN --> ACT_AUTO
        BRAIN --> ACT_HITL
        BRAIN --> ACT_DISTILL
    end

    ESC ==>|"Pass-by-reference: sub:// pointer + evidence"| BRAIN

    style STAGE1 fill:none,stroke-width:1.5px;
    style STAGE2 fill:none,stroke-width:1.5px;
    style STAGE3 fill:none,stroke-width:1.5px;
```

## Running the Three-Stage Closed Loop Demo Locally

Run the complete end-to-end simulation showing Case 1 (0-Token Edge Resolve) and Case 2 (Gate Escalation to Cloud Brain):

```bash
python examples/aiops_log_drain/main.py
```

## Running on Kubernetes Cluster

> **Prerequisite:** Ensure the Octpex multi-agent cluster is deployed and running by following the [Kubernetes Deployment Guide](../../docs/k8s_multi_agent_guide.md) (e.g. `kubectl apply -f deploy/k8s/all-in-one.yaml`).

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
