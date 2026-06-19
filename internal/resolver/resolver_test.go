package resolver

import (
	"os"
	"path/filepath"
	"testing"

	"pads-v3/internal/compiler"
	"pads-v3/internal/storage"
)

// TestResolveCallsNoMatch vérifie que ResolveCalls ne crash pas
// quand un appel est unresolved et qu'aucune définition correspondante n'existe.
func TestResolveCallsNoMatch(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	// A() appelle B() qui n'est pas définie → edge unresolved
	src := `package main

func A() {
	B()
}
`
	os.WriteFile(testFile, []byte(src), 0644)
	if _, err := compiler.IngestFile(db, testFile); err != nil {
		t.Fatalf("IngestFile: %v", err)
	}

	// Vérifier que l'edge unresolved existe
	var before int
	db.QueryRow(`SELECT COUNT(*) FROM edges WHERE relation = 'CALLS' AND target LIKE 'unresolved:%'`).Scan(&before)
	if before == 0 {
		t.Fatal("expected unresolved edge after ingest, got 0")
	}

	// ResolveCalls ne trouve pas B → l'edge reste unresolved
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

// TestResolveCallsWithMatch vérifie que ResolveCalls résout
// un appel unresolved quand une définition correspondante existe.
func TestResolveCallsWithMatch(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	tmpDir := t.TempDir()
	mainFile := filepath.Join(tmpDir, "main.go")
	utilFile := filepath.Join(tmpDir, "util.go")

	// main.go : appelle helper()
	os.WriteFile(mainFile, []byte(`package main

func main() {
	helper()
}
`), 0644)
	// util.go : définit helper()
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

	// helper a été défini dans le même package → devrait être résolu
	if n < 1 {
		t.Logf("ResolveCalls a résolu %d edges (peut être 0 si le resolver ne trouve pas helper dans main)", n)
	}

	// L'edge unresolved devrait avoir disparu (ou été réduit)
	var unresolvedAfter int
	db.QueryRow(`SELECT COUNT(*) FROM edges WHERE relation = 'CALLS' AND target LIKE 'unresolved:%'`).Scan(&unresolvedAfter)
	if unresolvedAfter >= unresolvedBefore {
		t.Errorf("unresolved edges non diminuées: avant=%d, après=%d", unresolvedBefore, unresolvedAfter)
	}
}

// TestResolveCallsEmpty vérifie que ResolveCalls ne crash pas
// sur une base vide.
func TestResolveCallsEmpty(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	n, err := ResolveCalls(db)
	if err != nil {
		t.Fatalf("ResolveCalls sur base vide: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 resolved sur base vide, got %d", n)
	}
}
