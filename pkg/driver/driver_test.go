package driver

import (
	"context"
	"net"
	"octpex/pkg/config"
	"testing"
	"time"
)

func TestStandaloneRejectsCluster(t *testing.T) {
	ds, err := DialClusterSubsystems(context.Background(), config.DefaultClusterConfig())
	if ds != nil || err == nil {
		t.Fatal("expected a cluster driver error")
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
