# Blog Publishing Multi-Agent Mesh Example

[Chinese Version](README_CN.md)

This example demonstrates a 5-arm collaborative workflow orchestrated across the Octpex biomimetic fabric:

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

    TOPIC(["Given Technical Topic"]) --> ARM1["Arm 1: Researcher Arm<br/>Extracts core architectural pillars -> sub://.../research.json"]
    ARM1 --> ARM2["Arm 2: Planner Arm<br/>Structures 5-chapter markdown outline -> sub://.../outline.json"]
    ARM2 --> ARM3["Arm 3: Writer Arm<br/>Drafts complete markdown article -> sub://.../draft.md"]

    ARM3 --> ARM4{"Arm 4: Critic Gate<br/>Gate 2/3 Objective Verification"}

    ARM4 -->|"Critique Veto (Actionable feedback)"| ARM3
    ARM4 -->|"Signed Off (98/100 score)"| ARM5["Arm 5: Publisher Arm<br/>Injects YAML FrontMatter & outputs to /data/blog"]

    ARM5 --> OUTPUT[("Shared Volume /data/blog/<br/>blog_xxx.md")]

    style ARM1 fill:none,stroke-width:1.5px;
    style ARM2 fill:none,stroke-width:1.5px;
    style ARM3 fill:none,stroke-width:1.5px;
    style ARM5 fill:none,stroke-width:1.5px;
```

1. **Researcher Arm**: Queries background facts and technical pillars (`sub://{session}/research.json`).
2. **Planner Arm**: Organizes findings into a 5-chapter structured outline (`sub://{session}/outline.json`).
3. **Writer Arm**: Generates markdown drafts based on the outline and incorporates feedback (`sub://{session}/draft.md`).
4. **Critic Gate Arm**: Enforces objective Gate 2 verification (depth, checklist, keywords). In round 1, vetoes the draft to trigger an iterative revision loop; approves in round 2.
5. **Publisher Arm**: Injects YAML FrontMatter and outputs the finalized article to a shared directory or volume (`/data/blog/`).

## Running Locally

```bash
python examples/blog_publish_mesh/main.py --topic "Kubernetes Biomimetic Multi-Agent Fabric on Octpex" --output "./output/blog"
```

The resulting article will be written to `output/blog/blog_*.md`.

## Running in Kubernetes

> **Prerequisite:** Ensure the Octpex multi-agent cluster is deployed and running by following the [Kubernetes Deployment Guide](../../docs/k8s_multi_agent_guide.md) (e.g. `kubectl apply -f deploy/k8s/all-in-one.yaml`).

Apply the Job manifest to your cluster:

```bash
kubectl apply -f examples/blog_publish_mesh/k8s-job.yaml
```

Follow the logs:

```bash
kubectl logs -n octpex-system job/octpex-blog-publishing-job -f
```

Clean up:

```bash
kubectl delete -f examples/blog_publish_mesh/k8s-job.yaml
```
