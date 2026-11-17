package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config represents the complete runtime configuration of Octpex.
type Config struct {
	Profile   string          `json:"profile"`
	Brain     BrainConfig     `json:"brain"`
	Ring      RingConfig      `json:"ring"`
	Substrate SubstrateConfig `json:"substrate"`
	Bus       BusConfig       `json:"bus"`
	Sandbox   SandboxConfig   `json:"sandbox"`
}

type BrainConfig struct {
	MaxParallelTasks    int           `json:"max_parallel_tasks"`
	EscalationTimeoutMs time.Duration `json:"escalation_timeout_ms"`
	FrontierProvider    string        `json:"frontier_provider"`
	FrontierBaseURL     string        `json:"frontier_base_url"`
	FrontierModel       string        `json:"frontier_model"`
	FrontierTemperature float64       `json:"frontier_temperature"`
}

type RingConfig struct {
	RegistryType     string        `json:"registry_type"` // "in_memory" or "etcd"
	HeartbeatTTL     time.Duration `json:"heartbeat_ttl"`
	SweepInterval    time.Duration `json:"sweep_interval"`
	Endpoints        []string      `json:"endpoints"`
	LeaseTTLSeconds  int           `json:"lease_ttl_seconds"`
}

type SubstrateConfig struct {
	Type       string        `json:"type"` // "in_memory" or "redis"
	DefaultTTL time.Duration `json:"default_ttl"`
	Endpoint   string        `json:"endpoint"`
	KeyPrefix  string        `json:"key_prefix"`
}

type BusConfig struct {
	Type            string `json:"type"` // "in_memory" or "nats_jetstream"
	BufferSize      int    `json:"buffer_size"`
	URL             string `json:"url"`
	QueueGroup      string `json:"queue_group"`
	VerticalSubject string `json:"vertical_subject"`
	LateralSubject  string `json:"lateral_subject"`
}

type SandboxConfig struct {
	Provider        string        `json:"provider"` // "abloex"
	MemoryLimitMB   int           `json:"memory_limit_mb"`
	Timeout         time.Duration `json:"timeout"`
	NetworkIsolated bool          `json:"network_isolated"`
}

// DefaultLocalConfig returns standalone developer defaults.
func DefaultLocalConfig() *Config {
	return &Config{
		Profile: "local",
		Brain: BrainConfig{
			MaxParallelTasks:    4,
			EscalationTimeoutMs: 10 * time.Second,
			FrontierProvider:    "ollama",
			FrontierBaseURL:     "http://localhost:11434",
			FrontierModel:       "qwen2.5-coder:7b",
			FrontierTemperature: 0.1,
		},
		Ring: RingConfig{
			RegistryType:  "in_memory",
			HeartbeatTTL: 5 * time.Second,
			SweepInterval: 2 * time.Second,
		},
		Substrate: SubstrateConfig{
			Type:       "in_memory",
			DefaultTTL: 1 * time.Hour,
			KeyPrefix:  "octpex:sub",
		},
		Bus: BusConfig{
			Type:            "in_memory",
			BufferSize:      512,
			VerticalSubject: "octpex.vertical.dispatch",
			LateralSubject:  "octpex.lateral.handoff",
		},
		Sandbox: SandboxConfig{
			Provider:        "abloex",
			MemoryLimitMB:   256,
			Timeout:         5 * time.Second,
			NetworkIsolated: true,
		},
	}
}

// DefaultClusterConfig returns enterprise distributed cluster defaults.
func DefaultClusterConfig() *Config {
	return &Config{
		Profile: "cluster",
		Brain: BrainConfig{
			MaxParallelTasks:    32,
			EscalationTimeoutMs: 30 * time.Second,
			FrontierProvider:    "gemini",
			FrontierBaseURL:     "https://generativelanguage.googleapis.com/v1beta/openai",
			FrontierModel:       "gemini-1.5-pro",
			FrontierTemperature: 0.2,
		},
		Ring: RingConfig{
			RegistryType:    "etcd",
			Endpoints:       []string{"http://etcd-cluster:2379"},
			LeaseTTLSeconds: 5,
			SweepInterval:   2 * time.Second,
		},
		Substrate: SubstrateConfig{
			Type:       "redis",
			Endpoint:   "redis-cluster:6379",
			KeyPrefix:  "octpex:sub",
			DefaultTTL: 24 * time.Hour,
		},
		Bus: BusConfig{
			Type:            "nats_jetstream",
			URL:             "nats://nats-cluster:4222",
			QueueGroup:      "arm-workers",
			VerticalSubject: "octpex.vertical.dispatch",
			LateralSubject:  "octpex.lateral.handoff",
		},
		Sandbox: SandboxConfig{
			Provider:        "abloex",
			MemoryLimitMB:   512,
			Timeout:         10 * time.Second,
			NetworkIsolated: true,
		},
	}
}

// LoadConfig parses a YAML configuration file with intelligent fallback defaults.
func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	cfg := DefaultLocalConfig()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

		// Remove inline comments
		if idx := strings.Index(val, "#"); idx != -1 {
			val = strings.TrimSpace(val[:idx])
		}

		switch key {
		case "profile":
			cfg.Profile = val
			if val == "cluster" {
				// Switch to cluster baseline
				cfg = DefaultClusterConfig()
			}
		case "max_parallel_tasks":
			if n, err := strconv.Atoi(val); err == nil {
				cfg.Brain.MaxParallelTasks = n
			}
		case "escalation_timeout_ms":
			if ms, err := strconv.ParseInt(val, 10, 64); err == nil {
				cfg.Brain.EscalationTimeoutMs = time.Duration(ms) * time.Millisecond
			}
		case "provider":
			if cfg.Brain.FrontierProvider == "" || cfg.Brain.FrontierProvider == "ollama" {
				cfg.Brain.FrontierProvider = val
			}
		case "base_url":
			cfg.Brain.FrontierBaseURL = val
		case "model":
			cfg.Brain.FrontierModel = val
		case "registry_type":
			cfg.Ring.RegistryType = val
		case "type":
			if cfg.Substrate.Type == "in_memory" {
				cfg.Substrate.Type = val
			}
		case "endpoint":
			cfg.Substrate.Endpoint = val
		case "buffer_size":
			if n, err := strconv.Atoi(val); err == nil {
				cfg.Bus.BufferSize = n
			}
		case "memory_limit_mb":
			if n, err := strconv.Atoi(val); err == nil {
				cfg.Sandbox.MemoryLimitMB = n
			}
		case "timeout_ms":
			if ms, err := strconv.ParseInt(val, 10, 64); err == nil {
				cfg.Sandbox.Timeout = time.Duration(ms) * time.Millisecond
			}
		case "network_isolated":
			cfg.Sandbox.NetworkIsolated = (val == "true")
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	return cfg, nil
}
