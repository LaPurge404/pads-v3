package autonomous

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"pads-v3/internal/agent"
	"pads-v3/internal/policy/evolution"
)

// TestGenerateIDNonEmpty verifies that generateID returns a non-empty string.
func TestGenerateIDNonEmpty(t *testing.T) {
	id := generateID()
	if id == "" {
		t.Error("generateID() returned empty string, want non-empty")
	}
}

// TestGenerateIDHexFormat verifies that generateID returns a valid hexadecimal string.
func TestGenerateIDHexFormat(t *testing.T) {
	id := generateID()
	// 8 bytes → 16 hex characters
	if len(id) != 16 {
		t.Errorf("generateID() length = %d, want 16", len(id))
	}
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("generateID() contains non-hex char %q", c)
			break
		}
	}
}

// TestGenerateIDUniqueness is a probabilistic check that IDs are not all identical.
func TestGenerateIDUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateID()
		if ids[id] {
			t.Logf("generateID() produced duplicate ID: %s (may indicate non-randomness)", id)
		}
		ids[id] = true
	}
	if len(ids) < 90 {
		t.Errorf("generateID() only produced %d unique IDs out of 100 calls", len(ids))
	}
}

// TestApplyAndCommitNoFiles verifies that applyAndCommit returns an error when there are no files to commit.
func TestApplyAndCommitNoFiles(t *testing.T) {
	m := New()
	tmpDir := t.TempDir()

	// Initialize a real git repo so git commands don't fail for other reasons.
	if err := initGitRepo(tmpDir); err != nil {
		t.Fatalf("initGitRepo: %v", err)
	}

	plan := agent.Plan{Steps: []agent.Action{}}
	committed, _, err := m.applyAndCommit(plan, tmpDir)
	if err == nil {
		t.Error("applyAndCommit with no files: expected error, got nil")
	}
	if committed {
		t.Error("applyAndCommit with no files: committed = true, want false")
	}
}

// TestApplyAndCommitFileNotFound verifies that applyAndCommit returns an error when target file does not exist.
func TestApplyAndCommitFileNotFound(t *testing.T) {
	m := New()
	tmpDir := t.TempDir()

	if err := initGitRepo(tmpDir); err != nil {
		t.Fatalf("initGitRepo: %v", err)
	}

	plan := agent.Plan{
		Steps: []agent.Action{
			{Kind: agent.ActionWriteFile, Target: filepath.Join(tmpDir, "nonexistent.go")},
		},
	}
	committed, _, err := m.applyAndCommit(plan, tmpDir)
	if err == nil {
		t.Error("applyAndCommit with nonexistent file: expected error, got nil")
	}
	if committed {
		t.Error("applyAndCommit with nonexistent file: committed = true, want false")
	}
}

// initGitRepo creates a minimal git repository in dir with a valid user config.
func initGitRepo(dir string) error {
	for _, cmd := range []struct {
		name string
		args []string
	}{
		{"git", []string{"init"}},
		{"git", []string{"config", "user.email", "test@test.com"}},
		{"git", []string{"config", "user.name", "test"}},
	} {
		c := exec.Command(cmd.name, cmd.args...)
		c.Dir = dir
		if err := c.Run(); err != nil {
			return fmt.Errorf("%s: %w", cmd.name, err)
		}
	}
	return nil
}

// TestApplyAndCommitSuccess verifies applyAndCommit succeeds when files exist.
func TestApplyAndCommitSuccess(t *testing.T) {
	m := New()
	tmpDir := t.TempDir()

	if err := initGitRepo(tmpDir); err != nil {
		t.Fatalf("initGitRepo: %v", err)
	}

	// Create a real file and stage it.
	filePath := filepath.Join(tmpDir, "real.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	plan := agent.Plan{
		Steps: []agent.Action{
			{Kind: agent.ActionWriteFile, Target: filePath},
		},
	}
	committed, hash, err := m.applyAndCommit(plan, tmpDir)
	if err != nil {
		t.Errorf("applyAndCommit with existing file: unexpected error: %v", err)
	}
	if !committed {
		t.Error("applyAndCommit with existing file: committed = false, want true")
	}
	if hash == "" {
		t.Error("applyAndCommit with existing file: hash is empty, want non-empty")
	}
}

// TestRunCycleDisabled is in the existing test file.

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