# Octpex 部署与集群运维综合指南

本文档涵盖 Octpex 的双模运行配置、生产集群依赖以及三大开箱即用业务实战示例的演练指南。

---

## 1. 运行与部署模式

Octpex 通过 `configs/` 目录下的配置文件进行运行时拓扑控制：

### 1.1 单机极速开发模式 (`configs/local.yaml`)

- **适用场景**：单机开发工作站、CI/CD 流水线与边缘离线测试。
- **运行拓扑**：零外部集群依赖（Go 内存通道总线、内存原子 CAS 数据基质、进程内注册表）。
- **边缘算力**：本地 Ollama 守护进程（`http://localhost:11434`），运行 `qwen2.5-coder:0.5b` 或 `7b`。
- **安全沙盒**：AbloEx 施加 256MB 内存与 5 秒超时硬配额。
- **启动指令**：
  ```bash
  go run cmd/octpex/main.go -profile=local
  ```

### 1.2 企业生产分布式集群模式 (`configs/cluster.yaml`)

- **适用场景**：Kubernetes 集群、跨物理机环境与大规模边缘计算节点。
- **神经总线**：基于 NATS JetStream 构建的双平面总线（`octpex.vertical.dispatch.*` 与 `octpex.lateral.handoff.*`），配置 `arm-workers` 消费组实现弹性伸缩。
- **共享数据基质**：基于 Redis / Valkey 集群配合 Lua 脚本实现原子 CAS 版本控制与分布式租赁锁。
- **能力注册表**：基于 etcd 租约心跳（5 秒 TTL）实现节点故障秒级感知与自动踢出。
- **启动指令**：
  ```bash
  go run cmd/octpex/main.go -profile=cluster -config=configs/cluster.yaml
  ```

### 1.3 Web 可视化实时看板与 REST API

Octpex 内核内嵌 HTTP 服务（默认端口 `8080`），提供实时的集群拓扑视图与财务审计分析：
- **Web UI 界面**：浏览器直接访问 `http://localhost:8080` 即可实时查看神经拓扑连接状态、健康的 Octo-Arm 节点及实时 FinOps 降本指标。
- **REST 接口**：
  - `GET /api/v1/topology`：查看当前已注册的 Arm 及其能力标签。
  - `GET /api/v1/finops`：获取财务审计指标（包括 Token 节约数、解耦比率与成本结余）。
  - `GET /api/v1/events`：实时 Server-Sent Events (SSE) 事件推流，动态呈现信封派发、门禁校验判定与脉冲跳跃。
  - `POST /api/v1/dispatch`：通过 HTTP POST 发送 JSON 神经信封直接派发任务。
- **自定义端口**：
  ```bash
  go run cmd/octpex/main.go -port=9090
  ```
  在无图形界面的极简环境或自动化测试下，传入 `-port=0` 即可完全禁用 HTTP 服务。

### 1.4 容器化与 Kubernetes 云原生编排

Octpex 在 `deploy/` 目录下提供了生产级开箱即用的集群交付配置：
- **Docker Compose (单机一键拉起集群中间件)**：
  ```bash
  docker compose -f deploy/docker-compose.yaml up -d
  ```
  一键拉起 Octpex 内核、NATS JetStream、Redis CAS 基质、etcd 注册表与 Ollama 容器。
- **Kubernetes 原生编排清单**：
  ```bash
  kubectl apply -f deploy/k8s/deployment.yaml
  ```
- **企业级 Helm Chart 包 (`deploy/helm/octpex`)**：
  ```bash
  helm install octpex deploy/helm/octpex -n octpex-system --create-namespace
  ```
  支持声明式配置副本数、弹性水平扩缩容 (HPA)、存活与就绪健康探针及自定义中间件连接。

---

## 2. 生产级业务实战示例演练

Octpex 在 `examples/` 目录下提供了四套开箱即用的完整实战代码：

### 2.1 示例一：单机极速上手 (`examples/local_quickstart/`)

验证章鱼式仿生协同链路（纵向派发 -> 横向多跳接力 -> 零智能原生 FSM 规则验证）：
- **Arm 1 (遥测日志去噪)**：过滤 92% 冗余日志，写入 `sub://sess/cleaned_log`，发起横向交接。
- **Arm 2 (补丁代码合成)**：零拷贝解析日志指针，生成修复代码写入 `sub://sess/patch.diff`，发起横向交接。
- **Arm 3 (原生 FSM 规则验证)**：纯 Go 确定性单测核验（< 2ms 完成，0 Token 消耗）。
- **运行**：
  ```bash
  go run examples/local_quickstart/main.go
  ```

### 2.2 示例二：AIOps 智能运维分诊网格 (`examples/aiops_triage_mesh/`)

模拟微服务高频故障排查与自主修复的完整链路：
- **Arm 1 (Loki 遥测数据分诊)**：生成标准事故卡片存入基质 `sub://sess/incident.json`。
- **Arm 2 (K8s AST 配置修复腕足)**：解析 Deployment 语法树，自动生成连接池扩容补丁。
- **Arm 3 (Prometheus 金丝雀验证腕足)**：Level 0 原生 Go 规则腕足，核验金丝雀指标是否满足 P99 SLA。
- **运行**：
  ```bash
  go run examples/aiops_triage_mesh/main.go
  ```

### 2.3 示例三：自进化解析器闭环 (“大脑教腕足”) (`examples/self_evolving_parser/`)

展示跨层级自进化与动态工具蒸馏闭环：
- **第一阶段**：腕足遇到全新专有格式日志（`OCTO-V2|...`），触发 Gate 2/Gate 3 门禁，向中心大脑升级。大脑动态合成解析工具，提交 AbloEx 静态 AST 扫描后准入 `DynamicMCPBox`。
- **第二阶段**：后续相同日志流入，腕足直接在本地命中蒸馏工具，以 **0 云端 Token** 和 **< 1ms 延迟** 本地消化。
- **运行**：
  ```bash
  go run examples/self_evolving_parser/main.go
  ```

### 2.4 示例四：Python 异构智能体网格演练 (`examples/python_heterogeneous_mesh/`)

演示完全基于 Python SDK 驱动的跨框架同级无缝协作（无需往返中心大模型）：
- **Arm 1 (Google ADK 诊断腕足)**：分析数据库连接池故障，将结构化诊断报告写入 `sub://sess/diagnosis.json`。
- **Arm 2 (Microsoft AutoGen 辩论腕足)**：Coder 与 Critic 智能体多轮审议生成修复补丁，写入 `sub://sess/patch.diff`。
- **Arm 3 (LangGraph 状态回归腕足)**：执行状态回归单测并通过 Gate 2 外部硬门禁核验，全程 0 云端 Token 消耗。
- **运行**：
  ```bash
  python examples/python_heterogeneous_mesh/main.py
  ```

---

## 3. FinOps 财务遥测与商业价值审计

Octpex 内置了 FinOps 财务审计计算引擎（`pkg/telemetry/finops.go`）：
- 实时记录云端 Token、边缘小模型 Token、耗时与升级率。
- 对比全量调用商业大模型的基准，动态计算为企业节省的美元资金（$ Saved USD）。
- 格式化输出高管财务汇报看板（实现 70%~85% 的推理成本节省）。
