package agent

import (
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
