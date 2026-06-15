package trace

import (
    "os"
    "testing"
)

func TestReadWALFile_Canonical(t *testing.T) {
    tmp, _ := os.CreateTemp("", "wal-*.log")
    path := tmp.Name()
    defer os.Remove(path)

    // Write a canonical event line
    canonicalJSON := `{"node_id":"n1","type":"CI_JOB_STARTED","job_id":"j1","step_id":"","status":"RUNNING","payload":"","time":0}` + "\n"
    if _, err := tmp.WriteString(canonicalJSON); err != nil {
        t.Fatal(err)
    }
    tmp.Close()

    events, err := ReadWALFile(path)
    if err != nil {
        t.Fatal(err)
    }
    if len(events) != 1 {
        t.Fatalf("expected 1 event, got %d", len(events))
    }
    if events[0].Type != "CI_JOB_STARTED" {
        t.Errorf("expected CI_JOB_STARTED, got %s", events[0].Type)
    }
}

func TestReadWALFile_Legacy(t *testing.T) {
    tmp, _ := os.CreateTemp("", "wal-*.log")
    path := tmp.Name()
    defer os.Remove(path)

    // Write a legacy event line
    legacyJSON := `{"seq":1,"type":"CI_STEP_STARTED","job_id":"j1","step_id":"s1","status":"RUNNING","payload":""}` + "\n"
    if _, err := tmp.WriteString(legacyJSON); err != nil {
        t.Fatal(err)
    }
    tmp.Close()

    events, err := ReadWALFile(path)
    if err != nil {
        t.Fatal(err)
    }
    if len(events) != 1 {
        t.Fatalf("expected 1 event, got %d", len(events))
    }
    if events[0].Type != "CI_STEP_STARTED" {
        t.Errorf("expected CI_STEP_STARTED, got %s", events[0].Type)
    }
}

func TestReadWALFile_CorruptedLine(t *testing.T) {
    tmp, _ := os.CreateTemp("", "wal-*.log")
    path := tmp.Name()
    defer os.Remove(path)

    // Write a corrupted line
    tmp.WriteString("not-json\n")
    tmp.Close()

    _, err := ReadWALFile(path)
    if err == nil {
        t.Error("expected error for corrupted line")
    }
}
