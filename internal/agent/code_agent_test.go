package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"pads-v3/internal/storage"
)

// TestTestNameForFile verifies test name generation.
func TestTestNameForFile(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"add.go", "TestAdd"},
		{"handler.go", "TestHandler"},
		{"add_test.go", "TestAdd_test"}, // .go trim → _test reste
		{"my_file.go", "TestMy_file"},
		{"server.go", "TestServer"},
		{"utils.go", "TestUtils"},
	}
	for _, tt := range tests {
		got := testNameForFile(tt.input)
		if got != tt.want {
			t.Errorf("testNameForFile(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestTestNameForFileEmpty verifies the empty name case.
func TestTestNameForFileEmpty(t *testing.T) {
	got := testNameForFile("")
	if got != "Test" {
		t.Errorf("testNameForFile(%q) = %q, want %q", "", got, "Test")
	}
}

// TestDetectLanguage verifies language detection.
func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"foo.go", "go"},
		{"bar.py", "python"},
		{"baz.js", "javascript"},
		{"qux.ts", "typescript"},
	}
	for _, tt := range tests {
		got := detectLanguage(tt.path)
		if got != tt.want {
			t.Errorf("detectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// TestStripDiffMarkers verifies diff marker cleaning.
func TestStripDiffMarkers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Basic case
		{"simple text", "package main", "package main"},

		// Standard full diff
		{
			"full diff",
			"diff --git a/foo.go b/foo.go\n" +
				"index 123..456 789\n" +
				"--- a/foo.go\n" +
				"+++ b/foo.go\n" +
				"@@ -1,3 +1,4 @@\n" +
				" package main\n" +
				"-import \"fmt\"\n" +
				"+import \"fmt\"\n" +
				"+import \"strings\"\n" +
				" func main() {}",
			" package main\n" +
				"+import \"fmt\"\n" +
				"+import \"strings\"\n" +
				" func main() {}",
		},

		// Empty file (new file)
		{
			"empty file diff",
			"diff --git a/new.go b/new.go\n" +
				"new file mode 100644\n" +
				"--- /dev/null\n" +
				"+++ b/new.go\n" +
				"@@ -0,0 +1,2 @@\n" +
				"+package new\n" +
				"+func New() {}",
			"+package new\n" +
				"+func New() {}",
		},

		// Binary patch (no usable content)
		{
			"binary diff",
			"diff --git a/logo.png b/logo.png\n" +
				"Binary files a/logo.png and b/logo.png differ",
			"",
		},

		// Unicode content
		{
			"unicode content",
			"--- a/emoji.go\n" +
				"+++ b/emoji.go\n" +
				"@@ -1 +1 @@\n" +
				"-const smiley = \"😀\"\n" +
				"+const smiley = \"😀🎉👋\"\n",
			"+const smiley = \"😀🎉👋\"\n",
		},

		// Filenames with + (e.g. c++, go++ test_name)
		{
			"filename with plus",
			"diff --git a/test++.go b/test++.go\n" +
				"--- a/test++.go\n" +
				"+++ b/test++.go\n" +
				"@@ -1,2 +1,3 @@\n" +
				" package main\n" +
				"+// line with + sign\n" +
				" func main() {}",
			" package main\n" +
				"+// line with + sign\n" +
				" func main() {}",
		},

		// Multiple hunks
		{
			"multiple hunks",
			"--- a/main.go\n" +
				"+++ b/main.go\n" +
				"@@ -1,3 +1,4 @@\n" +
				" package main\n" +
				"+import \"fmt\"\n" +
				" func main() {\n" +
				"-	fmt.Println(\"old\")\n" +
				"+	fmt.Println(\"new\")\n" +
				" }\n" +
				"@@ -10,3 +11,4 @@\n" +
				" func helper() {}\n" +
				"+// comment\n" +
				"+",
			" package main\n" +
				"+import \"fmt\"\n" +
				" func main() {\n" +
				"+	fmt.Println(\"new\")\n" +
				" }\n" +
				" func helper() {}\n" +
				"+// comment\n" +
				"+",
		},

		// Empty lines in diff
		{
			"empty lines in diff",
			"--- a/main.go\n" +
				"+++ b/main.go\n" +
				"@@ -1,2 +1,3 @@\n" +
				" package main\n" +
				"+\n" +
				"+\n",
			" package main\n" +
				"+\n" +
				"+\n",
		},

		// Old approach: removal lines "-" (case without header)
		// Without a diff header, the content is not treated as a diff,
		// so - and + lines are passed through as-is.
		{
			"old style deletion",
			"- removed line\n+ added line\n unchanged",
			"- removed line\n+ added line\n unchanged",
		},

		// Addition line in a diff (without @@ → no hunk)
		// The "+ new line" after the header is seen as a normal line,
		// so we exit in StateTextHunk and copy it.
		{
			"simple addition no hunk",
			"--- a/foo.go\n" +
				"+++ b/foo.go\n" +
				"+ new line",
			"+ new line",
		},

		// Addition line in a diff with @@
		{
			"simple addition with hunk",
			"--- a/foo.go\n" +
				"+++ b/foo.go\n" +
				"@@ -0,0 +1 @@\n" +
				"+ new line",
			"+ new line",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripDiffMarkers(tt.input)
			if got != tt.want {
				t.Errorf("stripDiffMarkers() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestIsDiff verifies diff format detection.
func TestIsDiff(t *testing.T) {
	tests := []struct {
		patch string
		want  bool
	}{
		{"diff --git a/foo.go b/foo.go", true},
		{"--- a/foo.go\n+++ b/foo.go", true},
		{"+++ b/foo.go", true},
		{"@@ -1,3 +1,4 @@", false},
		{"func main() {}", false},
		{"", false},
		// New cases: diff in the middle of text
		{"some text\n--- a/foo.go\n+++ b/foo.go", true},
		{"some text\ndiff --git a/foo.go b/foo.go", true},
	}
	for _, tt := range tests {
		got := isDiff(tt.patch)
		if got != tt.want {
			t.Errorf("isDiff(%q) = %v, want %v", tt.patch[:min(30, len(tt.patch))], got, tt.want)
		}
	}
}

// TestStripDiffMarkersEmpty verifies the empty patch case.
func TestStripDiffMarkersEmpty(t *testing.T) {
	got := stripDiffMarkers("")
	if got != "" {
		t.Errorf("stripDiffMarkers(\"\") = %q, want empty string", got)
	}
}

// TestStripDiffMarkersOnlyWhitespace verifies the whitespace-only case.
func TestStripDiffMarkersOnlyWhitespace(t *testing.T) {
	input := "   \n	\n"
	got := stripDiffMarkers(input)
	if got != input {
		t.Errorf("stripDiffMarkers(%q) = %q, want %q", input, got, input)
	}
}

// TestStripDiffMarkersContextPreserved verifies that context lines are preserved.
func TestStripDiffMarkersContextPreserved(t *testing.T) {
	input := "--- a/main.go\n" +
		"+++ b/main.go\n" +
		"@@ -1,3 +1,3 @@\n" +
		" package main\n" +
		"-import \"fmt\"\n" +
		"+import \"fmt\"\n" +
		" func main() {}"
	got := stripDiffMarkers(input)
	want := " package main\n" +
		"+import \"fmt\"\n" +
		" func main() {}"
	if got != want {
		t.Errorf("stripDiffMarkers() = %q, want %q", got, want)
	}
}

// TestDirForFile verifies directory extraction.
func TestDirForFile(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"internal/agent/llm.go", "internal/agent"},
		{"server.go", "."}, // no / → "."
		{"a/b/c.go", "a/b"},
		{"pkg/util.go", "pkg"},
	}
	for _, tt := range tests {
		got := dirForFile(tt.path)
		if got != tt.want {
			t.Errorf("dirForFile(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// TestBuildPrompt verifies that buildPrompt returns a complete CodePrompt.
func TestBuildPrompt(t *testing.T) {
	agent := NewCodeAgent(nil) // nil LLM → not called in this test
	prompt := agent.buildPrompt(
		Task{Kind: TaskFixBroken, Target: "add.go", Goal: "fix nil check"},
		Context{FilePath: "add.go"},
	)
	if prompt.Task == "" {
		t.Error("prompt.Task should not be empty")
	}
	if prompt.FilePath != "add.go" {
		t.Errorf("prompt.FilePath = %q, want %q", prompt.FilePath, "add.go")
	}
	if prompt.Language != "go" {
		t.Errorf("prompt.Language = %q, want %q", prompt.Language, "go")
	}
}

// TestBuildPlan verifies that buildPlan generates a Plan with the expected steps.
func TestBuildPlan(t *testing.T) {
	agent := NewCodeAgent(nil)
	plan := agent.buildPlan(
		Task{Kind: TaskFixBroken, Target: "add.go", Goal: "fix nil check"},
		&CodeResponse{
			Patch:       "func add(a, b int) int { return a + b }",
			Explanation: "added nil check",
			Confidence:  0.85,
		},
	)
	if len(plan.Steps) == 0 {
		t.Error("plan should have at least one step")
	}
	if plan.Steps[0].Kind != ActionWriteFile {
		t.Errorf("first step kind = %v, want ActionWriteFile", plan.Steps[0].Kind)
	}
	if plan.Steps[0].Target != "add.go" {
		t.Errorf("first step target = %q, want %q", plan.Steps[0].Target, "add.go")
	}
}

// TestBuildPromptWithSourceContent verifies buildPrompt includes source content.
func TestBuildPromptWithSourceContent(t *testing.T) {
	agent := NewCodeAgent(nil)
	prompt := agent.buildPrompt(
		Task{Kind: TaskFixBroken, Target: "add.go", Goal: "fix nil check"},
		Context{
			FilePath:      "add.go",
			SourceContent: "package main\n\nfunc add(a, b int) int { return a + b }",
		},
	)
	if !strings.Contains(prompt.Context, "Target source") {
		t.Error("prompt.Context should contain source content marker")
	}
	if !strings.Contains(prompt.Context, "func add") {
		t.Error("prompt.Context should contain the actual source content")
	}
}

// TestBuildPromptWithL2Events verifies buildPrompt includes L2 events.
func TestBuildPromptWithL2Events(t *testing.T) {
	agent := NewCodeAgent(nil)
	prompt := agent.buildPrompt(
		Task{Kind: TaskFixBroken, Target: "add.go", Goal: "fix nil check"},
		Context{
			FilePath: "add.go",
			L2Events: []storage.Event{
				{EventType: "build_failure", Payload: "compilation error at line 5"},
				{EventType: "test_failure", Payload: "TestAdd failed: expected 3, got 2"},
			},
		},
	)
	if !strings.Contains(prompt.Context, "Recent events") {
		t.Error("prompt.Context should contain 'Recent events'")
	}
	if !strings.Contains(prompt.Context, "build_failure") {
		t.Error("prompt.Context should contain event type 'build_failure'")
	}
	if !strings.Contains(prompt.Context, "test_failure") {
		t.Error("prompt.Context should contain event type 'test_failure'")
	}
}

// TestBuildPromptWithoutSourceContentOrEvents verifies buildPrompt when context is minimal.
func TestBuildPromptWithoutSourceContentOrEvents(t *testing.T) {
	agent := NewCodeAgent(nil)
	prompt := agent.buildPrompt(
		Task{Kind: TaskFixBroken, Target: "util.py", Goal: "add logging"},
		Context{FilePath: "util.py"},
	)
	if prompt.Language != "python" {
		t.Errorf("prompt.Language = %q, want %q", prompt.Language, "python")
	}
	// FilePath contributes "File: util.py\n" to context.
	if !strings.Contains(prompt.Context, "File: util.py") {
		t.Errorf("prompt.Context = %q, want to contain file path", prompt.Context)
	}
}

// TestBuildPromptWithUnknownLanguage verifies language detection falls back to unknown.
func TestBuildPromptWithUnknownLanguage(t *testing.T) {
	agent := NewCodeAgent(nil)
	prompt := agent.buildPrompt(
		Task{Kind: TaskFixBroken, Target: "Makefile", Goal: "fix target"},
		Context{},
	)
	if prompt.Language != "unknown" {
		t.Errorf("prompt.Language = %q, want %q", prompt.Language, "unknown")
	}
}

// mockLLM is a test double that returns configured responses.
type mockLLM struct {
	resp   *CodeResponse
	respErr error
}

func (m *mockLLM) GenerateCode(ctx context.Context, prompt CodePrompt) (*CodeResponse, error) {
	if m.respErr != nil {
		return nil, m.respErr
	}
	return m.resp, nil
}

// TestSolveWithMockLLMSuccess verifies Solve succeeds when mock LLM returns high confidence.
func TestSolveWithMockLLMSuccess(t *testing.T) {
	mock := &mockLLM{
		resp: &CodeResponse{
			Patch:       "func add(a, b int) int { return a + b }\n",
			Explanation: "mock fix",
			Confidence:  0.85,
		},
	}
	agent := NewCodeAgent(mock)
	task := Task{Kind: TaskFixBroken, Target: "add.go", Goal: "fix nil check"}
	ctx := Context{FilePath: "add.go"}

	plan, err := agent.Solve(task, ctx)
	if err != nil {
		t.Fatalf("Solve() unexpected error: %v", err)
	}
	if len(plan.Steps) == 0 {
		t.Error("plan should have at least one step")
	}
}

// TestSolveWithLowConfidence verifies Solve returns an error when confidence is below threshold.
func TestSolveWithLowConfidence(t *testing.T) {
	mock := &mockLLM{
		resp: &CodeResponse{
			Patch:       "func add(a, b int) int { return a + b }\n",
			Explanation: "low confidence fix",
			Confidence:  0.3, // below 0.6 threshold
		},
	}
	agent := NewCodeAgent(mock)
	task := Task{Kind: TaskFixBroken, Target: "add.go", Goal: "fix nil check"}
	ctx := Context{FilePath: "add.go"}

	_, err := agent.Solve(task, ctx)
	if err == nil {
		t.Fatal("Solve() expected error for low confidence, got nil")
	}
	if !strings.Contains(err.Error(), "low confidence") {
		t.Errorf("error = %q, want to contain 'low confidence'", err.Error())
	}
}

// TestSolveWithLLMError verifies Solve propagates LLM errors.
func TestSolveWithLLMError(t *testing.T) {
	mock := &mockLLM{
		respErr: fmt.Errorf("connection refused"),
	}
	agent := NewCodeAgent(mock)
	task := Task{Kind: TaskFixBroken, Target: "add.go", Goal: "fix nil check"}
	ctx := Context{FilePath: "add.go"}

	_, err := agent.Solve(task, ctx)
	if err == nil {
		t.Fatal("Solve() expected error for LLM failure, got nil")
	}
}

// TestEnrichWithExistingFile verifies Enrich populates SourceContent from an existing file.
func TestEnrichWithExistingFile(t *testing.T) {
	f, err := os.CreateTemp("", "enrichtest_*.go")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString("package main\n\nfunc add(a, b int) int { return a + b }\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	ctx := &Context{}
	ctx.Enrich(f.Name(), 50)
	if ctx.SourceContent == "" {
		t.Error("Enrich with existing file should populate SourceContent")
	}
	if !strings.Contains(ctx.SourceContent, "func add") {
		t.Error("SourceContent should contain the file content")
	}
}

// TestEnrichWithNonexistentFile verifies Enrich does not panic for a missing file.
func TestEnrichWithNonexistentFile(t *testing.T) {
	ctx := &Context{}
	// Must not panic.
	ctx.Enrich("/nonexistent/path/to/file.go", 50)
	if ctx.SourceContent != "" {
		t.Errorf("SourceContent = %q, want empty string for nonexistent file", ctx.SourceContent)
	}
}

// TestEnrichWithLineLimit verifies Enrich respects the maxLines limit.
func TestEnrichWithLineLimit(t *testing.T) {
	f, err := os.CreateTemp("", "enrichtest_*.go")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	// Write 10 lines.
	content := strings.Repeat("line\n", 10)
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()

	ctx := &Context{}
	ctx.Enrich(f.Name(), 3)
	lines := strings.Split(ctx.SourceContent, "\n")
	if len(lines)-1 > 3 { // -1 because final \n creates empty element
		t.Errorf("SourceContent has %d lines, want at most 3", len(lines)-1)
	}
}

// TestEnrichWithSemMemNil verifies Enrich does not panic when SemMem is nil.
func TestEnrichWithSemMemNil(t *testing.T) {
	f, err := os.CreateTemp("", "enrichtest_*.go")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString("package main\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	ctx := &Context{SemMem: nil}
	// Must not panic.
	ctx.Enrich(f.Name(), 50)
	// SourceContent should be populated since SemMem is nil but file exists.
	if ctx.SourceContent == "" {
		t.Error("Enrich with nil SemMem should still populate SourceContent from file")
	}
}

// TestSolveRejectsUnsupportedTask verifies Solve returns an error for unsupported task kinds.
func TestSolveRejectsUnsupportedTask(t *testing.T) {
	mock := &mockLLM{}
	agent := NewCodeAgent(mock)
	// Only TaskFixBroken is accepted; use an unsupported value.
	task := Task{Kind: TaskKind("some_other_kind"), Target: "add.go", Goal: "some task"}
	ctx := Context{FilePath: "add.go"}

	_, err := agent.Solve(task, ctx)
	if err == nil {
		t.Fatal("Solve() expected error for unsupported TaskKind, got nil")
	}
	if !strings.Contains(err.Error(), "TaskFixBroken") {
		t.Errorf("error = %q, want to mention TaskFixBroken", err.Error())
	}
}

// Note: TestSolveWithMockLLM is commented out because the mock LLM returns
// a confidence of 0.50 which is below the default threshold of 0.60.
// func TestSolveWithMockLLM(...) { ... }
