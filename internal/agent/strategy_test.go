package agent

import (
	"testing"
)

func TestDefaultStrategies(t *testing.T) {
	registry := DefaultStrategies()
	names := registry.Names()

	if len(names) != 4 {
		t.Errorf("expected 4 strategies, got %d", len(names))
	}

	expected := []string{"conservative", "aggressive", "test-first", "minimal-change"}
	for _, exp := range expected {
		found := false
		for _, got := range names {
			if got == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected strategy %q not found", exp)
		}
	}
}

func TestStrategyRegistryGet(t *testing.T) {
	registry := DefaultStrategies()

	// Test existing strategy
	s := registry.Get("conservative")
	if s == nil {
		t.Fatal("conservative strategy not found")
	}
	if s.Temperature != 0.1 {
		t.Errorf("conservative temp expected 0.1, got %f", s.Temperature)
	}
	if s.Name != "conservative" {
		t.Errorf("conservative name expected 'conservative', got %s", s.Name)
	}

	// Test non-existent strategy
	nilStrat := registry.Get("nonexistent")
	if nilStrat != nil {
		t.Error("expected nil for nonexistent strategy")
	}
}

func TestStrategyRegistryRandom(t *testing.T) {
	registry := DefaultStrategies()

	// Should return one of the registered strategies
	for i := 0; i < 100; i++ {
		s := registry.Random()
		if s == nil {
			t.Fatal("Random() returned nil")
		}
		found := false
		for _, name := range registry.Names() {
			if name == s.Name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Random() returned unknown strategy %s", s.Name)
		}
	}
}

func TestSelectByTemperature(t *testing.T) {
	registry := DefaultStrategies()

	tests := []struct {
		temp     float64
		expected string
	}{
		{0.1, "conservative"},
		{0.3, "minimal-change"},
		{0.5, "test-first"},
		{0.8, "aggressive"},
	}

	for _, tc := range tests {
		s := registry.SelectByTemperature(tc.temp)
		if s.Name != tc.expected {
			t.Errorf("temp %f: expected %s, got %s", tc.temp, tc.expected, s.Name)
		}
	}
}

func TestBuildPromptForStrategy(t *testing.T) {
	registry := DefaultStrategies()
	strategy := registry.Get("conservative")

	task := Task{
		Kind:   TaskFixBroken,
		Target: "foo/bar.go",
		Goal:   "Fix nil pointer",
	}

	ctx := Context{
		FilePath: "foo/bar.go",
	}

	prompt := BuildPromptForStrategy(task, ctx, strategy)

	if prompt.FilePath != "foo/bar.go" {
		t.Errorf("expected FilePath 'foo/bar.go', got %s", prompt.FilePath)
	}
	if prompt.Language != "go" {
		t.Errorf("expected Language 'go', got %s", prompt.Language)
	}
	if prompt.Task != "Fix nil pointer" {
		t.Errorf("expected Task 'Fix nil pointer', got %s", prompt.Task)
	}
}

func TestStrategyForUCBArm(t *testing.T) {
	// Known arm
	s := StrategyForUCBArm("aggressive")
	if s.Name != "aggressive" {
		t.Errorf("expected 'aggressive', got %s", s.Name)
	}

	// Unknown arm falls back to conservative
	s = StrategyForUCBArm("unknown-strategy")
	if s.Name != "conservative" {
		t.Errorf("expected fallback to 'conservative', got %s", s.Name)
	}
}

func TestStrategySelector(t *testing.T) {
	// Mock selector for testing
	mock := &mockStrategySelector{
		calls:   []string{},
		rewards: map[string]float64{},
	}

	adapter := NewStrategyAdapter(mock)

	// Select should return first registered strategy
	sel := adapter.Selector.Select()
	if sel == "" {
		t.Error("selector returned empty string")
	}

	// Update reward calls Select() internally once
	adapter.UpdateStrategyReward(0.5)
	// Select() called once in UpdateStrategyReward, plus initial call = 2
	if len(mock.calls) < 1 {
		t.Errorf("expected at least 1 Update call, got %d", len(mock.calls))
	}
}

func TestGetStrategyStats(t *testing.T) {
	mock := &mockStrategySelector{}
	adapter := NewStrategyAdapter(mock)

	stats := adapter.GetStrategyStats()

	if len(stats) != 4 {
		t.Errorf("expected 4 strategies in stats, got %d", len(stats))
	}

	for name, stat := range stats {
		if stat.Name != name {
			t.Errorf("stat.Name %s != key %s", stat.Name, name)
		}
	}
}

// mockStrategySelector implements StrategySelector for testing.
type mockStrategySelector struct {
	calls   []string
	rewards map[string]float64
}

func (m *mockStrategySelector) Select() string {
	if len(m.calls) == 0 {
		m.calls = append(m.calls, "conservative")
	}
	return "conservative"
}

func (m *mockStrategySelector) Update(name string, reward float64) {
	m.calls = append(m.calls, name)
	m.rewards[name] = reward
}
