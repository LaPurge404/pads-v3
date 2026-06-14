package integration

import (
    "os"
    "path/filepath"
    "strings"
    "testing"

    "pads-v3/internal/agent"
    "pads-v3/internal/compiler"
    "pads-v3/internal/engine"
    "pads-v3/internal/executor"
    "pads-v3/internal/storage"
)

// repairAgent is a test agent that knows how to fix a broken Add function.
type repairAgent struct{}

func (r repairAgent) Solve(task agent.Task, ctx agent.Context) (agent.Plan, error) {
    return agent.Plan{
        Steps: []agent.Action{
            {
                Kind:   agent.ActionWriteFile,
                Target: task.Target,
                Patch:  "package main\nfunc Add(a, b int) int { return a + b }\n",
            },
        },
    }, nil
}

// TestFullConvergenceLoop validates the closed-loop self-healing property:
// BROKEN → Agent → Executor → Engine → STABLE
func TestFullConvergenceLoop(t *testing.T) {
    tmpDir := t.TempDir()

    goMod := filepath.Join(tmpDir, "go.mod")
    if err := os.WriteFile(goMod, []byte("module testmod\n\ngo 1.24\n"), 0644); err != nil {
        t.Fatal(err)
    }

    dbPath := filepath.Join(tmpDir, "pads.db")
    db, err := storage.Open(dbPath)
    if err != nil {
        t.Fatal(err)
    }
    defer db.Close()

    brokenFile := filepath.Join(tmpDir, "broken.go")
    if err := os.WriteFile(brokenFile, []byte(`package main
func Add(a, b int) int { return a - b }
`), 0644); err != nil {
        t.Fatal(err)
    }

    brokenTest := filepath.Join(tmpDir, "broken_test.go")
    if err := os.WriteFile(brokenTest, []byte(`package main
import "testing"
func TestAdd(t *testing.T) {
    if Add(2, 3) != 5 {
        t.Error("wrong")
    }
}
`), 0644); err != nil {
        t.Fatal(err)
    }

    _, err = compiler.IngestFile(db, brokenFile)
    if err != nil {
        t.Fatal(err)
    }
    _, err = compiler.IngestFile(db, brokenTest)
    if err != nil {
        t.Fatal(err)
    }

    t.Log("Phase 1: Detecting broken state...")
    if err := engine.RunOnce(db); err != nil {
        t.Fatal(err)
    }

    tasks, err := agent.BuildTasks(db)
    if err != nil {
        t.Fatal(err)
    }
    if len(tasks) == 0 {
        t.Fatal("expected at least one BROKEN task")
    }
    t.Logf("Phase 1 complete: %d BROKEN tasks found", len(tasks))

    t.Log("Phase 2: Agent creating plan...")
    repair := repairAgent{}
    ctx, err := agent.BuildContext(db, tasks[0])
    if err != nil {
        t.Fatal(err)
    }
    plan, err := repair.Solve(tasks[0], ctx)
    if err != nil {
        t.Fatal(err)
    }
    t.Logf("Phase 2 complete: plan has %d steps", len(plan.Steps))

    t.Log("Phase 3: Executor applying fix...")
    exec := executor.New(db, false)
    if err := exec.Execute(plan); err != nil {
        t.Fatal(err)
    }

    content, err := os.ReadFile(brokenFile)
    if err != nil {
        t.Fatal(err)
    }
    if !strings.Contains(string(content), "return a + b") {
        t.Fatal("file was not actually fixed by Executor")
    }
    t.Log("Phase 3 complete: file verified on disk")

    // L'Engine doit maintenant détecter le changement de fichier et revalider automatiquement
    t.Log("Phase 4: Re-running engine for convergence...")
    if err := engine.RunOnce(db); err != nil {
        t.Fatal(err)
    }

    var brokenCount, stableCount int
    if err := db.QueryRow(`SELECT COUNT(*) FROM graph_state WHERE state = 'BROKEN'`).Scan(&brokenCount); err != nil {
        t.Fatal(err)
    }
    if err := db.QueryRow(`SELECT COUNT(*) FROM graph_state WHERE state = 'STABLE'`).Scan(&stableCount); err != nil {
        t.Fatal(err)
    }

    if brokenCount != 0 {
        t.Errorf("expected 0 BROKEN nodes after fix, got %d", brokenCount)
    }
    if stableCount == 0 {
        t.Fatal("expected at least one STABLE node after fix")
    }

    t.Logf("=== CONVERGENCE TRACE ===")
    t.Logf("BROKEN nodes: %d", brokenCount)
    t.Logf("STABLE nodes: %d", stableCount)
    t.Log("Full convergence loop validated: BROKEN → Agent → Executor → Engine → STABLE")
}
