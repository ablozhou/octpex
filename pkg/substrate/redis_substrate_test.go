package substrate

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRedisSubstrate_CASLifecycle(t *testing.T) {
	mockRedis := NewMockRedisCommander()
	sub := NewRedisSubstrate(mockRedis, "test:sub")
	ctx := context.Background()

	uri := "sub://shared/session-42/ast"

	// 1. First write (expectedVersion = 0)
	v1, err := sub.PutArtifact(ctx, uri, 0, []byte("AST_NODE_ROOT"), 1*time.Minute)
	if err != nil {
		t.Fatalf("first PutArtifact failed: %v", err)
	}
	if v1 != 1 {
		t.Fatalf("expected version 1, got %d", v1)
	}

	// 2. Fetch artifact
	data, ver, err := sub.GetArtifact(ctx, uri)
	if err != nil {
		t.Fatalf("GetArtifact failed: %v", err)
	}
	if ver != 1 || string(data) != "AST_NODE_ROOT" {
		t.Fatalf("unexpected artifact: ver=%d, data=%s", ver, string(data))
	}

	// 3. Concurrent conflicting write with expectedVersion = 0 must fail
	_, err = sub.PutArtifact(ctx, uri, 0, []byte("CONFLICTING_DATA"), 1*time.Minute)
	if err == nil || !errors.Is(err, ErrCASConflict) {
		t.Fatalf("expected ErrCASConflict, got %v", err)
	}

	// 4. Stale version write (expectedVersion = 99) must fail
	_, err = sub.PutArtifact(ctx, uri, 99, []byte("STALE_DATA"), 1*time.Minute)
	if err == nil || !errors.Is(err, ErrCASConflict) {
		t.Fatalf("expected ErrCASConflict, got %v", err)
	}

	// 5. Valid CAS update (expectedVersion = 1)
	v2, err := sub.PutArtifact(ctx, uri, 1, []byte("AST_NODE_UPDATED"), 1*time.Minute)
	if err != nil {
		t.Fatalf("valid CAS update failed: %v", err)
	}
	if v2 != 2 {
		t.Fatalf("expected version 2, got %d", v2)
	}

	// 6. Distributed lease lock test
	token1, err := sub.AcquireLease(ctx, uri, "arm-1", 5*time.Second)
	if err != nil {
		t.Fatalf("AcquireLease failed: %v", err)
	}
	if token1 == "" {
		t.Fatal("expected non-empty lease token")
	}

	// Competing lease should fail
	_, err = sub.AcquireLease(ctx, uri, "arm-2", 5*time.Second)
	if !errors.Is(err, ErrLeaseConflict) {
		t.Fatalf("expected ErrLeaseConflict, got %v", err)
	}

	// Release lease
	err = sub.ReleaseLease(ctx, uri, token1)
	if err != nil {
		t.Fatalf("ReleaseLease failed: %v", err)
	}

	// Delete artifact
	err = sub.DeleteArtifact(ctx, uri)
	if err != nil {
		t.Fatalf("DeleteArtifact failed: %v", err)
	}

	_, _, err = sub.GetArtifact(ctx, uri)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after deletion, got %v", err)
	}
}
