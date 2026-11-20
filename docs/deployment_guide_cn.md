# Octpex 部署与集群运维综合指南

本文档涵盖 Octpex 的双模运行配置、Kubernetes 与容器化集群依赖以及开箱即用业务实战示例的演练指南。

---

## 1. 运行与部署模式

Octpex 通过 `configs/` 目录下的配置文件进行运行时拓扑控制：

### 1.1 单机极速开发模式 (`configs/local.yaml`)

- **适用场景**：单机开发工作站、CI/CD 流水线与边缘离线测试。
- **运行拓扑**：零外部集群依赖（Go 内存通道总线、内存原子 CAS 数据基质、进程内注册表）。
- 默认遥测 Worker 使用原生 Go。嵌入 SLM 适配器的应用可另行配置 Ollama。
- 动态脚本由显式配置的隔离执行器运行。Docker 示例使用 256 MiB 和 5 秒超时。

#### 1.1.0 编译与运行方式 (Local Binary / Makefile / CLI)

在仓库根目录中，可以通过以下几种方式编译并执行 `octpex run`：

1. **一键编译本地二进制（推荐）**：
   ```bash
   make build
   ```
   该指令会自动以 `CGO_ENABLED=0` 调用 Go 静态编译，将原生二进制输出到 `./bin/` 目录下（Windows 下为 `bin/octpex.exe`，Linux/WSL 下为 `bin/octpex`）。
   若不使用 Makefile，直接使用原生 Go 命令编译时请指定 `CGO_ENABLED=0`（Octpex 为 100% 纯 Go，关闭 CGO 即可免除宿主机缺少 GCC 或 `stdlib.h` 库头文件的编译依赖）：
   ```bash
   CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/octpex ./cmd/octpex
   ```

2. **执行 `octpex run`**：
   - **通过编译出的二进制执行**：
     - Windows PowerShell：
       ```powershell
       .\bin\octpex.exe run -config configs/local.yaml
       ```
     - Linux / macOS / WSL：
       ```bash
       ./bin/octpex run -config configs/local.yaml
       ```
   - **通过 Makefile 一键运行（已参数化支持传入）**：
     ```bash
     # 1. 默认运行本地单机模式 (PROFILE=local PORT=8080)
     make run

     # 2. 运行分布式集群模式 (自动加载 configs/cluster.yaml)
     make run PROFILE=cluster

     # 3. 自定义端口运行 (例如改为 9090，或设为 0 禁用 HTTP 服务)
     make run PORT=9090

     # 4. 指定外部自定义配置文件或附加控制参数
     make run CONFIG=my_custom.yaml ARGS="-worker=false"
     ```
   - **免编译直接运行（使用 Go 原生工具链）**：
     ```bash
     go run ./cmd/octpex run -config configs/local.yaml
     ```
   - **全局直接敲 `octpex run`**：
     将仓库下的 `bin/` 路径添加到系统环境变量 `PATH` 中，即可在任意终端路径下直接执行 `octpex run`、`octpex eval` 或 `octpex bench`。

#### 1.1.1 WSL2 专属优化：无缝访问 Windows 宿主机本地 Ollama

在 Windows 工作站开发时，许多开发者习惯在 WSL2（Linux 子系统）内部运行 Octpex，而让 Ollama 运行在 Windows 宿主机上以直接利用 Windows 原生 GPU 驱动。在默认网络下，由于 WSL2 虚拟网卡与 Windows 隔离，经常出现无法连通 Windows 上 Ollama 的问题。

> [!TIP]
> **经过实测验证的最佳且唯一稳定的解决方案：开启 WSL2 镜像网络模式（Mirrored Networking）**
> 
> 1. **在 Windows 用户根目录配置 `.wslconfig`**：
>    在 `C:\Users\<您的用户名>\.wslconfig`（若文件不存在则直接创建）中写入：
>    ```ini
>    [wsl2]
>    networkingMode=mirrored
>    ```
> 2. **重启 WSL2**：
>    在 Windows 宿主机终端执行：
>    ```powershell
>    wsl --shutdown
>    ```
> 3. **确保 Windows 上的 Ollama 监听全局端口**：
>    在 Windows 系统环境变量中设置 `OLLAMA_HOST=0.0.0.0:11434`，并重启 Windows 版 Ollama。
> 4. **直接以 `localhost` 访问，零网关配置**：
>    在 WSL2 终端中直接访问 `http://localhost:11434` 即可直通 Windows 宿主机：
>    ```bash
>    curl http://localhost:11434/api/tags
>    ```
>    在 `configs/local.yaml` 中，`base_url` 直接保持默认的 `http://localhost:11434` 即可，完全不需要复杂的 IP 转换。

### 1.2 企业生产分布式集群模式 (`configs/cluster.yaml`)

- **适用场景**：Kubernetes 集群、跨物理机环境与大规模边缘计算节点。
- **神经总线**：基于 NATS JetStream 构建的双平面总线（`octpex.vertical.dispatch.*` 与 `octpex.lateral.handoff.*`），配置 `arm-workers` 消费组实现弹性伸缩。
- **共享数据基质**：基于 Redis / Valkey 集群配合 Lua 脚本实现原子 CAS 版本控制与分布式租赁锁。
- **能力注册表**：基于 etcd 租约心跳（5 秒 TTL）实现节点故障秒级感知与自动踢出。
- **启动指令**：
  ```bash
  (cd cluster && go run -mod=mod . -profile=cluster -config=../configs/cluster.yaml)
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

  双节点五容器验收可单独运行 `python cluster/test_workflow.py`，自动检查跨节点派发、Redis 产物和 B 节点离线。
- **Kubernetes 原生编排清单 (All-in-One 包含依赖一键拉起)**：
  详见 [k8s_multi_agent_guide_cn.md](k8s_multi_agent_guide_cn.md)：
  ```bash
  # 一键部署包含 Redis/NATS/etcd 与 Entry+Worker 多 Agent 的完整拓扑
  kubectl apply -f deploy/k8s/all-in-one.yaml
  ```
- **Kubernetes 生产独立部署清单**：
  ```bash
  kubectl apply -f deploy/k8s/deployment.yaml
  ```
- **企业级 Helm Chart 包 (`deploy/helm/octpex`) 与 SLM Sidecar 伴生注入**：
  ```bash
  helm install octpex deploy/helm/octpex -n octpex-system --create-namespace
  ```
  * 支持在 `values.yaml` 中集中声明多 LLM 前沿大模型池（`llm_providers`）与多 SLM 小模型库（`slm_catalog`）；
  * 腕足工作负载（`arms`）支持按需指定专用 SLM 伴生容器（如代码修复腕足挂载 `qwen2.5-coder-1.5b`），未指定时自动回退继承全局 `default_slm`（如 `qwen2.5-1.5b`）；
  * 提供即用型演练清单：`kubectl apply -f deploy/k8s/arm-sidecar-demo.yaml`，观察双容器 Pod 本地 Localhost 零跳低延迟伴生形态。

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
- 第一阶段返回升级结果并注册随示例提供的 Python 解析器。
- 第二阶段在隔离容器中处理实际输入，返回脚本标准输出并报告测得的耗时。
- **运行**：
  ```bash
  docker pull python:3.12-alpine
  go run ./examples/self_evolving_parser
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

## 4. 验证默认任务

```sh
go run ./cmd/octpex run -profile=local -port=8080
```

```sh
curl -s http://localhost:8080/api/v1/dispatch -H 'Content-Type: application/json' \
  -d '{"target_capability":"distill-telemetry","session_id":"demo","envelope_id":"e1","trace_id":"t1","parameters":{"log":"WARN retry\nWARN retry"}}'
```

预期返回 `SUCCESS`、一行 `WARN retry`、版本为 1 的产物引用和执行节点 ID。省略日志会返回 `ESCALATED` 及仲裁决定。FinOps 和 SSE 的观察范围为当前进程。
