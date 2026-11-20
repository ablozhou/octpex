# AIOps 业务日志治理与三段式自治自愈闭环示例

[English](README.md)

本示例完整演示针对微服务高频日志的仿生分布式自愈闭环流程：

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

    APP["业务微服务容器 (Order / Payment)"] -->|"写入共享目录 app.log"| VOL[("Kubernetes 共享卷")]

    subgraph STAGE1["阶段 1：边缘自治初筛 (Sidecar 探针 + Octo-Arm 腕足)"]
        direction TB
        SIDECAR["Sidecar 探针 (低开销监听与 10s 窗口打包)"]
        ARM["Octo-Arm 边缘腕足 (本地小模型 / FSM，0 云端 Token)<br/>• 流式去重：数百条同类重复 ERROR 归一压缩<br/>• 根因初筛：提取异常特征并写入 Redis CAS (sub://)<br/>• 本地自愈：已知轻度抖动本地调用工具闭环"]
        SIDECAR --> ARM
    end

    VOL --> SIDECAR

    subgraph STAGE2["阶段 2：客观硬门禁裁决 (Deterministic Gating)"]
        direction TB
        GATE{"Gate 1 / Gate 2 硬门禁"}
        PASS["常规轻度抖动 -> 本地自愈闭环 (0 Token)"]
        ESC["高危未知异常 / 重试超限 -> 封装证据链升级"]
        GATE -->|"放行"| PASS
        GATE -->|"触发升级"| ESC
    end

    ARM --> GATE

    subgraph STAGE3["阶段 3：中枢大脑战略仲裁 (Octo-Brain 前沿大模型)"]
        direction TB
        BRAIN["Octo-Brain 全局战略仲裁<br/>(不读万行冗余日志，仅审查结构化现场)"]
        ACT_AUTO["无损自愈：调度重启无状态 Pod"]
        ACT_HITL["高危防护：挂起并向人类 SRE 报警"]
        ACT_DISTILL["技能蒸馏：合成修复脚本下发，下次 0 Token"]
        BRAIN --> ACT_AUTO
        BRAIN --> ACT_HITL
        BRAIN --> ACT_DISTILL
    end

    ESC ==>|"仅传递轻量指针 sub:// 与核心证据"| BRAIN

    style STAGE1 fill:none,stroke-width:1.5px;
    style STAGE2 fill:none,stroke-width:1.5px;
    style STAGE3 fill:none,stroke-width:1.5px;
```

## 1. 本地极速运行三段闭环仿真演示

无需搭建 Kubernetes 集群，直接在本地终端运行完整的端到端仿真脚本，直观查看“低危抖动 0-Token 本地自愈”与“高危死锁升级中枢决策”两类典型场景：

```bash
python examples/aiops_log_drain/main.py
```

## 2. 在 Kubernetes 集群中部署运行

> **前置条件：** 请先参考 [Kubernetes 多 Agent 部署指引](../../docs/k8s_multi_agent_guide_cn.md) 部署好多智能体集群（如执行 `kubectl apply -f deploy/k8s/all-in-one.yaml` 并确认 Pod 正常运行）。

将包含共享存储卷的采集 Pod 部署到 `octpex-system` 命名空间：

```bash
kubectl apply -f examples/aiops_log_drain/k8s-deployment.yaml
```

查看 Sidecar 捕获日志并自动向 Octpex 神经总线派发信封的实时输出：

```bash
kubectl logs -n octpex-system deploy/mock-app-log-generator -c octpex-log-sidecar --tail=10
```

进入 Redis 验证自动完成流式去重与版本存证的数据：

```bash
kubectl exec -n octpex-system deploy/redis-cluster -- redis-cli KEYS "octpex:sub:sub://sess-k8s-autolog/*"
```

清理实验资源：

```bash
kubectl delete -f examples/aiops_log_drain/k8s-deployment.yaml
```
