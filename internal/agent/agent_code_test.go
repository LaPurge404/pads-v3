package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── Enrich ─────────────────────────────────────────────────────────────────

func TestEnrich(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sample.go")
	if err := os.WriteFile(path, []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := Context{FilePath: path}
	ctx.Enrich(path, 200)
	if ctx.SourceContent == "" {
		t.Error("Enrich should have populated SourceContent from file")
	}
	if !strings.Contains(ctx.SourceContent, "package main") {
		t.Errorf("SourceContent should contain 'package main', got %q", ctx.SourceContent)
	}
}

func TestEnrich_LineLimit(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sample.go")
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = "// line"
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := Context{}
	ctx.Enrich(path, 10)
	gotLines := countLines(ctx.SourceContent)
	if gotLines != 10 {
		t.Errorf("Enrich limit: expected 10 lines, got %d", gotLines)
	}
}

func TestEnrich_SkipIfPrePopulated(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sample.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := Context{SourceContent: "already set"}
	ctx.Enrich(path, 200)
	if ctx.SourceContent != "already set" {
		t.Errorf("Enrich must not overwrite existing SourceContent, got %q", ctx.SourceContent)
	}
}

func TestEnrich_NonExistentFile(t *testing.T) {
	ctx := Context{}
	ctx.Enrich("/does/not/exist.go", 200)
	// Must not panic — silently skip on error.
	if ctx.SourceContent != "" {
		t.Errorf("expected empty SourceContent for non-existent file, got %q", ctx.SourceContent)
	}
}

func TestEnrich_SemMemNil(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sample.go")
	if err := os.WriteFile(path, []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := Context{SemMem: nil}
	ctx.Enrich(path, 200)
	if ctx.SourceContent == "" {
		t.Error("Enrich should have read file even when SemMem is nil")
	}
}

// ─── readFileLines ───────────────────────────────────────────────────────────

// countLines counts lines in s (handles both trailing newline and no-trailing-newline cases).
func countLines(s string) int {
	n := strings.Count(s, "\n")
	if len(s) > 0 && s[len(s)-1] != '\n' {
		n++
	}
	return n
}

func TestReadFileLines(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "testfile.go")
	content := "// line 1\n// line 2\n// line 3\n// line 4\n// line 5\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		n         int
		wantLines int
	}{
		{3, 3},
		{10, 5}, // file only has 5 lines
		{0, 0},
		{5, 5},
	}

	for _, tc := range tests {
		got, err := readFileLines(path, tc.n)
		if err != nil {
			t.Errorf("readFileLines(%q, %d) unexpected error: %v", path, tc.n, err)
			continue
		}
		gotLines := countLines(got)
		if gotLines != tc.wantLines {
			t.Errorf("readFileLines(%q, %d) returned %d lines, want %d", path, tc.n, gotLines, tc.wantLines)
		}
	}
}

func TestReadFileLines_NonExistent(t *testing.T) {
	_, err := readFileLines("/does/not/exist.go", 10)
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestReadFileLines_ExactLines(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "three.go")
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := readFileLines(path, 5)
	if err != nil {
		t.Fatal(err)
	}
	if got != "a\nb\nc\n" {
		t.Errorf("readFileLines returned %q, want 'a\\nb\\nc\\n'", got)
	}
}

// ─── buildPrompt with SourceContent ─────────────────────────────────────────

func TestBuildPrompt_WithSourceContent(t *testing.T) {
	agent := NewCodeAgent(&mockLLMClient{conf: 0.8})
	task := Task{Kind: TaskFixBroken, Target: "foo/bar.go", Goal: "Fix nil pointer"}
	ctx := Context{
		FilePath:      "foo/bar.go",
		SourceContent: "package foo\nfunc Bar() {}\n",
	}

	prompt := agent.buildPrompt(task, ctx)
	if !strings.Contains(prompt.Context, "--- Target source ---") {
		t.Error("prompt.Context should contain source start marker")
	}
	if !strings.Contains(prompt.Context, "package foo") {
		t.Error("prompt.Context should contain source content")
	}
	if !strings.Contains(prompt.Context, "--- End source ---") {
		t.Error("prompt.Context should contain source end marker")
	}
}

func TestBuildPrompt_WithoutSourceContent(t *testing.T) {
	agent := NewCodeAgent(&mockLLMClient{conf: 0.8})
	task := Task{Kind: TaskFixBroken, Target: "foo/bar.go", Goal: "Fix nil pointer"}
	ctx := Context{FilePath: "foo/bar.go"}

	prompt := agent.buildPrompt(task, ctx)
	if strings.Contains(prompt.Context, "--- Target source ---") {
		t.Error("prompt.Context should NOT contain source markers when SourceContent is empty")
	}
}

func TestBuildPrompt_Language(t *testing.T) {
	agent := NewCodeAgent(&mockLLMClient{conf: 0.8})
	for _, tc := range []struct {
		target  string
		wanLang string
	}{
		{"foo.go", "go"},
		{"bar.py", "python"},
		{"baz.js", "javascript"},
		{"qux.ts", "typescript"},
	} {
		prompt := agent.buildPrompt(Task{Target: tc.target, Kind: TaskFixBroken, Goal: "fix"}, Context{})
		if prompt.Language != tc.wanLang {
			t.Errorf("detectLanguage(%s) = %q, want %q", tc.target, prompt.Language, tc.wanLang)
		}
	}
}

// ─── BuildPromptForStrategy with SourceContent ─────────────────────────────

func TestBuildPromptForStrategy_WithSource(t *testing.T) {
	registry := DefaultStrategies()
	strategy := registry.Get("aggressive")

	task := Task{Kind: TaskFixBroken, Target: "foo/bar.go", Goal: "Fix nil pointer"}
	ctx := Context{
		FilePath:      "foo/bar.go",
		SourceContent: "package foo\nfunc Bar() {}\n",
	}

	prompt := BuildPromptForStrategy(task, ctx, strategy)
	if !strings.Contains(prompt.Context, "--- Target source ---") {
		t.Error("prompt.Context should contain source start marker")
	}
	if !strings.Contains(prompt.Context, "package foo") {
		t.Error("prompt.Context should contain source content")
	}
}

func TestBuildPromptForStrategy_StrategyDescription(t *testing.T) {
	registry := DefaultStrategies()

	for _, name := range []string{"conservative", "aggressive", "test-first", "minimal-change"} {
		strategy := registry.Get(name)
		prompt := BuildPromptForStrategy(
			Task{Kind: TaskFixBroken, Target: "x.go", Goal: "fix"},
			Context{},
			strategy,
		)
		if !strings.Contains(prompt.Constraints, strategy.Description) {
			t.Errorf("BuildPromptForStrategy constraints should contain description for %s", name)
		}
	}
}

// ─── Solve ───────────────────────────────────────────────────────────────────

func TestSolve_ConfidenceTooLow(t *testing.T) {
	// mockLLMClient returns conf=0.5, below the 0.6 threshold.
	agent := NewCodeAgent(&mockLLMClient{conf: 0.5})
	task := Task{Kind: TaskFixBroken, Target: "foo.go", Goal: "fix nil"}
	ctx := Context{FilePath: "foo.go"}
	_, err := agent.Solve(task, ctx)
	if err == nil {
		t.Error("expected error when confidence below threshold")
	}
}

func TestSolve_Success(t *testing.T) {
	agent := NewCodeAgent(&mockLLMClient{conf: 0.8})
	task := Task{Kind: TaskFixBroken, Target: "foo.go", Goal: "fix nil"}
	ctx := Context{FilePath: "foo.go"}
	plan, err := agent.Solve(task, ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(plan.Steps) == 0 {
		t.Error("plan should have steps")
	}
}

func TestSolve_WrongTaskKind(t *testing.T) {
	agent := NewCodeAgent(&mockLLMClient{conf: 0.8})
	task := Task{Kind: "unknown-kind", Target: "foo.go", Goal: "fix"}
	ctx := Context{FilePath: "foo.go"}
	_, err := agent.Solve(task, ctx)
	if err == nil {
		t.Error("expected error for unknown task kind")
	}
}

// ─── mockLLMClient ──────────────────────────────────────────────────────────

// mockLLMClient implements LLMClient for tests.
type mockLLMClient struct {
	conf  float64
	patch string
}

func (m *mockLLMClient) GenerateCode(ctx context.Context, prompt CodePrompt) (*CodeResponse, error) {
	p := "package foo\nfunc Bar() { return 42 }\n"
	if m.patch != "" {
		p = m.patch
	}
	return &CodeResponse{
		Patch:      p,
		Confidence: m.conf,
		Warnings:   nil,
	}, nil
}

func (m *mockLLMClient) Close() error { return nil }

// TestCircuitBreakerStateClosed verifies State() returns "closed" initially.
func TestCircuitBreakerStateClosed(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Hour)
	if state := cb.State(); state != "closed" {
		t.Errorf("new CircuitBreaker State() = %q, want %q", state, "closed")
	}
}

// TestCircuitBreakerStateOpen verifies State() returns "open" after max failures.
func TestCircuitBreakerStateOpen(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Hour)
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	if state := cb.State(); state != "open" {
		t.Errorf("after 3 failures State() = %q, want %q", state, "open")
	}
}

// TestCircuitBreakerAllowClosed verifies Allow returns nil when circuit is closed.
func TestCircuitBreakerAllowClosed(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Hour)
	if err := cb.Allow(); err != nil {
		t.Errorf("Allow() on closed circuit = %v, want nil", err)
	}
}

// TestCircuitBreakerAllowOpen verifies Allow returns ErrCircuitOpen when circuit is open.
func TestCircuitBreakerAllowOpen(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Hour)
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	// Recovery window is 1 hour, so should still be open.
	if err := cb.Allow(); err != ErrCircuitOpen {
		t.Errorf("Allow() on open circuit = %v, want %v", err, ErrCircuitOpen)
	}
}

// TestCircuitBreakerRecordSuccessResetsFailures verifies RecordSuccess in closed state resets failure counter.
func TestCircuitBreakerRecordSuccessResetsFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Hour)
	cb.RecordFailure()
	cb.RecordFailure() // 2 failures, not at max yet
	cb.RecordSuccess() // reset failures to 0
	cb.RecordFailure() // back to 1
	cb.RecordFailure() // back to 2
	// Not at 3, so circuit should still be closed
	if state := cb.State(); state != "closed" {
		t.Errorf("after 2 failures + success + 2 failures State() = %q, want %q", state, "closed")
	}
}

// TestCircuitBreakerRecordFailureHalfOpen verifies failure in half-open reopens circuit.
func TestCircuitBreakerRecordFailureHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(1, 0) // recovery window = 0, immediate transition
	// Open the circuit
	cb.RecordFailure()
	// Recovery window is 0, so Allow() will transition to half-open
	cb.Allow() // transitions to half-open
	if state := cb.State(); state != "half-open" {
		t.Fatalf("after Allow on open cb with 0 recovery State() = %q, want %q", state, "half-open")
	}
	// Now record failure in half-open
	cb.RecordFailure()
	if state := cb.State(); state != "open" {
		t.Errorf("after RecordFailure in half-open State() = %q, want %q", state, "open")
	}
}

// TestIsValidActionKind verifies IsValidActionKind returns correct booleans.
func TestIsValidActionKind(t *testing.T) {
	validKinds := []ActionKind{ActionLog, ActionWriteFile, ActionRunCommand}
	for _, k := range validKinds {
		if !IsValidActionKind(k) {
			t.Errorf("IsValidActionKind(%q) = false, want true", k)
		}
	}

	invalidKinds := []ActionKind{"invalid", "", "TOTALLY_WRONG", ActionKind("")}
	for _, k := range invalidKinds {
		if IsValidActionKind(k) {
			t.Errorf("IsValidActionKind(%q) = true, want false", k)
		}
	}
}