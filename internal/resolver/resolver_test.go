package resolver

import (
    "os"
    "path/filepath"
    "testing"

    "pads-v3/internal/compiler"
    "pads-v3/internal/storage"
)

func TestNoUnresolvedCalls(t *testing.T) {
    db, _ := storage.Open(":memory:")
    defer db.Close()

    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "test.go")
    src := `package main

func A() {
    B()
    C()
}
func B() {}
func C() {}
`
    os.WriteFile(testFile, []byte(src), 0644)
    compiler.IngestFile(db, testFile)

    var unresolved int
    db.QueryRow(`SELECT COUNT(*) FROM edges WHERE relation = 'CALLS' AND target LIKE 'unresolved:%'`).Scan(&unresolved)
    if unresolved != 0 {
        t.Errorf("expected 0 unresolved CALLS, got %d", unresolved)
    }

    if !edgeExists(db, "main.A", "main.B") { t.Error("missing main.A -> main.B") }
    if !edgeExists(db, "main.A", "main.C") { t.Error("missing main.A -> main.C") }
}

func edgeExists(db *storage.DB, source, target string) bool {
    var count int
    db.QueryRow(`SELECT COUNT(*) FROM edges WHERE source = ? AND target = ? AND relation = 'CALLS'`, source, target).Scan(&count)
    return count > 0
}
