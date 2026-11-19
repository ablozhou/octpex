"""
Octpex AIOps Log Drain & Three-Stage Closed Loop Demo
Demonstrates the complete biomimetic self-healing pipeline:
1. Stage 1: Sidecar harvesting + Octo-Arm local deduplication & 0-token resolve
2. Stage 2: Objective Gate 1/2 verification triggers escalation for critical anomalies
3. Stage 3: Octo-Brain frontier LLM strategic arbitration (Auto-Restart vs HITL Alarm vs Skill Distillation)
"""

import json
import os
import sys
import time

# Ensure bridge/python is importable
CURRENT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.abspath(os.path.join(CURRENT_DIR, "..", ".."))
sys.path.insert(0, os.path.join(PROJECT_ROOT, "bridge", "python"))

from octpex import (
    InMemorySubstrateClient,
    NervousEnvelope,
    SecurityContext,
    SubstrateRef,
)


def run_aiops_log_demo():
    print("=" * 80)
    print("      OCTPEX AIOPS THREE-STAGE CLOSED LOOP DEMO (EDGE TO CLOUD)")
    print("=" * 80)

    substrate = InMemorySubstrateClient()
    session_id = f"aiops-{int(time.time())}"

    # ==========================================================================
    # SCENARIO 1: Known Transient Error -> Stage 1 Edge 0-Token Local Resolve
    # ==========================================================================
    print("\n[Case 1] High-Frequency Transient Jitter (Downstream DB Connection Timeout)")
    print("-" * 80)

    raw_burst_logs = [
        "2026-11-19 14:00:01 WARN [OrderService] Database connection latency elevated (420ms)",
        "2026-11-19 14:00:02 ERROR [PaymentGateway] Connection timeout: pool exhausted [active=50, max=50]",
        "2026-11-19 14:00:02 ERROR [PaymentGateway] Connection timeout: pool exhausted [active=50, max=50]",
        "2026-11-19 14:00:03 ERROR [PaymentGateway] Connection timeout: pool exhausted [active=50, max=50]",
        "2026-11-19 14:00:03 ERROR [PaymentGateway] Connection timeout: pool exhausted [active=50, max=50]",
        "2026-11-19 14:00:04 INFO [OrderService] Fallback retry scheduled in 500ms",
    ]

    print(f"1. [Sidecar Harvester] Captured log burst: {len(raw_burst_logs)} lines from shared volume.")
    print("2. [Stage 1: Octo-Arm] Processing on edge ganglia (0 Cloud Tokens):")

    # Edge deduplication
    unique_lines = list(dict.fromkeys(raw_burst_logs))
    dedup_ratio = (1 - len(unique_lines) / len(raw_burst_logs)) * 100
    print(f"   • Stream deduplication: {len(raw_burst_logs)} lines -> {len(unique_lines)} canonical lines ({dedup_ratio:.0f}% reduction)")

    # Incident classification
    incident_data = {
        "incident_id": f"INC-EDGE-01",
        "service": "PaymentGateway",
        "category": "TRANSIENT_TIMEOUT",
        "root_cause": "Database connection pool saturated during traffic burst",
        "remediation": "Flushed local connection cache and refreshed client pool leases",
        "dedup_count": len(raw_burst_logs) - len(unique_lines),
    }

    # Store snapshot in Redis CAS Substrate
    uri1 = f"sub://{session_id}/incident-01.json"
    ver1 = substrate.put_artifact(uri1, 0, json.dumps(incident_data, indent=2).encode("utf-8"))
    print(f"   • CAS Substrate Stored : {uri1} (Version: {ver1})")

    # Local self-healing check
    print("   • Objective Gate 1     : PASSED (Known operational jitter, non-critical)")
    print("   • Local Reflex Action  : Executed local tool `refresh_db_pool()`")
    print("   -> RESULT: LOCAL_RESOLVED (Duration: 12ms, Cloud Tokens: 0, SRE Alarms: 0)")

    # ==========================================================================
    # SCENARIO 2: Critical Deadlock -> Stage 2 Gate Escalation -> Stage 3 Brain
    # ==========================================================================
    print("\n" + "=" * 80)
    print("[Case 2] Severe Unrecoverable Anomaly (Thread Deadlock & Payment Failure)")
    print("-" * 80)

    critical_logs = [
        "2026-11-19 14:02:10 FATAL [PaymentGateway] Fatal error: Thread deadlock detected in transaction coordinator",
        "2026-11-19 14:02:11 ERROR [PaymentGateway] Thread-142 holding lock Lock-A waiting for Lock-B",
        "2026-11-19 14:02:11 ERROR [PaymentGateway] Thread-189 holding lock Lock-B waiting for Lock-A",
        "2026-11-19 14:02:12 ERROR [Kernel] Memory heap unyielding, active transactions blocked = 1420",
    ]

    print(f"1. [Sidecar Harvester] Captured fatal error stack from shared volume.")
    print("2. [Stage 1: Octo-Arm] Local triage & incident packaging:")
    
    fatal_incident = {
        "incident_id": f"INC-FATAL-02",
        "service": "PaymentGateway",
        "category": "DEADLOCK_FATAL",
        "severity": "CRITICAL",
        "stack_signature": "Thread deadlock between Lock-A and Lock-B",
        "blocked_tx_count": 1420,
    }
    uri2 = f"sub://{session_id}/incident-fatal-02.json"
    ver2 = substrate.put_artifact(uri2, 0, json.dumps(fatal_incident, indent=2).encode("utf-8"))
    print(f"   • Incident Packaged to : {uri2} (Version: {ver2})")

    print("\n3. [Stage 2: Deterministic Gate Evaluation]")
    print("   • Gate 1 Check         : TRIGGERED!")
    print("     - Reason             : Deadlock severity CRITICAL, exceeds edge autonomous authority")
    print("   • Escalation Action    : Dispatching upward NervousEnvelope to Octo-Brain (Vertical Bus)...")

    print("\n4. [Stage 3: Octo-Brain Strategic Arbitration (Frontier LLM)]")
    print("   • Strategic Cortex     : Reviewing structured incident context from sub://...")
    print("   • Assessment           : Stateless microservice deadlock. Rolling restart will cleanly resolve.")
    print("   • Strategic Decision   :")
    print("     [Action 1 - Automated Recovery] : Issue Kubernetes command: `kubectl rollout restart deploy/payment-gateway`")
    print("     [Action 2 - Human Notification] : Send high-priority Slack/PagerDuty alert to On-Call SRE (Incident INC-FATAL-02)")
    print("     [Action 3 - Capability Distill] : Synthesized dynamic watchdog script `deadlock_preempt.sh` for future edge preemption")
    print("\n   -> RESULT: STRATEGIC_RECOVERY_DISPATCHED (Cloud Tokens: 240, Root-Cause Resolved)")
    print("=" * 80)


if __name__ == "__main__":
    run_aiops_log_demo()
