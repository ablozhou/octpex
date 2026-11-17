package adapter

import (
	"context"
	"fmt"
	"octpex/pkg/protocol"
	"octpex/pkg/substrate"
	"sync/atomic"
	"time"
)

// DeterministicHandlerFunc is a pure Go function executing deterministic business logic.
type DeterministicHandlerFunc func(ctx context.Context, env *protocol.NervousEnvelope, sub substrate.DataSubstrateSPI) (*protocol.StepResult, error)

// NativeFSMAdapter is a Level 0 Zero-Intelligence execution worker.
// It costs 0 Tokens, runs in < 5ms, and executes deterministic rule-based tasks or MCP tools.
type NativeFSMAdapter struct {
	manifest       *protocol.ArmManifest
	substrate      substrate.DataSubstrateSPI
	handlers       map[string]DeterministicHandlerFunc
	activeQueue    int64
	completedTasks int64
	escalatedTasks int64
}

// NewNativeFSMAdapter creates a Level 0 worker bound to the shared substrate.
func NewNativeFSMAdapter(sub substrate.DataSubstrateSPI) *NativeFSMAdapter {
	return &NativeFSMAdapter{
		substrate: sub,
		handlers:  make(map[string]DeterministicHandlerFunc),
	}
}

// RegisterHandler binds a deterministic capability tag to a Go handler.
func (a *NativeFSMAdapter) RegisterHandler(capabilityTag string, handler DeterministicHandlerFunc) {
	a.handlers[capabilityTag] = handler
}

func (a *NativeFSMAdapter) Init(ctx context.Context, manifest *protocol.ArmManifest) error {
	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("native_fsm: invalid manifest: %w", err)
	}
	a.manifest = manifest
	return nil
}

func (a *NativeFSMAdapter) ExecuteStep(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
	atomic.AddInt64(&a.activeQueue, 1)
	defer atomic.AddInt64(&a.activeQueue, -1)

	// Route based on TargetCapability
	handler, ok := a.handlers[env.TargetCapability]
	if !ok {
		atomic.AddInt64(&a.escalatedTasks, 1)
		return &protocol.StepResult{
			Status:        "ESCALATED",
			OutputMessage: fmt.Sprintf("native_fsm: unhandled capability '%s'", env.TargetCapability),
		}, nil
	}

	result, err := handler(ctx, env, a.substrate)
	if err != nil {
		atomic.AddInt64(&a.escalatedTasks, 1)
		return nil, err
	}

	atomic.AddInt64(&a.completedTasks, 1)
	return result, nil
}

func (a *NativeFSMAdapter) Telemetry(ctx context.Context) (*protocol.ArmTelemetry, error) {
	return &protocol.ArmTelemetry{
		ActiveQueueDepth:  int(atomic.LoadInt64(&a.activeQueue)),
		Status:            "HEALTHY",
		LastHeartbeatUnix: time.Now().Unix(),
		CompletedTasks:    atomic.LoadInt64(&a.completedTasks),
		EscalatedTasks:    atomic.LoadInt64(&a.escalatedTasks),
	}, nil
}
