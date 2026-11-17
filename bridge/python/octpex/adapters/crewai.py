import json
from typing import Any, Optional, Dict, Callable
from ..arm import OctoArmWorker, ArmConfig
from ..protocol import NervousEnvelope
from ..substrate import SubstrateClient


class CrewAIArmAdapter(OctoArmWorker):
    """
    Wraps a CrewAI Agent or Crew kickoff as an Octpex Octo-Arm worker pod.
    """

    def __init__(
        self,
        config: ArmConfig,
        crew_kickoff_fn: Callable[[Dict[str, Any]], Any],
        substrate: Optional[SubstrateClient] = None,
    ):
        super().__init__(config, substrate)
        self.crew_kickoff_fn = crew_kickoff_fn

    def act(self, envelope: NervousEnvelope, critique: Optional[str] = None) -> str:
        inputs = dict(envelope.parameters)
        if critique:
            inputs["critique_feedback"] = critique

        # Inject substrate data
        if self.substrate and envelope.substrate_refs:
            for ref in envelope.substrate_refs:
                try:
                    data, _ = self.substrate.get_artifact(ref.uri)
                    inputs[f"substrate_data"] = data.decode("utf-8")
                except Exception:
                    pass

        result = self.crew_kickoff_fn(inputs)
        return str(result)
