package adapter

import (
	"octpex/pkg/protocol"
)

// DecisionQuestion defines a single question evaluated by the non-autoregressive System-1 engine.
type DecisionQuestion = protocol.DecisionQuestion

// DecisionResult represents the single-pass inference readout for a question.
type DecisionResult = protocol.DecisionResult

// DecisionEngineSPI defines the non-autoregressive System-1 Synaptic Engine contract.
type DecisionEngineSPI = protocol.DecisionEngineSPI
