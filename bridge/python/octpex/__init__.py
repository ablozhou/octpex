"""
Octpex Python SDK - Heterogeneous Multi-Agent Framework Bridge
Connects Google ADK, LangGraph, AutoGen, and CrewAI as biomimetic Octo-Arm workers.
"""

from .protocol import (
    NervousEnvelope,
    SubstrateRef,
    SecurityContext,
    StepResult,
    DecisionEngineProfile,
    DecisionQuestion,
    DecisionResult,
    System1Metrics,
    HARDWARE_TIER_0_CPU_EDGE,
    HARDWARE_TIER_1_LEGACY_8GB,
    HARDWARE_TIER_2_MID_16GB,
    HARDWARE_TIER_3_ENTERPRISE_GPU,
)
from .substrate import SubstrateClient, InMemorySubstrateClient
from .arm import OctoArmWorker, ArmConfig
from .worker import OctoArmDaemon

__version__ = "0.5.0"

__all__ = [
    "NervousEnvelope",
    "SubstrateRef",
    "SecurityContext",
    "StepResult",
    "DecisionEngineProfile",
    "DecisionQuestion",
    "DecisionResult",
    "System1Metrics",
    "HARDWARE_TIER_0_CPU_EDGE",
    "HARDWARE_TIER_1_LEGACY_8GB",
    "HARDWARE_TIER_2_MID_16GB",
    "HARDWARE_TIER_3_ENTERPRISE_GPU",
    "SubstrateClient",
    "InMemorySubstrateClient",
    "OctoArmWorker",
    "ArmConfig",
    "OctoArmDaemon",
]
