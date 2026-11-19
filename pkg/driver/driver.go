package driver

import (
	"context"
	"fmt"
	"net"
	"octpex/pkg/bus"
	"octpex/pkg/config"
	"octpex/pkg/ring"
	"octpex/pkg/substrate"
	"sync"
	"time"
)

// NetworkSubsystems bundles the instantiated distributed infrastructure components.
type NetworkSubsystems struct {
	CloseStorage func()
	Substrate    substrate.DataSubstrateSPI
	Bus          *bus.NATSNervousBus
	Registry     *ring.ETCDCapabilityRegistry
}

// Close releases the cluster clients opened by the driver factory.
func (n *NetworkSubsystems) Close() {
	_ = n.Bus.Close()
	_ = n.Registry.Close()
	if n.CloseStorage != nil {
		n.CloseStorage()
	}
}

// DialClusterSubsystems reports the missing production drivers in the standalone binary.
func DialClusterSubsystems(context.Context, *config.Config) (*NetworkSubsystems, error) {
	return nil, fmt.Errorf("cluster profile requires the cluster binary: build ./cluster or use the Compose image")
}

// LiveTCPProber checks if an endpoint is reachable.
type LiveTCPProber struct {
	mu sync.RWMutex
}

func (p *LiveTCPProber) Probe(endpoint string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", endpoint, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
