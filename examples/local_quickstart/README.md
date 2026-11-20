# Local Quickstart Example

[Chinese Version](README_CN.md)

This example demonstrates the zero-dependency, single-node in-memory kernel of Octpex:
- Dual-plane in-memory bus (`Vertical Dispatch` and `Lateral Handoff`)
- Compare-And-Swap (CAS) state synchronization over `sub://` references
- Objective escalation funnel with Gate 1, Gate 2, and Gate 3 checks
- FinOps token savings and cost telemetry

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

    DISPATCH["Task Arrives"] --> BUS["In-Memory Nervous Bus"]
    BUS --> ARM["Octo-Arm Worker"]
    ARM --> GATE{"Gate Checks"}
    GATE -->|"Pass"| SUB[("Shared Data Substrate<br/>(sub:// + CAS Leases)")]
    GATE -->|"Escalate"| BRAIN["Octo-Brain Arbiter"]

    style BUS fill:none,stroke-width:1.5px;
    style SUB fill:none,stroke-width:1.5px;
```

## Running the Example

```bash
go run examples/local_quickstart/main.go
```
