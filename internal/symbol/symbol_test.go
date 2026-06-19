package symbol

import (
	"os"
	"path/filepath"
	"testing"

	"pads-v3/internal/compiler"
	"pads-v3/internal/storage"
)

func TestBuildAndResolve(t *testing.T) {
	db, _ := storage.Open(":memory:")
	defer db.Close()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package main
func A() { B(); C() }
func B() {}
func C() {}
`
	os.WriteFile(testFile, []byte(src), 0644)
	compiler.IngestFile(db, testFile)

	st, err := BuildSymbolTable(db)
	if err != nil {
		t.Fatal(err)
	}

	// Vérification de la résolution directe
	for _, name := range []string{"A", "B", "C"} {
		if id := st.Resolve("main", name); id != "main."+name {
			t.Errorf("expected main.%s, got %q", name, id)
		}
	}

	// Symbole inexistant
	if id := st.Resolve("main", "D"); id != "" {
		t.Errorf("expected empty for D, got %q", id)
	}

	// Vérification que les CALLS sont déjà résolus dans le graphe
	// (le compilateur le fait directement, le résolveur n'a plus à les transformer)
	if !edgeExists(db, "main.A", "main.B") {
		t.Error("missing edge main.A -> main.B (should be resolved by compiler)")
	}
	if !edgeExists(db, "main.A", "main.C") {
		t.Error("missing edge main.A -> main.C (should be resolved by compiler)")
	}

	// Aucun edge unresolved ne doit subsister
	var unresolved int
	db.QueryRow(`SELECT COUNT(*) FROM edges WHERE relation = 'CALLS' AND target LIKE 'unresolved:%'`).Scan(&unresolved)
	if unresolved != 0 {
		t.Errorf("expected 0 unresolved, got %d", unresolved)
	}

	t.Log("symbol table and compiler pipeline OK")
}

func edgeExists(db *storage.DB, source, target string) bool {
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM edges WHERE source = ? AND target = ? AND relation = 'CALLS'`, source, target).Scan(&count)
	return count > 0
}
