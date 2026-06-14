package engine

import (
    "os"
    "path/filepath"
    "testing"

    "pads-v3/internal/compiler"
    "pads-v3/internal/storage"
)

func TestRunOnce(t *testing.T) {
    // Create a temporary Go project
    tmpDir := t.TempDir()
    src := `package main
func Add(a, b int) int { return a + b }
`
    testFile := filepath.Join(tmpDir, "add.go")
    os.WriteFile(testFile, []byte(src), 0644)

    testSrc := `package main
import "testing"
func TestAdd(t *testing.T) {
    if Add(2, 3) != 5 {
        t.Error("wrong sum")
    }
}
`
    testTestFile := filepath.Join(tmpDir, "add_test.go")
    os.WriteFile(testTestFile, []byte(testSrc), 0644)

    // Initialize go module
    // (go test needs a module, but we skip for now)

    // Setup PADS database
    dbPath := filepath.Join(tmpDir, "pads.db")
    db, err := storage.Open(dbPath)
    if err != nil {
        t.Fatal(err)
    }
    defer db.Close()

    // Ingest files
    _, err = compiler.IngestFile(db, testFile)
    if err != nil {
        t.Fatal(err)
    }
    _, err = compiler.IngestFile(db, testTestFile)
    if err != nil {
        t.Fatal(err)
    }

    // Verify nodes exist in nodes table
    var nodeCount int
    db.QueryRow(`SELECT COUNT(*) FROM nodes`).Scan(&nodeCount)
    if nodeCount == 0 {
        t.Fatal("expected nodes in nodes table")
    }

    // Verify nodes are NOT yet in graph_state
    var stateCount int
    db.QueryRow(`SELECT COUNT(*) FROM graph_state`).Scan(&stateCount)
    if stateCount != 0 {
        t.Fatal("expected empty graph_state before engine run")
    }

    // Run engine
    err = RunOnce(db)
    if err != nil {
        t.Fatal(err)
    }

    // Verify nodes are now in graph_state
    db.QueryRow(`SELECT COUNT(*) FROM graph_state`).Scan(&stateCount)
    if stateCount == 0 {
        t.Error("expected nodes to be inserted into graph_state")
    }
    t.Logf("graph_state now has %d entries", stateCount)
}
