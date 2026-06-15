package executor

import (
"os"
"path/filepath"
"testing"

"pads-v3/internal/agent"
"pads-v3/internal/storage"
)

func TestExecutorWriteFileDryRun(t *testing.T) {
tmpDir := t.TempDir()
dbPath := filepath.Join(tmpDir, "pads.db")
db, err := storage.Open(dbPath)
if err != nil {
t.Fatal(err)
}
defer db.Close()

e := New(db, true)
plan := agent.Plan{
Steps: []agent.Action{
{Kind: agent.ActionWriteFile, Target: "/nonexistent/test.go", Patch: "package main"},
},
}
if err := e.Execute(plan); err != nil {
t.Fatal(err)
}
}

func TestExecutorWriteFileReal(t *testing.T) {
tmpDir := t.TempDir()
dbPath := filepath.Join(tmpDir, "pads.db")
db, err := storage.Open(dbPath)
if err != nil {
t.Fatal(err)
}
defer db.Close()

testFile := filepath.Join(tmpDir, "test.go")
e := New(db, false)
plan := agent.Plan{
Steps: []agent.Action{
{Kind: agent.ActionWriteFile, Target: testFile, Patch: "package main"},
},
}
if err := e.Execute(plan); err != nil {
t.Fatal(err)
}
content, err := os.ReadFile(testFile)
if err != nil {
t.Fatal(err)
}
if string(content) != "package main" {
t.Errorf("expected 'package main', got '%s'", string(content))
}
}
