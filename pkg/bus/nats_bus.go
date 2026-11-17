package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"octpex/pkg/protocol"
	"sync"
)

// NATSClient abstracts raw NATS JetStream publish and subscribe operations.
type NATSClient interface {
	PublishMsg(ctx context.Context, subject string, data []byte) ([]byte, error)
	SubscribeQueue(subject, queueGroup string, handler func(ctx context.Context, data []byte) ([]byte, error)) error
	Close() error
}

// NATSNervousBus implements the distributed dual-plane nervous bus backed by NATS JetStream.
type NATSNervousBus struct {
	client       NATSClient
	queueGroup   string
	mu           sync.RWMutex
	handlers     map[string]MessageHandler
	verticalSubj string
	lateralSubj  string
}

// NewNATSNervousBus creates a NATS JetStream-backed distributed nervous bus.
func NewNATSNervousBus(client NATSClient, queueGroup string) *NATSNervousBus {
	if queueGroup == "" {
		queueGroup = "arm-workers"
	}
	return &NATSNervousBus{
		client:       client,
		queueGroup:   queueGroup,
		handlers:     make(map[string]MessageHandler),
		verticalSubj: "octpex.vertical.dispatch",
		lateralSubj:  "octpex.lateral.handoff",
	}
}

// Subscribe registers an Arm capability handler with the NATS queue group.
func (n *NATSNervousBus) Subscribe(capability string, handler MessageHandler) error {
	n.mu.Lock()
	n.handlers[capability] = handler
	n.mu.Unlock()

	// Subject: octpex.vertical.dispatch.<capability>
	subject := fmt.Sprintf("%s.%s", n.verticalSubj, capability)
	err := n.client.SubscribeQueue(subject, n.queueGroup, func(ctx context.Context, data []byte) ([]byte, error) {
		var env protocol.NervousEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			return nil, fmt.Errorf("failed to unmarshal nervous envelope: %w", err)
		}

		res, err := handler(ctx, &env)
		if err != nil {
			return nil, err
		}
		return json.Marshal(res)
	})
	if err != nil {
		return fmt.Errorf("nats: failed to subscribe to vertical subject %s: %w", subject, err)
	}

	// Lateral Subject: octpex.lateral.handoff.<capability>
	lateralSubject := fmt.Sprintf("%s.%s", n.lateralSubj, capability)
	err = n.client.SubscribeQueue(lateralSubject, n.queueGroup, func(ctx context.Context, data []byte) ([]byte, error) {
		var env protocol.NervousEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			return nil, fmt.Errorf("failed to unmarshal lateral envelope: %w", err)
		}

		// Security attenuation on lateral arrival
		env.Security.Attenuate(env.TargetCapability)

		res, err := handler(ctx, &env)
		if err != nil {
			return nil, err
		}
		return json.Marshal(res)
	})
	if err != nil {
		return fmt.Errorf("nats: failed to subscribe to lateral subject %s: %w", lateralSubject, err)
	}

	return nil
}

// Publish routes an envelope across the NATS JetStream distributed fabric.
func (n *NATSNervousBus) Publish(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
	if err := env.Validate(); err != nil {
		return nil, fmt.Errorf("nats bus: invalid envelope: %w", err)
	}

	var subject string
	if env.Plane == protocol.PlaneLateral {
		// Zero-Trust Hop enforcement
		env.HopCount++
		if env.HopCount > env.Security.MaxAllowedHops {
			return nil, fmt.Errorf("%w: envelope exceeded lateral hop ceiling (%d > %d)",
				ErrMaxHopsExceeded, env.HopCount, env.Security.MaxAllowedHops)
		}
		env.Security.Attenuate(env.TargetCapability)
		subject = fmt.Sprintf("%s.%s", n.lateralSubj, env.TargetCapability)
	} else {
		subject = fmt.Sprintf("%s.%s", n.verticalSubj, env.TargetCapability)
	}

	payload, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("nats bus: failed to marshal envelope: %w", err)
	}

	replyBytes, err := n.client.PublishMsg(ctx, subject, payload)
	if err != nil {
		return nil, fmt.Errorf("nats bus publish failed: %w", err)
	}

	var result protocol.StepResult
	if err := json.Unmarshal(replyBytes, &result); err != nil {
		return nil, fmt.Errorf("nats bus: failed to unmarshal reply result: %w", err)
	}

	return &result, nil
}

// Close disconnects the NATS client.
func (n *NATSNervousBus) Close() error {
	return n.client.Close()
}

// MockNATSClient provides an in-memory NATS JetStream mock engine for zero-dependency unit testing.
type MockNATSClient struct {
	mu          sync.RWMutex
	subscribers map[string]func(ctx context.Context, data []byte) ([]byte, error)
}

// NewMockNATSClient creates a mock NATS client.
func NewMockNATSClient() *MockNATSClient {
	return &MockNATSClient{
		subscribers: make(map[string]func(ctx context.Context, data []byte) ([]byte, error)),
	}
}

func (m *MockNATSClient) PublishMsg(ctx context.Context, subject string, data []byte) ([]byte, error) {
	m.mu.RLock()
	handler, ok := m.subscribers[subject]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("nats: no responders available for subject %s", subject)
	}

	return handler(ctx, data)
}

func (m *MockNATSClient) SubscribeQueue(subject, queueGroup string, handler func(ctx context.Context, data []byte) ([]byte, error)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscribers[subject] = handler
	return nil
}

func (m *MockNATSClient) Close() error {
	return nil
}
