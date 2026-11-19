# 基于 Kubernetes 从零搭建与验证 Octpex 多智能体分布式集群指引

本指引带您使用 **Docker Desktop 内置 Kubernetes**（或任何标准 K8s 集群：Minikube、Kind、云托管 K8s）从零开始完整部署一套 **Octpex 仿生多 Agent 分布式系统**，并完成端到端跨节点调度、数据基质存证、故障自愈与可视化看板的实测验收。

---

## 一、集群拓扑与多 Agent 角色分工

在本次部署中，Kubernetes 将在 `octpex-system` 命名空间中编排以下 5 类服务：

```
                              [ 用户请求 / Web 看板 ]
                                         │
                                         ▼
                            ┌────────────────────────┐
                            │  octpex-entry (中枢)   │
                            │ (调度派发, worker=false)│
                            └──────────┬─────────────┘
                                       │
           ┌───────────────────────────┼───────────────────────────┐
           │ (NATS 双平面消息流)       │ (etcd 动态能力注册)       │ (Redis CAS 共享底座)
           ▼                           ▼                           ▼
  ┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
  │  nats-cluster   │         │  etcd-cluster   │         │  redis-cluster  │
  │   (JetStream)   │         │  (5s Lease TTL) │         │ (Lua CAS 租约)  │
  └────────┬────────┘         └────────┬────────┘         └────────┬────────┘
           │                           │                           │
           └───────────────────────────┼───────────────────────────┘
                                       │
                   ┌───────────────────┴───────────────────┐
                   ▼                                       ▼
       ┌───────────────────────┐               ┌───────────────────────┐
       │  octpex-worker Pod 1  │               │  octpex-worker Pod 2  │
       │ (distill-telemetry腕足)│               │ (distill-telemetry腕足)│
       └───────────────────────┘               └───────────────────────┘
```

1. **基础设施三件套**：
   - `nats-cluster:4222`：NATS JetStream 消息总线，承载纵向派发与横向总线；
   - `etcd-cluster:2379`：基底神经环能力注册表，管理 5 秒租约与健康心跳；
   - `redis-cluster:6379`：共享数据底座（Shared Data Substrate），管理 `sub://` 零拷贝引用与原子 CAS 版本租约。
2. **多 Agent 节点**：
   - **中枢调度 Agent (`octpex-entry`)**：作为顶层大脑与 API 网关（`-worker=false`），负责任务接收、全局编排、拓扑维护与 Web 仪表盘暴露；
   - **边缘工作 Agent (`octpex-worker`，2 副本)**：作为专业执行腕足（搭载 `distill-telemetry` 遥测去噪能力），自注册至 etcd，并发监听 NATS 消费组，执行完毕将轻量指针写入 Redis。

---

## 二、前置准备与镜像构建

### 1. 确保 Kubernetes 正常运行
如果使用 **Docker Desktop**：
- 打开 Docker Desktop 设置 -> **Kubernetes** -> 勾选 **Enable Kubernetes** -> 点击 **Apply & restart**；
- 在终端验证：
  ```bash
  kubectl cluster-info
  kubectl get nodes
  ```
  确认输出 `Ready` 状态。

### 2. 构建 Octpex 集群节点镜像
在项目根目录下构建包含真实生产驱动（`cluster/` 模块）的容器镜像：

```bash
docker build -t octpex/kernel:0.5.0 -f deploy/docker/Dockerfile .
```

*注：Docker Desktop Kubernetes 原生共享本机的 Docker 镜像，无需 push 到外部镜像仓库。*

---

## 三、一键部署全套多 Agent 环境

项目中已为您准备好开箱即用的全要素资源编排清单 [deploy/k8s/all-in-one.yaml](file:///c:/Users/abloz/Documents/code/octpex/deploy/k8s/all-in-one.yaml)：

```bash
kubectl apply -f deploy/k8s/all-in-one.yaml
```

### 检查 Pod 部署状态
等待约 10-30 秒，拉起全部组件：

```bash
kubectl get pods -n octpex-system -o wide
```

预期看到以下全部处于 `Running` 状态的 Pod：
```text
NAME                             READY   STATUS    RESTARTS   AGE
etcd-cluster-xxxxxxxxx-xxxxx     1/1     Running   0          25s
nats-cluster-xxxxxxxxx-xxxxx     1/1     Running   0          25s
redis-cluster-xxxxxxxxx-xxxxx    1/1     Running   0          25s
octpex-entry-xxxxxxxxx-xxxxx     1/1     Running   0          25s
octpex-worker-xxxxxxxxx-xxxxx1   1/1     Running   0          25s
octpex-worker-xxxxxxxxx-xxxxx2   1/1     Running   0          25s
```

---

## 四、端到端跨节点多 Agent 协同验证

### 步骤 1：暴露中枢服务端口
在本地开启端口转发，映射中枢 Agent 的 HTTP 服务：

```bash
kubectl port-forward -n octpex-system svc/octpex-service 8080:8080
```

*(保持此终端窗口运行，在另一个新终端中执行以下验证指令)*

---

### 步骤 2：验证多 Agent 自动发现与拓扑感知
通过 REST API 查询当前集群中的活跃智能体：

```bash
curl -s http://localhost:8080/api/v1/topology | jq .
```

**预期输出：**
```json
{
  "active_arms": [
    {
      "arm_id": "arm-telemetry-01-octpex-worker-xxx-xxx",
      "capability_tags": [
        "distill-telemetry"
      ],
      "role_name": "telemetry-distiller",
      "runtime_adapter": "go-native-fsm",
      "telemetry": {
        "status": "HEALTHY"
      }
    },
    {
      "arm_id": "arm-telemetry-01-octpex-worker-yyy-yyy",
      "capability_tags": [
        "distill-telemetry"
      ],
      "role_name": "telemetry-distiller",
      "runtime_adapter": "go-native-fsm",
      "telemetry": {
        "status": "HEALTHY"
      }
    }
  ],
  "arm_count": 2
}
```
**判定标准**：`arm_count` 为 `2`，表示两个 Worker Pod 已通过 etcd 成功注册并建立了心跳。

---

### 步骤 3：跨节点任务分发与去噪闭环
向 `octpex-entry` 派发一个包含重复日志的复杂任务，观察中枢如何将任务通过 NATS 跨节点分发至 Worker Pod：

```bash
curl -s -X POST http://localhost:8080/api/v1/dispatch \
  -H "Content-Type: application/json" \
  -d '{
    "target_capability": "distill-telemetry",
    "envelope_id": "env-k8s-001",
    "session_id": "k8s-session-101",
    "trace_id": "trace-k8s-101",
    "parameters": {
      "log": "2026-11-19 WARN Connection timeout\n2026-11-19 WARN Connection timeout\n2026-11-19 INFO Reconnected successfully"
    }
  }' | jq .
```

**预期输出：**
```json
{
  "status": "SUCCESS",
  "output_message": "2026-11-19 WARN Connection timeout\n2026-11-19 INFO Reconnected successfully",
  "substrate_refs": [
    {
      "uri": "sub://k8s-session-101/distilled/env-k8s-001",
      "version": 1,
      "content_type": "text/plain",
      "byte_size": 75
    }
  ],
  "metadata": {
    "trace_id": "trace-k8s-101",
    "unique_lines": 2,
    "worker_id": "arm-telemetry-01-octpex-worker-xxx-xxx"
  }
}
```
**判定标准**：
1. `worker_id` 显示任务是由后台的某个 `octpex-worker` Pod 执行的，而非入口 `entry` 执行；
2. 重复的日志被就地去噪压缩；
3. 生成了指向共享底座的轻量 URI：`sub://k8s-session-101/distilled/env-k8s-001`（版本为 1）。

---

### 步骤 4：验证共享数据底座（Redis CAS）真实存证
直接进入 Redis Pod，验证 Worker 写入的数据是否真实持久化：

```bash
kubectl exec -n octpex-system deploy/redis-cluster -- redis-cli KEYS "octpex:sub:*"
```

查看去噪产物内容与单调递增版本号：
```bash
kubectl exec -n octpex-system deploy/redis-cluster -- redis-cli HGETALL "octpex:sub:sub://k8s-session-101/distilled/env-k8s-001"
```
**预期输出：**
```text
1) "data"
2) "2026-11-19 WARN Connection timeout\n2026-11-19 INFO Reconnected successfully"
3) "version"
4) "1"
```
证明零拷贝指针（Pass-by-Reference）已在分布式共享底座中可靠存证。

---

### 步骤 5：验证 FinOps 独立财务核算
检查 Entry 节点与 Worker 节点的 Token 与成本统计：

```bash
curl -s http://localhost:8080/api/v1/finops | jq .
```

**判定标准**：由于 `octpex-entry` 只做调度，执行任务发生在 Worker 上，系统将精确区分中枢协调成本与边缘执行成本。

---

### 步骤 6：故障注入与高可用自愈验证（模拟 Agent 崩溃）
测试当其中一个 Agent 发生 Pod 异常崩溃时，系统是否能在 5 秒内自动剔除并由 K8s 重启恢复：

1. **强行杀死其中一个 Worker Pod**：
   ```bash
   kubectl delete pod -n octpex-system -l app=octpex-worker --force --grace-period=0
   ```
2. **立即观察拓扑变化**：
   ```bash
   curl -s http://localhost:8080/api/v1/topology | jq .arm_count
   ```
   *由于 etcd 租约机制，坏死节点的租约在 5 秒内失效，存活腕足数从 2 短暂变为 1 或重新拉起恢复为 2。*
3. **K8s 重新拉起新 Pod 并自动注册**：
   ```bash
   kubectl get pods -n octpex-system -l app=octpex-worker
   ```
   新 Pod 启动后自动建立新的 etcd 租约，`arm_count` 自动恢复为 `2`，整个过程对调用端无感。

---

### 步骤 7：在浏览器查看实时可视化看板

在浏览器中直接打开：
```text
http://localhost:8080
```

看板将直观呈现：
- **实时拓扑结构图**：`Octo-Brain (中枢)` 与下属多个 `Octo-Arms (腕足)` 的实时心跳与健康状态；
- **Server-Sent Events (SSE) 事件脉冲流**：观察每一次任务分发（`ENVELOPE_DISPATCH`）与节点响应；
- **FinOps 降本看板**：查看 0-Token 闭环率、节省的总 Token 数量与折算美元 ROI。

---

## 五、环境清理

验证完成后，一键清理所有 Kubernetes 资源：

```bash
kubectl delete -f deploy/k8s/all-in-one.yaml
```
该操作会干净移除 `octpex-system` 命名空间及其中所有 Pod、Service 和配置。
