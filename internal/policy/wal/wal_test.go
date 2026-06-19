package wal

import (
	"os"
	"testing"

	"pads-v3/internal/policy"
)

func TestPolicyWAL_AppendAndRead(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := tmpDir + "/policy.log"

	wal, err := NewPolicyWAL(walPath)
	if err != nil {
		t.Fatal(err)
	}

	event := PolicyEvent{
		DecisionID:  "trace-1",
		Score:       85.0,
		Status:      "WARN",
		Trace:       policy.PolicyTrace{FinalScore: 85, FinalStatus: policy.StatusWarn},
		Explanation: policy.PolicyExplanation{Summary: "test"},
	}

	if err := wal.Append(event); err != nil {
		t.Fatal(err)
	}

	events, err := wal.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].DecisionID != "trace-1" {
		t.Errorf("expected trace-1, got %s", events[0].DecisionID)
	}
}

func TestPolicyWAL_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := tmpDir + "/empty.log"

	wal, err := NewPolicyWAL(walPath)
	if err != nil {
		t.Fatal(err)
	}

	events, err := wal.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestPolicyWAL_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := tmpDir + "/create.log"

	_, err := NewPolicyWAL(walPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(walPath); os.IsNotExist(err) {
		t.Error("WAL file should exist after creation")
	}
}

// TestPolicyWAL_CorruptionResilience verifies that ReadAll skips invalid JSON lines
// and returns partial results without panicking.
func TestPolicyWAL_CorruptionResilience(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := tmpDir + "/corrupted.log"

	wal, err := NewPolicyWAL(walPath)
	if err != nil {
		t.Fatal(err)
	}

	// Write 2 valid events, then corrupt the file, then write 1 more valid event.
	event1 := PolicyEvent{DecisionID: "trace-1", Score: 80}
	event2 := PolicyEvent{DecisionID: "trace-2", Score: 85}
	event3 := PolicyEvent{DecisionID: "trace-3", Score: 90}
	if err := wal.Append(event1); err != nil {
		t.Fatal(err)
	}
	if err := wal.Append(event2); err != nil {
		t.Fatal(err)
	}

	// Append corrupted lines (invalid JSON) directly to the WAL file.
	f, err := os.OpenFile(walPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	// Two invalid lines that must be skipped by ReadAll.
	f.WriteString("{not valid json at all line 1}\n")
	f.WriteString(`{"incomplete json":` + "\n")
	f.Close()

	// Write the last valid event after the corrupted block.
	if err := wal.Append(event3); err != nil {
		t.Fatal(err)
	}

	// ReadAll must skip invalid lines and return valid events without error.
	// The parser must not panic on corrupted input.
	events, err := wal.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll returned error on partially corrupted WAL: %v", err)
	}

	// Should have 3 valid events; 2 corrupted lines are skipped.
	if len(events) != 3 {
		t.Fatalf("expected 3 events (2 valid + 1 after corruption), got %d", len(events))
	}

	ids := make(map[string]bool)
	for _, e := range events {
		ids[e.DecisionID] = true
	}
	if !ids["trace-1"] || !ids["trace-2"] || !ids["trace-3"] {
		t.Errorf("missing events: trace-1=%v trace-2=%v trace-3=%v",
			ids["trace-1"], ids["trace-2"], ids["trace-3"])
	}
}