import time
from dataclasses import dataclass, field
from typing import Dict, Any, List, Optional


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
