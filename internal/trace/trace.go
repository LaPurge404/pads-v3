package trace

import (
    "encoding/json"
    "fmt"
    "os"
    "strings"

    "pads-v3/internal/event"
)

// ReadWALFile reads all events from a WAL file.
// It returns CanonicalEvent objects, converting legacy EventRecord formats on the fly.
func ReadWALFile(path string) ([]event.CanonicalEvent, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    lines := strings.Split(strings.TrimSpace(string(data)), "\n")
    var events []event.CanonicalEvent
    for _, line := range lines {
        if line == "" {
            continue
        }
        // First try CanonicalEvent
        var ce event.CanonicalEvent
        if err := json.Unmarshal([]byte(line), &ce); err == nil {
            events = append(events, ce)
            continue
        }
        // Fallback to legacy EventRecord and convert
        var le struct {
            Seq     int64  `json:"seq"`
            Type    string `json:"type"`
            JobID   string `json:"job_id"`
            StepID  string `json:"step_id"`
            Status  string `json:"status"`
            Payload string `json:"payload"`
        }
        if err := json.Unmarshal([]byte(line), &le); err != nil {
            return nil, fmt.Errorf("wal parse: %w (line: %s)", err, line)
        }
        ce = event.CanonicalEvent{
            Type:    le.Type,
            JobID:   le.JobID,
            StepID:  le.StepID,
            Status:  le.Status,
            Payload: le.Payload,
        }
        events = append(events, ce)
    }
    return events, nil
}
