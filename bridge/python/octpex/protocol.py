import time
from dataclasses import dataclass, field
from typing import Dict, Any, List, Optional

HARDWARE_TIER_0_CPU_EDGE = "TIER_0_CPU_EDGE"
HARDWARE_TIER_1_LEGACY_8GB = "TIER_1_LEGACY_8GB"
HARDWARE_TIER_2_MID_16GB = "TIER_2_MID_16GB"
HARDWARE_TIER_3_ENTERPRISE_GPU = "TIER_3_ENTERPRISE_GPU"


@dataclass
class DecisionEngineProfile:
    primary_engine: str = "laya-onnx-int8"
    secondary_engine: str = "karpo-decision-2b-4bit"
    endpoint: str = "http://127.0.0.1:8091/v1/systemone"
    safe_decision_context: int = 32768
    vram_budget_mb: int = 0

    def to_dict(self) -> Dict[str, Any]:
        return {
            "primary_engine": self.primary_engine,
            "secondary_engine": self.secondary_engine,
            "endpoint": self.endpoint,
            "safe_decision_context": self.safe_decision_context,
            "vram_budget_mb": self.vram_budget_mb,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "DecisionEngineProfile":
        return cls(
            primary_engine=data.get("primary_engine", "laya-onnx-int8"),
            secondary_engine=data.get("secondary_engine", "karpo-decision-2b-4bit"),
            endpoint=data.get("endpoint", "http://127.0.0.1:8091/v1/systemone"),
            safe_decision_context=data.get("safe_decision_context", 32768),
            vram_budget_mb=data.get("vram_budget_mb", 0),
        )


@dataclass
class DecisionQuestion:
    qid: str
    qtype: str  # "choice" | "noul" | "score"
    prompt: str
    options: List[str] = field(default_factory=list)

    def to_dict(self) -> Dict[str, Any]:
        return {
            "qid": self.qid,
            "qtype": self.qtype,
            "prompt": self.prompt,
            "options": self.options,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "DecisionQuestion":
        return cls(
            qid=data["qid"],
            qtype=data["qtype"],
            prompt=data["prompt"],
            options=data.get("options", []),
        )


@dataclass
class DecisionResult:
    qid: str
    probabilities: Dict[str, float]
    top_option: str
    confidence: float
    engine_used: str = "laya-onnx-int8"
    latency_ms: int = 0
    score_value: float = 0.0

    def to_dict(self) -> Dict[str, Any]:
        return {
            "qid": self.qid,
            "probabilities": self.probabilities,
            "top_option": self.top_option,
            "confidence": self.confidence,
            "engine_used": self.engine_used,
            "latency_ms": self.latency_ms,
            "score_value": self.score_value,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "DecisionResult":
        return cls(
            qid=data["qid"],
            probabilities=data.get("probabilities", {}),
            top_option=data.get("top_option", ""),
            confidence=data.get("confidence", 0.0),
            engine_used=data.get("engine_used", "laya-onnx-int8"),
            latency_ms=data.get("latency_ms", 0),
            score_value=data.get("score_value", 0.0),
        )


@dataclass
class System1Metrics:
    system1_engine: str = ""
    system1_confidence: float = 0.0
    reverse_option_consistent: bool = True
    decision_latency_ms: int = 0

    def to_dict(self) -> Dict[str, Any]:
        return {
            "system1_engine": self.system1_engine,
            "system1_confidence": self.system1_confidence,
            "reverse_option_consistent": self.reverse_option_consistent,
            "decision_latency_ms": self.decision_latency_ms,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "System1Metrics":
        return cls(
            system1_engine=data.get("system1_engine", ""),
            system1_confidence=data.get("system1_confidence", 0.0),
            reverse_option_consistent=data.get("reverse_option_consistent", True),
            decision_latency_ms=data.get("decision_latency_ms", 0),
        )


@dataclass
class SubstrateRef:
    uri: str
    version: int
    content_type: str = "application/json"
    byte_size: int = 0

    def to_dict(self) -> Dict[str, Any]:
        return {
            "uri": self.uri,
            "version": self.version,
            "content_type": self.content_type,
            "byte_size": self.byte_size,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "SubstrateRef":
        return cls(
            uri=data["uri"],
            version=data["version"],
            content_type=data.get("content_type", "application/json"),
            byte_size=data.get("byte_size", 0),
        )


@dataclass
class SecurityContext:
    principal_id: str = "octo-agent"
    session_token: str = ""
    allowed_capabilities: List[str] = field(default_factory=list)
    allow_mutation: bool = True
    max_allowed_hops: int = 3

    def attenuate(self, target_capability: str = "") -> None:
        self.max_allowed_hops -= 1
        if self.max_allowed_hops <= 1:
            self.allow_mutation = False
        if target_capability:
            self.allowed_capabilities = [target_capability]

    def to_dict(self) -> Dict[str, Any]:
        return {
            "principal_id": self.principal_id,
            "session_token": self.session_token,
            "allowed_capabilities": self.allowed_capabilities,
            "allow_mutation": self.allow_mutation,
            "max_allowed_hops": self.max_allowed_hops,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "SecurityContext":
        return cls(
            principal_id=data.get("principal_id", "octo-agent"),
            session_token=data.get("session_token", ""),
            allowed_capabilities=data.get("allowed_capabilities", []),
            allow_mutation=data.get("allow_mutation", True),
            max_allowed_hops=data.get("max_allowed_hops", 3),
        )


@dataclass
class NervousEnvelope:
    envelope_id: str
    trace_id: str
    session_id: str
    plane: str  # "VERTICAL" or "LATERAL"
    message_type: str  # "TASK_DISPATCH", "HANDOFF_REQUEST", "STEP_COMPLETE", "ESCALATION"
    source_node: str
    target_capability: str = ""
    target_node: str = ""
    hop_count: int = 0
    substrate_refs: List[SubstrateRef] = field(default_factory=list)
    parameters: Dict[str, Any] = field(default_factory=dict)
    security: SecurityContext = field(default_factory=SecurityContext)
    timestamp_unix_nano: int = field(default_factory=lambda: int(time.time() * 1e9))

    def create_lateral_handoff(
        self,
        new_envelope_id: str,
        source_arm: str,
        target_capability: str,
        new_refs: Optional[List[SubstrateRef]] = None,
        extra_parameters: Optional[Dict[str, Any]] = None,
    ) -> "NervousEnvelope":
        """
        Creates an attenuated lateral handoff envelope from this envelope.
        Increments hop_count, switches plane to LATERAL, and attenuates security.
        """
        attenuated_sec = SecurityContext(
            principal_id=self.security.principal_id,
            session_token=self.security.session_token,
            allowed_capabilities=list(self.security.allowed_capabilities),
            allow_mutation=self.security.allow_mutation,
            max_allowed_hops=self.security.max_allowed_hops,
        )
        attenuated_sec.attenuate(target_capability)

        params = dict(self.parameters)
        if extra_parameters:
            params.update(extra_parameters)

        refs = new_refs if new_refs is not None else list(self.substrate_refs)

        return NervousEnvelope(
            envelope_id=new_envelope_id,
            trace_id=self.trace_id,
            session_id=self.session_id,
            plane="LATERAL",
            message_type="HANDOFF_REQUEST",
            source_node=source_arm,
            target_capability=target_capability,
            hop_count=self.hop_count + 1,
            substrate_refs=refs,
            parameters=params,
            security=attenuated_sec,
            timestamp_unix_nano=int(time.time() * 1e9),
        )

    def to_dict(self) -> Dict[str, Any]:
        return {
            "envelope_id": self.envelope_id,
            "trace_id": self.trace_id,
            "session_id": self.session_id,
            "plane": self.plane,
            "message_type": self.message_type,
            "source_node": self.source_node,
            "target_capability": self.target_capability,
            "target_node": self.target_node,
            "hop_count": self.hop_count,
            "substrate_refs": [ref.to_dict() for ref in self.substrate_refs],
            "parameters": self.parameters,
            "security": self.security.to_dict(),
            "timestamp_unix_nano": self.timestamp_unix_nano,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "NervousEnvelope":
        return cls(
            envelope_id=data["envelope_id"],
            trace_id=data["trace_id"],
            session_id=data["session_id"],
            plane=data.get("plane", "VERTICAL"),
            message_type=data.get("message_type", "TASK_DISPATCH"),
            source_node=data.get("source_node", "octo-brain"),
            target_capability=data.get("target_capability", ""),
            target_node=data.get("target_node", ""),
            hop_count=data.get("hop_count", 0),
            substrate_refs=[SubstrateRef.from_dict(r) for r in data.get("substrate_refs", [])],
            parameters=data.get("parameters", {}),
            security=SecurityContext.from_dict(data.get("security", {})),
            timestamp_unix_nano=data.get("timestamp_unix_nano", int(time.time() * 1e9)),
        )


@dataclass
class StepResult:
    status: str  # "SUCCESS", "RETRY", "ESCALATED", "FAILED"
    output_message: str = ""
    substrate_refs: List[SubstrateRef] = field(default_factory=list)
    metadata: Dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> Dict[str, Any]:
        return {
            "status": self.status,
            "output_message": self.output_message,
            "substrate_refs": [ref.to_dict() for ref in self.substrate_refs],
            "metadata": self.metadata,
        }
