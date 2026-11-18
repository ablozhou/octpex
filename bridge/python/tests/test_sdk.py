import json
import os
import sys
import unittest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from octpex import (
    NervousEnvelope,
    SubstrateRef,
    SecurityContext,
    InMemorySubstrateClient,
    ArmConfig,
    OctoArmDaemon,
    DecisionEngineProfile,
    DecisionQuestion,
    DecisionResult,
    System1Metrics,
    HARDWARE_TIER_1_LEGACY_8GB,
)
from octpex.adapters import LangGraphArmAdapter, CrewAIArmAdapter, GoogleADKArmAdapter, AutoGenArmAdapter


class TestOctpexPythonSDK(unittest.TestCase):

    def test_decision_engine_profile_and_metrics(self):
        profile = DecisionEngineProfile(
            primary_engine="laya-onnx-int8",
            secondary_engine="karpo-decision-2b-4bit",
            endpoint="http://127.0.0.1:8091/v1/systemone",
            safe_decision_context=32768,
            vram_budget_mb=0,
        )
        d = profile.to_dict()
        self.assertEqual(d["vram_budget_mb"], 0)
        self.assertEqual(d["primary_engine"], "laya-onnx-int8")

        reconstructed = DecisionEngineProfile.from_dict(d)
        self.assertEqual(reconstructed.safe_decision_context, 32768)

        # DecisionQuestion & Result
        q = DecisionQuestion(qid="q-1", qtype="choice", prompt="Direct resolve?", options=["YES", "NO"])
        self.assertEqual(len(q.to_dict()["options"]), 2)

        res = DecisionResult(
            qid="q-1",
            probabilities={"YES": 0.95, "NO": 0.05},
            top_option="YES",
            confidence=0.95,
            engine_used="laya-onnx-int8",
            latency_ms=12,
        )
        self.assertEqual(res.to_dict()["top_option"], "YES")

        # System1Metrics
        m = System1Metrics(
            system1_engine="laya-onnx-int8",
            system1_confidence=0.95,
            reverse_option_consistent=True,
            decision_latency_ms=12,
        )
        self.assertTrue(m.to_dict()["reverse_option_consistent"])

        # ArmConfig with hardware tier
        cfg = ArmConfig(
            arm_id="arm-synaptic-1",
            capability="discriminative",
            hardware_tier=HARDWARE_TIER_1_LEGACY_8GB,
            decision_profile=profile,
        )
        self.assertEqual(cfg.hardware_tier, HARDWARE_TIER_1_LEGACY_8GB)
        self.assertIsNotNone(cfg.decision_profile)

    def test_envelope_serialization(self):
        env = NervousEnvelope(
            envelope_id="env-py-1",
            trace_id="trace-py-1",
            session_id="session-py-1",
            plane="LATERAL",
            message_type="TASK_DISPATCH",
            source_node="arm-log-filter",
            target_capability="code_patcher",
            hop_count=1,
            substrate_refs=[SubstrateRef(uri="sub://sess/ast", version=2)],
            parameters={"target": "service.go"},
            security=SecurityContext(principal_id="test", max_allowed_hops=2, allow_mutation=True),
        )

        d = env.to_dict()
        self.assertEqual(d["envelope_id"], "env-py-1")
        self.assertEqual(d["plane"], "LATERAL")
        self.assertEqual(len(d["substrate_refs"]), 1)
        self.assertEqual(d["substrate_refs"][0]["version"], 2)

        # Roundtrip
        deserialized = NervousEnvelope.from_dict(d)
        self.assertEqual(deserialized.envelope_id, env.envelope_id)
        self.assertEqual(deserialized.security.max_allowed_hops, 2)

    def test_security_attenuation(self):
        sec = SecurityContext(max_allowed_hops=2, allow_mutation=True)
        sec.attenuate("code_patcher")
        self.assertEqual(sec.max_allowed_hops, 1)
        self.assertFalse(sec.allow_mutation)
        self.assertEqual(sec.allowed_capabilities, ["code_patcher"])

    def test_in_memory_substrate_cas(self):
        sub = InMemorySubstrateClient()
        uri = "sub://sess/data"

        # 1. First write (expected_version = 0)
        v1 = sub.put_artifact(uri, 0, b"initial payload")
        self.assertEqual(v1, 1)

        # 2. Get artifact
        data, ver = sub.get_artifact(uri)
        self.assertEqual(data, b"initial payload")
        self.assertEqual(ver, 1)

        # 3. Conflict when writing with stale expected_version
        with self.assertRaises(ValueError):
            sub.put_artifact(uri, 0, b"conflicting payload")

        # 4. Valid CAS update
        v2 = sub.put_artifact(uri, 1, b"updated payload")
        self.assertEqual(v2, 2)

    def test_langgraph_adapter_reflex_loop_success(self):
        sub = InMemorySubstrateClient()
        sub.put_artifact("sub://sess/log", 0, b"ERROR: null pointer at L42")

        attempts = 0

        def mock_langgraph_runner(state):
            nonlocal attempts
            attempts += 1
            critique = state.get("critique")
            if not critique:
                # First attempt: invalid output format missing 'status'
                return {"result": "partial patch"}
            # Second attempt after critique: valid output
            return {"status": "ok", "patch": "if (x != null) ..."}

        config = ArmConfig(arm_id="arm-langgraph-1", capability="patcher", max_reflex_retries=2)
        adapter = LangGraphArmAdapter(config, mock_langgraph_runner, substrate=sub)

        # Set Gate 2 verifier: requires JSON containing "status": "ok"
        def gate2_verifier(output_str):
            try:
                data = json.loads(output_str)
                if data.get("status") != "ok":
                    return "missing status=ok field"
                return None
            except Exception as e:
                return f"invalid json: {e}"

        adapter.set_gate2_verifier(gate2_verifier)

        env = NervousEnvelope(
            envelope_id="env-1",
            trace_id="tr-1",
            session_id="sess",
            plane="VERTICAL",
            message_type="TASK_DISPATCH",
            source_node="octo-brain",
            substrate_refs=[SubstrateRef(uri="sub://sess/log", version=1)],
        )

        res = adapter.execute_envelope(env)
        self.assertEqual(res.status, "SUCCESS")
        self.assertEqual(res.metadata["retries"], 1)
        self.assertIn("status", res.output_message)
        self.assertEqual(attempts, 2)

    def test_crewai_adapter_retry_exhaustion_escalation(self):
        def mock_crew_kickoff(inputs):
            # Always fails validation
            return "improper result"

        config = ArmConfig(arm_id="arm-crew-1", capability="analyzer", max_reflex_retries=2)
        adapter = CrewAIArmAdapter(config, mock_crew_kickoff)

        # Gate 2 verifier that rejects
        adapter.set_gate2_verifier(lambda out: "always reject for test")

        env = NervousEnvelope(
            envelope_id="env-2",
            trace_id="tr-2",
            session_id="sess",
            plane="VERTICAL",
            message_type="TASK_DISPATCH",
            source_node="octo-brain",
        )

        res = adapter.execute_envelope(env)
        self.assertEqual(res.status, "ESCALATED")
        self.assertTrue(res.metadata.get("escalated"))

    def test_autogen_adapter_success(self):
        sub = InMemorySubstrateClient()
        sub.put_artifact("sub://sess/debate_topic", 0, b"Evaluate security vulnerability in auth.go")

        def mock_autogen_debate(context):
            return {
                "consensus": "CVE verified and patch proposed",
                "rounds": context.get("max_rounds", 3),
                "patch": "validateToken(token)",
            }

        config = ArmConfig(arm_id="arm-autogen-1", capability="security_debate", max_reflex_retries=2)
        adapter = AutoGenArmAdapter(config, mock_autogen_debate, max_debate_rounds=3, substrate=sub)

        # Gate 2 verifier checks for "consensus" in output
        adapter.set_gate2_verifier(lambda out: None if "consensus" in out else "no consensus")

        env = NervousEnvelope(
            envelope_id="env-autogen-1",
            trace_id="tr-autogen-1",
            session_id="sess",
            plane="VERTICAL",
            message_type="TASK_DISPATCH",
            source_node="octo-brain",
            substrate_refs=[SubstrateRef(uri="sub://sess/debate_topic", version=1)],
        )

        res = adapter.execute_envelope(env)
        self.assertEqual(res.status, "SUCCESS")
        self.assertIn("CVE verified", res.output_message)

    def test_octo_arm_daemon_local_execution(self):
        def echo_run(ctx):
            params = ctx.get("inputs", {})
            return "Echo: " + str(params.get("text", "none"))

        config = ArmConfig(arm_id="arm-daemon-test", capability="echo")
        adapter = GoogleADKArmAdapter(config, echo_run)
        daemon = OctoArmDaemon(adapter, server_url="http://127.0.0.1:8080")

        env = NervousEnvelope(
            envelope_id="env-daemon-1",
            trace_id="tr-daemon-1",
            session_id="sess",
            plane="LATERAL",
            message_type="TASK_DISPATCH",
            source_node="arm-sender",
            parameters={"text": "hello daemon"},
        )

        result = daemon.handle_envelope(env)
        self.assertEqual(result.status, "SUCCESS")
        self.assertIn("hello daemon", result.output_message)


if __name__ == "__main__":
    unittest.main()
