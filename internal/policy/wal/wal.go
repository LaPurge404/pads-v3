package wal

import (
    "encoding/json"
    "fmt"
    "os"
    "strings"

    "pads-v3/internal/policy"
)

// PolicyEvent is the record persisted for each CI policy decision.
type PolicyEvent struct {
    Timestamp   int64                   `json:"timestamp"`
    DecisionID  string                  `json:"decision_id"`
    Score       float64                 `json:"score"`
    Status      string                  `json:"status"`
    Trace       policy.PolicyTrace      `json:"trace"`
    Explanation policy.PolicyExplanation `json:"explanation"`
}

// PolicyWAL is an append-only log of policy decisions.
type PolicyWAL struct {
    path string
}

// NewPolicyWAL creates or opens the policy WAL file.
func NewPolicyWAL(path string) (*PolicyWAL, error) {
    f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        return nil, err
    }
    f.Close()
    return &PolicyWAL{path: path}, nil
}

// Append writes a policy event to the WAL.
func (w *PolicyWAL) Append(event PolicyEvent) error {
    f, err := os.OpenFile(w.path, os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        return err
    }
    defer f.Close()

    data, err := json.Marshal(event)
    if err != nil {
        return err
    }
    data = append(data, '\n')
    _, err = f.Write(data)
    return err
}

// ReadAll reads all policy events from the WAL.
func (w *PolicyWAL) ReadAll() ([]PolicyEvent, error) {
    data, err := os.ReadFile(w.path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil
        }
        return nil, err
    }
    lines := strings.Split(strings.TrimSpace(string(data)), "\n")
    var events []PolicyEvent
    for _, line := range lines {
        if line == "" {
            continue
        }
        var e PolicyEvent
        if err := json.Unmarshal([]byte(line), &e); err != nil {
            return nil, fmt.Errorf("wal parse: %w (line: %s)", err, line)
        }
        events = append(events, e)
    }
    return events, nil
}

// LoadTraces converts WAL events into policy traces.
func LoadTraces(events []PolicyEvent) []policy.PolicyTrace {
    traces := make([]policy.PolicyTrace, 0, len(events))
    for _, e := range events {
        traces = append(traces, e.Trace)
    }
    return traces
}
