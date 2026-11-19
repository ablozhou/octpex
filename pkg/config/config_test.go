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
