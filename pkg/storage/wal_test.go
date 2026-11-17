package storage

import (
	"octpex/pkg/protocol"
	"path/filepath"
	"testing"
)

func TestWriteAheadLog_AppendAndReplay(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "arm_offline.wal")

	wal, err := NewWriteAheadLog(logPath)
	if err != nil {
		t.Fatalf("failed to create WAL: %v", err)
	}

	env1 := &protocol.NervousEnvelope{
		EnvelopeID: "env-wal-01",
		TraceID:    "trace-wal-01",
		SessionID:  "session-wal-01",
		SourceNode: "arm-offline-01",
	}

	env2 := &protocol.NervousEnvelope{
		EnvelopeID: "env-wal-02",
		TraceID:    "trace-wal-02",
		SessionID:  "session-wal-02",
		SourceNode: "arm-offline-01",
	}

	// 1. Append 2 entries
	seq1, err := wal.Append(env1, "Gate 3 escalation during brain disconnect")
	if err != nil || seq1 != 1 {
		t.Fatalf("expected seq 1, got %d, err: %v", seq1, err)
	}

	seq2, err := wal.Append(env2, "Gate 2 retry exhausted during offline mode")
	if err != nil || seq2 != 2 {
		t.Fatalf("expected seq 2, got %d, err: %v", seq2, err)
	}

	_ = wal.Close()

	// 2. Re-open WAL (verifying crash recovery and seq ID resumption)
	wal2, err := NewWriteAheadLog(logPath)
	if err != nil {
		t.Fatalf("failed to reopen WAL: %v", err)
	}
	defer wal2.Close()

	var replayedIDs []string
	count, err := wal2.ReplayPending(func(entry WALEntry) error {
		replayedIDs = append(replayedIDs, entry.Envelope.EnvelopeID)
		return nil
	})

	if err != nil {
		t.Fatalf("ReplayPending failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 replayed items, got %d", count)
	}
	if len(replayedIDs) != 2 || replayedIDs[0] != "env-wal-01" || replayedIDs[1] != "env-wal-02" {
		t.Fatalf("unexpected replayed envelopes: %+v", replayedIDs)
	}

	// 3. Second replay attempt must find 0 pending entries
	count2, err := wal2.ReplayPending(func(entry WALEntry) error {
		return nil
	})
	if err != nil || count2 != 0 {
		t.Fatalf("expected 0 pending items on second replay, got %d", count2)
	}
}
