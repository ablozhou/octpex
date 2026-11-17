package storage

import (
	"context"
	"errors"
	"math"
	"sort"
	"sync"
	"time"
)

var (
	ErrKeyNotFound      = errors.New("ganglion_store: key not found in private store")
	ErrDimensionMismatch = errors.New("ganglion_store: vector dimension mismatch")
)

// VectorRecord represents an embedded chunk stored in the arm's private vector space.
type VectorRecord struct {
	ID        string            `json:"id"`
	Vector    []float32         `json:"vector"`
	Metadata  map[string]string `json:"metadata"`
	Timestamp time.Time         `json:"timestamp"`
}

// VectorSearchResult encapsulates the similarity score and retrieved record.
type VectorSearchResult struct {
	Record     VectorRecord `json:"record"`
	Similarity float32      `json:"similarity"` // Cosine similarity in range [-1.0, 1.0]
}

// GanglionStoreSPI defines the interface for an Arm's private local storage.
// It keeps raw sensitive logs, few-shot prompt demonstrations, and draft embeddings
// strictly isolated from the central brain and other arms.
type GanglionStoreSPI interface {
	// PutKV stores a private key-value pair.
	PutKV(ctx context.Context, key string, val []byte) error

	// GetKV retrieves a private key-value pair.
	GetKV(ctx context.Context, key string) ([]byte, error)

	// DeleteKV removes a key from the private store.
	DeleteKV(ctx context.Context, key string) error

	// IndexVector stores an embedding vector with associated metadata.
	IndexVector(ctx context.Context, id string, vector []float32, metadata map[string]string) error

	// SearchVector searches for top-K nearest neighbors using cosine similarity.
	SearchVector(ctx context.Context, query []float32, topK int) ([]VectorSearchResult, error)
}

// LocalGanglionStore provides an in-process, thread-safe implementation of GanglionStoreSPI.
type LocalGanglionStore struct {
	mu      sync.RWMutex
	kvStore map[string][]byte
	vectors map[string]VectorRecord
}

// NewLocalGanglionStore creates a new private storage engine for an Arm ganglion.
func NewLocalGanglionStore() *LocalGanglionStore {
	return &LocalGanglionStore{
		kvStore: make(map[string][]byte),
		vectors: make(map[string]VectorRecord),
	}
}

func (s *LocalGanglionStore) PutKV(ctx context.Context, key string, val []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kvStore[key] = val
	return nil
}

func (s *LocalGanglionStore) GetKV(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.kvStore[key]
	if !ok {
		return nil, ErrKeyNotFound
	}
	res := make([]byte, len(val))
	copy(res, val)
	return res, nil
}

func (s *LocalGanglionStore) DeleteKV(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.kvStore, key)
	return nil
}

func (s *LocalGanglionStore) IndexVector(ctx context.Context, id string, vector []float32, metadata map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.vectors[id] = VectorRecord{
		ID:        id,
		Vector:    vector,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}
	return nil
}

func (s *LocalGanglionStore) SearchVector(ctx context.Context, query []float32, topK int) ([]VectorSearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if topK <= 0 {
		topK = 5
	}

	results := make([]VectorSearchResult, 0, len(s.vectors))

	for _, rec := range s.vectors {
		if len(rec.Vector) != len(query) {
			return nil, ErrDimensionMismatch
		}

		sim := cosineSimilarity(query, rec.Vector)
		results = append(results, VectorSearchResult{
			Record:     rec,
			Similarity: sim,
		})
	}

	// Sort descending by similarity
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

func cosineSimilarity(a, b []float32) float32 {
	var dotProduct float32
	var normA float32
	var normB float32

	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / float32(math.Sqrt(float64(normA))*math.Sqrt(float64(normB)))
}
