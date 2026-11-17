import json
from typing import Any, Optional, Dict, Callable
from ..arm import OctoArmWorker, ArmConfig
from ..protocol import NervousEnvelope
from ..substrate import SubstrateClient


class LangGraphArmAdapter(OctoArmWorker):
    """
    Wraps a LangGraph CompiledGraph / StateGraph as an Octpex Octo-Arm worker pod.
    """

    def __init__(
        self,
        config: ArmConfig,
        graph_runner: Callable[[Dict[str, Any]], Dict[str, Any]],
        substrate: Optional[SubstrateClient] = None,
    ):
        super().__init__(config, substrate)
        self.graph_runner = graph_runner

    def act(self, envelope: NervousEnvelope, critique: Optional[str] = None) -> str:
        # Build LangGraph input state
        initial_state = {
            "parameters": envelope.parameters,
            "session_id": envelope.session_id,
            "trace_id": envelope.trace_id,
            "critique": critique,
        }

        # Resolve substrate references if present
        if self.substrate and envelope.substrate_refs:
            for ref in envelope.substrate_refs:
                try:
                    data, ver = self.substrate.get_artifact(ref.uri)
                    initial_state[f"ref_{ref.uri}"] = data.decode("utf-8")
                except Exception as e:
                    initial_state[f"ref_err_{ref.uri}"] = str(e)

        # Run LangGraph workflow
        result_state = self.graph_runner(initial_state)

        # Extract output from graph state
        if isinstance(result_state, dict):
            if "output" in result_state:
                return str(result_state["output"])
            return json.dumps(result_state)
        return str(result_state)
