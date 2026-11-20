# Self-Evolving Dynamic Tool Synthesis Example

[Chinese Version](README_CN.md)

This example demonstrates the complete "Brain-teaches-Arm" capability distillation closed loop:
1. **Octo-Arm** encounters an unknown unstructured log format and exhausts local retries.
2. The task escalates to the **Octo-Brain**, which writes a dedicated parser script.
3. The script passes pre-admission AST security scanning (AbloEx sandbox).
4. The synthesized tool is compiled and hot-deployed into the **Dynamic MCP Box**.
5. Subsequent tasks are resolved by edge arms locally with **zero cloud tokens**!

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

    TASK(["Unknown Format Task"]) --> ARM["Octo-Arm Worker"]
    ARM -->|"Capability Gap"| BRAIN["Octo-Brain LLM"]
    BRAIN -->|"Synthesizes Code"| SCAN["AbloEx AST Security Scan"]
    SCAN -->|"Passed"| BOX["Dynamic MCP Tool Box"]
    BOX -.->|"Hot-Loaded (Next time 0 tokens)"| ARM

    style ARM fill:none,stroke-width:1.5px;
    style BRAIN fill:none,stroke-width:1.5px;
```

## Running the Example

```bash
go run examples/self_evolving_parser/main.go
```
