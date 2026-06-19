package oracle

import (
	"testing"

	"pads-v3/internal/ci"
	"pads-v3/internal/event"
)

func TestCaptureSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := tmpDir + "/test.wal"

	wal, _ := ci.NewWAL(walPath)
	wal.AppendCanonical(event.CanonicalEvent{Type: "CI_JOB_STARTED", JobID: "j1"})
	wal.Close()

	snap, err := Capture(walPath, "v1")
	if err != nil {
		t.Fatal(err)
	}
	if snap.Version != "v1" {
		t.Errorf("expected version v1, got %s", snap.Version)
	}
	if snap.Digest == "" {
		t.Error("expected non-empty digest")
	}
	if len(snap.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(snap.Events))
	}
	if snap.Events[0].Type != "CI_JOB_STARTED" {
		t.Errorf("expected CI_JOB_STARTED, got %s", snap.Events[0].Type)
	}
}
