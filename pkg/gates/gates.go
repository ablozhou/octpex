package gates

import (
	"context"
	"encoding/json"
	"fmt"
	"octpex/pkg/protocol"
	"time"
)

// PreFlightCheck evaluates Gate 1: Context capacity, policy constraints, and capability distance.
func PreFlightCheck(manifest *protocol.ArmManifest, env *protocol.NervousEnvelope, capabilityDistance float32, promptTokenEstimate int) (passed bool, reason string) {
	// 1. Context Overflow check
	if manifest.SLMProfile.SafeContextLimit > 0 && promptTokenEstimate > manifest.SLMProfile.SafeContextLimit {
		return false, fmt.Sprintf("gate 1: prompt token estimate (%d) exceeds arm safe context limit (%d)",
			promptTokenEstimate, manifest.SLMProfile.SafeContextLimit)
	}

	// 2. Out-Of-Distribution (OOD) capability distance check
	if manifest.EscalationGates.MinCapabilitySimilarity > 0 && capabilityDistance < manifest.EscalationGates.MinCapabilitySimilarity {
		return false, fmt.Sprintf("gate 1: capability similarity (%0.2f) below escalation threshold (%0.2f)",
			capabilityDistance, manifest.EscalationGates.MinCapabilitySimilarity)
	}

	// 3. Danger / mutating policy check (e.g. drop table, rm -rf)
	if dangerReason := checkDestructivePolicy(env.Parameters); dangerReason != "" {
		return false, fmt.Sprintf("gate 1: high risk mutation blocked: %s", dangerReason)
	}

	return true, ""
}

func checkDestructivePolicy(params map[string]any) string {
	if params == nil {
		return ""
	}
	if cmd, ok := params["command"].(string); ok {
		if cmd == "rm -rf /" || cmd == "drop database" {
			return "prohibited destructive system command"
		}
	}
	return ""
}

// VerifierCheck evaluates Gate 2: Deterministic syntax, schema, and exit codes.
type VerifierFunc func(ctx context.Context, output []byte) error

// JSONSchemaVerifier checks JSON syntax for arms without a field contract.
func JSONSchemaVerifier(ctx context.Context, output []byte) error {
	var js json.RawMessage
	if err := json.Unmarshal(output, &js); err != nil {
		return fmt.Errorf("gate 2 verifier: invalid json schema: %w", err)
	}
	return nil
}

// BehavioralCheck evaluates Gate 3: Mathematical token logprob and lateral hop count limit.
func BehavioralCheck(manifest *protocol.ArmManifest, env *protocol.NervousEnvelope, avgLogprob float32) (passed bool, reason string) {
	// 1. Hop limit check
	maxHops := manifest.EscalationGates.MaxLateralHops
	if maxHops <= 0 {
		maxHops = 3
	}
	if env.HopCount >= maxHops {
		return false, fmt.Sprintf("gate 3: lateral hop count (%d) reached maximum limit (%d)", env.HopCount, maxHops)
	}

	// 2. Token logprob confidence floor
	minLogprob := manifest.EscalationGates.MinTokenLogprob
	if minLogprob != 0 && avgLogprob < minLogprob {
		return false, fmt.Sprintf("gate 3: average token logprob (%0.3f) below threshold (%0.3f)", avgLogprob, minLogprob)
	}

	return true, ""
}

// BuildEscalation creates an EscalationPayload when any gate is tripped.
func BuildEscalation(env *protocol.NervousEnvelope, gate protocol.EscalationGateType, reason string, attempts int, critiques []string) *protocol.EscalationPayload {
	return &protocol.EscalationPayload{
		EscalationID:     fmt.Sprintf("esc-%d", time.Now().UnixNano()),
		OriginalEnvelope: *env,
		TriggeredGate:    gate,
		FailureReason:    reason,
		AttemptCount:     attempts,
		ReflexCritiques:  critiques,
		TimestampNano:    time.Now().UnixNano(),
	}
}

// BuildEscalationWithSystem1 creates an EscalationPayload with non-autoregressive synaptic metrics.
func BuildEscalationWithSystem1(env *protocol.NervousEnvelope, gate protocol.EscalationGateType, reason string, attempts int, critiques []string, sys1 *protocol.System1Metrics) *protocol.EscalationPayload {
	payload := BuildEscalation(env, gate, reason, attempts, critiques)
	payload.System1Metrics = sys1
	return payload
}

// VerifyObjectFields enforces the configured top-level JSON field types.
func VerifyObjectFields(output []byte, fields map[string]string) error {
	var obj map[string]any
	if err := json.Unmarshal(output, &obj); err != nil {
		return err
	}
	for key, want := range fields {
		value, ok := obj[key]
		if !ok {
			return fmt.Errorf("required output field %q missing", key)
		}
		var got string
		switch value.(type) {
		case string:
			got = "string"
		case float64:
			got = "number"
		case bool:
			got = "boolean"
		case []any:
			got = "array"
		case map[string]any:
			got = "object"
		case nil:
			got = "null"
		}
		if got != want {
			return fmt.Errorf("output field %q: expected %s, got %s", key, want, got)
		}
	}
	return nil
}
