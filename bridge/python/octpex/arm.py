import logging
from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import Dict, Any, List, Optional, Tuple, Callable, Union

from .protocol import (
    NervousEnvelope,
    StepResult,
    SubstrateRef,
    HARDWARE_TIER_0_CPU_EDGE,
    DecisionEngineProfile,
)
from .substrate import SubstrateClient
from .skill import Skill, SkillRegistry, parse_markdown_skill

logger = logging.getLogger("octpex.arm")


@dataclass
class ArmConfig:
    arm_id: str
    capability: str
    max_reflex_retries: int = 3
    timeout_seconds: float = 30.0
    hardware_tier: str = HARDWARE_TIER_0_CPU_EDGE
    decision_profile: Optional[DecisionEngineProfile] = None
    slm_profile: Optional[Dict[str, Any]] = None


class OctoArmWorker(ABC):
    """
    Base class for Python-based Octo-Arm workers.
    Wraps existing frameworks (Google ADK, LangGraph, CrewAI, AutoGen)
    with the local biomimetic reflex loop and Gate 2 schema validation.
    Also provides a generic Markdown-driven skill registry and SLM executor.
    """

    def __init__(self, config: ArmConfig, substrate: Optional[SubstrateClient] = None):
        self.config = config
        self.substrate = substrate
        self._gate2_verifier: Optional[Callable[[str], Optional[str]]] = None
        self.skills = SkillRegistry()

    def load_skill_file(self, file_path: str, skill_id: Optional[str] = None) -> Skill:
        """
        Loads and registers a skill defined in a Markdown file.
        """
        return self.skills.load_skill_file(file_path, skill_id=skill_id)

    def load_skills_from_directory(self, dir_path: str) -> List[Skill]:
        """
        Scans a directory for all .md files and registers them as skills.
        """
        return self.skills.load_skills_from_directory(dir_path)

    def load_skills_from_manifest(self, declarations: List[Dict[str, Any]]) -> List[Skill]:
        """
        Loads skills declared in arm.yaml manifest declarations.
        """
        return self.skills.load_from_manifest_declarations(declarations)

    def execute_skill(
        self,
        skill_id: str,
        input_payload: Union[str, Dict[str, Any]],
        slm_profile: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        """
        Executes a registered skill using the arm's configured SLM profile.
        """
        profile = slm_profile or self.config.slm_profile or {}
        return self.skills.execute_skill(skill_id, input_payload, slm_profile=profile)

    def set_gate2_verifier(self, verifier: Callable[[str], Optional[str]]) -> None:
        """
        Registers an objective verifier function.
        Returns None if output passes, or an error string with critique on failure.
        """
        self._gate2_verifier = verifier

    @abstractmethod
    def act(self, envelope: NervousEnvelope, critique: Optional[str] = None) -> str:
        """
        Core task execution method implemented by the framework adapter.
        If critique is provided, the worker must adjust its output accordingly.
        """
        pass

    def execute_envelope(self, envelope: NervousEnvelope) -> StepResult:
        """
        Executes the envelope within the local reflex loop:
        Act -> Inspect (Gate 2) -> Retry with critique -> Escalate on exhaustion.
        """
        retries = 0
        critique: Optional[str] = None

        while retries <= self.config.max_reflex_retries:
            try:
                # 1. Local execution (Act)
                output = self.act(envelope, critique=critique)

                # 2. Objective Gate 2 inspection
                if self._gate2_verifier:
                    err = self._gate2_verifier(output)
                    if err:
                        retries += 1
                        critique = f"Gate 2 validation failed: {err}"
                        logger.warning(
                            f"Arm {self.config.arm_id} Gate 2 failure (attempt {retries}/{self.config.max_reflex_retries}): {err}"
                        )
                        continue

                # 3. Passed inspection
                return StepResult(
                    status="SUCCESS",
                    output_message=output,
                    metadata={"retries": retries, "arm_id": self.config.arm_id},
                )

            except Exception as e:
                retries += 1
                critique = f"Runtime execution exception: {str(e)}"
                logger.error(f"Arm {self.config.arm_id} runtime error: {e}")

        # Retries exhausted -> Escalate to Octo-Brain
        return StepResult(
            status="ESCALATED",
            output_message=f"Arm {self.config.arm_id} exhausted {self.config.max_reflex_retries} retries. Final critique: {critique}",
            metadata={"retries": retries, "arm_id": self.config.arm_id, "escalated": True},
        )
