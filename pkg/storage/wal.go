package storage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"octpex/pkg/protocol"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// WALEntry represents a single durably logged escalation entry.
type WALEntry struct {
	SeqID     int64                     `json:"seq_id"`
	Timestamp time.Time                 `json:"timestamp"`
	Envelope  *protocol.NervousEnvelope `json:"envelope"`
	Reason    string                    `json:"reason"`
	Replayed  bool                      `json:"replayed"`
}

// WriteAheadLog provides durable local disk logging for air-gapped edge arm resilience.
type WriteAheadLog struct {
	mu      sync.Mutex
	logPath string
	file    *os.File
	seqID   int64
}

// NewWriteAheadLog creates or opens an existing WAL file.
func NewWriteAheadLog(filePath string) (*WriteAheadLog, error) {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, fmt.Errorf("wal: failed to create directories: %w", err)
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("wal: failed to open log file: %w", err)
	}

	wal := &WriteAheadLog{
		logPath: filePath,
		file:    file,
	}

	// Scan file to find highest seqID
	wal.recoverSeqID()
	return wal, nil
}

func (w *WriteAheadLog) recoverSeqID() {
	scanner := bufio.NewScanner(w.file)
	var maxID int64
	for scanner.Scan() {
		var entry WALEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err == nil {
			if entry.SeqID > maxID {
				maxID = entry.SeqID
			}
		}
	}
	w.seqID = maxID
}

// Append durably writes a pending escalation envelope to disk.
func (w *WriteAheadLog) Append(env *protocol.NervousEnvelope, reason string) (int64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	seq := atomic.AddInt64(&w.seqID, 1)
	entry := WALEntry{
		SeqID:     seq,
		Timestamp: time.Now(),
		Envelope:  env,
		Reason:    reason,
		Replayed:  false,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return 0, fmt.Errorf("wal: failed to marshal entry: %w", err)
	}
	data = append(data, '\n')

	if _, err := w.file.Write(data); err != nil {
		return 0, fmt.Errorf("wal: failed to write to file: %w", err)
	}

	_ = w.file.Sync()
	return seq, nil
}

// ReplayPending iterates over all pending (unreplayed) entries, invoking the handler for each.
func (w *WriteAheadLog) ReplayPending(handler func(entry WALEntry) error) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Reopen for reading
	f, err := os.Open(w.logPath)
	if err != nil {
		return 0, fmt.Errorf("wal: failed to read log file: %w", err)
	}
	defer f.Close()

	var replayedCount int
	scanner := bufio.NewScanner(f)
	var allEntries []WALEntry

	for scanner.Scan() {
		var entry WALEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		if !entry.Replayed {
			if err := handler(entry); err == nil {
				entry.Replayed = true
				replayedCount++
			}
		}
		allEntries = append(allEntries, entry)
	}

	// Compact / Rewrite log with updated replayed flags
	_ = w.file.Close()
	rewriteFile, err := os.OpenFile(w.logPath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	if err != nil {
		return replayedCount, err
	}
	w.file = rewriteFile

	for _, e := range allEntries {
		data, _ := json.Marshal(e)
		data = append(data, '\n')
		_, _ = w.file.Write(data)
	}
	_ = w.file.Sync()

	return replayedCount, nil
}

// Close flushes and closes the WAL file handle.
func (w *WriteAheadLog) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}
