package bus

import (
	"context"
	"errors"
	"fmt"
	"octpex/pkg/protocol"
	"sync"
)

var (
	ErrBusClosed       = errors.New("bus: message bus is closed")
	ErrNoRouteFound    = errors.New("bus: no worker registered for capability")
	ErrBufferOverflow  = errors.New("bus: channel buffer full")
	ErrMaxHopsExceeded = errors.New("bus: maximum lateral hop count exceeded")
)

// MessageHandler is invoked when an envelope arrives for a registered capability.
type MessageHandler func(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error)

// LocalNervousBus provides an in-memory dual-plane nervous bus for single-node kernel profiles.
type LocalNervousBus struct {
	mu           sync.RWMutex
	handlers     map[string]MessageHandler
	verticalChan chan *protocol.NervousEnvelope
	lateralChan  chan *protocol.NervousEnvelope
	closed       bool
}

// NewLocalNervousBus creates a buffered dual-plane in-memory bus.
func NewLocalNervousBus(bufferSize int) *LocalNervousBus {
	if bufferSize <= 0 {
		bufferSize = 256
	}
	return &LocalNervousBus{
		handlers:     make(map[string]MessageHandler),
		verticalChan: make(chan *protocol.NervousEnvelope, bufferSize),
		lateralChan:  make(chan *protocol.NervousEnvelope, bufferSize),
	}
}

// Subscribe registers an Arm's handler function for a specific capability on the bus.
func (b *LocalNervousBus) Subscribe(capability string, handler MessageHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[capability] = handler
}

// Publish routes an envelope through the appropriate nervous bus plane.
func (b *LocalNervousBus) Publish(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
	if err := env.Validate(); err != nil {
		return nil, fmt.Errorf("bus: invalid envelope: %w", err)
	}

	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return nil, ErrBusClosed
	}

	// Automatic hop count tracking and security token attenuation for lateral hops (AbloEx Zero-Trust)
	if env.Plane == protocol.PlaneLateral {
		env.HopCount++
		env.Security.Attenuate(env.TargetCapability)
	}

	handler, ok := b.handlers[env.TargetCapability]
	b.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: '%s'", ErrNoRouteFound, env.TargetCapability)
	}

	return handler(ctx, env)
}

// Close gracefully closes the message bus.
func (b *LocalNervousBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed {
		b.closed = true
		close(b.verticalChan)
		close(b.lateralChan)
	}
}
