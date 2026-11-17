package substrate

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound       = errors.New("substrate: artifact not found")
	ErrCASConflict    = errors.New("substrate: cas version mismatch")
	ErrLeaseConflict  = errors.New("substrate: active lease held by another worker")
	ErrInvalidURI     = errors.New("substrate: invalid sub:// URI")
)

// Artifact represents a versioned item stored in the Shared Data Substrate.
type Artifact struct {
	URI         string
	Version     int64
	Data        []byte
	ContentType string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ExpiresAt   time.Time
}

// DataSubstrateSPI defines the zero-copy shared coordination storage contract.
type DataSubstrateSPI interface {
	// PutArtifact writes an artifact with Compare-And-Swap (CAS) version validation.
	// If expectedVersion is 0, the write succeeds only if the artifact does not yet exist.
	// Returns the newly assigned monotonic version number.
	PutArtifact(ctx context.Context, uri string, expectedVersion int64, data []byte, ttl time.Duration) (newVersion int64, err error)

	// GetArtifact retrieves the data payload and current version of an artifact.
	GetArtifact(ctx context.Context, uri string) (data []byte, version int64, err error)

	// AcquireLease attempts to claim an exclusive write lock on an artifact URI.
	// Returns a lease token if successful.
	AcquireLease(ctx context.Context, uri string, workerID string, ttl time.Duration) (leaseToken string, err error)

	// ReleaseLease explicitly yields a previously acquired lease token.
	ReleaseLease(ctx context.Context, uri string, leaseToken string) error

	// DeleteArtifact purges an artifact from the substrate.
	DeleteArtifact(ctx context.Context, uri string) error
}
