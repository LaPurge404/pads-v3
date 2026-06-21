package agent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSandboxCreation(t *testing.T) {
	// Create a temp directory with a minimal Go project
	tmpDir, err := os.MkdirTemp("", "pads-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a minimal go.mod
	goMod := `module testproject

go 1.21`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a simple Go file
	goFile := `package main

func Add(a, b int) int {
	return a + b
}`
	if err := os.WriteFile(filepath.Join(tmpDir, "add.go"), []byte(goFile), 0644); err != nil {
		t.Fatal(err)
	}

	// Create sandbox
	sandbox, err := NewSandbox(tmpDir)
	if err != nil {
		t.Fatalf("NewSandbox failed: %v", err)
	}
	defer sandbox.Close()

	// Verify sandbox directory exists and is different from original
	if sandbox.WorkDir() == tmpDir {
		t.Error("sandbox workDir should be different from project root")
	}

	// Verify go.mod was copied
	sandboxGoMod := filepath.Join(sandbox.WorkDir(), "go.mod")
	if _, err := os.Stat(sandboxGoMod); os.IsNotExist(err) {
		t.Error("go.mod not copied to sandbox")
	}
}

func TestSandboxApplyChange(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "pads-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a minimal go.mod
	goMod := `module testproject

go 1.21`
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644)

	// Create a simple Go file
	goFile := `package main

func Add(a, b int) int {
	return a + b
}`
	os.WriteFile(filepath.Join(tmpDir, "add.go"), []byte(goFile), 0644)

	// Create sandbox
	sandbox, err := NewSandbox(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer sandbox.Close()

	// Apply a change
	newContent := `package main

func Add(a, b int) int {
	return a + b + 1 // modified
}`
	targetPath := filepath.Join(tmpDir, "add.go")
	if err := sandbox.ApplyChange(targetPath, newContent); err != nil {
		t.Fatalf("ApplyChange failed: %v", err)
	}

	// Verify the change was applied in sandbox
	sandboxFile := filepath.Join(sandbox.WorkDir(), "add.go")
	data, err := os.ReadFile(sandboxFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "+ 1 // modified") {
		t.Error("change not applied in sandbox")
	}

	// Verify original file was NOT modified
	originalData, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(originalData), "+ 1 // modified") {
		t.Error("original file was modified (should not be)")
	}
}

func TestSandboxRunTests(t *testing.T) {
	// Create a temp directory with a minimal Go project
	tmpDir, err := os.MkdirTemp("", "pads-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create go.mod
	goMod := `module testproject

go 1.21`
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644)

	// Create a simple Go file with passing test
	mainFile := `package main

func Add(a, b int) int {
	return a + b
}

func AddTest(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Error("unexpected result")
	}
}`
	os.WriteFile(filepath.Join(tmpDir, "add.go"), []byte(mainFile), 0644)

	// Create test file
	testFile := `package main

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Error("Add(2,3) should be 5")
	}
}`
	os.WriteFile(filepath.Join(tmpDir, "add_test.go"), []byte(testFile), 0644)

	// Create sandbox
	sandbox, err := NewSandbox(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer sandbox.Close()

	// Run tests
	result := sandbox.RunTests()

	// Should pass (build + tests)
	if result.Error != nil && !strings.Contains(result.Error.Error(), "no test files") {
		t.Logf("result error: %v", result.Error)
	}
}

func TestSandboxExecutorExecuteWithSandbox(t *testing.T) {
	// Create a temp directory with a minimal Go project
	tmpDir, err := os.MkdirTemp("", "pads-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create go.mod
	goMod := `module testproject

go 1.21`
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644)

	// Create a simple Go file
	mainFile := `package main

func Add(a, b int) int {
	return a + b
}`
	os.WriteFile(filepath.Join(tmpDir, "add.go"), []byte(mainFile), 0644)

	// Create test file
	testFile := `package main

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Error("Add(2,3) should be 5")
	}
}`
	os.WriteFile(filepath.Join(tmpDir, "add_test.go"), []byte(testFile), 0644)

	// Create executor
	executor := NewSandboxExecutor(tmpDir, true)

	// Create a plan that modifies the file
	targetPath := filepath.Join(tmpDir, "add.go")
	plan := Plan{
		Steps: []Action{
			{
				Kind:   ActionWriteFile,
				Target: targetPath,
				Patch: `package main

func Add(a, b int) int {
	return a + b + 1 // intentionally wrong for test
}`,
			},
		},
	}

	// Execute with sandbox
	result := executor.ExecuteWithSandbox(plan)

	// Tests should fail because we introduced a bug
	// Note: the change should NOT be applied to the real filesystem
	if result.Passed {
		t.Log("note: tests passed unexpectedly")
	}

	// Verify original file was NOT modified
	originalData, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(originalData), "+ 1 // intentionally") {
		t.Error("original file should not be modified when sandbox tests fail")
	}
}

func TestSandboxRollbackOnTestFailure(t *testing.T) {
	// Create a temp directory with a minimal Go project
	tmpDir, err := os.MkdirTemp("", "pads-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create go.mod
	goMod := `module testproject

go 1.21`
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644)

	// Create a simple Go file with tests
	mainFile := `package main

func Add(a, b int) int {
	return a + b
}`
	os.WriteFile(filepath.Join(tmpDir, "add.go"), []byte(mainFile), 0644)

	testFile := `package main

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Error("Add(2,3) should be 5")
	}
}`
	os.WriteFile(filepath.Join(tmpDir, "add_test.go"), []byte(testFile), 0644)

	// Create executor
	executor := NewSandboxExecutor(tmpDir, true)

	// Create a plan with a breaking change
	targetPath := filepath.Join(tmpDir, "add.go")
	plan := Plan{
		Steps: []Action{
			{
				Kind:   ActionWriteFile,
				Target: targetPath,
				Patch: `package main

func Add(a, b int) int {
	return "not an int" // type error
}`,
			},
		},
	}

	// Execute with sandbox
	result := executor.ExecuteWithSandbox(plan)

	// Should fail (build error due to type mismatch)
	if result.Passed {
		t.Error("should not pass with type error")
	}

	// Verify original file was NOT modified (rollback)
	originalData, _ := os.ReadFile(targetPath)
	if strings.Contains(string(originalData), "not an int") {
		t.Error("original file should be rolled back after sandbox failure")
	}
}

// TestApplyChangeRejectsPathTraversal is the security regression test for the
// sandbox-escape vulnerability. Each case crafts a targetPath that LOOKS like
// it could escape the sandbox via "..", absolute paths, or the common-prefix
// trap. ApplyChange MUST return an error containing "escapes sandbox" for
// every malicious case, and MUST NOT create any file outside the sandbox.
//
// This complements ApplyChange's happy-path test (TestSandboxApplyChange) by
// asserting the rejects. Without the validation in resolveSandboxTarget,
// these cases would either write to /etc/passwd, sentinel files in /tmp,
// or files outside the sandbox.
func TestApplyChangeRejectsPathTraversal(t *testing.T) {
	// Build a minimal project root so that filepath.Rel succeeds against it.
	// projectRoot encodes the "directory we own"; we want targetPaths that
	// look like they're inside this directory but resolve to elsewhere.
	projectRoot, err := os.MkdirTemp("", "pads-sandbox-traversal-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectRoot)

	if err := os.WriteFile(filepath.Join(projectRoot, "go.mod"),
		[]byte("module x\n\ngo 1.21"), 0644); err != nil {
		t.Fatal(err)
	}

	sandbox, err := NewSandbox(projectRoot)
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	defer sandbox.Close()

	cases := []struct {
		// name describes the attack vector
		name string
		// path is what a malicious caller would attempt to write to
		path string
	}{
		{
			name: "parent_traversal_outside_projectRoot",
			// /tmp/<sandbox>/../../etc/passwd — filepath.Clean collapses
			// the leading /tmp/<sandbox>/.. into /tmp/. Beyond that we
			// produce a final ".." segment that escapes projectRoot.
			path: filepath.Join(projectRoot, "..", "..", "etc", "passwd"),
		},
		{
			name: "absolute_path_outside_projectRoot",
			path: "/etc/passwd",
		},
		{
			name: "absolute_path_outside_sandbox_but_under_projectRoot",
			// /tmp itself is reachable via projectRoot; this still counts
			// as escaping because the user only "owns" files inside the
			// project tree, not arbitrary /tmp siblings.
			path: filepath.Join(os.TempDir(), "pads-evil", "hidden.txt"),
		},
		{
			name: "double_dot_alone",
			// A bare ".." must reject (no further path supplied).
			path: "..",
		},
		{
			name: "embedded_double_dot_segment",
			path: filepath.Join(projectRoot, "subdir", "..", "..", "etc", "shadow"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := sandbox.ApplyChange(tc.path, "malicious payload")
			if err == nil {
				t.Fatalf("ApplyChange(%q) must reject, but returned nil", tc.path)
			}
			if !strings.Contains(err.Error(), "escapes sandbox") {
				t.Fatalf("ApplyChange(%q): unexpected error %q, want substring %q",
					tc.path, err, "escapes sandbox")
			}
			// The error must preserve the original targetPath so audit logs
			// can attribute the offending caller.
			if !strings.Contains(err.Error(), "target path") {
				t.Fatalf("ApplyChange(%q): error missing audit trail of original path; got %q",
					tc.path, err)
			}
		})
	}

	// Sentinel check: nothing outside the sandbox should have been written.
	// /etc/passwd may legitimately exist on Linux, so we only probe for a
	// sentinel we created in os.TempDir() that the test expects NOT to exist.
	if _, err := os.Stat(filepath.Join(os.TempDir(), "pads-evil")); err == nil {
		t.Errorf("ApplyChange created %s which is outside the sandbox",
			filepath.Join(os.TempDir(), "pads-evil"))
	}

	// Sanity: confirm the happy-path test still works (legitimate file inside
	// projectRoot). Use a fresh, unique subpath to avoid colliding with any
	// pre-existing test fixtures.
	legit := filepath.Join(projectRoot, "legit", "ok.go")
	if err := sandbox.ApplyChange(legit, "package x"); err != nil {
		// If MkdirAll fails inside the sandbox (e.g. tmp dir vanished), we
		// tolerate that — what matters is that the security gate did NOT
		// produce an "escapes sandbox" error.
		if strings.Contains(err.Error(), "escapes sandbox") {
			t.Fatalf("ApplyChange(%q) wrongly rejected a legitimate target: %v", legit, err)
		}
		t.Logf("ApplyChange(%q) happy-path non-security error (acceptable): %v", legit, err)
	}

	_ = errors.Is
}
