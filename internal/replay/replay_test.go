package replay

import (
	"os"
	"path/filepath"
	"testing"

	"pads-v3/internal/compiler"
	"pads-v3/internal/storage"
)

func TestReplayEngine(t *testing.T) {
	tmpDir := t.TempDir()
	goMod := filepath.Join(tmpDir, "go.mod")
	os.WriteFile(goMod, []byte("module testmod\n\ngo 1.24\n"), 0644)

	dbPath := filepath.Join(tmpDir, "pads.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create a single self-contained test file
	selfContained := filepath.Join(tmpDir, "self_test.go")
	os.WriteFile(selfContained, []byte(`package main

import "testing"

func Add(a, b int) int { return a + b }

func TestAdd(t *testing.T) {
    if Add(2, 3) != 5 {
        t.Error("wrong")
    }
}
`), 0644)

	compiler.IngestFile(db, selfContained)

	snapshot, err := CaptureSnapshot(db)
	if err != nil {
		t.Fatal(err)
	}

	engine := &ReplayEngine{}
	result, err := engine.Run(db, snapshot)
	if err != nil {
		t.Fatal(err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected successful execution (exit 0), got exit code %d\nOutput: %s", result.ExitCode, result.Output)
	}
	t.Logf("Replay result: exit=%d, duration=%dms", result.ExitCode, result.DurationMs)
}
