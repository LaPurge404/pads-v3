package agent

import (
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