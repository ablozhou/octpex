package protocol

import (
	"errors"
	"time"
)

// SLMProfile describes the local small language model hosted in an Arm's ganglion.
type SLMProfile struct {
	Provider         string  `json:"provider"`                    // e.g. "ollama", "llamacpp", "vllm", "mock", "none"
	Model            string  `json:"model"`                       // e.g. "qwen2.5:3b", "none"
	SafeContextLimit int     `json:"safe_context_limit"`          // Max token capacity
	Temperature      float32 `json:"temperature"`
}

// EscalationGates defines thresholds that trigger escalation to Octo-Brain.
type EscalationGates struct {
	MinCapabilitySimilarity float32 `json:"min_capability_similarity"` // Gate 1: Embedding distance < threshold triggers escalation
	MinTokenLogprob         float32 `json:"min_token_logprob"`         // Gate 3: Average logprob floor
	MaxReflexRetries        int     `json:"max_reflex_retries"`        // Gate 2: Local repair attempts before exhausting
	MaxLateralHops          int     `json:"max_lateral_hops"`          // Gate 3: Hop limit before escalation
}

// DefaultEscalationGates returns battle-tested default thresholds.
func DefaultEscalationGates() EscalationGates {
	return EscalationGates{
		MinCapabilitySimilarity: 0.65,
		MinTokenLogprob:         -0.51,
		MaxReflexRetries:        3,
		MaxLateralHops:          3,
	}
}

// ArmTelemetry contains live operational metrics for routing arbitration.
type ArmTelemetry struct {
	ActiveQueueDepth  int    `json:"active_queue_depth"`
	Status            string `json:"status"` // HEALTHY, DEGRADED, DRAINING, OFFLINE
	LastHeartbeatUnix int64  `json:"last_heartbeat_unix"`
	CompletedTasks    int64  `json:"completed_tasks"`
	EscalatedTasks    int64  `json:"escalated_tasks"`
}

// ArmManifest is the registration card published by an Octo-Arm to the Basal Nerve Ring registry.
type ArmManifest struct {
	ArmID           string          `json:"arm_id"`
	RegionID        string          `json:"region_id,omitempty"` // Multi-cluster region identifier (e.g. "us-east-1", "edge-a")
	RuntimeAdapter  string          `json:"runtime_adapter"`     // e.g. "go-native-fsm", "adk", "langgraph", "autogen", "ollama"
	RoleName        string          `json:"role_name"`
	CapabilityTags  []string        `json:"capability_tags"`
	RoleEmbedding   []float32       `json:"role_embedding,omitempty"`
	Endpoint        string          `json:"endpoint"`
	SLMProfile      SLMProfile      `json:"slm_profile"`
	EscalationGates EscalationGates `json:"escalation_gates"`
	Telemetry       ArmTelemetry    `json:"telemetry"`
}

// Validate checks manifest completeness.
func (m *ArmManifest) Validate() error {
	if m.ArmID == "" {
		return errors.New("arm_id is required")
	}
	if m.RuntimeAdapter == "" {
		return errors.New("runtime_adapter is required")
	}
	if m.RoleName == "" {
		return errors.New("role_name is required")
	}
	if len(m.CapabilityTags) == 0 {
		return errors.New("capability_tags must contain at least one tag")
	}
	if m.Endpoint == "" {
		return errors.New("endpoint is required")
	}
	return nil
}

// HasCapability checks whether this arm supports a specific capability tag.
func (m *ArmManifest) HasCapability(tag string) bool {
	for _, t := range m.CapabilityTags {
		if t == tag {
			return true
		}
	}
	return false
}

// TouchHeartbeat updates the live heartbeat timestamp.
func (m *ArmManifest) TouchHeartbeat() {
	m.Telemetry.LastHeartbeatUnix = time.Now().Unix()
}
