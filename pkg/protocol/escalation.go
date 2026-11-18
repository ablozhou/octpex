package protocol

// EscalationGateType identifies which objective gate triggered an escalation.
type EscalationGateType string

const (
	Gate1PreFlightPolicy EscalationGateType = "GATE_1_PREFLIGHT_POLICY" // OOD capability, dangerous action, context overflow
	Gate2VerifierFailed  EscalationGateType = "GATE_2_VERIFIER_FAILED"   // Schema invalid, AST parse error, retry budget exhausted
	Gate3BehaviorAnomaly EscalationGateType = "GATE_3_BEHAVIOR_ANOMALY"  // Low token logprob, infinite loop stagnation, hop limit exceeded
)

// System1Metrics tracks non-autoregressive decision performance and calibration consistency.
type System1Metrics struct {
	System1Engine            string  `json:"system1_engine,omitempty"`
	System1Confidence        float64 `json:"system1_confidence,omitempty"`
	ReverseOptionConsistent bool    `json:"reverse_option_consistent"`
	DecisionLatencyMs        int64   `json:"decision_latency_ms,omitempty"`
}

// EscalationPayload encapsulates the context, diagnosis, and evidence bundle
// packaged by an Octo-Arm when a task exceeds its local reflex capability.
type EscalationPayload struct {
	EscalationID     string             `json:"escalation_id"`
	OriginalEnvelope NervousEnvelope    `json:"original_envelope"`
	TriggeredGate    EscalationGateType `json:"triggered_gate"`
	FailureReason    string             `json:"failure_reason"`
	AttemptCount     int                `json:"attempt_count"`
	ReflexCritiques  []string           `json:"reflex_critiques,omitempty"`
	System1Metrics   *System1Metrics    `json:"system1_metrics,omitempty"`
	ContextDiffURI   string             `json:"context_diff_uri,omitempty"` // sub:// reference to raw failure trace
	TimestampNano    int64              `json:"timestamp_nano"`
}
