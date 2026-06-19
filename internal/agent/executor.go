package agent

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Executor applies Actions produced by an Agent.
type Executor struct {
	DryRun bool // if true, log but do not modify files
}

// ExecutionResult is the outcome of executing one Action.
type ExecutionResult struct {
	Action     Action
	Output     string
	ExitCode   int
	Applied    bool
	Error      string
	DurationMs int64
}

// Execute runs a single action and returns its result.
func (e *Executor) Execute(action Action) ExecutionResult {
	if !IsValidActionKind(action.Kind) {
		return ExecutionResult{
			Action: action,
			Error:  fmt.Sprintf("invalid action kind: %s", action.Kind),
		}
	}

	switch action.Kind {
	case ActionWriteFile:
		return e.writeFile(action)
	case ActionRunCommand:
		return e.runCommand(action)
	case ActionLog:
		return ExecutionResult{Action: action, Applied: true, Output: "log action"}
	default:
		return ExecutionResult{Action: action, Error: "unhandled action kind"}
	}
}

// ExecutePlan runs all steps in a plan sequentially.
// Execution stops on first failure unless ContinueOnError is true.
func (e *Executor) ExecutePlan(plan Plan, continueOnError bool) []ExecutionResult {
	results := make([]ExecutionResult, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		res := e.Execute(step)
		results = append(results, res)
		if res.Error != "" && !continueOnError {
			break
		}
	}
	return results
}

// --- private helpers ---

func (e *Executor) writeFile(action Action) ExecutionResult {
	target := action.Target
	if target == "" {
		return ExecutionResult{Action: action, Error: "write_file: missing target"}
	}

	content := action.Patch
	if content == "" {
		// Patch is the full content for write_file
		content = action.Patch
	}

	if e.DryRun {
		return ExecutionResult{
			Action:  action,
			Applied: false,
			Output:  fmt.Sprintf("[dry-run] would write %d bytes to %s", len(content), target),
		}
	}

	// Ensure parent directory exists
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ExecutionResult{Action: action, Error: fmt.Sprintf("mkdirAll: %v", err)}
	}

	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		return ExecutionResult{Action: action, Error: fmt.Sprintf("writefile: %v", err)}
	}

	return ExecutionResult{
		Action:  action,
		Applied: true,
		Output:  fmt.Sprintf("wrote %d bytes to %s", len(content), target),
	}
}

func (e *Executor) runCommand(action Action) ExecutionResult {
	var cmdStr string
	var args []string

	switch {
	case len(action.Command) > 0:
		cmdStr = action.Command[0]
		args = action.Command[1:]
	case action.Target != "":
		parts := strings.Fields(action.Target)
		if len(parts) == 0 {
			return ExecutionResult{Action: action, Error: "run_command: empty target"}
		}
		cmdStr = parts[0]
		args = parts[1:]
	default:
		return ExecutionResult{Action: action, Error: "run_command: no command specified"}
	}

	if e.DryRun {
		return ExecutionResult{
			Action:  action,
			Applied: false,
			Output:  fmt.Sprintf("[dry-run] would run: %s %v", cmdStr, args),
		}
	}

	cmd := exec.Command(cmdStr, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Inherit env but strip dangerous variables
	cmd.Env = filterEnv(os.Environ())

	if err := cmd.Run(); err != nil {
		return ExecutionResult{
			Action:   action,
			Applied:  false,
			Output:   stdout.String() + stderr.String(),
			ExitCode: cmd.ProcessState.ExitCode(),
			Error:    fmt.Sprintf("command failed: %v", err),
		}
	}

	return ExecutionResult{
		Action:   action,
		Applied:  true,
		Output:   stdout.String() + stderr.String(),
		ExitCode: 0,
	}
}

// filterEnv removes potentially dangerous env vars before running
// untrusted commands in the sandbox.
func filterEnv(env []string) []string {
	const dangerous = "LD_PRELOAD LD_LIBRARY_PATH DYLD_INSERT_LIBRARIES DYLD_LIBRARY_PATH"
	bad := make(map[string]bool)
	for _, k := range strings.Fields(dangerous) {
		bad[k] = true
	}
	filtered := make([]string, 0, len(env))
	for _, v := range env {
		kv := strings.SplitN(v, "=", 2)
		if len(kv) == 2 && bad[kv[0]] {
			continue
		}
		filtered = append(filtered, v)
	}
	return filtered
}
