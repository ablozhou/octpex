package telemetry

import (
	"strings"
	"testing"
	"time"
)

func TestFinOpsCollector_ROIAndDecoupling(t *testing.T) {
	rates := DefaultPricingRates()
	collector := NewFinOpsCollector(rates)

	// Simulate 100 edge tasks (Level 1 SLM, 0 cloud tokens)
	for i := 0; i < 85; i++ {
		collector.RecordTask(TaskRecord{
			TraceID:     "trace-edge",
			ArmID:       "arm-edge-1",
			Level:       "L1_EDGE_SLM",
			CloudTokens: 0,
			EdgeTokens:  120,
			Duration:    50 * time.Millisecond,
			Escalated:   false,
		})
	}

	// Simulate 15 escalated tasks (requiring brain intervention, 1200 cloud tokens each)
	for i := 0; i < 15; i++ {
		collector.RecordTask(TaskRecord{
			TraceID:     "trace-brain",
			ArmID:       "octo-brain",
			Level:       "BRAIN",
			CloudTokens: 1200,
			EdgeTokens:  0,
			Duration:    1200 * time.Millisecond,
			Escalated:   true,
		})
	}

	summary := collector.GenerateSummary(1500)

	if summary.TotalTasks != 100 {
		t.Fatalf("expected 100 total tasks, got %d", summary.TotalTasks)
	}
	if summary.EdgeResolvedTasks != 85 {
		t.Fatalf("expected 85 edge tasks, got %d", summary.EdgeResolvedTasks)
	}
	if summary.EscalatedTasks != 15 {
		t.Fatalf("expected 15 escalations, got %d", summary.EscalatedTasks)
	}
	if summary.EdgeDecouplingRatio != 0.85 {
		t.Fatalf("expected 85%% decoupling, got %f", summary.EdgeDecouplingRatio)
	}

	// 100 tasks * 1500 = 150,000 baseline tokens. Cloud used: 15 * 1200 = 18,000.
	// Saved: 150,000 - 18,000 = 132,000 tokens
	if summary.EstimatedTokensSaved != 132000 {
		t.Fatalf("expected 132,000 tokens saved, got %d", summary.EstimatedTokensSaved)
	}
	if summary.EstimatedCostSavedUSD <= 0 {
		t.Fatalf("expected positive cost savings, got %f", summary.EstimatedCostSavedUSD)
	}

	report := collector.PrintReport(1500)
	if !strings.Contains(report, "85.0% Decoupling") {
		t.Fatalf("expected report to contain '85.0%% Decoupling', got:\n%s", report)
	}
}
