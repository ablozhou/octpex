package ring

import (
	"context"
	"encoding/json"
	"fmt"
	"octpex/pkg/protocol"
	"sort"
	"strings"
	"sync"
	"time"
)

// ETCDClient abstracts the etcd v3 KV and Lease operations.
type ETCDClient interface {
	GrantLease(ctx context.Context, ttlSeconds int64) (int64, error)
	KeepAliveOnce(ctx context.Context, leaseID int64) error
	RevokeLease(ctx context.Context, leaseID int64) error
	PutWithLease(ctx context.Context, key string, val string, leaseID int64) error
	GetPrefix(ctx context.Context, prefix string) (map[string]string, error)
	WatchPrefix(ctx context.Context, prefix string, onChange func(eventType, key, val string)) error
	Close() error
}

// ETCDCapabilityRegistry implements dynamic arm registration and discovery backed by etcd.
type ETCDCapabilityRegistry struct {
	client    ETCDClient
	prefix    string
	leaseTTL  int64
	mu        sync.RWMutex
	cache     map[string]*protocol.ArmManifest
	leases    map[string]int64
	stopCh    chan struct{}
	closeOnce sync.Once
}

// NewETCDCapabilityRegistry creates a new etcd-backed dynamic registry.
func NewETCDCapabilityRegistry(client ETCDClient, leaseTTLSeconds int64) (*ETCDCapabilityRegistry, error) {
	if leaseTTLSeconds <= 0 {
		leaseTTLSeconds = 5
	}
	r := &ETCDCapabilityRegistry{
		client:   client,
		prefix:   "/octpex/ring/arms/",
		leaseTTL: leaseTTLSeconds,
		cache:    make(map[string]*protocol.ArmManifest),
		leases:   make(map[string]int64),
		stopCh:   make(chan struct{}),
	}

	// 1. Initial sync
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	items, err := client.GetPrefix(ctx, r.prefix)
	if err != nil {
		return nil, fmt.Errorf("read etcd registry: %w", err)
	}
	{
		for _, v := range items {
			var m protocol.ArmManifest
			if err := json.Unmarshal([]byte(v), &m); err == nil {
				r.cache[m.ArmID] = &m
			}
		}
	}

	// 2. Watch for dynamic topology changes (pod crashes, scale-outs)
	err = client.WatchPrefix(context.Background(), r.prefix, func(eventType, key, val string) {
		armID := strings.TrimPrefix(key, r.prefix)
		r.mu.Lock()
		defer r.mu.Unlock()

		if eventType == "DELETE" {
			delete(r.cache, armID)
		} else if eventType == "PUT" {
			var m protocol.ArmManifest
			if err := json.Unmarshal([]byte(val), &m); err == nil {
				r.cache[m.ArmID] = &m
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start etcd watch: %w", err)
	}

	return r, nil
}

// RegisterArm registers an arm manifest with an attached etcd heartbeat lease.
func (r *ETCDCapabilityRegistry) RegisterArm(ctx context.Context, manifest *protocol.ArmManifest) error {
	if err := manifest.Validate(); err != nil {
		return err
	}

	leaseID, err := r.client.GrantLease(ctx, r.leaseTTL)
	if err != nil {
		return fmt.Errorf("etcd: failed to grant lease: %w", err)
	}

	payload, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal arm manifest: %w", err)
	}

	key := fmt.Sprintf("%s%s", r.prefix, manifest.ArmID)
	if err := r.client.PutWithLease(ctx, key, string(payload), leaseID); err != nil {
		return fmt.Errorf("etcd: failed to put arm with lease: %w", err)
	}

	r.mu.Lock()
	r.cache[manifest.ArmID] = manifest
	r.leases[manifest.ArmID] = leaseID
	r.mu.Unlock()

	// Launch background keepalive renewal routine
	go r.keepAliveLoop(manifest.ArmID, leaseID)
	return nil
}

func (r *ETCDCapabilityRegistry) keepAliveLoop(armID string, leaseID int64) {
	ticker := time.NewTicker(time.Duration(r.leaseTTL) * time.Second / 2)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.mu.RLock()
			currentLease, exists := r.leases[armID]
			r.mu.RUnlock()

			if !exists || currentLease != leaseID {
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err := r.client.KeepAliveOnce(ctx, leaseID)
			cancel()
			if err != nil {
				// Lease lost or expired
				r.mu.Lock()
				delete(r.cache, armID)
				delete(r.leases, armID)
				r.mu.Unlock()
				return
			}
		}
	}
}

// UnregisterArm gracefully revokes the arm lease and deletes it from etcd.
func (r *ETCDCapabilityRegistry) UnregisterArm(ctx context.Context, armID string) error {
	r.mu.Lock()
	leaseID, ok := r.leases[armID]
	delete(r.cache, armID)
	delete(r.leases, armID)
	r.mu.Unlock()

	if ok {
		_ = r.client.RevokeLease(ctx, leaseID)
	}
	return nil
}

// ListHealthyArms returns all active arms currently recognized by the registry cache.
func (r *ETCDCapabilityRegistry) ListHealthyArms() []*protocol.ArmManifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*protocol.ArmManifest, 0, len(r.cache))
	for _, a := range r.cache {
		result = append(result, a)
	}
	return result
}

// RouteLowestQueue routes to the arm with the shortest active queue.
func (r *ETCDCapabilityRegistry) RouteLowestQueue(capabilityTag string) (*protocol.ArmManifest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var candidates []*protocol.ArmManifest
	for _, arm := range r.cache {
		if arm.HasCapability(capabilityTag) && arm.Telemetry.Status != "OFFLINE" {
			candidates = append(candidates, arm)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("%w: '%s'", ErrNoHealthyArmFound, capabilityTag)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Telemetry.ActiveQueueDepth < candidates[j].Telemetry.ActiveQueueDepth
	})

	return candidates[0], nil
}

// Close gracefully closes the registry and terminates all keepalive renewal workers.
func (r *ETCDCapabilityRegistry) Close() error {
	r.closeOnce.Do(func() {
		close(r.stopCh)
	})
	return r.client.Close()
}

// MockETCDClient implements an in-memory etcd v3 client for test verification.
type MockETCDClient struct {
	mu       sync.RWMutex
	store    map[string]string
	leases   map[int64]time.Time
	leaseSeq int64
	watchers []func(eventType, key, val string)
}

// NewMockETCDClient creates an in-memory mock etcd client.
func NewMockETCDClient() *MockETCDClient {
	return &MockETCDClient{
		store:    make(map[string]string),
		leases:   make(map[int64]time.Time),
		watchers: make([]func(eventType, key, val string), 0),
	}
}

func (m *MockETCDClient) GrantLease(ctx context.Context, ttlSeconds int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.leaseSeq++
	id := m.leaseSeq
	m.leases[id] = time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	return id, nil
}

func (m *MockETCDClient) KeepAliveOnce(ctx context.Context, leaseID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.leases[leaseID]; !ok {
		return fmt.Errorf("lease not found or expired")
	}
	m.leases[leaseID] = time.Now().Add(5 * time.Second)
	return nil
}

func (m *MockETCDClient) RevokeLease(ctx context.Context, leaseID int64) error {
	m.mu.Lock()
	delete(m.leases, leaseID)
	m.mu.Unlock()
	return nil
}

func (m *MockETCDClient) PutWithLease(ctx context.Context, key, val string, leaseID int64) error {
	m.mu.Lock()
	m.store[key] = val
	watchers := make([]func(eventType, key, val string), len(m.watchers))
	copy(watchers, m.watchers)
	m.mu.Unlock()

	for _, w := range watchers {
		w("PUT", key, val)
	}
	return nil
}

func (m *MockETCDClient) GetPrefix(ctx context.Context, prefix string) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make(map[string]string)
	for k, v := range m.store {
		if strings.HasPrefix(k, prefix) {
			res[k] = v
		}
	}
	return res, nil
}

func (m *MockETCDClient) WatchPrefix(ctx context.Context, prefix string, onChange func(eventType, key, val string)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.watchers = append(m.watchers, onChange)
	return nil
}

// SimulateCrash deletes key and fires DELETE watch event to simulate sudden pod death.
func (m *MockETCDClient) SimulateCrash(key string) {
	m.mu.Lock()
	delete(m.store, key)
	watchers := make([]func(eventType, key, val string), len(m.watchers))
	copy(watchers, m.watchers)
	m.mu.Unlock()

	for _, w := range watchers {
		w("DELETE", key, "")
	}
}

func (m *MockETCDClient) Close() error {
	return nil
}
