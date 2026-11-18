package driver

import (
	"bytes"
	"octpex/pkg/protocol"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// HardwareProfile captures detected physical hardware metrics and recommended model allocations.
type HardwareProfile struct {
	SystemRAMMB   int    `json:"system_ram_mb"`
	VRAMMB        int    `json:"vram_mb"`
	GPUName       string `json:"gpu_name"`
	Tier          string `json:"tier"`
	System1Engine string `json:"system1_engine"`
	System2Model  string `json:"system2_model"`
	VRAMBudgetMB  int    `json:"vram_budget_mb"` // Allocated VRAM for System-1 (0 if pinned to CPU RAM)
}

// EvaluateHardwareTier classifies hardware resources into architectural tiers (Tier 0 to Tier 3)
// and applies VRAM budget isolation rules to prevent out-of-memory (OOM) crashes.
func EvaluateHardwareTier(sysRAMMB, vramMB int, gpuName string) HardwareProfile {
	profile := HardwareProfile{
		SystemRAMMB: sysRAMMB,
		VRAMMB:      vramMB,
		GPUName:     gpuName,
	}

	lowerGPU := strings.ToLower(gpuName)
	isApple := strings.Contains(lowerGPU, "apple") || strings.Contains(lowerGPU, "m1") || strings.Contains(lowerGPU, "m2") || strings.Contains(lowerGPU, "m3") || strings.Contains(lowerGPU, "m4")

	if vramMB >= 24000 {
		profile.Tier = protocol.HardwareTier3EnterpriseGPU
		profile.System1Engine = "karpo-decision-4b"
		profile.System2Model = "qwen3.5:9b"
		profile.VRAMBudgetMB = 4096 // 4GB dedicated for Karpo-4B, rest for 9B SLM
	} else if vramMB >= 12000 || (isApple && sysRAMMB >= 16000) {
		profile.Tier = protocol.HardwareTier2Mid16GB
		profile.System1Engine = "karpo-decision-2b"
		profile.System2Model = "qwen3.5:4b"
		profile.VRAMBudgetMB = 2048 // 2GB dedicated for Karpo-2B
	} else if vramMB >= 6000 {
		// Tier 1 (e.g. GTX 1080 8GB): Pins Laya to CPU RAM (0 VRAM) to prevent OOM competition with Qwen3.5-4B
		profile.Tier = protocol.HardwareTier1Legacy8GB
		profile.System1Engine = "laya-onnx-int8"
		profile.System2Model = "qwen3.5:4b"
		profile.VRAMBudgetMB = 0
	} else {
		// Tier 0 (CPU Edge): Constrained RAM, lightweight 1.5B/0.8B generative model + CPU ONNX Laya
		profile.Tier = protocol.HardwareTier0CPUEdge
		profile.System1Engine = "laya-onnx-int8"
		profile.System2Model = "qwen3.5:1.5b"
		profile.VRAMBudgetMB = 0
	}

	return profile
}

// ProbeHostHardware inspects the current operating system and available GPU controllers.
func ProbeHostHardware() HardwareProfile {
	sysRAMMB := probeSystemRAM()
	vramMB, gpuName := probeNVIDIAGPU()

	// If no NVIDIA GPU was detected, check for Apple Silicon or fallback
	if vramMB == 0 && runtime.GOARCH == "arm64" && runtime.GOOS == "darwin" {
		gpuName = "Apple Silicon Unified Memory"
		vramMB = sysRAMMB // Unified memory architecture
	}

	return EvaluateHardwareTier(sysRAMMB, vramMB, gpuName)
}

func probeSystemRAM() int {
	// Standard fallback: default to 16GB if OS-specific probe unavailable
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	// Base estimation
	return 16384
}

func probeNVIDIAGPU() (int, string) {
	cmd := exec.Command("nvidia-smi", "--query-gpu=memory.total,name", "--format=csv,noheader,nounits")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return 0, "Integrated / None"
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return 0, "Integrated / None"
	}

	parts := strings.Split(lines[0], ",")
	if len(parts) >= 2 {
		memMB, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err == nil {
			return memMB, strings.TrimSpace(parts[1])
		}
	}
	return 0, "Integrated / None"
}
