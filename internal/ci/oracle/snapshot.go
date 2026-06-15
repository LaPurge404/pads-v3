package oracle

import (
    "fmt"

    "pads-v3/internal/ci"
    "pads-v3/internal/event"
    "pads-v3/internal/trace"
)

// Snapshot represents a versioned oracle reference.
type Snapshot struct {
    Version string
    Digest  string
    Events  []event.CanonicalEvent
}

// Capture creates a new Oracle Snapshot from a WAL file.
func Capture(walPath string, version string) (*Snapshot, error) {
    events, err := trace.ReadWALFile(walPath)
    if err != nil {
        return nil, fmt.Errorf("capture snapshot: %w", err)
    }

    digest, err := ci.ComputeDigest(walPath)
    if err != nil {
        return nil, fmt.Errorf("compute digest: %w", err)
    }

    return &Snapshot{
        Version: version,
        Digest:  digest,
        Events:  events,
    }, nil
}
