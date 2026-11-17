package protocol

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// NervousPlane defines the communication plane of the nervous bus.
type NervousPlane string

const (
	PlaneVertical NervousPlane = "VERTICAL" // Brain <-> Arm
	PlaneLateral  NervousPlane = "LATERAL"  // Arm <-> Arm
)

// OrchestrationPattern specifies the execution pattern of a task.
type OrchestrationPattern string

const (
	PatternCoordinator OrchestrationPattern = "COORDINATOR" // Central dispatch fan-out / fan-in
	PatternSequential  OrchestrationPattern = "SEQUENTIAL"  // Pipeline handoff along tentacles
	PatternScatter     OrchestrationPattern = "SCATTER"     // Parallel broadcast to all matching arms
)

// MessageType indicates the nature of the message envelope.
type MessageType string

const (
	MsgTaskDispatch    MessageType = "TASK_DISPATCH"
	MsgHandoffRequest  MessageType = "HANDOFF_REQUEST"
	MsgStepComplete    MessageType = "STEP_COMPLETE"
	MsgEscalation      MessageType = "ESCALATION"
	MsgBackpressure    MessageType = "BACKPRESSURE"
)

// SubstrateRef points to an immutable or CAS-versioned artifact in the Shared Data Substrate.
type SubstrateRef struct {
	URI         string `json:"uri"`          // Scheme: sub://{session_id}/{artifact_id}
	Version     int64  `json:"version"`      // CAS version number
	ContentType string `json:"content_type"` // e.g. application/json, text/plain
	ByteSize    int64  `json:"byte_size"`
}

// Validate ensures the substrate reference conforms to specification.
func (r *SubstrateRef) Validate() error {
	if !strings.HasPrefix(r.URI, "sub://") {
		return fmt.Errorf("invalid substrate URI: must start with 'sub://', got: %s", r.URI)
	}
	parts := strings.Split(strings.TrimPrefix(r.URI, "sub://"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid substrate URI structure: expected sub://{session_id}/{artifact_id}, got: %s", r.URI)
	}
	if r.Version < 1 {
		return fmt.Errorf("invalid substrate version: must be >= 1, got: %d", r.Version)
	}
	return nil
}

// ParseSubstrateURI extracts the sessionID and artifactID from a sub:// URI.
func ParseSubstrateURI(uri string) (sessionID, artifactID string, err error) {
	if !strings.HasPrefix(uri, "sub://") {
		return "", "", errors.New("URI must start with sub://")
	}
	trimmed := strings.TrimPrefix(uri, "sub://")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("malformed sub:// URI: %s", uri)
	}
	return parts[0], strings.Join(parts[1:], "/"), nil
}

// SecurityContext defines the attenuated zero-trust execution bounds (AbloEx compatible).
type SecurityContext struct {
	PrincipalID         string   `json:"principal_id,omitempty"`
	SessionToken        string   `json:"session_token,omitempty"`
	AllowedCapabilities []string `json:"allowed_capabilities,omitempty"`
	AllowMutation       bool     `json:"allow_mutation"`
	MaxAllowedHops      int      `json:"max_allowed_hops"`
}

// DefaultSecurityContext creates a baseline scoped security token.
func DefaultSecurityContext(sessionID string) SecurityContext {
	return SecurityContext{
		PrincipalID:    "octo-agent",
		SessionToken:   "tok-" + sessionID,
		AllowMutation:  true,
		MaxAllowedHops: 3,
	}
}

// Attenuate monotonically constrains privileges across lateral hops (AbloEx zero-trust attenuation).
func (s *SecurityContext) Attenuate(targetCapability string) {
	s.MaxAllowedHops--
	if s.MaxAllowedHops <= 1 {
		// Privileged mutations are revoked after serial hops
		s.AllowMutation = false
	}
	if targetCapability != "" {
		s.AllowedCapabilities = []string{targetCapability}
	}
}

// NervousEnvelope is the canonical message unit carried over both planes of the Nervous Bus.
type NervousEnvelope struct {
	EnvelopeID           string               `json:"envelope_id"`
	TraceID              string               `json:"trace_id"`
	SessionID            string               `json:"session_id"`
	Plane                NervousPlane         `json:"plane"`
	OrchestrationPattern OrchestrationPattern `json:"orchestration_pattern"`
	MessageType          MessageType          `json:"message_type"`
	SourceNode           string               `json:"source_node"`
	TargetCapability     string               `json:"target_capability,omitempty"`
	TargetNode           string               `json:"target_node,omitempty"`
	HopCount             int                  `json:"hop_count"`
	SubstrateRefs        []SubstrateRef       `json:"substrate_refs,omitempty"`
	Parameters           map[string]any       `json:"parameters,omitempty"`
	Security             SecurityContext      `json:"security"`
	TimestampUnixNano    int64                `json:"timestamp_unix_nano"`
}

// NewEnvelope creates a properly initialized NervousEnvelope with timestamp.
func NewEnvelope(envelopeID, traceID, sessionID string, plane NervousPlane, msgType MessageType, sourceNode string) *NervousEnvelope {
	return &NervousEnvelope{
		EnvelopeID:        envelopeID,
		TraceID:           traceID,
		SessionID:         sessionID,
		Plane:             plane,
		MessageType:       msgType,
		SourceNode:        sourceNode,
		HopCount:          0,
		SubstrateRefs:     make([]SubstrateRef, 0),
		Parameters:        make(map[string]any),
		Security:          DefaultSecurityContext(sessionID),
		TimestampUnixNano: time.Now().UnixNano(),
	}
}

// Validate checks structural integrity and protocol invariants.
func (e *NervousEnvelope) Validate() error {
	if e.EnvelopeID == "" {
		return errors.New("envelope_id is required")
	}
	if e.TraceID == "" {
		return errors.New("trace_id is required")
	}
	if e.SessionID == "" {
		return errors.New("session_id is required")
	}
	if e.SourceNode == "" {
		return errors.New("source_node is required")
	}
	if e.HopCount < 0 {
		return errors.New("hop_count cannot be negative")
	}
	for i, ref := range e.SubstrateRefs {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("substrate_ref[%d] invalid: %w", i, err)
		}
	}
	return nil
}

// StepResult is returned by an Arm worker after completing an execution step.
type StepResult struct {
	Status        string         `json:"status"` // SUCCESS, RETRY, ESCALATED, FAILED
	OutputMessage string         `json:"output_message,omitempty"`
	SubstrateRefs []SubstrateRef `json:"substrate_refs,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}
