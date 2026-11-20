package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig_Local(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "local.yaml")
	content := `
profile: "local"
max_parallel_tasks: 8
memory_limit_mb: 128
timeout_ms: 3000
`
	if err := os.WriteFile(confPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfig(confPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Profile != "local" {
		t.Fatalf("expected profile local, got %s", cfg.Profile)
	}
	if cfg.Brain.MaxParallelTasks != 8 {
		t.Fatalf("expected 8 tasks, got %d", cfg.Brain.MaxParallelTasks)
	}
	if cfg.Sandbox.MemoryLimitMB != 128 {
		t.Fatalf("expected 128 MB, got %d", cfg.Sandbox.MemoryLimitMB)
	}
	if cfg.Sandbox.Timeout != 3*time.Second {
		t.Fatalf("expected 3s timeout, got %v", cfg.Sandbox.Timeout)
	}
}

func TestLoadConfig_Cluster(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "cluster.yaml")
	content := `
profile: "cluster"
max_parallel_tasks: 64
`
	if err := os.WriteFile(confPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfig(confPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Profile != "cluster" {
		t.Fatalf("expected profile cluster, got %s", cfg.Profile)
	}
	if cfg.Brain.MaxParallelTasks != 64 {
		t.Fatalf("expected 64 tasks, got %d", cfg.Brain.MaxParallelTasks)
	}
	if cfg.Ring.RegistryType != "etcd" {
		t.Fatalf("expected etcd registry, got %s", cfg.Ring.RegistryType)
	}
	if cfg.Substrate.Type != "redis" {
		t.Fatalf("expected redis substrate, got %s", cfg.Substrate.Type)
	}
}

func TestShippedProfilesAndSections(t *testing.T) {
	for _, profile := range []string{"cluster", "local"} {
		c, err := LoadConfig("../../configs/" + profile + ".yaml")
		if err != nil || c.Profile != profile {
			t.Fatalf("%s: %v", profile, err)
		}
	}
	file := filepath.Join(t.TempDir(), "override.yaml")
	if err := os.WriteFile(file, []byte("bus:\n  url: nats://other:4223\n  queue_group: other\nring:\n  endpoints: [http://other:2379]\nprofile: cluster\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := LoadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	if c.Bus.URL != "nats://other:4223" || c.Bus.QueueGroup != "other" || c.Ring.Endpoints[0] != "http://other:2379" {
		t.Fatalf("lost override: %+v", c)
	}
}

func TestLoadConfig_MultiLLM_And_SLMCatalog(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "k8s_cluster.yaml")
	content := `
profile: "cluster"
llm_providers:
  gemini-flash:
    provider: "gemini"
    model: "gemini-1.5-flash"
    base_url: "https://generativelanguage.googleapis.com/v1beta/openai"
  gpt-4o:
    provider: "openai"
    model: "gpt-4o"
slm_catalog:
  default_slm: "qwen2.5-1.5b"
  profiles:
    qwen2.5-1.5b:
      engine: "ollama"
      image: "octpex/slm-runtime:qwen2.5-1.5b"
      port: 11434
    qwen2.5-coder-1.5b:
      engine: "ollama"
      image: "octpex/slm-runtime:qwen2.5-coder-1.5b"
      port: 11434
arms:
  code-patcher:
    replicas: 2
    worker_image: "octpex/worker-code-patch:0.5.0"
    slm_profile: "qwen2.5-coder-1.5b"
    capability_tags: ["git_patch_repair"]
  weather-arm:
    replicas: 1
    worker_image: "octpex/worker-weather:0.5.0"
    capability_tags: ["get_current_temperature"]
  metrics-filter:
    replicas: 3
    worker_image: "octpex/worker-metrics:0.5.0"
    slm_profile: "none"
`
	if err := os.WriteFile(confPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfig(confPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// 1. Verify LLM providers
	if len(cfg.LLMProviders) != 2 {
		t.Fatalf("expected 2 LLM providers, got %d", len(cfg.LLMProviders))
	}
	if cfg.LLMProviders["gpt-4o"].Model != "gpt-4o" {
		t.Fatalf("expected gpt-4o, got %s", cfg.LLMProviders["gpt-4o"].Model)
	}

	// 2. Verify SLM catalog default
	if cfg.SLMCatalog.DefaultSLM != "qwen2.5-1.5b" {
		t.Fatalf("expected default slm qwen2.5-1.5b, got %s", cfg.SLMCatalog.DefaultSLM)
	}

	// 3. Verify ResolveSLMForArm with explicit profile
	codeSLM, err := cfg.ResolveSLMForArm("code-patcher")
	if err != nil || codeSLM == nil {
		t.Fatalf("expected code-patcher SLM profile, err: %v", err)
	}
	if codeSLM.Image != "octpex/slm-runtime:qwen2.5-coder-1.5b" {
		t.Fatalf("expected coder image, got %s", codeSLM.Image)
	}

	// 4. Verify ResolveSLMForArm fallback to default_slm
	weatherSLM, err := cfg.ResolveSLMForArm("weather-arm")
	if err != nil || weatherSLM == nil {
		t.Fatalf("expected weather-arm fallback SLM profile, err: %v", err)
	}
	if weatherSLM.Image != "octpex/slm-runtime:qwen2.5-1.5b" {
		t.Fatalf("expected default image for weather arm, got %s", weatherSLM.Image)
	}

	// 5. Verify ResolveSLMForArm with "none"
	metricsSLM, err := cfg.ResolveSLMForArm("metrics-filter")
	if err != nil {
		t.Fatalf("unexpected error for none SLM: %v", err)
	}
	if metricsSLM != nil {
		t.Fatalf("expected nil SLM for none profile, got %+v", metricsSLM)
	}
}
