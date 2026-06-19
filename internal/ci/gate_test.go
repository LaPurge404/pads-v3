package ci

import (
	"os"
	"path/filepath"
	"testing"

	"pads-v3/internal/compiler"
	"pads-v3/internal/engine"
	"pads-v3/internal/storage"
)

func TestGateRejectsBrokenCode(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid Go module
	goMod := filepath.Join(tmpDir, "go.mod")
	os.WriteFile(goMod, []byte("module testmod\n\ngo 1.24\n"), 0644)

	dbPath := filepath.Join(tmpDir, "pads.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create a broken test file (subtraction instead of addition)
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

	// Ingest files via API métier
	_, err = compiler.IngestFile(db, brokenFile)
	if err != nil {
		t.Fatal(err)
	}
	_, err = compiler.IngestFile(db, brokenTest)
	if err != nil {
		t.Fatal(err)
	}

	// Execute engine via API métier
	if err := engine.RunOnce(db); err != nil {
		t.Fatal(err)
	}

	// CI Gate must reject BROKEN code
	result, err := Validate(db)
	if err != nil {
		t.Fatal(err)
	}
	if result.OK {
		t.Error("expected gate to reject broken code")
	}
	t.Logf("Gate correctly rejected: %s", result.Reason)
}

func TestGateAcceptsCleanCode(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid Go module
	goMod := filepath.Join(tmpDir, "go.mod")
	os.WriteFile(goMod, []byte("module testmod\n\ngo 1.24\n"), 0644)

	dbPath := filepath.Join(tmpDir, "pads.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create a correct test file
	cleanFile := filepath.Join(tmpDir, "clean.go")
	os.WriteFile(cleanFile, []byte(`package main
func Add(a, b int) int { return a + b }
`), 0644)
	cleanTest := filepath.Join(tmpDir, "clean_test.go")
	os.WriteFile(cleanTest, []byte(`package main
import "testing"
func TestAdd(t *testing.T) {
    if Add(2, 3) != 5 {
        t.Error("wrong")
    }
}
`), 0644)

	// Ingest files via API métier
	_, err = compiler.IngestFile(db, cleanFile)
	if err != nil {
		t.Fatal(err)
	}
	_, err = compiler.IngestFile(db, cleanTest)
	if err != nil {
		t.Fatal(err)
	}

	// Execute engine via API métier
	if err := engine.RunOnce(db); err != nil {
		t.Fatal(err)
	}

	// CI Gate must accept clean code
	result, err := Validate(db)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Errorf("expected gate to accept clean code, got: %s", result.Reason)
	}
	t.Log("Gate correctly accepted clean code")
}
