package driver

import (
	"octpex/pkg/protocol"
	"testing"
)

func TestEvaluateHardwareTier_Tier0_CPUEdge(t *testing.T) {
	p := EvaluateHardwareTier(8192, 0, "Intel UHD Graphics 620")
	if p.Tier != protocol.HardwareTier0CPUEdge {
		t.Fatalf("expected Tier 0, got %s", p.Tier)
	}
	if p.System1Engine != "laya-onnx-int8" {
		t.Fatalf("expected laya-onnx-int8, got %s", p.System1Engine)
	}
	if p.System2Model != "qwen3.5:1.5b" {
		t.Fatalf("expected qwen3.5:1.5b, got %s", p.System2Model)
	}
	if p.VRAMBudgetMB != 0 {
		t.Fatalf("expected VRAMBudgetMB=0, got %d", p.VRAMBudgetMB)
	}
}

func TestEvaluateHardwareTier_Tier1_Legacy8GB(t *testing.T) {
	// GTX 1080 with 8192MB VRAM
	p := EvaluateHardwareTier(16384, 8192, "NVIDIA GeForce GTX 1080")
	if p.Tier != protocol.HardwareTier1Legacy8GB {
		t.Fatalf("expected Tier 1, got %s", p.Tier)
	}
	// Crucial rule: Laya must be pinned to CPU (0 VRAM) to leave all 8GB for Qwen3.5-4B
	if p.System1Engine != "laya-onnx-int8" {
		t.Fatalf("expected laya-onnx-int8 for Tier 1 CPU pinning, got %s", p.System1Engine)
	}
	if p.System2Model != "qwen3.5:4b" {
		t.Fatalf("expected qwen3.5:4b, got %s", p.System2Model)
	}
	if p.VRAMBudgetMB != 0 {
		t.Fatalf("expected VRAMBudgetMB=0 to prevent OOM, got %d", p.VRAMBudgetMB)
	}
}

func TestEvaluateHardwareTier_Tier2_MidRange16GB(t *testing.T) {
	// RTX 4070 with 12GB VRAM
	p := EvaluateHardwareTier(32768, 12288, "NVIDIA GeForce RTX 4070")
	if p.Tier != protocol.HardwareTier2Mid16GB {
		t.Fatalf("expected Tier 2, got %s", p.Tier)
	}
	if p.System1Engine != "karpo-decision-2b" {
		t.Fatalf("expected karpo-decision-2b, got %s", p.System1Engine)
	}
	if p.System2Model != "qwen3.5:4b" {
		t.Fatalf("expected qwen3.5:4b, got %s", p.System2Model)
	}
	if p.VRAMBudgetMB != 2048 {
		t.Fatalf("expected VRAMBudgetMB=2048, got %d", p.VRAMBudgetMB)
	}
}

func TestEvaluateHardwareTier_Tier2_AppleSilicon(t *testing.T) {
	p := EvaluateHardwareTier(16384, 0, "Apple M2 Pro")
	if p.Tier != protocol.HardwareTier2Mid16GB {
		t.Fatalf("expected Tier 2 for Apple Silicon, got %s", p.Tier)
	}
	if p.System1Engine != "karpo-decision-2b" {
		t.Fatalf("expected karpo-decision-2b, got %s", p.System1Engine)
	}
}

func TestEvaluateHardwareTier_Tier3_EnterpriseGPU(t *testing.T) {
	// A100 with 40GB VRAM
	p := EvaluateHardwareTier(65536, 40960, "NVIDIA A100-SXM4-40GB")
	if p.Tier != protocol.HardwareTier3EnterpriseGPU {
		t.Fatalf("expected Tier 3, got %s", p.Tier)
	}
	if p.System1Engine != "karpo-decision-4b" {
		t.Fatalf("expected karpo-decision-4b, got %s", p.System1Engine)
	}
	if p.System2Model != "qwen3.5:9b" {
		t.Fatalf("expected qwen3.5:9b, got %s", p.System2Model)
	}
	if p.VRAMBudgetMB != 4096 {
		t.Fatalf("expected VRAMBudgetMB=4096, got %d", p.VRAMBudgetMB)
	}
}

func TestProbeHostHardware(t *testing.T) {
	p := ProbeHostHardware()
	if p.Tier == "" {
		t.Fatalf("expected valid detected tier, got empty")
	}
	if p.System1Engine == "" {
		t.Fatalf("expected valid system1 engine, got empty")
	}
}
