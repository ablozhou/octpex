package ring

import (
	"errors"
	"fmt"
	"octpex/pkg/protocol"
	"sort"
	"sync"
	"time"
)

var (
	ErrArmAlreadyRegistered = errors.New("ring: arm already registered with this ID")
	ErrArmNotFound          = errors.New("ring: arm not found in registry")
	ErrNoHealthyArmFound    = errors.New("ring: no healthy arm matching capability")
)

// RoutingStrategy defines how candidate arms are selected.
type RoutingStrategy string

const (
	RouteLowestQueue RoutingStrategy = "LOWEST_QUEUE" // Select arm with lowest active_queue_depth
	RouteRoundRobin  RoutingStrategy = "ROUND_ROBIN"
)

// CapabilityRegistry manages dynamic worker registration, heartbeats, and routing resolution.
type CapabilityRegistry struct {
	mu           sync.RWMutex
	arms         map[string]*protocol.ArmManifest
	heartbeatTTL time.Duration
	rrIndex      map[string]int
}

// NewCapabilityRegistry initializes a registry with configurable heartbeat TTL.
func NewCapabilityRegistry(heartbeatTTL time.Duration) *CapabilityRegistry {
	if heartbeatTTL <= 0 {
		heartbeatTTL = 15 * time.Second
	}
	return &CapabilityRegistry{
		arms:         make(map[string]*protocol.ArmManifest),
		heartbeatTTL: heartbeatTTL,
		rrIndex:      make(map[string]int),
	}
}

// RegisterArm registers an arm manifest into the Basal Nerve Ring.
func (r *CapabilityRegistry) RegisterArm(manifest *protocol.ArmManifest) error {
	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("registry: invalid manifest: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.arms[manifest.ArmID]; exists {
		return ErrArmAlreadyRegistered
	}

	manifestCopy := *manifest
	manifestCopy.TouchHeartbeat()
	r.arms[manifest.ArmID] = &manifestCopy
	return nil
}

// Heartbeat renews an active arm lease.
func (r *CapabilityRegistry) Heartbeat(armID string, queueDepth int, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	arm, exists := r.arms[armID]
	if !exists {
		return ErrArmNotFound
	}

	arm.TouchHeartbeat()
	arm.Telemetry.ActiveQueueDepth = queueDepth
	if status != "" {
		arm.Telemetry.Status = status
	}
	return nil
}

// DeregisterArm explicitly removes an arm from the registry.
func (r *CapabilityRegistry) DeregisterArm(armID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.arms[armID]; !exists {
		return ErrArmNotFound
	}
	delete(r.arms, armID)
	return nil
}

// ResolveWorker finds the optimal healthy arm that matches the capability tag.
// It automatically purges expired heartbeats and selects the arm with the lowest active queue depth.
func (r *CapabilityRegistry) ResolveWorker(capabilityTag string, strategy RoutingStrategy) (*protocol.ArmManifest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().Unix()
	ttlSeconds := int64(r.heartbeatTTL.Seconds())

	candidates := make([]*protocol.ArmManifest, 0)

	for _, arm := range r.arms {
		// 1. Check heartbeat freshness
		if now-arm.Telemetry.LastHeartbeatUnix > ttlSeconds {
			arm.Telemetry.Status = "OFFLINE"
			continue
		}

		// 2. Filter by status
		if arm.Telemetry.Status != "HEALTHY" {
			continue
		}

		// 3. Match capability tag
		if arm.HasCapability(capabilityTag) {
			candidates = append(candidates, arm)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("%w: '%s'", ErrNoHealthyArmFound, capabilityTag)
	}

	// Route based on strategy
	switch strategy {
	case RouteRoundRobin:
		idx := r.rrIndex[capabilityTag] % len(candidates)
		r.rrIndex[capabilityTag] = idx + 1
		result := *candidates[idx]
		return &result, nil

	default: // RouteLowestQueue
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Telemetry.ActiveQueueDepth < candidates[j].Telemetry.ActiveQueueDepth
		})
		result := *candidates[0]
		return &result, nil
	}
}

// ListHealthyArms returns a snapshot of all active arms.
func (r *CapabilityRegistry) ListHealthyArms() []*protocol.ArmManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	now := time.Now().Unix()
	ttlSeconds := int64(r.heartbeatTTL.Seconds())

	list := make([]*protocol.ArmManifest, 0, len(r.arms))
	for _, arm := range r.arms {
		if now-arm.Telemetry.LastHeartbeatUnix <= ttlSeconds && arm.Telemetry.Status == "HEALTHY" {
			armCopy := *arm
			list = append(list, &armCopy)
		}
	}
	return list
}
