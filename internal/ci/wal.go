package ci

import (
	"encoding/json"
	"os"
	"sync"

	"pads-v3/internal/event"
)

// WAL is an append-only event log stored on disk.
// It now stores exclusively CanonicalEvent objects.
type WAL struct {
	mu   sync.Mutex
	file *os.File
}

// NewWAL opens or creates a WAL file at the given path.
func NewWAL(path string) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	return &WAL{file: f}, nil
}

// AppendCanonical writes a CanonicalEvent to the WAL.
func (w *WAL) AppendCanonical(e event.CanonicalEvent) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if _, err := w.file.Write(b); err != nil {
		return err
	}
	return w.file.Sync()
}

// Close closes the WAL file.
func (w *WAL) Close() error {
	return w.file.Close()
}

// Path returns the WAL file path.
func (w *WAL) Path() string {
	if w == nil || w.file == nil {
		return ""
	}
	return w.file.Name()
}
