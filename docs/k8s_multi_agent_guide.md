# Step-by-Step Guide: Deploying and Verifying Octpex Multi-Agent Cluster on Kubernetes

This guide walks through deploying and verifying a **distributed Octpex multi-agent cluster** from scratch using **Docker Desktop's built-in Kubernetes** (or any standard K8s cluster such as Minikube, Kind, or managed cloud K8s).

---

## 1. Architecture & Multi-Agent Roles

In this deployment, Kubernetes orchestrates five services inside the `octpex-system` namespace:

```
                            [ User Requests / Web UI ]
                                         │
                                         ▼
                            ┌────────────────────────┐
                            │  octpex-entry (Cortex) │
                            │ (Dispatch, worker=false│
                            └──────────┬─────────────┘
                                       │
           ┌───────────────────────────┼───────────────────────────┐
           │ (NATS Dual-Plane Bus)     │ (etcd Capability Registry)│ (Redis CAS Substrate)
           ▼                           ▼                           ▼
  ┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
  │  nats-cluster   │         │  etcd-cluster   │         │  redis-cluster  │
  │   (JetStream)   │         │  (5s Lease TTL) │         │ (Lua CAS Leases)│
  └────────┬────────┘         └────────┬────────┘         └────────┬────────┘
           │                           │                           │
           └───────────────────────────┼───────────────────────────┘
                                       │
                   ┌───────────────────┴───────────────────┐
                   ▼                                       ▼
       ┌───────────────────────┐               ┌───────────────────────┐
       │  octpex-worker Pod 1  │               │  octpex-worker Pod 2  │
       │ (distill-telemetry Arm)│              │ (distill-telemetry Arm)│
       └───────────────────────┘               └───────────────────────┘
```

1. **Infrastructure Tier**:
   - `nats-cluster:4222`: NATS JetStream dual-plane bus handling task dispatch and lateral handoffs.
   - `etcd-cluster:2379`: Basal Nerve Ring registry enforcing 5-second lease heartbeats.
   - `redis-cluster:6379`: Shared Data Substrate managing `sub://` references and atomic Lua CAS version leases.
2. **Multi-Agent Runtimes**:
   - **Central Dispatch Agent (`octpex-entry`)**: Central brain and API gateway (`-worker=false`) exposing REST APIs and Web UI.
   - **Edge Ganglion Agents (`octpex-worker`, 2 replicas)**: Specialized execution arms registered with capability `distill-telemetry`.

---

## 2. Prerequisites & Image Build

### 2.1 Enable Kubernetes in Docker Desktop
- Open Docker Desktop Settings -> **Kubernetes** -> Check **Enable Kubernetes** -> Click **Apply & restart**.
- Verify:
  ```bash
  kubectl cluster-info
  kubectl get nodes
  ```

### 2.2 Build Octpex Kernel Image
Build the container image embedding the production cluster driver:

```bash
docker build -t octpex/kernel:0.5.0 -f deploy/docker/Dockerfile .
```

*Note: Docker Desktop Kubernetes natively shares local Docker images.*

---

## 3. One-Click Cluster Deployment

Deploy the complete manifest [deploy/k8s/all-in-one.yaml](file:///c:/Users/abloz/Documents/code/octpex/deploy/k8s/all-in-one.yaml):

```bash
kubectl apply -f deploy/k8s/all-in-one.yaml
```

### Check Pod Status
Wait 10-30 seconds, then inspect all pods:

```bash
kubectl get pods -n octpex-system -o wide
```

All 6 pods should transition to `Running`:
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

## 4. End-to-End Multi-Agent Verification

### Step 1: Forward Entry Port
Expose the HTTP API:

```bash
kubectl port-forward -n octpex-system svc/octpex-service 8080:8080
```

---

### Step 2: Verify Dynamic Arm Discovery
Query the topology endpoint:

```bash
curl -s http://localhost:8080/api/v1/topology | jq .
```

Verify that `arm_count` equals `2`, confirming both worker pods registered successfully with etcd.

---

### Step 3: Dispatch Cross-Node Task
Send a telemetry deduplication workload to `octpex-entry`:

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

**Verification checks:**
1. `metadata.worker_id` points to one of the `octpex-worker` pods, proving cross-node NATS dispatch;
2. `substrate_refs` contains `sub://k8s-session-101/distilled/env-k8s-001` with version `1`.

---

### Step 4: Verify Redis CAS Shared Substrate Readback
Inspect raw Redis storage inside the Redis pod:

```bash
kubectl exec -n octpex-system deploy/redis-cluster -- redis-cli HGETALL "octpex:sub:sub://k8s-session-101/distilled/env-k8s-001"
```

Confirms persistent Pass-by-Reference storage with monotonic versioning.

---

### Step 5: Verify FinOps Audit Metrics
Check cost and token metrics:

```bash
curl -s http://localhost:8080/api/v1/finops | jq .
```

Confirms tasks are attributed to edge workers with zero cloud token consumption.

---

### Step 6: Test Pod Failure Recovery
Simulate pod death:

```bash
kubectl delete pod -n octpex-system -l app=octpex-worker --force --grace-period=0
```

Monitor topology:
```bash
curl -s http://localhost:8080/api/v1/topology | jq .arm_count
```
The dead arm lease expires in `< 5s` and Kubernetes brings up a replacement that auto-registers.

---

### Step 7: Open Web UI Dashboard
Open your browser to:
```text
http://localhost:8080
```
View the live bionic topology diagram, Server-Sent Events stream, and FinOps cost curves.

---

## 5. Teardown

```bash
kubectl delete -f deploy/k8s/all-in-one.yaml
```
Deletes the `octpex-system` namespace and all attached resources cleanly.
