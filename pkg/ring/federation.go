package ring

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"octpex/pkg/protocol"
	"slices"
	"sync"
	"time"
)

// ClusterPeer represents a remote regional cluster participating in the federation.
type ClusterPeer struct {
	RegionID     string    `json:"region_id"`    // e.g. "us-west-2", "eu-central-1", "edge-site-01"
	Endpoint     string    `json:"endpoint"`     // e.g. "http://octpex-eu:8080"
	Capabilities []string  `json:"capabilities"` // Capability tags available in this remote cluster
	LatencyMs    float64   `json:"latency_ms"`   // Estimated roundtrip latency
	Healthy      bool      `json:"healthy"`      // Peer health status
	LastProbe    time.Time `json:"last_probe"`   // Timestamp of last health probe
}

// FederationBridge coordinates cross-region routing and lateral envelope handoffs across clusters.
type FederationBridge struct {
	localRegionID string
	httpClient    *http.Client
	peers         map[string]*ClusterPeer // Key: RegionID
	capIndex      map[string]string       // Key: capabilityTag -> RegionID
	mu            sync.RWMutex
}

// NewFederationBridge creates an initialized cross-cluster federation router.
func NewFederationBridge(localRegionID string, httpClient *http.Client) *FederationBridge {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &FederationBridge{
		localRegionID: localRegionID,
		httpClient:    httpClient,
		peers:         make(map[string]*ClusterPeer),
		capIndex:      make(map[string]string),
	}
}

// RegisterPeer registers or updates a remote cluster peer.
func (fb *FederationBridge) RegisterPeer(peer ClusterPeer) {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	p := peer
	p.Healthy = true
	p.LastProbe = time.Now()
	fb.peers[p.RegionID] = &p

	for _, capTag := range p.Capabilities {
		fb.capIndex[capTag] = p.RegionID
	}
}

// FindRoute finds the remote cluster region hosting the target capability.
func (fb *FederationBridge) FindRoute(capability string) (*ClusterPeer, bool) {
	fb.mu.RLock()
	defer fb.mu.RUnlock()

	regionID, found := fb.capIndex[capability]
	if !found {
		return nil, false
	}
	peer, exists := fb.peers[regionID]
	if !exists || !peer.Healthy {
		return nil, false
	}
	return peer, true
}

// ForwardEnvelope forwards a NervousEnvelope to the appropriate remote cluster peer.
func (fb *FederationBridge) ForwardEnvelope(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
	if env == nil {
		return nil, errors.New("cannot forward nil envelope")
	}

	peer, found := fb.FindRoute(env.TargetCapability)
	if !found {
		return nil, fmt.Errorf("no healthy remote cluster route found for capability: %s", env.TargetCapability)
	}

	// Attenuate security bounds for inter-cluster boundary hop
	if env.Security.MaxAllowedHops <= 1 {
		return nil, fmt.Errorf("hop limit reached; envelope cannot traverse cluster boundary (max_allowed_hops=%d)", env.Security.MaxAllowedHops)
	}

	if len(env.Security.AllowedCapabilities) > 0 && !slices.Contains(env.Security.AllowedCapabilities, env.TargetCapability) {
		return nil, fmt.Errorf("federation target outside allowed capabilities")
	}
	forwardEnv := *env
	forwardEnv.HopCount++
	forwardEnv.Plane = protocol.PlaneLateral
	forwardEnv.Security.Attenuate(env.TargetCapability)

	payloadBytes, err := json.Marshal(forwardEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize envelope for federation forward: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/dispatch", peer.Endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create inter-cluster request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Octpex-Federation-Source", fb.localRegionID)

	resp, err := fb.httpClient.Do(req)
	if err != nil {
		fb.mu.Lock()
		peer.Healthy = false
		fb.mu.Unlock()
		return nil, fmt.Errorf("inter-cluster dispatch failed to %s (%s): %w", peer.RegionID, url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote cluster %s returned HTTP %d", peer.RegionID, resp.StatusCode)
	}

	var stepRes protocol.StepResult
	if err := json.NewDecoder(resp.Body).Decode(&stepRes); err != nil {
		return nil, fmt.Errorf("failed to decode response from remote cluster %s: %w", peer.RegionID, err)
	}

	return &stepRes, nil
}

// ListPeers returns all registered cluster peers.
func (fb *FederationBridge) ListPeers() []ClusterPeer {
	fb.mu.RLock()
	defer fb.mu.RUnlock()

	result := make([]ClusterPeer, 0, len(fb.peers))
	for _, p := range fb.peers {
		result = append(result, *p)
	}
	return result
}

// ProbePeers probes all registered peers and updates their health status and roundtrip latency.
func (fb *FederationBridge) ProbePeers(ctx context.Context) map[string]bool {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	results := make(map[string]bool)
	for regionID, peer := range fb.peers {
		start := time.Now()
		url := fmt.Sprintf("%s/api/v1/topology", peer.Endpoint)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			peer.Healthy = false
			results[regionID] = false
			continue
		}
		req.Header.Set("X-Octpex-Federation-Source", fb.localRegionID)

		resp, err := fb.httpClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			peer.Healthy = false
			results[regionID] = false
		} else {
			peer.Healthy = true
			peer.LatencyMs = float64(time.Since(start).Microseconds()) / 1000.0
			results[regionID] = true
			_ = resp.Body.Close()
		}
		peer.LastProbe = time.Now()
	}
	return results
}
