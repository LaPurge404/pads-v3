package autonomous

import (
	"os"
	"path/filepath"
	"testing"

	"pads-v3/internal/agent"
	"pads-v3/internal/policy/evolution"
)

func TestModeToggle(t *testing.T) {
	m := New()
	if m.IsEnabled() {
		t.Error("new mode should be disabled by default")
	}

	m.Enable()
	if !m.IsEnabled() {
		t.Error("Enable() should activate mode")
	}

	m.Disable()
	if m.IsEnabled() {
		t.Error("Disable() should deactivate mode")
	}

	// Toggle
	enabled := m.Toggle()
	if !enabled {
		t.Error("first Toggle should enable")
	}
	enabled = m.Toggle()
	if enabled {
		t.Error("second Toggle should disable")
	}
}

func TestRunCycleDisabled(t *testing.T) {
	m := New()
	// Mode is disabled by default.

	codeAgent := agent.NewCodeAgent(agent.NewDefaultLLMClient())
	tmpDir := t.TempDir()

	// Create minimal project structure.
	pkgDir := filepath.Join(tmpDir, "mypkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "file.go"), []byte("package mypkg\n"), 0644); err != nil {
		t.Fatal(err)
	}

	sandboxExec := agent.NewSandboxExecutor(tmpDir, true)
	loop := evolution.NewSafeEvolutionLoopV3(
		evolution.NewOrchestrator(evolution.NewMultiCycleEvaluator(), evolution.NewStabilityGate()),
		evolution.NewEventStore(filepath.Join(tmpDir, "evolution.log")),
		evolution.NewWAL(filepath.Join(tmpDir, "evolution.wal")),
		evolution.NewAntiCollapseDetector(5, 10.0),
		evolution.ModeStable, evolution.NewUCBSelector(42),
	)
	agentLoop := evolution.NewAgentLoop(loop, evolution.NewUCBSelector(42), evolution.DeltaRewarder{})

	task := agent.Task{
		Kind:   agent.TaskFixBroken,
		Target: filepath.Join(tmpDir, "mypkg", "file.go"),
		Goal:   "Add a function",
	}

	result := m.RunCycle(task, codeAgent, sandboxExec, agentLoop, tmpDir, 0.0, nil)

	if result.Error == "" {
		t.Error("RunCycle should return an error when mode is disabled")
	}
}

func TestModeCycleNum(t *testing.T) {
	m := New()
	if m.CycleNum() != 0 {
		t.Errorf("initial cycle num should be 0, got %d", m.CycleNum())
	}

	m.Enable()
	// Call RunCycle with disabled loop to increment counter.
	codeAgent := agent.NewCodeAgent(agent.NewDefaultLLMClient())
	tmpDir := t.TempDir()

	sandboxExec := agent.NewSandboxExecutor(tmpDir, false)
	loop := evolution.NewSafeEvolutionLoopV3(
		evolution.NewOrchestrator(evolution.NewMultiCycleEvaluator(), evolution.NewStabilityGate()),
		evolution.NewEventStore(filepath.Join(tmpDir, "evolution.log")),
		evolution.NewWAL(filepath.Join(tmpDir, "evolution.wal")),
		evolution.NewAntiCollapseDetector(5, 10.0),
		evolution.ModeStable, evolution.NewUCBSelector(42),
	)
	agentLoop := evolution.NewAgentLoop(loop, evolution.NewUCBSelector(42), evolution.DeltaRewarder{})

	task := agent.Task{
		Kind:   agent.TaskFixBroken,
		Target: filepath.Join(tmpDir, "mypkg", "file.go"),
		Goal:   "Add a function",
	}

	// Run 3 cycles (they will fail at Solve due to low confidence, but counter increments).
	for i := 0; i < 3; i++ {
		m.RunCycle(task, codeAgent, sandboxExec, agentLoop, tmpDir, 0.0, nil)
	}

	if m.CycleNum() != 3 {
		t.Errorf("cycle num should be 3, got %d", m.CycleNum())
	}
}

func TestComputeSandboxScore(t *testing.T) {
	cases := []struct {
		name  string
		res   agent.SandboxResult
		minScore int
	}{
		{
			name: "passed no warnings",
			res: agent.SandboxResult{Passed: true, BuildOutput: "ok", TestOutput: ""},
			minScore: 100, // 30 (build) + 50 (pass) + 20 (no warnings)
		},
		{
			name: "passed with warnings",
			res: agent.SandboxResult{Passed: true, BuildOutput: "warning", TestOutput: "warning"},
			minScore: 80, // 30 + 50, no warning bonus
		},
		{
			name: "build error",
			res: agent.SandboxResult{BuildOutput: "error", TestOutput: ""},
			minScore: 0, // no build bonus, no pass bonus
		},
		{
			name: "partial tests",
			res: agent.SandboxResult{Passed: false, TestsPassed: 3, TestsFailed: 1, BuildOutput: "ok", TestOutput: "ok"},
			minScore: 67, // 30 + 37 (50*3/4) + 0 (warnings)
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			score := agent.ComputeSandboxScore(tc.res)
			if score < tc.minScore {
				t.Errorf("score %d < min %d", score, tc.minScore)
			}
		})
	}
}