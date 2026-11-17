import json
from typing import Any, Optional, Dict, Callable
from ..arm import OctoArmWorker, ArmConfig
from ..protocol import NervousEnvelope
from ..substrate import SubstrateClient


class AutoGenArmAdapter(OctoArmWorker):
    """
    Wraps a Microsoft AutoGen GroupChat or multi-agent debate team
    as an Octpex Level 3 Octo-Arm worker pod.
    """

    def __init__(
        self,
        config: ArmConfig,
        autogen_run_fn: Callable[[Dict[str, Any]], Any],
        max_debate_rounds: int = 3,
        substrate: Optional[SubstrateClient] = None,
    ):
        super().__init__(config, substrate)
        self.autogen_run_fn = autogen_run_fn
        self.max_debate_rounds = max_debate_rounds

    def act(self, envelope: NervousEnvelope, critique: Optional[str] = None) -> str:
        debate_context = {
            "task": envelope.parameters,
            "session_id": envelope.session_id,
            "max_rounds": self.max_debate_rounds,
        }
        if critique:
            debate_context["critic_guidance"] = critique

        # Inject substrate artifacts if present
        if self.substrate and envelope.substrate_refs:
            for ref in envelope.substrate_refs:
                try:
                    data, _ = self.substrate.get_artifact(ref.uri)
                    debate_context["shared_artifact"] = data.decode("utf-8")
                except Exception:
                    pass

        result = self.autogen_run_fn(debate_context)
        if isinstance(result, (dict, list)):
            return json.dumps(result)
        return str(result)
