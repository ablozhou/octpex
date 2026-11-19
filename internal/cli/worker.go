package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"octpex/pkg/adapter"
	"octpex/pkg/brain"
	"octpex/pkg/bus"
	"octpex/pkg/gates"
	"octpex/pkg/protocol"
	"octpex/pkg/substrate"
	"octpex/pkg/telemetry"
	"strings"
	"time"
)

func telemetryWorker(ctx context.Context, id string, sub substrate.DataSubstrateSPI, ttl time.Duration, costs *telemetry.FinOpsCollector) (*protocol.ArmManifest, bus.MessageHandler, error) {
	manifest := &protocol.ArmManifest{ArmID: id, RuntimeAdapter: "go-native-fsm", RoleName: "telemetry-distiller", CapabilityTags: []string{"distill-telemetry"}, Endpoint: "local://" + id, EscalationGates: protocol.DefaultEscalationGates()}
	manifest.Telemetry.Status = "HEALTHY"
	manifest.TouchHeartbeat()
	arm := adapter.NewNativeFSMAdapter(sub)
	if err := arm.Init(ctx, manifest); err != nil {
		return nil, nil, err
	}
	arbiter := brain.NewOctoBrain(nil)
	arm.RegisterHandler("distill-telemetry", func(ctx context.Context, env *protocol.NervousEnvelope, storage substrate.DataSubstrateSPI) (*protocol.StepResult, error) {
		started := time.Now()
		raw, _ := env.Parameters["log"].(string)
		if raw == "" {
			raw, _ = env.Parameters["prompt"].(string)
		}
		passed, reason := gates.PreFlightCheck(manifest, env, 1, 0)
		if strings.TrimSpace(raw) == "" && passed {
			passed = false
			reason = "expected a non-empty log or prompt"
		}
		if !passed {
			decision, err := arbiter.Arbitrate(ctx, gates.BuildEscalation(env, protocol.Gate1PreFlightPolicy, reason, 1, nil))
			if err != nil {
				return nil, err
			}
			costs.RecordTask(telemetry.TaskRecord{TraceID: env.TraceID, ArmID: id, Level: "BRAIN", Escalated: true, Duration: time.Since(started)})
			return &protocol.StepResult{Status: "ESCALATED", OutputMessage: reason, Metadata: map[string]any{"arbitration": decision, "trace_id": env.TraceID, "worker_id": id}}, nil
		}
		seen := make(map[string]bool)
		var unique []string
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !seen[line] {
				unique = append(unique, line)
				seen[line] = true
			}
		}
		data := []byte(strings.Join(unique, "\n"))
		uri := "sub://" + url.PathEscape(env.SessionID) + "/distilled/" + url.PathEscape(env.EnvelopeID)
		version, err := storage.PutArtifact(ctx, uri, 0, data, ttl)
		if errors.Is(err, substrate.ErrCASConflict) {
			existing, v, readErr := storage.GetArtifact(ctx, uri)
			if readErr != nil {
				return nil, readErr
			}
			if !bytes.Equal(existing, data) {
				return nil, fmt.Errorf("%w: envelope ID already used for a different result", substrate.ErrCASConflict)
			}
			version = v
		} else if err != nil {
			return nil, err
		} else {
			costs.RecordTask(telemetry.TaskRecord{TraceID: env.TraceID, ArmID: id, Level: "L0_DETERMINISTIC", Duration: time.Since(started)})
		}
		return &protocol.StepResult{Status: "SUCCESS", OutputMessage: string(data), SubstrateRefs: []protocol.SubstrateRef{{URI: uri, Version: version, ContentType: "text/plain", ByteSize: int64(len(data))}}, Metadata: map[string]any{"trace_id": env.TraceID, "worker_id": id, "unique_lines": len(unique)}}, nil
	})
	return manifest, arm.ExecuteStep, nil
}
