package storage

import (
	"context"
	"errors"
	"testing"
)

func TestLocalGanglionStore_KV(t *testing.T) {
	store := NewLocalGanglionStore()
	ctx := context.Background()

	// 1. Put and Get
	err := store.PutKV(ctx, "session_draft_1", []byte("raw_unverified_ast"))
	if err != nil {
		t.Fatalf("PutKV failed: %v", err)
	}

	val, err := store.GetKV(ctx, "session_draft_1")
	if err != nil {
		t.Fatalf("GetKV failed: %v", err)
	}
	if string(val) != "raw_unverified_ast" {
		t.Fatalf("unexpected val: %s", string(val))
	}

	// 2. Delete and Get (must return ErrKeyNotFound)
	err = store.DeleteKV(ctx, "session_draft_1")
	if err != nil {
		t.Fatalf("DeleteKV failed: %v", err)
	}

	_, err = store.GetKV(ctx, "session_draft_1")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestLocalGanglionStore_VectorCosineSearch(t *testing.T) {
	store := NewLocalGanglionStore()
	ctx := context.Background()

	// Index 3 vectors
	// Vector A: [1.0, 0.0, 0.0] -> Angle 0 with query [1.0, 0.0, 0.0], similarity = 1.0
	// Vector B: [0.707, 0.707, 0.0] -> 45 degrees, similarity ~ 0.707
	// Vector C: [0.0, 1.0, 0.0] -> 90 degrees, orthogonal, similarity = 0.0
	_ = store.IndexVector(ctx, "doc-a", []float32{1.0, 0.0, 0.0}, map[string]string{"type": "exact"})
	_ = store.IndexVector(ctx, "doc-b", []float32{0.7071, 0.7071, 0.0}, map[string]string{"type": "partial"})
	_ = store.IndexVector(ctx, "doc-c", []float32{0.0, 1.0, 0.0}, map[string]string{"type": "orthogonal"})

	query := []float32{1.0, 0.0, 0.0}
	results, err := store.SearchVector(ctx, query, 2)
	if err != nil {
		t.Fatalf("SearchVector failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Record.ID != "doc-a" {
		t.Fatalf("expected top result to be doc-a, got %s", results[0].Record.ID)
	}
	if results[0].Similarity < 0.99 {
		t.Fatalf("expected similarity close to 1.0, got %f", results[0].Similarity)
	}
	if results[1].Record.ID != "doc-b" {
		t.Fatalf("expected second result to be doc-b, got %s", results[1].Record.ID)
	}

	// Dimension mismatch check
	_, err = store.SearchVector(ctx, []float32{1.0, 0.0}, 2)
	if !errors.Is(err, ErrDimensionMismatch) {
		t.Fatalf("expected ErrDimensionMismatch, got %v", err)
	}
}
