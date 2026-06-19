package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"pads-v3/internal/storage"
)

func TestIngestIdempotent(t *testing.T) {
	db, _ := storage.Open(":memory:")
	defer db.Close()
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package main
func A() { B() }
func B() {}
`
	os.WriteFile(testFile, []byte(src), 0644)
	res1, _ := IngestFile(db, testFile)
	res2, _ := IngestFile(db, testFile)
	if res1.NodesAdded != res2.NodesAdded || res1.EdgesAdded != res2.EdgesAdded {
		t.Errorf("idempotence failed")
	}
	t.Logf("idempotent OK")
}

func TestIngestDeterministic(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package main
func Foo(x int) int { return x + 1 }
type Bar struct {}
`
	os.WriteFile(testFile, []byte(src), 0644)
	db1, _ := storage.Open(":memory:")
	defer db1.Close()
	db2, _ := storage.Open(":memory:")
	defer db2.Close()
	IngestFile(db1, testFile)
	IngestFile(db2, testFile)
	nodes1, edges1 := getNodes(t, db1), getEdges(t, db1)
	nodes2, edges2 := getNodes(t, db2), getEdges(t, db2)
	if len(nodes1) != len(nodes2) {
		t.Errorf("nodes count mismatch")
	}
	if len(edges1) != len(edges2) {
		t.Errorf("edges count mismatch")
	}
	for i := range nodes1 {
		if nodes1[i] != nodes2[i] {
			t.Errorf("node mismatch")
		}
	}
	for i := range edges1 {
		if edges1[i] != edges2[i] {
			t.Errorf("edge mismatch")
		}
	}
	t.Logf("deterministic OK")
}

func TestSignatureHashStable(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package main
func Add(a int, b int) int { return a + b }
`
	os.WriteFile(testFile, []byte(src), 0644)
	db1, _ := storage.Open(":memory:")
	defer db1.Close()
	db2, _ := storage.Open(":memory:")
	defer db2.Close()
	IngestFile(db1, testFile)
	IngestFile(db2, testFile)
	hashes1 := getHashes(t, db1)
	hashes2 := getHashes(t, db2)
	for id, h1 := range hashes1 {
		if h2, ok := hashes2[id]; !ok || h1 != h2 {
			t.Errorf("hash mismatch for %s", id)
		}
	}
	t.Logf("signature hash stable OK")
}

func TestHashChangesWhenSignatureChanges(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "a.go")
	file2 := filepath.Join(tmpDir, "b.go")
	os.WriteFile(file1, []byte(`package main
func Add(a int, b int) int { return a + b }
`), 0644)
	os.WriteFile(file2, []byte(`package main
func Add(a string, b string) string { return a + b }
`), 0644)

	db1, _ := storage.Open(":memory:")
	defer db1.Close()
	db2, _ := storage.Open(":memory:")
	defer db2.Close()
	IngestFile(db1, file1)
	IngestFile(db2, file2)
	h1 := getHashes(t, db1)["main.Add"]
	h2 := getHashes(t, db2)["main.Add"]
	if h1 == h2 {
		t.Errorf("hash should differ for different signatures")
	}
	t.Logf("hash changes OK")
}

func TestClearFileNodes(t *testing.T) {
	db, _ := storage.Open(":memory:")
	defer db.Close()
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package main
func X() {}
`
	os.WriteFile(testFile, []byte(src), 0644)
	res1, _ := IngestFile(db, testFile)
	db.ClearFileNodes(testFile)
	if countNodes(t, db, testFile) != 0 {
		t.Errorf("nodes not cleared")
	}
	res2, _ := IngestFile(db, testFile)
	if res1.NodesAdded != res2.NodesAdded || res1.EdgesAdded != res2.EdgesAdded {
		t.Errorf("re-insert mismatch")
	}
	t.Logf("clear file nodes OK")
}

func TestLocalScopeOnly(t *testing.T) {
	db, _ := storage.Open(":memory:")
	defer db.Close()
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package main
import "fmt"
func A() { fmt.Println("hello"); B() }
func B() {}
`
	os.WriteFile(testFile, []byte(src), 0644)
	IngestFile(db, testFile)
	calls := getCalls(t, db)
	foundB := false
	foundExternal := false
	for _, c := range calls {
		if c == "main.B" {
			foundB = true
		}
		if c == "main.Println" {
			foundExternal = true
		}
	}
	if !foundB {
		t.Error("missing CALLS to local function B")
	}
	if foundExternal {
		t.Error("external call to Println should be ignored")
	}
	t.Logf("local scope only OK")
}

func TestMethodExtraction(t *testing.T) {
	db, _ := storage.Open(":memory:")
	defer db.Close()
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	src := `package main
type Server struct {}
func (s *Server) Start() {}
`
	os.WriteFile(testFile, []byte(src), 0644)
	IngestFile(db, testFile)
	nodes := getNodes(t, db)
	found := false
	for _, n := range nodes {
		if n == "main.*Server.Start" {
			found = true
		}
	}
	if !found {
		t.Error("method not extracted")
	}
	t.Logf("method extraction OK")
}

// helpers
func getNodes(t *testing.T, db *storage.DB) []string {
	t.Helper()
	rows, err := db.Query("SELECT id FROM nodes ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var list []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		list = append(list, id)
	}
	return list
}

func getEdges(t *testing.T, db *storage.DB) []string {
	t.Helper()
	rows, err := db.Query("SELECT source, target, relation FROM edges ORDER BY source, target, relation")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var list []string
	for rows.Next() {
		var s, t, r string
		rows.Scan(&s, &t, &r)
		list = append(list, s+"|"+t+"|"+r)
	}
	return list
}

func getHashes(t *testing.T, db *storage.DB) map[string]string {
	t.Helper()
	rows, err := db.Query("SELECT id, signature_hash FROM nodes ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	m := make(map[string]string)
	for rows.Next() {
		var id, h string
		rows.Scan(&id, &h)
		m[id] = h
	}
	return m
}

func countNodes(t *testing.T, db *storage.DB, filePath string) int {
	t.Helper()
	var c int
	err := db.QueryRow("SELECT COUNT(*) FROM nodes WHERE file_path=?", filePath).Scan(&c)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func getCalls(t *testing.T, db *storage.DB) []string {
	t.Helper()
	rows, err := db.Query("SELECT target FROM edges WHERE relation='CALLS' ORDER BY target")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var res []string
	for rows.Next() {
		var targ string
		rows.Scan(&targ)
		res = append(res, targ)
	}
	return res
}
