package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"pads-v3/internal/policy/evolution"
)

func TestAgentPoolCreation(t *testing.T) {
	pool := NewAgentPool(3, "/tmp/fake_project", nil)

	if pool.Len() != 3 {
		t.Errorf("expected pool size 3, got %d", pool.Len())
	}

	strategies := make(map[string]bool)
	for i := 0; i < pool.Len(); i++ {
		agent := pool.AgentAt(i)
		if agent.Strategy == "" {
			t.Errorf("agent %d has empty strategy", i)
		}
		if strategies[agent.Strategy] {
			t.Errorf("duplicate strategy %q", agent.Strategy)
		}
		strategies[agent.Strategy] = true
	}
}

func TestPoolStatsNoResults(t *testing.T) {
	pool := NewAgentPool(3, "/tmp/fake_project", nil)
	stats := pool.PoolStats()

	// Before any run, stats should exist for all agents.
	if len(stats) != 3 {
		t.Errorf("expected 3 stats entries, got %d", len(stats))
	}
}

func TestBestResultNilBeforeRun(t *testing.T) {
	pool := NewAgentPool(3, "/tmp/fake_project", nil)
	if pool.BestResult() != nil {
		t.Error("BestResult should be nil before RunAll")
	}
}

func TestPoolStatsAfterResults(t *testing.T) {
	pool := NewAgentPool(3, "/tmp/fake_project", nil)

	// Simulate results by directly setting LastResult on agents.
	pool.agents[0].mu.Lock()
	pool.agents[0].LastResult = &evolution.AgentResult{Score: 50, Accepted: true}
	pool.agents[0].mu.Unlock()

	pool.agents[1].mu.Lock()
	pool.agents[1].LastResult = &evolution.AgentResult{Score: 80, Accepted: true}
	pool.agents[1].mu.Unlock()

	pool.agents[2].mu.Lock()
	pool.agents[2].LastResult = &evolution.AgentResult{Score: 30, Accepted: false}
	pool.agents[2].mu.Unlock()

	best := pool.BestResult()
	if best == nil {
		t.Fatal("BestResult should not be nil")
	}
	if best.Score != 80 {
		t.Errorf("expected best score 80, got %d", best.Score)
	}
}

func TestBestResultTieBreak(t *testing.T) {
	pool := NewAgentPool(2, "/tmp/fake_project", nil)

	// Both agents have same score but different reason counts.
	pool.agents[0].mu.Lock()
	pool.agents[0].LastResult = &evolution.AgentResult{Score: 60, SemanticReasons: []string{"a", "b", "c"}}
	pool.agents[0].mu.Unlock()

	pool.agents[1].mu.Lock()
	pool.agents[1].LastResult = &evolution.AgentResult{Score: 60, SemanticReasons: []string{"a"}}
	pool.agents[1].mu.Unlock()

	best := pool.BestResult()
	if best == nil {
		t.Fatal("BestResult should not be nil")
	}
	// Tie-break: fewer reasons is better → agent 1 (index 1) should win.
	if len(best.SemanticReasons) != 1 {
		t.Errorf("expected 1 reason (tie-breaker), got %d", len(best.SemanticReasons))
	}
}

func TestPoolRunAllIntegration(t *testing.T) {
	// Create a temp project with a minimal Go file so sandbox can initialize.
	tmpDir := t.TempDir()

	// Write go.mod
	goMod := "module testproject\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	// Write a simple .go file
	mainGo := `package main

func Add(a, b int) int {
	return a + b
}

func main() {}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	// Create pool with 3 agents with different strategies.
	pool := NewAgentPool(3, tmpDir, nil)
	if pool.Len() != 3 {
		t.Fatalf("expected pool size 3, got %d", pool.Len())
	}

	// Verify all 3 strategies are distinct.
	strategies := make(map[string]bool)
	for i := 0; i < pool.Len(); i++ {
		strategy := pool.AgentAt(i).Strategy
		if strategy == "" {
			t.Errorf("agent %d has empty strategy", i)
		}
		if strategies[strategy] {
			t.Errorf("duplicate strategy %q", strategy)
		}
		strategies[strategy] = true
	}

	// Create a TaskFixBroken task.
	task := Task{
		Kind:   TaskFixBroken,
		Target: filepath.Join(tmpDir, "main.go"),
		Goal:   "Add a Multiply function that multiplies two ints",
	}
	ctx := Context{
		FilePath: filepath.Join(tmpDir, "main.go"),
	}

	// Run all agents.
	results, err := pool.RunAll(context.Background(), task, ctx)
	if err != nil {
		t.Fatalf("RunAll failed: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Verify all results have a strategy set.
	// Note: mock LLM returns confidence=0.5 < 0.6 threshold, so Solve fails and score=0.
	// This is expected behavior - BestResult still returns a valid result.
	for i, res := range results {
		if res == nil {
			t.Errorf("result %d is nil", i)
			continue
		}
		if res.UCBArm == "" {
			t.Errorf("result %d has empty UCBArm", i)
		}
		// Score may be 0 if Solve failed due to low confidence, or 50 if sandbox error.
	}

	// BestResult should return a valid result (even with score 0).
	best := pool.BestResult()
	if best == nil {
		t.Fatal("BestResult should not be nil after RunAll")
	}
	if best.UCBArm == "" {
		t.Error("BestResult has empty UCBArm")
	}

	// PoolStats should have 3 entries.
	stats := pool.PoolStats()
	if len(stats) != 3 {
		t.Errorf("expected 3 stats entries, got %d", len(stats))
	}

	// Each strategy should have a stat entry.
	for strategy := range strategies {
		if _, ok := stats[strategy]; !ok {
			t.Errorf("missing stats for strategy %q", strategy)
		}
	}
}
