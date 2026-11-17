package ring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"octpex/pkg/protocol"
	"testing"
)

func TestFederationBridge_RoutingAndForwarding(t *testing.T) {
	// 1. Mock remote cluster HTTP server
	remoteSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Octpex-Federation-Source") != "us-west-local" {
			http.Error(w, "missing federation source header", http.StatusBadRequest)
			return
		}

		if r.URL.Path == "/api/v1/topology" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"arm_count": 2, "status": "HEALTHY"}`))
			return
		}

		var env protocol.NervousEnvelope
		if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		if env.TargetCapability != "remote_analysis" {
			http.Error(w, "unexpected capability", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(protocol.StepResult{
			Status:        "SUCCESS",
			OutputMessage: "remote analysis complete in eu-central",
		})
	}))
	defer remoteSrv.Close()

	// 2. Initialize FederationBridge
	bridge := NewFederationBridge("us-west-local", remoteSrv.Client())

	bridge.RegisterPeer(ClusterPeer{
		RegionID:     "eu-central-1",
		Endpoint:     remoteSrv.URL,
		Capabilities: []string{"remote_analysis", "compliance_audit"},
	})

	// 3. Test route resolution
	peer, found := bridge.FindRoute("remote_analysis")
	if !found {
		t.Fatal("expected to find route for remote_analysis")
	}
	if peer.RegionID != "eu-central-1" {
		t.Fatalf("expected region eu-central-1, got %s", peer.RegionID)
	}

	_, notFound := bridge.FindRoute("non_existent_cap")
	if notFound {
		t.Fatal("expected false for non-existent capability")
	}

	// 4. Test envelope forwarding
	env := &protocol.NervousEnvelope{
		EnvelopeID:       "env-fed-1",
		TraceID:          "tr-fed-1",
		SessionID:        "sess-fed",
		Plane:            protocol.PlaneVertical,
		SourceNode:       "local-arm",
		TargetCapability: "remote_analysis",
		HopCount:         0,
		Security:         protocol.SecurityContext{MaxAllowedHops: 3, AllowMutation: true},
	}

	res, err := bridge.ForwardEnvelope(context.Background(), env)
	if err != nil {
		t.Fatalf("ForwardEnvelope failed: %v", err)
	}

	if res.Status != "SUCCESS" {
		t.Fatalf("expected SUCCESS, got %s", res.Status)
	}
	if res.OutputMessage != "remote analysis complete in eu-central" {
		t.Fatalf("unexpected output message: %s", res.OutputMessage)
	}

	// 5. Test hop exhaustion interception
	envExhausted := &protocol.NervousEnvelope{
		EnvelopeID:       "env-fed-2",
		TraceID:          "tr-fed-2",
		SessionID:        "sess-fed",
		Plane:            protocol.PlaneVertical,
		SourceNode:       "local-arm",
		TargetCapability: "remote_analysis",
		HopCount:         3,
		Security:         protocol.SecurityContext{MaxAllowedHops: 1, AllowMutation: false},
	}
	_, errExhausted := bridge.ForwardEnvelope(context.Background(), envExhausted)
	if errExhausted == nil {
		t.Fatal("expected error for exhausted hop count, got nil")
	}

	// 6. Test ProbePeers
	probeResults := bridge.ProbePeers(context.Background())
	if !probeResults["eu-central-1"] {
		t.Errorf("expected peer eu-central-1 to probe healthy, got false")
	}
	peers := bridge.ListPeers()
	if len(peers) != 1 || !peers[0].Healthy {
		t.Errorf("expected 1 healthy peer, got %+v", peers)
	}
}
