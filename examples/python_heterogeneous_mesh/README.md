# Python Heterogeneous Multi-Agent Mesh Example

[Chinese Version](README_CN.md)

This example demonstrates seamless biomimetic lateral coordination across heterogeneous agent frameworks using the Octpex Python SDK:
- **Arm 1 (Google ADK)**: Wrapped single agent diagnosing raw incident telemetry.
- **Arm 2 (Microsoft AutoGen)**: Coder-critic multi-agent debate crew synthesizing code patches.
- **Arm 3 (LangGraph)**: Stateful cyclic workflow agent executing unit tests and Gate 2 verification.
All exchanging zero-copy pointers over the Shared Data Substrate (`sub://`).

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

    RAW[("sub://alert.json")] --> A1["Arm 1: Google ADK<br/>(Triage & Root Cause)"]
    A1 ==>|"Lateral Handoff (hop=1)"| A2["Arm 2: AutoGen Crew<br/>(Coder-Critic Debate Team)"]
    A2 ==>|"Lateral Handoff (hop=2)"| A3["Arm 3: LangGraph<br/>(Unit Test & Gate 2 Verifier)"]
    A3 --> DONE(["Resolved Locally (0 Cloud Tokens)"])

    style A1 fill:none,stroke-width:1.5px;
    style A2 fill:none,stroke-width:1.5px;
    style A3 fill:none,stroke-width:1.5px;
```

## Running the Example

```bash
python examples/python_heterogeneous_mesh/main.py
```
