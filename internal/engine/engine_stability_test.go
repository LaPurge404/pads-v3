package engine

import (
	"os"
	"path/filepath"
	"testing"

	"pads-v3/internal/compiler"
	"pads-v3/internal/storage"
)

func TestEngine_StabilityFixPoint(t *testing.T) {
	tmpDir := t.TempDir()
	goMod := filepath.Join(tmpDir, "go.mod")
	os.WriteFile(goMod, []byte("module testmod\n\ngo 1.24\n"), 0644)

	dbPath := filepath.Join(tmpDir, "pads.db")
	db, _ := storage.Open(dbPath)
	defer db.Close()

	cleanFile := filepath.Join(tmpDir, "clean.go")
	os.WriteFile(cleanFile, []byte(`package main
func Add(a, b int) int { return a + b }
`), 0644)
	cleanTest := filepath.Join(tmpDir, "clean_test.go")
	os.WriteFile(cleanTest, []byte(`package main
import "testing"
func TestAdd(t *testing.T) {
    if Add(2, 3) != 5 { t.Error("fail") }
}
`), 0644)

	compiler.IngestFile(db, cleanFile)
	compiler.IngestFile(db, cleanTest)

	// Run multiple times and verify stable state
	for i := 0; i < 3; i++ {
		if err := RunOnce(db); err != nil {
			t.Fatal(err)
		}
	}

	var broken int
	db.QueryRow(`SELECT COUNT(*) FROM graph_state WHERE state='BROKEN'`).Scan(&broken)
	if broken != 0 {
		t.Errorf("expected 0 BROKEN after fixpoint, got %d", broken)
	}
}
