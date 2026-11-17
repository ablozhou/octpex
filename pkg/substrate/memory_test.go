package substrate

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestInMemorySubstrateCAS(t *testing.T) {
	ctx := context.Background()
	sub := NewInMemorySubstrate()
	uri := "sub://sess-1/payload"

	// 1. Initial write with expectedVersion = 0
	v1, err := sub.PutArtifact(ctx, uri, 0, []byte("data-v1"), time.Minute)
	if err != nil {
		t.Fatalf("unexpected error on v1: %v", err)
	}
	if v1 != 1 {
		t.Fatalf("expected version 1, got %d", v1)
	}

	// 2. Read back
	data, ver, err := sub.GetArtifact(ctx, uri)
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}
	if string(data) != "data-v1" || ver != 1 {
		t.Fatalf("unexpected data or version: %s, v%d", string(data), ver)
	}

	// 3. Stale update with wrong expectedVersion should fail
	_, err = sub.PutArtifact(ctx, uri, 0, []byte("data-v2-stale"), time.Minute)
	if err != ErrCASConflict {
		t.Fatalf("expected ErrCASConflict, got: %v", err)
	}

	// 4. Update with correct expectedVersion = 1
	v2, err := sub.PutArtifact(ctx, uri, 1, []byte("data-v2"), time.Minute)
	if err != nil {
		t.Fatalf("unexpected error on v2: %v", err)
	}
	if v2 != 2 {
		t.Fatalf("expected version 2, got %d", v2)
	}
}

func TestInMemorySubstrateConcurrency(t *testing.T) {
	ctx := context.Background()
	sub := NewInMemorySubstrate()
	uri := "sub://sess-concurrent/counter"

	// Initial write
	_, err := sub.PutArtifact(ctx, uri, 0, []byte("0"), time.Minute)
	if err != nil {
		t.Fatalf("initial write failed: %v", err)
	}

	// Spawn 20 goroutines attempting to write next version
	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			_, putErr := sub.PutArtifact(ctx, uri, 1, []byte("worker-data"), time.Minute)
			if putErr == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	// Exactly 1 goroutine must have succeeded CAS update from v1 to v2
	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful CAS update, got: %d", successCount)
	}
}

func TestInMemorySubstrateLease(t *testing.T) {
	ctx := context.Background()
	sub := NewInMemorySubstrate()
	uri := "sub://sess-lease/locked-item"

	token1, err := sub.AcquireLease(ctx, uri, "worker-1", 500*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error on acquire lease: %v", err)
	}

	// Worker 2 attempts to claim lease while active
	_, err = sub.AcquireLease(ctx, uri, "worker-2", 500*time.Millisecond)
	if err != ErrLeaseConflict {
		t.Fatalf("expected ErrLeaseConflict, got: %v", err)
	}

	// Worker 1 releases lease
	err = sub.ReleaseLease(ctx, uri, token1)
	if err != nil {
		t.Fatalf("release lease failed: %v", err)
	}

	// Worker 2 can now claim lease
	token2, err := sub.AcquireLease(ctx, uri, "worker-2", 500*time.Millisecond)
	if err != nil {
		t.Fatalf("worker-2 failed to acquire lease: %v", err)
	}
	if token2 == "" {
		t.Fatal("empty lease token received")
	}
}
