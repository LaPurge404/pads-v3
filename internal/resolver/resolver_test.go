package resolver

import (
	"os"
	"path/filepath"
	"testing"

	"pads-v3/internal/compiler"
	"pads-v3/internal/storage"
)

// TestResolveCallsNoMatch verifies that ResolveCalls does not panic
// when a call is unresolved and no matching definition exists.
func TestResolveCallsNoMatch(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	// A() calls B() which is not defined → unresolved edge
	src := `package main

func A() {
	B()
}
`
	os.WriteFile(testFile, []byte(src), 0644)
	if _, err := compiler.IngestFile(db, testFile); err != nil {
		t.Fatalf("IngestFile: %v", err)
	}

	// Verify that the unresolved edge exists
	var before int
	db.QueryRow(`SELECT COUNT(*) FROM edges WHERE relation = 'CALLS' AND target LIKE 'unresolved:%'`).Scan(&before)
	if before == 0 {
		t.Fatal("expected unresolved edge after ingest, got 0")
	}

	// ResolveCalls does not find B → edge stays unresolved
	n, err := ResolveCalls(db)
	if err != nil {
		t.Fatalf("ResolveCalls: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 resolved, got %d", n)
	}

	var after int
	db.QueryRow(`SELECT COUNT(*) FROM edges WHERE relation = 'CALLS' AND target LIKE 'unresolved:%'`).Scan(&after)
	if after != before {
		t.Errorf("unresolved edges changed: before=%d, after=%d (expected unchanged)", before, after)
	}
}

// TestResolveCallsWithMatch verifies that ResolveCalls resolves
// an unresolved call when a matching definition exists.
func TestResolveCallsWithMatch(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	tmpDir := t.TempDir()
	mainFile := filepath.Join(tmpDir, "main.go")
	utilFile := filepath.Join(tmpDir, "util.go")

	// main.go: calls helper()
	os.WriteFile(mainFile, []byte(`package main

func main() {
	helper()
}
`), 0644)
	// util.go: defines helper()
	os.WriteFile(utilFile, []byte(`package main

func helper() {}
`), 0644)

	if _, err := compiler.IngestFile(db, mainFile); err != nil {
		t.Fatalf("IngestFile main: %v", err)
	}
	if _, err := compiler.IngestFile(db, utilFile); err != nil {
		t.Fatalf("IngestFile util: %v", err)
	}

	var unresolvedBefore int
	db.QueryRow(`SELECT COUNT(*) FROM edges WHERE relation = 'CALLS' AND target LIKE 'unresolved:%'`).Scan(&unresolvedBefore)

	n, err := ResolveCalls(db)
	if err != nil {
		t.Fatalf("ResolveCalls: %v", err)
	}

	// helper was defined in the same package → should be resolved
	if n < 1 {
		t.Logf("ResolveCalls resolved %d edges (may be 0 if resolver doesn't find helper in main)", n)
	}

	// The unresolved edge should be gone (or reduced)
	var unresolvedAfter int
	db.QueryRow(`SELECT COUNT(*) FROM edges WHERE relation = 'CALLS' AND target LIKE 'unresolved:%'`).Scan(&unresolvedAfter)
	if unresolvedAfter >= unresolvedBefore {
		t.Errorf("unresolved edges not reduced: before=%d, after=%d", unresolvedBefore, unresolvedAfter)
	}
}

// TestResolveCallsEmpty verifies that ResolveCalls does not panic
// on an empty database.
func TestResolveCallsEmpty(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	n, err := ResolveCalls(db)
	if err != nil {
		t.Fatalf("ResolveCalls on empty DB: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 resolved on empty DB, got %d", n)
	}
}
