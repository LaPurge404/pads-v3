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

	wal, _ := NewPolicyWAL(walPath)
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
