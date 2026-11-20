package protocol

import (
	"errors"
	"strings"
	"time"
)

// SLMProfile describes the local small language model hosted in an Arm's ganglion.
type SLMProfile struct {
	Provider         string  `json:"provider"`           // e.g. "ollama", "llamacpp", "vllm", "mock", "none"
	Model            string  `json:"model"`              // e.g. "qwen2.5:3b", "none"
	SafeContextLimit int     `json:"safe_context_limit"` // Max token capacity
	Temperature      float32 `json:"temperature"`
}

// Hardware tier classifications for adaptive VRAM allocation and OOM prevention.
const (
	HardwareTier0CPUEdge       = "TIER_0_CPU_EDGE"
	HardwareTier1Legacy8GB     = "TIER_1_LEGACY_8GB"
	HardwareTier2Mid16GB       = "TIER_2_MID_16GB"
	HardwareTier3EnterpriseGPU = "TIER_3_ENTERPRISE_GPU"
)

// DecisionEngineProfile configures the System-1 non-autoregressive synaptic decision engine.
type DecisionEngineProfile struct {
	PrimaryEngine       string `json:"primary_engine"`        // e.g. "laya-onnx-int8", "karpo-decision-2b-4bit", "mock"
	SecondaryEngine     string `json:"secondary_engine"`      // e.g. "karpo-decision-2b-4bit", "none"
	Endpoint            string `json:"endpoint"`              // e.g. "http://127.0.0.1:8091/v1/systemone"
	SafeDecisionContext int    `json:"safe_decision_context"` // Max token capacity (e.g. 32768)
	VRAMBudgetMB        int    `json:"vram_budget_mb"`        // Allocated VRAM budget in MB (0 if pinned to CPU RAM)
}

// EscalationGates defines thresholds that trigger escalation to Octo-Brain.
type EscalationGates struct {
	MinCapabilitySimilarity float32 `json:"min_capability_similarity"` // Gate 1: Embedding distance < threshold triggers escalation
	MinTokenLogprob         float32 `json:"min_token_logprob"`         // Gate 3: Average logprob floor
	MaxReflexRetries        int     `json:"max_reflex_retries"`        // Gate 2: Local repair attempts before exhausting
	MaxLateralHops          int     `json:"max_lateral_hops"`          // Gate 3: Hop limit before escalation
	MinSystem1Confidence    float32 `json:"min_system1_confidence"`    // System-1 high confidence threshold for direct resolve (default 0.75)
	UncertaintyBandLow      float32 `json:"uncertainty_band_low"`      // FlexDM reverse calibration lower threshold (default 0.50)
}

// DefaultEscalationGates returns battle-tested default thresholds.
func DefaultEscalationGates() EscalationGates {
	return EscalationGates{
		MinCapabilitySimilarity: 0.65,
		MinTokenLogprob:         -0.51,
		MaxReflexRetries:        3,
		MaxLateralHops:          3,
		MinSystem1Confidence:    0.75,
		UncertaintyBandLow:      0.50,
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

// SkillManifest represents a dynamic model-driven skill declared by an arm and registered to Octo-Brain.
// Supports hierarchical taxonomy DAG categorization (multi-category assignment) and DAG dependencies.
type SkillManifest struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Category       string         `json:"category,omitempty"`        // Primary category path, e.g. "geospatial/disambiguation"
	Categories     []string       `json:"categories,omitempty"`      // Multi-parent DAG classification paths
	ParentCategory string         `json:"parent_category,omitempty"` // Primary parent category, e.g. "geospatial"
	Dependencies   []string       `json:"dependencies,omitempty"`    // Prerequisite upstream skills forming execution DAG
	Description    string         `json:"description"`
	SystemPrompt   string         `json:"system_prompt,omitempty"`
	Version        string         `json:"version,omitempty"`
	Parameters     map[string]any `json:"parameters,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// SkillTreeNode represents a node in the hierarchical Skill Tree / DAG projection of the central Octo-Brain.
type SkillTreeNode struct {
	Path     string           `json:"path"`
	Name     string           `json:"name"`
	IsLeaf   bool             `json:"is_leaf"`
	Skill    *SkillManifest   `json:"skill,omitempty"`
	ArmID    string           `json:"arm_id,omitempty"`
	Children []*SkillTreeNode `json:"children,omitempty"`
}

// BuildSkillTree aggregates registered arms and organizes their skills into a hierarchical taxonomy DAG projection.
// A skill with multiple categories appears in each corresponding category branch (polyhierarchy DAG).
func BuildSkillTree(arms []*ArmManifest) *SkillTreeNode {
	root := &SkillTreeNode{
		Path:     "root",
		Name:     "Global Skill Catalog",
		IsLeaf:   false,
		Children: make([]*SkillTreeNode, 0),
	}

	for _, arm := range arms {
		for i := range arm.Skills {
			skill := &arm.Skills[i]

			// Collect all categories (primary category + multi-parent categories)
			catSet := make(map[string]bool)
			if skill.Category != "" {
				catSet[skill.Category] = true
			}
			for _, c := range skill.Categories {
				if c != "" {
					catSet[c] = true
				}
			}
			if len(catSet) == 0 {
				if skill.ParentCategory != "" {
					catSet[skill.ParentCategory] = true
				} else {
					catSet["general"] = true
				}
			}

			// Insert this skill into every category branch it belongs to
			for cat := range catSet {
				segments := strings.Split(strings.Trim(cat, "/"), "/")
				curr := root
				currentPath := ""

				for _, seg := range segments {
					if seg == "" {
						continue
					}
					if currentPath == "" {
						currentPath = seg
					} else {
						currentPath = currentPath + "/" + seg
					}

					var nextChild *SkillTreeNode
					for _, ch := range curr.Children {
						if ch.Name == seg && !ch.IsLeaf {
							nextChild = ch
							break
						}
					}
					if nextChild == nil {
						nextChild = &SkillTreeNode{
							Path:     currentPath,
							Name:     seg,
							IsLeaf:   false,
							Children: make([]*SkillTreeNode, 0),
						}
						curr.Children = append(curr.Children, nextChild)
					}
					curr = nextChild
				}

				// Add leaf node for this skill under its category
				leafPath := currentPath + "/" + skill.ID
				curr.Children = append(curr.Children, &SkillTreeNode{
					Path:   leafPath,
					Name:   skill.Name,
					IsLeaf: true,
					Skill:  skill,
					ArmID:  arm.ArmID,
				})
			}
		}
	}

	return root
}

// SkillGroupMember defines a member skill within a group and its DAG dependencies.
type SkillGroupMember struct {
	SkillID   string   `json:"skill_id"`
	FileRef   string   `json:"file_ref,omitempty"`
	Role      string   `json:"role,omitempty"`
	DependsOn []string `json:"depends_on,omitempty"` // Upstream skill prerequisites within group
	Order     int      `json:"order,omitempty"`
}

// SkillGroupManifest represents a metafile-defined bundle of related skills forming a DAG suite.
type SkillGroupManifest struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Category    string             `json:"category,omitempty"`
	Description string             `json:"description"`
	Version     string             `json:"version,omitempty"`
	Skills      []SkillGroupMember `json:"skills"`
	Metadata    map[string]any     `json:"metadata,omitempty"`
}

// ArmManifest is the registration card published by an Octo-Arm to the Basal Nerve Ring registry.
type ArmManifest struct {
	RequiredOutputFields  map[string]string     `json:"required_output_fields,omitempty"`
	ArmID                 string                `json:"arm_id"`
	RegionID              string                `json:"region_id,omitempty"` // Multi-cluster region identifier (e.g. "us-east-1", "edge-a")
	RuntimeAdapter        string                `json:"runtime_adapter"`     // e.g. "go-native-fsm", "adk", "langgraph", "autogen", "ollama"
	RoleName              string                `json:"role_name"`
	CapabilityTags        []string              `json:"capability_tags"`
	Skills                []SkillManifest       `json:"skills,omitempty"`
	SkillGroups           []SkillGroupManifest  `json:"skill_groups,omitempty"`
	ToolSchema            map[string]any        `json:"tool_schema,omitempty"`
	RoleEmbedding         []float32             `json:"role_embedding,omitempty"`
	Endpoint              string                `json:"endpoint"`
	HardwareTier          string                `json:"hardware_tier,omitempty"`
	DecisionEngineProfile DecisionEngineProfile `json:"decision_engine_profile,omitempty"`
	SLMProfile            SLMProfile            `json:"slm_profile"`
	EscalationGates       EscalationGates       `json:"escalation_gates"`
	Telemetry             ArmTelemetry          `json:"telemetry"`
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
