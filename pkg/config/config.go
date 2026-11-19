package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/goccy/go-yaml"
	"os"

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
	FrontierAPIKeyEnv   string        `json:"frontier_api_key_env"`
	FrontierTemperature float64       `json:"frontier_temperature"`
}

type RingConfig struct {
	RegistryType    string        `json:"registry_type"` // "in_memory" or "etcd"
	HeartbeatTTL    time.Duration `json:"heartbeat_ttl"`
	SweepInterval   time.Duration `json:"sweep_interval"`
	Endpoints       []string      `json:"endpoints"`
	LeaseTTLSeconds int           `json:"lease_ttl_seconds"`
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
			HeartbeatTTL:  5 * time.Second,
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
			BufferSize:      4096,
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

// LoadConfig accepts the shipped nested YAML and the original flat configuration.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	var header struct{ Profile string }
	if err := json.Unmarshal(raw, &header); err != nil {
		return nil, err
	}
	cfg := DefaultLocalConfig()
	switch header.Profile {
	case "", "local":
	case "cluster":
		cfg = DefaultClusterConfig()
	default:
		return nil, fmt.Errorf("unknown profile %q", header.Profile)
	}
	var file struct {
		Profile          string
		MaxParallelTasks *int   `json:"max_parallel_tasks"`
		MemoryLimit      *int   `json:"memory_limit_mb"`
		Timeout          *int64 `json:"timeout_ms"`
		Brain            struct {
			BrainConfig
			Orchestrator struct {
				MaxParallelTasks  *int   `json:"max_parallel_tasks"`
				EscalationTimeout *int64 `json:"escalation_timeout_ms"`
			}
			Frontier struct {
				Provider, Model string
				BaseURL         string `json:"base_url"`
				APIKeyEnv       string `json:"api_key_env"`
				Temperature     *float64
			}
		}
		Ring struct {
			RingConfig
			Heartbeat *int64 `json:"heartbeat_ttl_ms"`
			Sweep     *int64 `json:"sweep_interval_ms"`
		}
		Substrate struct {
			SubstrateConfig
			TTL *int64 `json:"default_ttl_ms"`
		}
		Bus     BusConfig
		Sandbox struct {
			SandboxConfig
			Milliseconds *int64 `json:"timeout_ms"`
		}
	}
	file.Brain.BrainConfig = cfg.Brain
	file.Brain.EscalationTimeoutMs /= time.Millisecond
	file.Ring.RingConfig = cfg.Ring
	file.Substrate.SubstrateConfig = cfg.Substrate
	file.Bus = cfg.Bus
	file.Sandbox.SandboxConfig = cfg.Sandbox
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	cfg.Brain = file.Brain.BrainConfig
	cfg.Brain.EscalationTimeoutMs *= time.Millisecond
	if file.MaxParallelTasks != nil {
		cfg.Brain.MaxParallelTasks = *file.MaxParallelTasks
	}
	orch := file.Brain.Orchestrator
	if orch.MaxParallelTasks != nil {
		cfg.Brain.MaxParallelTasks = *orch.MaxParallelTasks
	}
	if orch.EscalationTimeout != nil {
		cfg.Brain.EscalationTimeoutMs = time.Duration(*orch.EscalationTimeout) * time.Millisecond
	}
	frontier := file.Brain.Frontier
	if frontier.Provider != "" {
		cfg.Brain.FrontierProvider = frontier.Provider
	}
	if frontier.Model != "" {
		cfg.Brain.FrontierModel = frontier.Model
	}
	if frontier.BaseURL != "" {
		cfg.Brain.FrontierBaseURL = frontier.BaseURL
	}
	if frontier.APIKeyEnv != "" {
		cfg.Brain.FrontierAPIKeyEnv = frontier.APIKeyEnv
	}
	if frontier.Temperature != nil {
		cfg.Brain.FrontierTemperature = *frontier.Temperature
	}
	cfg.Ring, cfg.Substrate, cfg.Bus, cfg.Sandbox = file.Ring.RingConfig, file.Substrate.SubstrateConfig, file.Bus, file.Sandbox.SandboxConfig
	if file.Ring.Heartbeat != nil {
		cfg.Ring.HeartbeatTTL = time.Duration(*file.Ring.Heartbeat) * time.Millisecond
	}
	if file.Ring.Sweep != nil {
		cfg.Ring.SweepInterval = time.Duration(*file.Ring.Sweep) * time.Millisecond
	}
	if file.Substrate.TTL != nil {
		cfg.Substrate.DefaultTTL = time.Duration(*file.Substrate.TTL) * time.Millisecond
	}
	if file.MemoryLimit != nil {
		cfg.Sandbox.MemoryLimitMB = *file.MemoryLimit
	}
	if file.Timeout != nil {
		cfg.Sandbox.Timeout = time.Duration(*file.Timeout) * time.Millisecond
	}
	if file.Sandbox.Milliseconds != nil {
		cfg.Sandbox.Timeout = time.Duration(*file.Sandbox.Milliseconds) * time.Millisecond
	}
	if cfg.Bus.BufferSize <= 0 || cfg.Sandbox.MemoryLimitMB <= 0 || cfg.Sandbox.Timeout <= 0 || cfg.Substrate.DefaultTTL <= 0 || cfg.Brain.EscalationTimeoutMs <= 0 {
		return nil, fmt.Errorf("invalid buffer size, quota or timeout in %s", path)
	}
	if cfg.Profile == "local" && cfg.Ring.HeartbeatTTL < time.Second {
		return nil, fmt.Errorf("heartbeat TTL must be at least one second")
	}
	return cfg, nil
}
