# AIOps Triage Mesh Example

[Chinese Version](README_CN.md)

This example demonstrates peer-to-peer lateral coordination across heterogeneous Octo-Arms:
- **Arm 1 (Telemetry Triage)**: Triages Loki alerts, stores incidents to `sub://{session}/incident.json`, and triggers a lateral handoff.
- **Arm 2 (Config Patcher)**: Reads incident snapshot via CAS reference, attempts an AST patch, detects a high-risk mutation, and escalates to the Octo-Brain.
- **Octo-Brain**: Arbitrates the task, evaluates risks, and requests Human-in-the-Loop (HITL) approval.

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

    ALERT(["Loki Telemetry Alert"]) --> ARM1["Arm 1: Telemetry Triage<br/>(Deduplicates & creates incident.json)"]
    ARM1 ==>|"Lateral Handoff (hop=1)"| ARM2["Arm 2: K8s Config Patcher<br/>(Attempts AST refactoring)"]
    ARM2 --> G1{"Gate 1: High-Risk Action Check"}
    G1 -->|"Escalate Triggered"| BRAIN["Octo-Brain Strategic Arbiter<br/>(Requests HITL approval)"]

    style ARM1 fill:none,stroke-width:1.5px;
    style ARM2 fill:none,stroke-width:1.5px;
```

## Running the Example

```bash
go run examples/aiops_triage_mesh/main.go
```
