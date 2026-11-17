package adapter

import (
	"context"
	"octpex/pkg/protocol"
)

// ArmRuntimeAdapterSPI defines the pluggable lifecycle and step execution contract
// implemented by all Arm worker runtimes (Level 0 Native, Level 1 SLM, Level 2 ADK/LangGraph, Level 3 AutoGen/CrewAI).
type ArmRuntimeAdapterSPI interface {
	// Init initializes the worker with its capability manifest.
	Init(ctx context.Context, manifest *protocol.ArmManifest) error

	// ExecuteStep processes an incoming NervousEnvelope and returns a StepResult.
	ExecuteStep(ctx context.Context, envelope *protocol.NervousEnvelope) (*protocol.StepResult, error)

	// Telemetry reports active queue depth and runtime health status.
	Telemetry(ctx context.Context) (*protocol.ArmTelemetry, error)
}
