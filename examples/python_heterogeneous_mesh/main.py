"""
Octpex Heterogeneous Multi-Agent Mesh Example (Python SDK)
Demonstrates seamless biomimetic lateral coordination across:
- Arm 1: Google ADK (Level 2 Wrapped Single Agent: Incident Triage)
- Arm 2: Microsoft AutoGen (Level 3 Multi-Agent Crew: Coder-Critic Debate)
- Arm 3: LangGraph (Level 2 Stateful Workflow: Unit Test & Gate 2 Verifier)
All exchanging zero-copy pointers over the Shared Data Substrate (sub://).
"""

import json
import os
import sys
import time

# Ensure bridge/python is in sys.path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", "bridge", "python")))

from octpex import (
    NervousEnvelope,
    SubstrateRef,
    SecurityContext,
    InMemorySubstrateClient,
    ArmConfig,
)
from octpex.adapters import (
    GoogleADKArmAdapter,
    AutoGenArmAdapter,
    LangGraphArmAdapter,
)


def run_heterogeneous_mesh():
    print("=" * 80)
    print("      OCTPEX HETEROGENEOUS AGENT MESH: ADK + AUTOGEN + LANGGRAPH")
    print("=" * 80)

    # 1. Initialize Shared Data Substrate
    substrate = InMemorySubstrateClient()
    session_id = "sess-mesh-8801"

    # Seed initial telemetry into substrate
    raw_alert = {
        "alert_id": "ALT-904",
        "service": "order-gateway",
        "error": "database connection pool exhausted: active=100 max=100",
        "host": "node-us-east-1a",
    }
    substrate.put_artifact(
        f"sub://{session_id}/alert.json", 0, json.dumps(raw_alert).encode("utf-8")
    )
    print(f"[Substrate] Seeded raw alert data at sub://{session_id}/alert.json (Version: 1)")

    # --------------------------------------------------------------------------
    # Arm 1: Google ADK (Incident Diagnosis Agent)
    # --------------------------------------------------------------------------
    def adk_triage_runner(ctx: dict) -> dict:
        alert_data = raw_alert
        return {
            "root_cause": "Connection leak in handleCheckout() transaction loop",
            "recommended_action": "Add defer db.Close() and increase pool idle timeout",
            "target_file": "gateway/checkout.go",
        }

    adk_config = ArmConfig(arm_id="arm-adk-triage", capability="triage_diagnosis")
    adk_arm = GoogleADKArmAdapter(adk_config, adk_triage_runner, substrate=substrate)

    # --------------------------------------------------------------------------
    # Arm 2: Microsoft AutoGen (Coder-Critic Debate Team)
    # --------------------------------------------------------------------------
    def autogen_debate_runner(ctx: dict) -> dict:
        # Simulates AutoGen Coder and Critic agents reaching consensus
        return {
            "consensus": "Patch generated and approved by Critic agent",
            "diff": "--- gateway/checkout.go\n+++ gateway/checkout.go\n+ defer tx.Rollback()\n+ conn.SetConnMaxIdleTime(5 * time.Minute)",
            "file": "gateway/checkout.go",
        }

    autogen_config = ArmConfig(arm_id="arm-autogen-coder", capability="patch_synthesis")
    autogen_arm = AutoGenArmAdapter(autogen_config, autogen_debate_runner, max_debate_rounds=3, substrate=substrate)

    # --------------------------------------------------------------------------
    # Arm 3: LangGraph (Stateful Verification & Regression Test Node)
    # --------------------------------------------------------------------------
    def langgraph_verifier_runner(state: dict) -> dict:
        # Simulates executing test suite in sandboxed container
        return {
            "status": "ok",
            "tests_passed": 24,
            "tests_failed": 0,
            "regression_detected": False,
        }

    langgraph_config = ArmConfig(arm_id="arm-langgraph-verifier", capability="unit_regression_test")
    langgraph_arm = LangGraphArmAdapter(langgraph_config, langgraph_verifier_runner, substrate=substrate)

    # Attach Gate 2 objective verifier to LangGraph Arm
    def gate2_verifier(output_str: str) -> str | None:
        try:
            data = json.loads(output_str)
            if data.get("status") != "ok":
                return "Output status is not 'ok'"
            if data.get("tests_failed", 0) > 0:
                return "Unit tests failed"
            return None
        except Exception as e:
            return f"Invalid JSON: {e}"

    langgraph_arm.set_gate2_verifier(gate2_verifier)

    # ==========================================================================
    # BIOMIMETIC PIPELINE EXECUTION (Lateral Handoffs)
    # ==========================================================================
    print("\n[Step 1: Vertical Dispatch -> Arm 1 (Google ADK)]")
    env1 = NervousEnvelope(
        envelope_id="env-step-1",
        trace_id="tr-mesh-01",
        session_id=session_id,
        plane="VERTICAL",
        message_type="TASK_DISPATCH",
        source_node="octo-brain",
        target_capability="triage_diagnosis",
        substrate_refs=[SubstrateRef(uri=f"sub://{session_id}/alert.json", version=1)],
        security=SecurityContext(principal_id="brain", max_allowed_hops=3, allow_mutation=True),
    )
    res1 = adk_arm.execute_envelope(env1)
    print(f"  -> Arm 1 Output: {res1.output_message}")

    # Write Arm 1 diagnosis to Substrate
    substrate.put_artifact(f"sub://{session_id}/diagnosis.json", 0, res1.output_message.encode("utf-8"))
    print(f"  -> Stored diagnosis at sub://{session_id}/diagnosis.json (Version: 1)")

    # Lateral Hop: Arm 1 -> Arm 2
    print("\n[Step 2: Lateral Handoff -> Arm 2 (Microsoft AutoGen)]")
    env2 = env1.create_lateral_handoff(
        new_envelope_id="env-step-2",
        source_arm="arm-adk-triage",
        target_capability="patch_synthesis",
        new_refs=[SubstrateRef(uri=f"sub://{session_id}/diagnosis.json", version=1)],
    )
    res2 = autogen_arm.execute_envelope(env2)
    print(f"  -> Arm 2 (AutoGen Debate Consensus): {res2.output_message}")

    # Write Arm 2 patch to Substrate
    substrate.put_artifact(f"sub://{session_id}/patch.diff", 0, res2.output_message.encode("utf-8"))
    print(f"  -> Stored patch at sub://{session_id}/patch.diff (Version: 1)")

    # Lateral Hop: Arm 2 -> Arm 3
    print("\n[Step 3: Lateral Handoff -> Arm 3 (LangGraph + Gate 2)]")
    env3 = env2.create_lateral_handoff(
        new_envelope_id="env-step-3",
        source_arm="arm-autogen-coder",
        target_capability="unit_regression_test",
        new_refs=[SubstrateRef(uri=f"sub://{session_id}/patch.diff", version=1)],
    )
    res3 = langgraph_arm.execute_envelope(env3)
    print(f"  -> Arm 3 Gate 2 Result: {res3.status}")
    print(f"  -> Verification Report : {res3.output_message}")

    print("\n" + "=" * 80)
    print("                     MESH PIPELINE COMPLETE: SUCCESS")
    print("=" * 80)
    print(f"Final Pipeline Status   : {res3.status}")
    print("Lateral Handoffs Made   : 2 peer-to-peer hops (0 Central Brain roundtrips)")
    print("Estimated Cloud Cost    : $0.00 USD (Resolved 100% via Edge & Local Agents)")
    print("Security Attenuation    : Remaining hops allowed = " + str(env3.security.max_allowed_hops))
    print("=" * 80)


if __name__ == "__main__":
    run_heterogeneous_mesh()
