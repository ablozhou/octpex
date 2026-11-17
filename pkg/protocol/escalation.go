package protocol

// EscalationGateType identifies which objective gate triggered an escalation.
type EscalationGateType string

const (
	Gate1PreFlightPolicy EscalationGateType = "GATE_1_PREFLIGHT_POLICY" // OOD capability, dangerous action, context overflow
	Gate2VerifierFailed  EscalationGateType = "GATE_2_VERIFIER_FAILED"   // Schema invalid, AST parse error, retry budget exhausted
	Gate3BehaviorAnomaly EscalationGateType = "GATE_3_BEHAVIOR_ANOMALY"  // Low token logprob, infinite loop stagnation, hop limit exceeded
)

// EscalationPayload encapsulates the context, diagnosis, and evidence bundle
// packaged by an Octo-Arm when a task exceeds its local reflex capability.
type EscalationPayload struct {
	EscalationID   string             `json:"escalation_id"`
	OriginalEnvelope NervousEnvelope  `json:"original_envelope"`
	TriggeredGate  EscalationGateType `json:"triggered_gate"`
	FailureReason  string             `json:"failure_reason"`
	AttemptCount   int                `json:"attempt_count"`
	ReflexCritiques []string          `json:"reflex_critiques,omitempty"`
	ContextDiffURI string             `json:"context_diff_uri,omitempty"` // sub:// reference to raw failure trace
	TimestampNano  int64              `json:"timestamp_nano"`
}
