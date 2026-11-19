package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"octpex/pkg/bus"
	"octpex/pkg/protocol"
	"octpex/pkg/ring"
	"octpex/pkg/server"
	"octpex/pkg/substrate"
	"octpex/pkg/telemetry"
	"strings"
	"testing"
	"time"
)

func TestTelemetryHTTPWorkflow(t *testing.T) {
	ctx := context.Background()
	sub := substrate.NewInMemorySubstrate()
	costs := telemetry.NewFinOpsCollector(telemetry.DefaultPricingRates())
	b := bus.NewLocalNervousBus(2)
	defer b.Close()
	registry := ring.NewCapabilityRegistry(time.Minute)
	m, handle, err := telemetryWorker(ctx, "worker-1", sub, time.Hour, costs)
	if err != nil {
		t.Fatal(err)
	}
	if err = registry.RegisterArm(m); err != nil {
		t.Fatal(err)
	}
	b.Subscribe("distill-telemetry", handle)
	srv := server.NewHTTPServer(server.ServerConfig{}, b, registry, sub, costs)
	dispatch := func(body string) protocol.StepResult {
		t.Helper()
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/dispatch", strings.NewReader(body)))
		if w.Code != 200 {
			t.Fatalf("dispatch: %d %s", w.Code, w.Body.String())
		}
		var result protocol.StepResult
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	body := `{"envelope_id":"e","trace_id":"tr","session_id":"s","target_capability":"distill-telemetry","parameters":{"log":"WARN x\nWARN x\nOK"}}`
	for i := 0; i < 2; i++ {
		r := dispatch(body)
		if r.Status != "SUCCESS" || len(r.SubstrateRefs) != 1 || r.Metadata["trace_id"] != "tr" {
			t.Fatalf("bad reply %+v", r)
		}
		data, v, err := sub.GetArtifact(ctx, r.SubstrateRefs[0].URI)
		if err != nil || string(data) != "WARN x\nOK" || v != 1 {
			t.Fatalf("artifact %q v%d: %v", data, v, err)
		}
	}
	esc := dispatch(`{"target_capability":"distill-telemetry"}`)
	if esc.Status != "ESCALATED" || esc.Metadata["arbitration"].(map[string]any)["decision"] != "ABORT" {
		t.Fatalf("missing arbitration: %+v", esc)
	}
	summary := costs.GenerateSummary(1500)
	if summary.TotalTasks != 2 || summary.EdgeResolvedTasks != 1 || summary.EscalatedTasks != 1 || summary.EstimatedTokensSaved != 1500 {
		t.Fatalf("summary %+v", summary)
	}
	if len(registry.ListHealthyArms()) != 1 {
		t.Fatal("worker offline")
	}
}
