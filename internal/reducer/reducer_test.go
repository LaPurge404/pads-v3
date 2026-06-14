package reducer

import (
    "os"
    "testing"

    "pads-v3/internal/storage"
)

func TestRunReductionLoop(t *testing.T) {
    tmpFile, err := os.CreateTemp("", "pads-test-*.db")
    if err != nil {
        t.Fatal(err)
    }
    tmpPath := tmpFile.Name()
    tmpFile.Close()
    defer os.Remove(tmpPath)

    db, err := storage.Open(tmpPath)
    if err != nil {
        t.Fatal(err)
    }
    defer db.Close()

    // Inject simulated L2 events
    events := []struct {
        eventID   string
        eventType string
        exitCode  int
        nodeIDs   []string
    }{
        {"evt-1", "OS_EXEC_RESULT", 0, []string{"main.A", "main.B"}},
        {"evt-2", "OS_EXEC_RESULT", 1, []string{"main.C"}},
    }
    for _, e := range events {
        _, err := db.InsertEvent(e.eventID, e.eventType, "test-payload", e.exitCode)
        if err != nil {
            t.Fatalf("insert event %s: %v", e.eventID, err)
        }
        for _, nodeID := range e.nodeIDs {
            if err := db.InsertEventNode(e.eventID, nodeID); err != nil {
                t.Fatalf("insert event node %s:%s: %v", e.eventID, nodeID, err)
            }
        }
    }

    processed, err := RunReductionLoop(db)
    if err != nil {
        t.Fatalf("reduction loop failed: %v", err)
    }
    if processed != 2 {
        t.Errorf("expected 2 events processed, got %d", processed)
    }

    var state string
    db.QueryRow(`SELECT state FROM graph_state WHERE node_id = 'main.A'`).Scan(&state)
    if state != "STABLE" {
        t.Errorf("expected main.A to be STABLE, got %s", state)
    }
    db.QueryRow(`SELECT state FROM graph_state WHERE node_id = 'main.C'`).Scan(&state)
    if state != "BROKEN" {
        t.Errorf("expected main.C to be BROKEN, got %s", state)
    }
    t.Log("reduction loop OK")
}
