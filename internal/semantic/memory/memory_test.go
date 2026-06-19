package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIndexAndQuery(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := t.TempDir()

	// Create a minimal Go project with intra-file calls
	pkgDir := filepath.Join(projectDir, "mypkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	// File 1: defines Bar() and calls internal helper()
	file1 := filepath.Join(pkgDir, "file1.go")
	if err := os.WriteFile(file1, []byte(`package mypkg

func Bar() int {
	return helper()
}

func helper() int {
	return 42
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	// File 2: defines Exported() that calls Bar() from file1
	file2 := filepath.Join(pkgDir, "file2.go")
	if err := os.WriteFile(file2, []byte(`package mypkg

// Exported is a public function that calls Bar().
func Exported() int {
	return Bar() + 1
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	// Create the memory index
	mem, err := New(projectDir, tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer mem.Close()

	// Index the project
	if err := mem.IncrementallyIndex(); err != nil {
		t.Fatal(err)
	}

	// Verify symbol count
	n, err := mem.SymbolCount()
	if err != nil {
		t.Fatal(err)
	}
	if n < 3 {
		t.Errorf("expected at least 3 symbols, got %d", n)
	}

	// Test ExportedSymbols — Exported and Bar should be visible
	exported, err := mem.ExportedSymbols("")
	if err != nil {
		t.Fatal(err)
	}
	if len(exported) < 2 {
		t.Errorf("expected at least 2 exported symbols, got %d", len(exported))
	}
	// helper() is not exported
	for _, s := range exported {
		if s.Name == "helper" {
			t.Error("helper should not be in exported symbols")
		}
	}

	// Test SymbolByName
	bar, err := mem.SymbolByName("Bar", "mypkg")
	if err != nil {
		t.Fatal(err)
	}
	if bar == nil {
		t.Fatal("Bar symbol not found")
	}
	if bar.Kind != "func" {
		t.Errorf("Bar kind = %q, want %q", bar.Kind, "func")
	}
	if !bar.Exported {
		t.Error("Bar should be exported")
	}

	// Test CalleesOf — what does Bar() call?
	callees, err := mem.CalleesOf("Bar", "mypkg")
	if err != nil {
		t.Fatal(err)
	}
	if len(callees) == 0 {
		t.Error("Bar should have at least one callee (helper)")
	}

	// Test SymbolsInFile
	syms, err := mem.SymbolsInFile(file1)
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) < 2 {
		t.Errorf("file1 should have at least 2 symbols, got %d", len(syms))
	}

	// Test SymbolImpact — Bar has at least one direct caller (Exported)
	direct, trans, err := mem.SymbolImpact("Bar", "mypkg")
	if err != nil {
		t.Fatal(err)
	}
	if direct < 1 {
		t.Errorf("Bar should have at least 1 direct caller (Exported), got %d", direct)
	}
	t.Logf("Bar impact: direct=%d transitive=%d", direct, trans)
}

func TestIncrementalIndex(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := t.TempDir()

	pkgDir := filepath.Join(projectDir, "incpkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	file1 := filepath.Join(pkgDir, "a.go")
	if err := os.WriteFile(file1, []byte(`package incpkg
func One() int { return 1 }
func Two() int { return 2 }
`), 0644); err != nil {
		t.Fatal(err)
	}

	mem, err := New(projectDir, tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer mem.Close()

	// First index
	if err := mem.IncrementallyIndex(); err != nil {
		t.Fatal(err)
	}
	n1, err := mem.SymbolCount()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("After first index: %d symbols", n1)
	if n1 < 2 {
		t.Fatalf("expected ≥2 symbols after first index, got %d", n1)
	}

	// Second index with no changes — should be fast, no new symbols
	if err := mem.IncrementallyIndex(); err != nil {
		t.Fatal(err)
	}
	n2, err := mem.SymbolCount()
	if err != nil {
		t.Fatal(err)
	}
	if n2 != n1 {
		t.Errorf("symbol count changed after re-index with no changes: %d → %d", n1, n2)
	}

	// Modify file1: add a new exported function Three()
	if err := os.WriteFile(file1, []byte(`package incpkg
func One() int { return 1 }
func Two() int { return 2 }
func Three() int { return 3 }
`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := mem.IncrementallyIndex(); err != nil {
		t.Fatal(err)
	}
	n3, err := mem.SymbolCount()
	if err != nil {
		t.Fatal(err)
	}
	if n3 <= n1 {
		t.Errorf("expected more symbols after adding Three(), got %d (was %d)", n3, n1)
	}
	t.Logf("After adding Three(): %d symbols (was %d)", n3, n1)

	// Verify Three() is indexed
	three, err := mem.SymbolByName("Three", "incpkg")
	if err != nil {
		t.Fatal(err)
	}
	if three == nil {
		t.Error("Three() not found after incremental index")
	}
}

func TestIndexFile(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := t.TempDir()

	pkgDir := filepath.Join(projectDir, "singlepkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create one file
	file1 := filepath.Join(pkgDir, "start.go")
	if err := os.WriteFile(file1, []byte(`package singlepkg
func Start() int { return helper() }
func helper() int { return 0 }
`), 0644); err != nil {
		t.Fatal(err)
	}

	// Create memory but don't index yet
	mem, err := New(projectDir, tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer mem.Close()

	// Index only the single file
	if err := mem.IndexFile(file1); err != nil {
		t.Fatal(err)
	}

	n, err := mem.SymbolCount()
	if err != nil {
		t.Fatal(err)
	}
	if n < 2 {
		t.Errorf("expected ≥2 symbols after IndexFile, got %d", n)
	}

	// Verify helpers aren't exported
	exported, err := mem.ExportedSymbols("")
	if err != nil {
		t.Fatal(err)
	}
	if len(exported) < 1 {
		t.Error("expected at least 1 exported symbol (Start)")
	}
	for _, s := range exported {
		if s.Name == "helper" {
			t.Error("helper should not be exported")
		}
	}

	// Verify callees of Start
	callees, err := mem.CalleesOf("Start", "singlepkg")
	if err != nil {
		t.Fatal(err)
	}
	if len(callees) == 0 {
		t.Error("Start should call helper")
	}
}

func TestPublicAPISurface(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := t.TempDir()

	pkgDir := filepath.Join(projectDir, "surfacepkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	f := filepath.Join(pkgDir, "api.go")
	os.WriteFile(f, []byte(`package surfacepkg
type MyStruct struct {}
func (m *MyStruct) Do() {}
func PublicFunc() {}
func privateFunc() {}
const PublicConst = 1
`), 0644)

	mem, err := New(projectDir, tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer mem.Close()

	if err := mem.IncrementallyIndex(); err != nil {
		t.Fatal(err)
	}

	surface, err := mem.PublicAPISurface()
	if err != nil {
		t.Fatal(err)
	}

	syms := surface["surfacepkg"]
	if len(syms) == 0 {
		t.Fatal("no symbols in surfacepkg")
	}
	for _, s := range syms {
		if s.Name == "privateFunc" {
			t.Error("privateFunc should not be in public API surface")
		}
	}
	t.Logf("Public API surface for surfacepkg: %d symbols", len(syms))
}
