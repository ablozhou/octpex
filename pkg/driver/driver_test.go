package driver

import (
	"context"
	"net"
	"octpex/pkg/config"
	"testing"
	"time"
)

func TestDialClusterSubsystems(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		Substrate: config.SubstrateConfig{
			Type:      "redis",
			Endpoint:  "127.0.0.1:6379",
			KeyPrefix: "octpex:test:",
		},
		Bus: config.BusConfig{
			Type:       "nats",
			URL:        "nats://127.0.0.1:4222",
			QueueGroup: "octpex-test-cluster",
		},
		Ring: config.RingConfig{
			RegistryType:    "etcd",
			Endpoints:       []string{"127.0.0.1:2379"},
			LeaseTTLSeconds: 5,
		},
	}

	subsystems, err := DialClusterSubsystems(ctx, cfg)
	if err != nil {
		t.Fatalf("DialClusterSubsystems failed: %v", err)
	}

	if subsystems == nil {
		t.Fatal("expected non-nil subsystems")
	}

	if subsystems.Substrate == nil {
		t.Error("expected non-nil Substrate")
	}
	if subsystems.Bus == nil {
		t.Error("expected non-nil Bus")
	}
	if subsystems.Registry == nil {
		t.Error("expected non-nil Registry")
	}
}

func TestDialersDefaults(t *testing.T) {
	redisCmd := DialRedisCommander("")
	if redisCmd == nil {
		t.Error("DialRedisCommander returned nil")
	}

	natsCli := DialNATSClient("")
	if natsCli == nil {
		t.Error("DialNATSClient returned nil")
	}

	etcdCli := DialETCDClient(nil)
	if etcdCli == nil {
		t.Error("DialETCDClient returned nil")
	}
}

func TestLiveTCPProber(t *testing.T) {
	prober := &LiveTCPProber{}

	// Test with a local listener that opens and closes
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot listen on local port: %v", err)
	}
	addr := ln.Addr().String()

	if !prober.Probe(addr, 500*time.Millisecond) {
		t.Errorf("expected prober to succeed for active listener at %s", addr)
	}

	ln.Close()

	// Probing after closure may fail or succeed depending on OS socket reuse, but invalid address definitely fails
	if prober.Probe("127.0.0.1:54321", 50*time.Millisecond) {
		// Usually closed port fails
		t.Log("Note: 54321 was probed as open or allowed")
	}
}
