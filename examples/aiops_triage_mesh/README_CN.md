# AIOps 智能运维分级接力协同示例

[English](README.md)

本示例展示异构 Octo-Arm 腕足节点之间的横向点对点（P2P）接力流转与分级协同：
- **腕足 1（Loki 遥测初筛腕足）**：分析告警日志，将现场写入 `sub://{session}/incident.json`，并触发横向接力；
- **腕足 2（K8s 配置补丁腕足）**：通过指针读取现场，尝试 AST 代码修补，检测到写库高危操作后触发门禁升级；
- **Octo-Brain 中枢中枢**：战略仲裁任务，挂起高危动作并向人类工程师发起 HITL 审批。

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

    ALERT(["Loki 监控告警"]) --> ARM1["腕足 1: 遥测初筛<br/>(去噪并写入 incident.json)"]
    ARM1 ==>|"横向总线接力 (hop=1)"| ARM2["腕足 2: K8s 配置修补<br/>(尝试 AST 代码重构)"]
    ARM2 --> G1{"Gate 1: 高危写操作拦截"}
    G1 -->|"触发升级"| BRAIN["Octo-Brain 战略中枢<br/>(挂起并请求 HITL 审批)"]

    style ARM1 fill:none,stroke-width:1.5px;
    style ARM2 fill:none,stroke-width:1.5px;
```

## 运行示例

```bash
go run examples/aiops_triage_mesh/main.go
```
