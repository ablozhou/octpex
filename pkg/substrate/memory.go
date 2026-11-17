package substrate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

type storedItem struct {
	artifact Artifact
	lease    *leaseRecord
}

type leaseRecord struct {
	token     string
	workerID  string
	expiresAt time.Time
}

// InMemorySubstrate is a zero-dependency, thread-safe implementation of DataSubstrateSPI.
type InMemorySubstrate struct {
	mu    sync.RWMutex
	store map[string]*storedItem
}

// NewInMemorySubstrate creates an initialized in-memory substrate.
func NewInMemorySubstrate() *InMemorySubstrate {
	return &InMemorySubstrate{
		store: make(map[string]*storedItem),
	}
}

func (m *InMemorySubstrate) PutArtifact(ctx context.Context, uri string, expectedVersion int64, data []byte, ttl time.Duration) (int64, error) {
	if !strings.HasPrefix(uri, "sub://") {
		return 0, ErrInvalidURI
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	item, exists := m.store[uri]

	// Check lease
	if exists && item.lease != nil && item.lease.expiresAt.After(now) {
		// Active lease exists
		return 0, ErrLeaseConflict
	}

	if !exists {
		if expectedVersion != 0 {
			return 0, ErrCASConflict
		}
		newVersion := int64(1)
		var exp time.Time
		if ttl > 0 {
			exp = now.Add(ttl)
		}
		dataCopy := make([]byte, len(data))
		copy(dataCopy, data)

		m.store[uri] = &storedItem{
			artifact: Artifact{
				URI:       uri,
				Version:   newVersion,
				Data:      dataCopy,
				CreatedAt: now,
				UpdatedAt: now,
				ExpiresAt: exp,
			},
		}
		return newVersion, nil
	}

	// Check expiration
	if !item.artifact.ExpiresAt.IsZero() && now.After(item.artifact.ExpiresAt) {
		// Existing expired, treat as not existing
		if expectedVersion != 0 {
			return 0, ErrCASConflict
		}
	} else if item.artifact.Version != expectedVersion {
		return 0, ErrCASConflict
	}

	newVersion := item.artifact.Version + 1
	var exp time.Time
	if ttl > 0 {
		exp = now.Add(ttl)
	}
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	item.artifact.Version = newVersion
	item.artifact.Data = dataCopy
	item.artifact.UpdatedAt = now
	item.artifact.ExpiresAt = exp

	return newVersion, nil
}

func (m *InMemorySubstrate) GetArtifact(ctx context.Context, uri string) ([]byte, int64, error) {
	if !strings.HasPrefix(uri, "sub://") {
		return nil, 0, ErrInvalidURI
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	item, exists := m.store[uri]
	if !exists {
		return nil, 0, ErrNotFound
	}

	if !item.artifact.ExpiresAt.IsZero() && time.Now().After(item.artifact.ExpiresAt) {
		return nil, 0, ErrNotFound
	}

	dataCopy := make([]byte, len(item.artifact.Data))
	copy(dataCopy, item.artifact.Data)
	return dataCopy, item.artifact.Version, nil
}

func (m *InMemorySubstrate) AcquireLease(ctx context.Context, uri string, workerID string, ttl time.Duration) (string, error) {
	if !strings.HasPrefix(uri, "sub://") {
		return "", ErrInvalidURI
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	item, exists := m.store[uri]
	if !exists {
		// Create empty placeholder with lease
		item = &storedItem{
			artifact: Artifact{
				URI:       uri,
				Version:   0,
				CreatedAt: now,
			},
		}
		m.store[uri] = item
	}

	if item.lease != nil && item.lease.expiresAt.After(now) && item.lease.workerID != workerID {
		return "", ErrLeaseConflict
	}

	tokenBytes := make([]byte, 16)
	rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	item.lease = &leaseRecord{
		token:     token,
		workerID:  workerID,
		expiresAt: now.Add(ttl),
	}

	return token, nil
}

func (m *InMemorySubstrate) ReleaseLease(ctx context.Context, uri string, leaseToken string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, exists := m.store[uri]
	if !exists || item.lease == nil {
		return nil
	}
	if item.lease.token == leaseToken {
		item.lease = nil
	}
	return nil
}

func (m *InMemorySubstrate) DeleteArtifact(ctx context.Context, uri string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, uri)
	return nil
}
