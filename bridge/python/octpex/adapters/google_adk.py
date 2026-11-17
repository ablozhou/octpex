import json
from typing import Any, Optional, Dict, Callable
from ..arm import OctoArmWorker, ArmConfig
from ..protocol import NervousEnvelope
from ..substrate import SubstrateClient


class GoogleADKArmAdapter(OctoArmWorker):
    """
    Wraps a Google Agent Development Kit (ADK) agent execution as an Octpex Octo-Arm worker pod.
    """

    def __init__(
        self,
        config: ArmConfig,
        adk_run_fn: Callable[[Dict[str, Any]], Any],
        substrate: Optional[SubstrateClient] = None,
    ):
        super().__init__(config, substrate)
        self.adk_run_fn = adk_run_fn

    def act(self, envelope: NervousEnvelope, critique: Optional[str] = None) -> str:
        prompt_context = {
            "inputs": envelope.parameters,
            "session_id": envelope.session_id,
        }
        if critique:
            prompt_context["revision_instructions"] = critique

        result = self.adk_run_fn(prompt_context)
        if isinstance(result, (dict, list)):
            return json.dumps(result)
        return str(result)
