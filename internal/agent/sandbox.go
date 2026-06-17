package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Sandbox provides an isolated environment to test code changes
// before applying them to the production codebase.
type Sandbox struct {
	// Working directory for the sandbox
	workDir string
	// Original project root
	projectRoot string
	// Cleanup function
	cleanup func()
}

// SandboxResult contains the outcome of testing changes in the sandbox.
type SandboxResult struct {
	// Whether the changes passed all tests
	Passed bool
	// Number of tests passed
	TestsPassed int
	// Number of tests failed
	TestsFailed int
	// Build output (stdout/stderr)
	BuildOutput string
	// Test output (stdout/stderr)
	TestOutput string
	// Any errors during sandbox execution
	Error error
	// Path to the sandbox directory (for inspection)
	SandboxPath string
}

// NewSandbox creates a new sandbox by copying the project to a temp directory.
func NewSandbox(projectRoot string) (*Sandbox, error) {
	// Create a unique temp directory
	tmpDir, err := os.MkdirTemp("", "pads-sandbox-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	// Copy project to sandbox
	if err := copyDir(projectRoot, tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("copy project to sandbox: %w", err)
	}

	return &Sandbox{
		workDir:     tmpDir,
		projectRoot: projectRoot,
		cleanup: func() {
			os.RemoveAll(tmpDir)
		},
	}, nil
}

// Close cleans up the sandbox directory.
func (s *Sandbox) Close() {
	if s.cleanup != nil {
		s.cleanup()
	}
}

// WorkDir returns the sandbox working directory.
func (s *Sandbox) WorkDir() string {
	return s.workDir
}

// ApplyChange applies a file change to the sandbox.
func (s *Sandbox) ApplyChange(targetPath string, content string) error {
	// Map the target path from project root to sandbox
	relPath, err := filepath.Rel(s.projectRoot, targetPath)
	if err != nil {
		return fmt.Errorf("compute relative path: %w", err)
	}
	sandboxPath := filepath.Join(s.workDir, relPath)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(sandboxPath), 0755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	// Write the file
	if err := os.WriteFile(sandboxPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// ApplyPatch applies a diff/patch to the sandbox.
// Returns the number of files changed.
func (s *Sandbox) ApplyPatch(patch string) (int, error) {
	// Create a patch file
	patchFile := filepath.Join(s.workDir, ".sandbox.patch")
	if err := os.WriteFile(patchFile, []byte(patch), 0644); err != nil {
		return 0, fmt.Errorf("write patch file: %w", err)
	}
	defer os.Remove(patchFile)

	// Apply the patch
	cmd := exec.Command("patch", "-p1", "-i", ".sandbox.patch")
	cmd.Dir = s.workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("apply patch: %w\n%s", err, output)
	}

	// Count changed files by looking at patch output
	changedFiles := countChangedFiles(string(output))
	return changedFiles, nil
}

// RunTests executes tests in the sandbox and returns the result.
func (s *Sandbox) RunTests() SandboxResult {
	var res SandboxResult
	res.SandboxPath = s.workDir

	// Build first to catch compilation errors
	buildCmd := exec.Command("go", "build", "./...")
	buildCmd.Dir = s.workDir
	buildOutput, buildErr := buildCmd.CombinedOutput()
	res.BuildOutput = string(buildOutput)

	if buildErr != nil {
		res.Error = fmt.Errorf("build failed: %w", buildErr)
		res.Passed = false
		return res
	}

	// Run tests
	testCmd := exec.Command("go", "test", "./...", "-count=1", "-v")
	testCmd.Dir = s.workDir
	testOutput, testErr := testCmd.CombinedOutput()
	res.TestOutput = string(testOutput)

	// Parse test results
	lines := strings.Split(string(testOutput), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ok ") {
			res.TestsPassed++
		} else if strings.HasPrefix(trimmed, "FAIL ") {
			res.TestsFailed++
		}
	}

	if testErr != nil {
		if res.TestsFailed == 0 {
			res.TestsFailed = 1
		}
		res.Passed = false
	} else {
		res.Passed = res.TestsFailed == 0
	}

	return res
}

// RunTestsForFile runs tests specific to a file in the sandbox.
func (s *Sandbox) RunTestsForFile(targetPath string) SandboxResult {
	var res SandboxResult
	res.SandboxPath = s.workDir

	// Map path to sandbox
	relPath, err := filepath.Rel(s.projectRoot, targetPath)
	if err != nil {
		res.Error = fmt.Errorf("compute relative path: %w", err)
		return res
	}
	sandboxPath := filepath.Join(s.workDir, relPath)
	dir := filepath.Dir(sandboxPath)

	// Build the package
	buildCmd := exec.Command("go", "build", "./...")
	buildCmd.Dir = dir
	buildOutput, buildErr := buildCmd.CombinedOutput()
	res.BuildOutput = string(buildOutput)

	if buildErr != nil {
		res.Error = fmt.Errorf("build failed: %w", buildErr)
		return res
	}

	// Run tests for this package
	testCmd := exec.Command("go", "test", "./...", "-count=1")
	testCmd.Dir = dir
	testOutput, testErr := testCmd.CombinedOutput()
	res.TestOutput = string(testOutput)

	// Parse results
	lines := strings.Split(string(testOutput), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ok ") {
			res.TestsPassed++
		} else if strings.HasPrefix(trimmed, "FAIL ") {
			res.TestsFailed++
		}
	}

	res.Passed = res.TestsFailed == 0 && testErr == nil
	return res
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Compute relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Skip .git and hidden dirs
		if relPath == "." {
			return nil
		}
		if strings.HasPrefix(relPath, ".git") || strings.HasPrefix(relPath, ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Compute destination path
		destPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		// Copy file
		return copyFile(path, destPath, info.Mode())
	})
}

// copyFile copies a single file.
func copyFile(src, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, mode)
}

// countChangedFiles parses patch output to count changed files.
func countChangedFiles(output string) int {
	count := 0
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "patching file ") {
			count++
		}
	}
	return count
}

// SandboxExecutor wraps an Executor with sandbox testing.
type SandboxExecutor struct {
	executor    *Executor
	sandbox     *Sandbox
	projectRoot string
	autoCleanup bool
}

// NewSandboxExecutor creates a new executor with sandbox testing.
func NewSandboxExecutor(projectRoot string, autoCleanup bool) *SandboxExecutor {
	return &SandboxExecutor{
		executor:    &Executor{DryRun: false},
		projectRoot: projectRoot,
		autoCleanup: autoCleanup,
	}
}

// ExecuteWithSandbox executes actions in a sandbox, tests, and returns result.
// If tests pass, applies to real filesystem. If tests fail, rolls back.
func (e *SandboxExecutor) ExecuteWithSandbox(plan Plan) SandboxResult {
	// Create sandbox
	sandbox, err := NewSandbox(e.projectRoot)
	if err != nil {
		return SandboxResult{Error: fmt.Errorf("create sandbox: %w", err)}
	}

	// Execute plan in sandbox
	for _, action := range plan.Steps {
		if action.Kind == ActionWriteFile {
			if err := sandbox.ApplyChange(action.Target, action.Patch); err != nil {
				if e.autoCleanup {
					sandbox.Close()
				}
				return SandboxResult{Error: fmt.Errorf("apply change: %w", err)}
			}
		} else if action.Kind == ActionRunCommand {
			// For run_command actions, we need to apply them in sandbox
			// This requires mapping the command to run in the sandbox context
			if err := e.executeCommandInSandbox(sandbox, action); err != nil {
				if e.autoCleanup {
					sandbox.Close()
				}
				return SandboxResult{Error: fmt.Errorf("execute command: %w", err)}
			}
		}
	}

	// Run tests in sandbox
	result := sandbox.RunTests()

	if e.autoCleanup {
		sandbox.Close()
	}

	// If tests passed, apply to real filesystem
	if result.Passed {
		for _, action := range plan.Steps {
			if action.Kind == ActionWriteFile {
				if err := os.WriteFile(action.Target, []byte(action.Patch), 0644); err != nil {
					result.Error = fmt.Errorf("apply to real fs: %w", err)
					result.Passed = false
					return result
				}
			}
		}
	}

	return result
}

// executeCommandInSandbox runs a command in the sandbox directory.
func (e *SandboxExecutor) executeCommandInSandbox(sandbox *Sandbox, action Action) error {
	var cmdStr string
	var args []string

	if len(action.Command) > 0 {
		cmdStr = action.Command[0]
		args = action.Command[1:]
	} else if action.Target != "" {
		parts := strings.Fields(action.Target)
		if len(parts) == 0 {
			return fmt.Errorf("empty command")
		}
		cmdStr = parts[0]
		args = parts[1:]
	} else {
		return fmt.Errorf("no command specified")
	}

	cmd := exec.Command(cmdStr, args...)
	cmd.Dir = sandbox.WorkDir()
	cmd.Env = filterEnv(os.Environ())

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", cmdStr, err, output)
	}

	return nil
}

// DryRunWithSandbox executes the plan in sandbox and returns results without applying.
func (e *SandboxExecutor) DryRunWithSandbox(plan Plan) SandboxResult {
	sandbox, err := NewSandbox(e.projectRoot)
	if err != nil {
		return SandboxResult{Error: fmt.Errorf("create sandbox: %w", err)}
	}

	// Execute in sandbox (don't cleanup automatically)
	se := &SandboxExecutor{
		executor:    e.executor,
		sandbox:     sandbox,
		projectRoot: e.projectRoot,
		autoCleanup: false, // We'll cleanup manually
	}

	result := se.executeSandboxPlan(plan, sandbox)

	// Cleanup after getting results
	sandbox.Close()

	return result
}

func (e *SandboxExecutor) executeSandboxPlan(plan Plan, sandbox *Sandbox) SandboxResult {
	for _, action := range plan.Steps {
		if action.Kind == ActionWriteFile {
			if err := sandbox.ApplyChange(action.Target, action.Patch); err != nil {
				return SandboxResult{Error: fmt.Errorf("apply change: %w", err)}
			}
		} else if action.Kind == ActionRunCommand {
			if err := e.executeCommandInSandbox(sandbox, action); err != nil {
				return SandboxResult{Error: fmt.Errorf("execute command: %w", err)}
			}
		}
	}

	return sandbox.RunTests()
}