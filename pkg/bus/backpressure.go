package bus

import (
	"context"
	"errors"
	"fmt"
	"octpex/pkg/protocol"
	"sync"
	"sync/atomic"
)

var (
	ErrBackpressureThrottled = errors.New("backpressure: arm queue depth exceeded throttle ceiling")
)

// WorkStealingRouter can discover sibling arms and route lowest queue.
type WorkStealingRouter interface {
	RouteLowestQueue(capabilityTag string) (*protocol.ArmManifest, error)
}

// ArmQueueMonitor tracks live queue depth and backpressure thresholds for an arm.
type ArmQueueMonitor struct {
	ArmID            string
	CapabilityTag    string
	MaxQueueCapacity int64
	currentDepth     int64
	throttledCount   int64
	stolenCount      int64
}

// NewArmQueueMonitor creates a queue monitor for an arm with a specified backpressure limit.
func NewArmQueueMonitor(armID, capability string, maxCapacity int64) *ArmQueueMonitor {
	if maxCapacity <= 0 {
		maxCapacity = 10
	}
	return &ArmQueueMonitor{
		ArmID:            armID,
		CapabilityTag:    capability,
		MaxQueueCapacity: maxCapacity,
	}
}

// Enqueue attempts to increment the queue depth.
// Returns ErrBackpressureThrottled if the depth exceeds MaxQueueCapacity.
func (m *ArmQueueMonitor) Enqueue() error {
	depth := atomic.AddInt64(&m.currentDepth, 1)
	if depth > m.MaxQueueCapacity {
		atomic.AddInt64(&m.currentDepth, -1)
		atomic.AddInt64(&m.throttledCount, 1)
		return fmt.Errorf("%w (arm %s: %d/%d)", ErrBackpressureThrottled, m.ArmID, depth, m.MaxQueueCapacity)
	}
	return nil
}

// Dequeue decrements the active queue depth.
func (m *ArmQueueMonitor) Dequeue() {
	atomic.AddInt64(&m.currentDepth, -1)
}

// CurrentDepth returns the instantaneous queue size.
func (m *ArmQueueMonitor) CurrentDepth() int64 {
	return atomic.LoadInt64(&m.currentDepth)
}

// WorkStealingController manages backpressure and delegates surplus tasks to idle peer arms.
type WorkStealingController struct {
	mu       sync.RWMutex
	monitors map[string]*ArmQueueMonitor
	router   WorkStealingRouter
	bus      *LocalNervousBus
}

// NewWorkStealingController creates a work-stealing controller.
func NewWorkStealingController(router WorkStealingRouter, bus *LocalNervousBus) *WorkStealingController {
	return &WorkStealingController{
		monitors: make(map[string]*ArmQueueMonitor),
		router:   router,
		bus:      bus,
	}
}

// RegisterArm registers an arm and its backpressure threshold.
func (c *WorkStealingController) RegisterArm(armID, capability string, capacity int64) *ArmQueueMonitor {
	c.mu.Lock()
	defer c.mu.Unlock()

	mon := NewArmQueueMonitor(armID, capability, capacity)
	c.monitors[armID] = mon
	return mon
}

// DispatchWithBackpressure dispatches an envelope, attempting local enqueue first.
// If the target arm is backpressured, it automatically steals the task and routes it
// to the lowest-queue peer arm supporting the capability.
func (c *WorkStealingController) DispatchWithBackpressure(
	ctx context.Context,
	targetArmID string,
	env *protocol.NervousEnvelope,
) (*protocol.StepResult, string, error) {
	c.mu.RLock()
	mon, exists := c.monitors[targetArmID]
	c.mu.RUnlock()

	if exists {
		err := mon.Enqueue()
		if err == nil {
			// Successfully queued locally
			defer mon.Dequeue()
			res, pubErr := c.bus.Publish(ctx, env)
			return res, targetArmID, pubErr
		}
	}

	// Backpressure triggered! Steal task and find lowest-queue sibling arm
	if c.router != nil {
		sibling, routeErr := c.router.RouteLowestQueue(env.TargetCapability)
		if routeErr == nil && sibling != nil && sibling.ArmID != targetArmID {
			// Work stolen and re-routed to sibling!
			if exists {
				atomic.AddInt64(&mon.stolenCount, 1)
			}

			// Publish to sibling
			res, pubErr := c.bus.Publish(ctx, env)
			return res, sibling.ArmID, pubErr
		}
	}

	return nil, "", ErrBackpressureThrottled
}
