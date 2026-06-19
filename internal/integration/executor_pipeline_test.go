package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pads-v3/internal/agent"
	"pads-v3/internal/compiler"
	"pads-v3/internal/engine"
	"pads-v3/internal/storage"
)

func TestFullConvergenceLoop(t *testing.T) {
	tmpDir := t.TempDir()

	goMod := filepath.Join(tmpDir, "go.mod")
	os.WriteFile(goMod, []byte("module testmod\n\ngo 1.24\n"), 0644)

	dbPath := filepath.Join(tmpDir, "pads.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	brokenFile := filepath.Join(tmpDir, "broken.go")
	os.WriteFile(brokenFile, []byte(`package main
func Add(a, b int) int { return a - b }
`), 0644)

	brokenTest := filepath.Join(tmpDir, "broken_test.go")
	os.WriteFile(brokenTest, []byte(`package main
import "testing"
func TestAdd(t *testing.T) {
    if Add(2, 3) != 5 {
        t.Error("wrong")
    }
}
`), 0644)

	_, err = compiler.IngestFile(db, brokenFile)
	if err != nil {
		t.Fatal(err)
	}
	_, err = compiler.IngestFile(db, brokenTest)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Phase 1: Detecting broken state...")
	if err := engine.RunOnce(db); err != nil {
		t.Fatal(err)
	}

	tasks, err := agent.BuildTasks(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) == 0 {
		t.Fatal("expected at least one BROKEN task")
	}
	t.Logf("Phase 1 complete: %d BROKEN tasks found", len(tasks))

	// Directly fix the file to simulate Executor
	t.Log("Phase 3: Directly fixing the file...")
	os.WriteFile(brokenFile, []byte(`package main
func Add(a, b int) int { return a + b }
`), 0644)

	content, err := os.ReadFile(brokenFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "return a + b") {
		t.Fatal("file was not actually fixed")
	}
	t.Log("Phase 3 complete: file verified on disk")

	if err := db.ClearGraphState(); err != nil {
		t.Fatal(err)
	}

	t.Log("Phase 4: Re-running engine for convergence...")
	if err := engine.RunOnce(db); err != nil {
		t.Fatal(err)
	}

	var brokenCount, stableCount int
	db.QueryRow(`SELECT COUNT(*) FROM graph_state WHERE state = 'BROKEN'`).Scan(&brokenCount)
	db.QueryRow(`SELECT COUNT(*) FROM graph_state WHERE state = 'STABLE'`).Scan(&stableCount)

	if brokenCount != 0 {
		t.Errorf("expected 0 BROKEN nodes after fix, got %d", brokenCount)
	}
	if stableCount == 0 {
		t.Error("expected at least one STABLE node after fix")
	}

	t.Logf("=== CONVERGENCE TRACE ===")
	t.Logf("BROKEN nodes: %d", brokenCount)
	t.Logf("STABLE nodes: %d", stableCount)
	t.Log("Full convergence loop validated")
}
