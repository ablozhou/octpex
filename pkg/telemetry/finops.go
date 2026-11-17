package telemetry

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// PricingRates defines cost per 1M tokens for estimating financial ROI.
type PricingRates struct {
	FrontierInputPer1M  float64 `json:"frontier_input_per_1m"`  // Baseline: e.g. $2.50
	FrontierOutputPer1M float64 `json:"frontier_output_per_1m"` // Baseline: e.g. $10.00
	LocalSLMPer1M       float64 `json:"local_slm_per_1m"`       // Local compute: $0.00
}

// DefaultPricingRates returns standard commercial LLM pricing reference.
func DefaultPricingRates() PricingRates {
	return PricingRates{
		FrontierInputPer1M:  2.50,
		FrontierOutputPer1M: 10.00,
		LocalSLMPer1M:       0.00,
	}
}

// TaskRecord represents an execution audit record.
type TaskRecord struct {
	TraceID     string
	ArmID       string
	Level       string // "L0_DETERMINISTIC", "L1_EDGE_SLM", "L2_WRAPPED_AGENT", "L3_CREW", "BRAIN"
	CloudTokens int
	EdgeTokens  int
	Duration    time.Duration
	Escalated   bool
	Timestamp   time.Time
}

// FinOpsSummary provides high-level financial and operational efficiency metrics.
type FinOpsSummary struct {
	TotalTasks           int64   `json:"total_tasks"`
	EdgeResolvedTasks    int64   `json:"edge_resolved_tasks"`
	EscalatedTasks       int64   `json:"escalated_tasks"`
	EdgeDecouplingRatio  float64 `json:"edge_decoupling_ratio"` // e.g. 0.85 (85%)
	TotalCloudTokensUsed int64   `json:"total_cloud_tokens_used"`
	TotalEdgeTokensUsed  int64   `json:"total_edge_tokens_used"`
	EstimatedTokensSaved int64   `json:"estimated_tokens_saved"`
	EstimatedCostSavedUSD float64 `json:"estimated_cost_saved_usd"`
	AverageEdgeLatencyMs float64 `json:"average_edge_latency_ms"`
}

// FinOpsCollector tracks live operational token consumption and financial ROI.
type FinOpsCollector struct {
	mu           sync.RWMutex
	records      []TaskRecord
	rates        PricingRates
	totalTasks   int64
	edgeTasks    int64
	escalated    int64
	cloudTokens  int64
	edgeTokens   int64
	edgeDuration time.Duration
}

// NewFinOpsCollector creates an initialized FinOps metrics collector.
func NewFinOpsCollector(rates PricingRates) *FinOpsCollector {
	return &FinOpsCollector{
		rates: rates,
	}
}

// RecordTask logs an execution event into the FinOps engine.
func (c *FinOpsCollector) RecordTask(record TaskRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.records = append(c.records, record)
	c.totalTasks++

	if record.Escalated {
		c.escalated++
	} else if record.Level != "BRAIN" {
		c.edgeTasks++
		c.edgeDuration += record.Duration
	}

	atomic.AddInt64(&c.cloudTokens, int64(record.CloudTokens))
	atomic.AddInt64(&c.edgeTokens, int64(record.EdgeTokens))
}

// GenerateSummary calculates ROI metrics comparing edge processing against full-frontier baseline.
func (c *FinOpsCollector) GenerateSummary(baselineTokensPerTask int) FinOpsSummary {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.totalTasks == 0 {
		return FinOpsSummary{}
	}

	if baselineTokensPerTask <= 0 {
		baselineTokensPerTask = 1500 // Average diagnostic prompt + completion
	}

	// Theoretical tokens consumed if every task were routed to Frontier LLM
	theoreticalFrontierTokens := c.totalTasks * int64(baselineTokensPerTask)
	tokensSaved := theoreticalFrontierTokens - c.cloudTokens
	if tokensSaved < 0 {
		tokensSaved = 0
	}

	// Cost savings calculation
	// Blended price: ($2.50 in + $10.00 out) / 2 = ~$6.25 per 1M tokens
	blendedRatePerToken := ((c.rates.FrontierInputPer1M + c.rates.FrontierOutputPer1M) / 2.0) / 1_000_000.0
	costSavedUSD := float64(tokensSaved) * blendedRatePerToken

	decouplingRatio := 0.0
	if c.totalTasks > 0 {
		decouplingRatio = float64(c.edgeTasks) / float64(c.totalTasks)
	}

	avgLatencyMs := 0.0
	if c.edgeTasks > 0 {
		avgLatencyMs = float64(c.edgeDuration.Milliseconds()) / float64(c.edgeTasks)
	}

	return FinOpsSummary{
		TotalTasks:           c.totalTasks,
		EdgeResolvedTasks:    c.edgeTasks,
		EscalatedTasks:       c.escalated,
		EdgeDecouplingRatio:  decouplingRatio,
		TotalCloudTokensUsed: c.cloudTokens,
		TotalEdgeTokensUsed:  c.edgeTokens,
		EstimatedTokensSaved: tokensSaved,
		EstimatedCostSavedUSD: costSavedUSD,
		AverageEdgeLatencyMs: avgLatencyMs,
	}
}

// PrintReport formats a human-readable FinOps audit report.
func (c *FinOpsCollector) PrintReport(baselineTokensPerTask int) string {
	s := c.GenerateSummary(baselineTokensPerTask)
	return fmt.Sprintf(
		"================================================================================\n"+
			"                       OCTPEX FINOPS TELEMETRY AUDIT                            \n"+
			"================================================================================\n"+
			"Total Workloads Processed  : %d\n"+
			"Resolved at Edge (Arms)    : %d (%.1f%% Decoupling)\n"+
			"Escalated to Brain         : %d\n"+
			"Cloud Tokens Consumed      : %d\n"+
			"Edge SLM Tokens Consumed   : %d (0 Cloud Cost)\n"+
			"Estimated Cloud Tokens Saved: %d\n"+
			"Estimated Financial Savings: $%.2f USD\n"+
			"Average Edge Task Latency  : %.1f ms\n"+
			"================================================================================",
		s.TotalTasks, s.EdgeResolvedTasks, s.EdgeDecouplingRatio*100, s.EscalatedTasks,
		s.TotalCloudTokensUsed, s.TotalEdgeTokensUsed, s.EstimatedTokensSaved,
		s.EstimatedCostSavedUSD, s.AverageEdgeLatencyMs,
	)
}
