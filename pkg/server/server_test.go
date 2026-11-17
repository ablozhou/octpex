package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"octpex/pkg/bus"
	"octpex/pkg/protocol"
	"octpex/pkg/ring"
	"octpex/pkg/substrate"
	"octpex/pkg/telemetry"
	"testing"
	"time"
)

func TestHTTPServer_Endpoints(t *testing.T) {
	localBus := bus.NewLocalNervousBus(64)
	registry := ring.NewCapabilityRegistry(15 * time.Second)
	sub := substrate.NewInMemorySubstrate()
	rates := telemetry.DefaultPricingRates()
	finops := telemetry.NewFinOpsCollector(rates)

	// Register sample arm
	_ = registry.RegisterArm(&protocol.ArmManifest{
		ArmID:          "arm-test-01",
		RuntimeAdapter: "go-native-fsm",
		RoleName:       "tester",
		CapabilityTags: []string{"test_exec"},
		Endpoint:       "in-process",
		Telemetry: protocol.ArmTelemetry{
			Status: "HEALTHY",
		},
	})

	localBus.Subscribe("test_exec", func(ctx context.Context, env *protocol.NervousEnvelope) (*protocol.StepResult, error) {
		return &protocol.StepResult{
			Status:        "SUCCESS",
			OutputMessage: "executed_from_test",
		}, nil
	})

	srv := NewHTTPServer(ServerConfig{Port: 8089}, localBus, registry, sub, finops)

	// 1. Test Index HTML
	reqIndex := httptest.NewRequest(http.MethodGet, "/", nil)
	wIndex := httptest.NewRecorder()
	srv.handleIndex(wIndex, reqIndex)
	if wIndex.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", wIndex.Code)
	}

	// 2. Test Topology API
	reqTopo := httptest.NewRequest(http.MethodGet, "/api/v1/topology", nil)
	wTopo := httptest.NewRecorder()
	srv.handleTopology(wTopo, reqTopo)
	if wTopo.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for topology, got %d", wTopo.Code)
	}
	var topoResp map[string]any
	if err := json.Unmarshal(wTopo.Body.Bytes(), &topoResp); err != nil {
		t.Fatalf("failed to parse topology json: %v", err)
	}
	if topoResp["arm_count"].(float64) != 1 {
		t.Fatalf("expected 1 active arm, got %v", topoResp["arm_count"])
	}

	// 3. Test FinOps API
	reqFin := httptest.NewRequest(http.MethodGet, "/api/v1/finops", nil)
	wFin := httptest.NewRecorder()
	srv.handleFinOps(wFin, reqFin)
	if wFin.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for finops, got %d", wFin.Code)
	}

	// 4. Test Dispatch API
	dispatchBody := []byte(`{"plane":"VERTICAL","target_capability":"test_exec","parameters":{"test":"1"}}`)
	reqDisp := httptest.NewRequest(http.MethodPost, "/api/v1/dispatch", bytes.NewReader(dispatchBody))
	wDisp := httptest.NewRecorder()
	srv.handleDispatch(wDisp, reqDisp)
	if wDisp.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for dispatch, got %d (body: %s)", wDisp.Code, wDisp.Body.String())
	}
	var stepRes protocol.StepResult
	if err := json.Unmarshal(wDisp.Body.Bytes(), &stepRes); err != nil {
		t.Fatalf("failed to decode step result: %v", err)
	}
	if stepRes.Status != "SUCCESS" || stepRes.OutputMessage != "executed_from_test" {
		t.Fatalf("unexpected step result: %+v", stepRes)
	}

	// 5. Test Events SSE endpoint
	ctx, cancel := context.WithCancel(context.Background())
	reqEvents := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil).WithContext(ctx)
	wEvents := httptest.NewRecorder()

	go func() {
		time.Sleep(50 * time.Millisecond)
		srv.BroadcastEvent(SystemEvent{
			Type:    "TEST_PULSE",
			Source:  "unit-test",
			Target:  "ring",
			Details: "ping",
		})
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	srv.handleEvents(wEvents, reqEvents)
	if wEvents.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for events stream, got %d", wEvents.Code)
	}
	if !bytes.Contains(wEvents.Body.Bytes(), []byte("CONNECTED")) {
		t.Fatalf("expected CONNECTED in event stream body, got: %s", wEvents.Body.String())
	}
	if !bytes.Contains(wEvents.Body.Bytes(), []byte("TEST_PULSE")) {
		t.Fatalf("expected TEST_PULSE in event stream body, got: %s", wEvents.Body.String())
	}
}
