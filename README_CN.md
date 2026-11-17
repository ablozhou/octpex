# Octpex

<p align="center">
  <strong>仿生分布式认知拓扑与自我进化的端云协同智能体底座</strong><br>
  <em>Biomimetic Distributed Cognitive Topology & Self-Evolving Edge-Cloud Agent Fabric</em>
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License"></a>
  <a href="README.md"><img src="https://img.shields.io/badge/README-English-blue.svg" alt="English Readme"></a>
  <a href="docs/PRD_CN.md"><img src="https://img.shields.io/badge/PRD-中文版-brightgreen.svg" alt="PRD CN"></a>
  <a href="docs/PRD.md"><img src="https://img.shields.io/badge/PRD-English-brightgreen.svg" alt="PRD"></a>
  <a href="docs/ARCHITECTURE_CN.md"><img src="https://img.shields.io/badge/架构规范-中文版-orange.svg" alt="Architecture CN"></a>
  <a href="docs/ARCHITECTURE.md"><img src="https://img.shields.io/badge/Architecture-English-orange.svg" alt="Architecture"></a>
</p>

---

## 目录

1. [项目愿景与背景](#项目愿景与背景)
2. [为什么是仿生架构？（章鱼拓扑启发）](#为什么是仿生架构章鱼拓扑启发)
3. [核心痛点与破局之道](#核心痛点与破局之道)
4. [系统整体拓扑架构](#系统整体拓扑架构)
5. [三道门禁客观升级漏斗（零信任小模型自评）](#三道门禁客观升级漏斗零信任小模型自评)
6. [跨层自进化：“大脑教腕足”与 Dynamic MCP](#跨层自进化大脑教腕足与-dynamic-mcp)
7. [多智能体协作范式与多框架生态](#多智能体协作范式与多框架生态)
8. [核心 SPI 契约规范](#核心-spi-契约规范)
9. [安全治理与离线韧性](#安全治理与离线韧性)
10. [双模部署支持与核心特性架构](#双模部署支持与核心特性架构)
11. [文档索引](#文档索引)

---

## 项目愿景与背景

**Octpex** 旨在让企业级多智能体系统（Multi-Agent Systems）在经济成本上可持续、在响应速度上达到亚秒级、在能力上具备持续自我进化特性。

* **Octpex 是什么：** 一个领域无关的**分布式认知调度底座与运行时（Distributed Cognitive Fabric & Scheduling Runtime）**。它通过共享协调神经环（**Basal Nerve Ring**），在顶层中枢推理模型（**Octo-Brain**）的统筹下，调度海量搭载本地小模型（SLM, 0.5B–3B）的自主边缘执行节点（**Octo-Arms**）。
* **Octpex 不是什么：** Octpex **不是**单机/单进程 Agent 编排 SDK（如 LangGraph、Google ADK 或 CrewAI），也**不是**底层容器隔离沙盒（如 AbloEx）。它充当连接异构智能体的**“分布式神经系统”**，跨节点路由协同任务，就地消化 70%–85% 的常规低阶推理，并将高危工具执行委托给可插拔安全沙盒。

---

## 为什么是仿生架构？（章鱼拓扑启发）

自然界中，头足类生物（章鱼）拥有极高智慧，但其神经结构与哺乳动物迥然不同：
* **三分之二的神经元分布在八条腕足中**，每条腕足都具备独立的触觉感知、模式识别与反射运动能力；
* **中枢大脑专注于高层战略意图与多腕足协调**，绝不事无巨细地指挥腕足的每一个触手动作。

Octpex 严格仿生这一自然进化奇迹：
* **Octo-Brain（中枢大脑）：** 采用前沿云端大模型，专注全局意图分解、跨领域冲突仲裁、疑难任务攻坚与动态代码合成。
* **Basal Nerve Ring（腕足根部神经环）：** 轻量级分布式控制与协同记忆层，负责动态能力发现、引用传递式共享数据基质（Shared Data Substrate）与节点间横向通信。
* **Octo-Arms（边缘腕足）：** 边缘工作节点，配备**腕足私有数据区**与本地小模型（0.5B–3B），通过 `Act -> Inspect -> Retry` 局部自愈微循环就地消化常规任务。

---

## 核心痛点与破局之道

| 核心痛点 | 传统中心化 Agent 架构根源 | Octpex 仿生破局之道 |
| :--- | :--- | :--- |
| **Token 推理成本爆炸** | 日志去噪、格式转换、API 重试等琐碎步骤全调云端大模型，80% 预算被低价值调用浪费。 | **边缘分流与数据蒸馏：** 腕足内置 0.5B–3B SLM，零边际成本就地消化 70%–85% 高频任务。 |
| **端到端响应延迟过高** | 多轮公网 API 思考循环累积 5–30 秒延迟，无法满足实时生产 SLA。 | **局部反思微循环与横向总线：** $<800\text{ ms}$ 本地自愈闭环，腕足间点对点总线交接 $<15\text{ ms}$。 |
| **上下文污染与小模型盲目自信** | 原始日志或数据库多表直接塞入大模型引发幻觉；小模型对陌生问题极易盲目自信。 | **两级数据存储与客观门禁：** 腕足私有区隔离高频敏感原始数据；跨节点协作通过共享数据基质传输轻量指针（`sub://`）。 |
| **工具静态僵化与框架孤岛** | 跨团队 Agent（Go/ADK/LangGraph）技术栈割裂，历史经验与新工具无法沉淀共享。 | **根部神经环与动态 MCP 蒸馏：** 大脑解决的难题自动合成动态 MCP 工具广播全网复用。 |

---

## 系统整体拓扑架构

```mermaid
%%{init: {"themeVariables": {"background": "transparent", "mainBkg": "transparent", "subGraphBkg": "transparent", "clusterBkg": "transparent", "edgeLabelBackground": "transparent"}}}%%
flowchart TD
    classDef default fill:none;

    %% 1. OCTO-BRAIN (章鱼大脑 / 中央皮层: 纯单向横排)
    subgraph BRAIN["OCTO-BRAIN (章鱼大脑 / 中央皮层)"]
        direction LR
        B_ORCH["全局编排器 (Global Orchestrator)"] --> B_CORE["前沿大模型 (战略仲裁与复杂决策)"] --> B_FINOPS["FinOps 成本分析与动态 MCP 蒸馏器"]
    end

    %% 2. BASAL NERVE RING (基底神经环: 横向环状底座)
    subgraph RING["BASAL NERVE RING (基底神经环 / 共享底座)"]
        direction LR
        RING_REG["能力与寻址注册表 (NATS KV / etcd)"] --> RING_SUB["共享数据底座 (sub:// + CAS 租约)"] --> RING_BOX["动态 MCP 盒子 (共享工具注册表)"]
    end

    %% 垂直神经总线连接大脑外框与神经环外框
    BRAIN <==>|"垂直神经总线 (gRPC / NATS JetStream)"| RING

    %% 3. OCTO-ARMS (横向展开的腕足网格: 3 支独立实框腕足并列横排)
    subgraph ARMS_LAYER["OCTO-ARMS MESH (横向展开的腕足网格)"]
        direction LR

        %% 左侧腕足 1: 遥测数据蒸馏腕足
        subgraph ARM1["Octo-Arm 1: 遥测数据蒸馏腕足"]
            direction TB
            A1_CORE["适配器与局部小模型 0.5B (日志过滤压缩)"]
            A1_PRIV["神经节私有存储 (原始日志与私有向量库)"]
            A1_TOOL["吸盘 / 工具 SPI (Prometheus & Loki MCP)"]
            A1_CORE --> A1_PRIV --> A1_TOOL
        end

        %% 中间腕足 2: 补丁合成与修复腕足
        subgraph ARM2["Octo-Arm 2: 补丁合成与修复腕足"]
            direction TB
            A2_CORE["适配器与局部小模型 3B (代码重构与单测)"]
            A2_PRIV["神经节私有存储 (配置 AST 与 Few-Shot 样本)"]
            A2_TOOL["吸盘 / 工具 SPI (Git 提交与编译沙箱 MCP)"]
            A2_CORE --> A2_PRIV --> A2_TOOL
        end

        %% 右侧腕足 3: 安全合规审计腕足 (代表分布式腕足群 3..8)
        subgraph ARM3["Octo-Arm 3: 安全合规审计腕足 (腕足 3..8)"]
            direction TB
            A3_CORE["适配器与局部小模型 1.5B (规则安全门禁)"]
            A3_PRIV["神经节私有存储 (CVE 规则库与 RBAC 策略)"]
            A3_TOOL["吸盘 / 工具 SPI (Falco & 云合规 MCP)"]
            A3_CORE --> A3_PRIV --> A3_TOOL
        end
    end

    %% 从神经环下发 3 条专用神经主干连线至腕足网格 (在容器层级分流 3 条通道，保护内部 direction LR 横排不被破坏)
    RING ==>|"神经主干 1"| ARMS_LAYER
    RING ==>|"神经主干 2 (sub:// 底座总线)"| ARMS_LAYER
    RING ==>|"神经主干 3"| ARMS_LAYER

    %% 4. 外部环境与吸盘交互 (底部目标: 纯单向横排)
    subgraph ENV["外部环境 (感知与操作交互目标)"]
        direction LR
        ENV_OBS["可观测性系统 (指标、链路与日志)"] --> ENV_DEV["研发交付环境 (Git 与执行沙箱)"] --> ENV_SEC["运行时安全 (K8s API 与 Falco 审计)"]
    end

    %% 腕足网格直连外部环境外框
    ARMS_LAYER -->|"感知与吸盘执行交互"| ENV

    style BRAIN fill:none;
    style RING fill:none;
    style ARMS_LAYER fill:none,stroke-dasharray: 4 4;
    style ARM1 fill:none,stroke-width:1.5px;
    style ARM2 fill:none,stroke-width:1.5px;
    style ARM3 fill:none,stroke-width:1.5px;
    style ENV fill:none;
```

---

## 三道门禁客观升级漏斗（零信任小模型自评）

> **工程原则：** 永远不要相信本地小模型（0.5B–3B）对自己输出置信度的主观打分。所有升级决策由宿主运行时代码通过确定性客观硬门禁强制裁决。

```mermaid
%%{init: {"themeVariables": {"background": "transparent", "mainBkg": "transparent", "subGraphBkg": "transparent", "clusterBkg": "transparent", "edgeLabelBackground": "transparent"}}}%%
flowchart TD
    classDef default fill:none;
    START["子任务到达 Octo-Arm 腕足"] --> G1{"Gate 1: 前置策略与语义距离硬门禁"}
    G1 -->|"高危生产策略 | 陌生领域相似度 < 0.65 | 上下文超限"| ESC1["强制升级至 Octo-Brain (Gate 1 触发)"]
    G1 -->|"通过前置门禁"| ACT["Act: 本地 SLM 生成输出并尝试调用本地工具"]
    ACT --> G2{"Gate 2: 外部确定性验证器 (Schema / AST / 规则 / 状态码)"}
    G2 -->|"验证通过"| G3{"Gate 3: 数学对数概率与行为停滞门禁"}
    G2 -->|"验证失败"| RETRY_CHK{"当前重试次数 < max_reflex_retries (默认3次)?"}
    RETRY_CHK -->|"是 (未耗尽)"| RETRY["将确定性校验报错原因作为上下文反思反馈"] --> ACT
    RETRY_CHK -->|"否 (重试耗尽)"| ESC2["强制升级至 Octo-Brain (Gate 2 耗尽)"]
    G3 -->|"关键字段平均 Logprob 过低 | 行为死循环 | 横向跳数超限"| ESC3["强制升级至 Octo-Brain (Gate 3 触发)"]
    G3 -->|"三道门禁全部通过"| RESOLVE["LOCAL_RESOLVE: 将协同产物写入共享数据基质 (sub://) 并继续流转"]
```

---

## 跨层自进化：“大脑教腕足”与 Dynamic MCP

```mermaid
%%{init: {"themeVariables": {"background": "transparent", "mainBkg": "transparent", "subGraphBkg": "transparent", "clusterBkg": "transparent", "edgeLabelBackground": "transparent"}}}%%
flowchart TD
    classDef default fill:none;
    ARM_GAP["边缘腕足触发能力缺失或校验耗尽升级"]
    BRAIN_SOLVE["Octo-Brain 前沿大模型解决难题并合成可执行脚本"]
    subgraph EVOLUTION["跨层蒸馏与自主进化闭环"]
        TOOL_REG["1. 将可执行脚本注册为标准动态 MCP 工具并存入 Dynamic MCP Box"]
        FEW_SHOT["2. 将成功轨迹 Few-Shot 样本与 HITL 约束补丁下发至腕足私有存储区"]
    end
    LOCAL_PASS["后续同类任务到达: 边缘腕足本地调用动态工具 100% 自主闭环消化"]
    ARM_GAP --> BRAIN_SOLVE
    BRAIN_SOLVE --> TOOL_REG
    BRAIN_SOLVE --> FEW_SHOT
    TOOL_REG --> LOCAL_PASS
    FEW_SHOT --> LOCAL_PASS
    style EVOLUTION fill:none;
```

* **闭环沉淀：** 大脑解决问题不仅返回答案，还合成标准化 MCP 脚本存入 **Dynamic MCP Box**；
* **复用递增：** 随着系统运行时间增加，本地消化率（Local Offload Rate）从最初的 75% 逐步自然攀升至 85%–90% 以上。

---

## 多智能体协作范式与多框架生态

### 四大原生支持的协作范式
1. **协调器模式（Coordinator Pattern）：** `Octo-Brain` 宏观分解任务，按能力标签并发扇出给多条 `Octo-Arm`，并汇总终局结果。
2. **顺序流水线（Sequential Pattern）：** 腕足间在横向总线上进行点对点接力（`Arm A -> Arm B -> Arm C`），仅传递共享数据基质引用指针（`sub://`），不绕行中枢。
3. **迭代优化模式（Iterative Refinement Pattern）：** 支持腕足内部“生成 $\rightarrow$ 校验器批评 $\rightarrow$ 重试”微循环，以及跨腕足“生成节点 $\leftrightarrow$ 评判节点”对抗闭环。
4. **人机协同模式（Human-in-the-Loop, HITL）：** 高危操作触发拦截后状态机安全挂起，人类审批反馈永久沉淀为腕足的局部硬规则。

### 多框架生态兼容矩阵

| 框架 / 协议 | 对应层级 | 集成角色与价值 |
| :--- | :--- | :--- |
| **LangGraph** | 全局编排层 / 腕足运行时 | 作为全局带环状态机驱动 Brain，或封装为复杂的单腕足子图。 |
| **Google ADK** | 腕足运行时 | 快速将 Go/Python ADK 智能体包装为分布式 Octo-Arm 执行单元。 |
| **AutoGen / Semantic Kernel** | 腕足运行时 | 挂载多智能体讨论辩论组或企业级语义插件节点。 |
| **CrewAI** | 腕足运行时 | 封装角色扮演团队，通过共享数据基质交换产物。 |
| **Temporal** | 宏观编排持久化 | 为跨天级企业复杂工作流提供事件溯源与状态容灾。 |
| **Model Context Protocol (MCP)** | 工具与协议交互层 | 原生一等公民支持外部工具与动态生成工具的标准化调用。 |
| **AbloEx** | 安全与沙盒隔离层 | 承接高危指令与动态合成代码的微沙盒硬件级隔离执行。 |

---

## 核心 SPI 契约规范

### 五大核心服务提供者接口 (SPI)
* **`GlobalOrchestratorSPI`**：全局规划、协调器扇出与 HITL 挂起。
* **`ArmRuntimeAdapterSPI`**：非侵入式包装 ADK、LangGraph、原生 FSM 的腕足运行环境。
* **`SLMInferenceSPI`**：统一本地推理后端（Ollama / llama.cpp / vLLM）并提供关键字段 Logprob 提取。
* **`DataSubstrateSPI`**：支持 CAS 乐观锁版本控制与分布式租约的引用传递共享数据基质。
* **`ToolInvocationSPI`**：统一静态与动态 MCP 工具调用及安全沙盒隔离接口。

---

## 安全治理与离线韧性

* **动态工具静态 AST 扫描与微沙盒：** 动态代码执行前必经语法树安全审查，运行在受限微容器/Wasm 沙盒中，强限内存与 CPU 配额。
* **零信任与权限逐跳衰减：** 共享数据基质访问按 `session_id` 鉴权；横向总线多跳接力时权限随 `hop_count` 递增而单调收敛。
* **断网自治与离线降级：** 纵向总线失联时边缘腕足切入断网模式，凡命中 Gate 1/Gate 2 升级请求严格挂起存入本地 WAL 队列或转就地人工排队，绝不盲目放行。

---

## 双模部署支持与核心特性架构

### 双模部署支持
* **单机开发模式（`profile: local`）：** 零集群依赖，单机工作站运行本地小模型运行时（Ollama），配合内存共享数据基质与进程内注册表，适合敏捷开发与原型验证。
* **企业集群模式（`profile: cluster`）：** 基于 NATS JetStream 双平面总线与 Redis/Valkey 分布式共享数据基质，跨 Kubernetes 与边缘服务器弹性调度海量异构腕足。

### 命令行快速指令 (CLI)
```bash
octpex run       # 启动神经底座守护进程 (默认, 支持 -profile=local|cluster, -port=8080)
octpex eval      # 执行全内核自检与子系统连通性核验
octpex bench     # 执行超高吞吐性能与微秒级延迟基准压测 (>200万 ops/s)
octpex finops    # 输出 FinOps 财务降本与 Token ROI 审计分析报表
octpex version   # 查看当前内核版本与运行架构信息
```

### 核心系统特性与生产级能力矩阵
1. **单机极速仿生内核（`profile: local`）：**
   * 零外部集群依赖，提供纯内存原子 CAS 数据基质与本地双平面神经总线。
   * 完整支持全谱系智能适配：Level 0（Go 原生确定性 FSM，0 Token，< 2ms）与 Level 1（基于 Ollama 的边缘小模型本地反思闭环）。
   * 三道客观升级漏斗（前置策略 Gate 1、确定性 Schema/AST Gate 2、数学对数概率与防死循环 Gate 3）。
   * 中心大脑（Octo-Brain）对接前沿 LLM 服务，提供确定性 DAG 任务规划与 FinOps 财务遥测引擎。
2. **企业分布式集群中枢（`profile: cluster`）：**
   * 基于 NATS JetStream 构建双平面总线（纵向派发 `vertical.dispatch.*` 与横向接力 `lateral.handoff.*`），带消费组弹性负载均衡。
   * 基于 Redis/Valkey Lua 脚本实现原子 CAS 版本控制的分布式共享数据基质（`sub://`），带分布式 TTL 租约。
   * etcd 基底神经环注册表，5 秒租约心跳，节点故障秒级感知并自动踢出。
   * 跨地域多集群联邦路由中枢（`pkg/ring/federation.go`），支持跨数据中心信封流转与权限单调衰减。
3. **异构多框架生态与 Python SDK（`octpex-sdk`）：**
   * 原生提供 **Google ADK**、**LangGraph**、**Microsoft AutoGen** 与 **CrewAI** 官方生产级适配包装器。
   * 提供 `OctoArmDaemon`，支持将 Python 智能体作为常驻独立进程运行，通过 REST 接口与 Go 内核握手。
   * 包含开箱即用的四层异构多智能体同级接力实战示例（`examples/python_heterogeneous_mesh/`）。
4. **跨层级自进化与动态 MCP 工具箱：**
   * “大脑教腕足”能力蒸馏闭环：腕足遇到难题升级后，中枢大脑动态合成代码工具并注入基底神经环。
   * 完整支持标准模型上下文协议（MCP 2024-11-05），支持 `stdio` 与流式 HTTP/SSE 客户端。
   * AbloEx 静态 AST 语法树安全扫描审查，结合配额微沙盒实施硬安全隔离。
5. **实时动态看板与微秒级极速压测：**
   * 嵌入式 HTTP 服务内置实时 Server-Sent Events（SSE: `/api/v1/events`）脉冲流，单页 Web 动态看板免刷新呈现集群脉冲。
   * 完备的 REST API：拓扑感知、任务直接派发与 FinOps 审计报表输出。
   * 经实测验证的微秒级吞吐：**基质 CAS > 960 万 ops/s**、**总线信封 > 810 万 msgs/s**、**门禁核验 > 160 万 ops/s**。
6. **云原生敏捷交付与企业级 Helm 生产包：**
   * 25MB 极简生产级多阶段 Dockerfile、一键拉起全集群依赖的 Docker Compose 配置与 K8s 清单。
   * 企业级 Helm v3 Chart 生产包（`deploy/helm/octpex`），内置水平弹性伸缩（HPA）与存活/就绪健康探针。

---

## 文档索引

### 核心架构与产品规范
* [系统架构规范 (中文版)](docs/ARCHITECTURE_CN.md)
* [System Architecture Specification (English)](docs/ARCHITECTURE.md)
* [产品需求文档 (中文版)](docs/PRD_CN.md)
* [Product Requirements Document (English)](docs/PRD.md)

### 开发者实战与集群运维指南
* [开发者与生态集成综合指南 (中文版)](docs/developer_guide_cn.md)
* [Comprehensive Developer & Integration Guide (English)](docs/developer_guide.md)
* [部署运维与业务实战指南 (中文版)](docs/deployment_guide_cn.md)
* [Comprehensive Deployment & Operations Guide (English)](docs/deployment_guide.md)
* [English README (README.md)](README.md)

---

## 开源许可证

本项目基于 [Apache License 2.0](LICENSE) 协议开源。
