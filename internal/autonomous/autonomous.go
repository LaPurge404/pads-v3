// Package autonomous implements the optional closed-loop autonomous mode for PADS.
// When enabled, it continuously evaluates, applies, and commits improvements.
package autonomous

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"

	"pads-v3/internal/agent"
	"pads-v3/internal/policy/evolution"
)

// Mode tracks whether autonomous mode is active and holds cycle statistics.
type Mode struct {
	enabled  bool
	cycleNum int64
}

// New creates a new autonomous Mode.
func New() *Mode {
	return &Mode{enabled: false, cycleNum: 0}
}

// IsEnabled reports whether autonomous mode is active.
func (m *Mode) IsEnabled() bool { return m.enabled }

// Enable activates autonomous mode.
func (m *Mode) Enable() {
	m.enabled = true
	slog.Info("autonomous: mode enabled")
}

// Disable deactivates autonomous mode.
func (m *Mode) Disable() {
	m.enabled = false
	slog.Info("autonomous: mode disabled")
}

// CycleNum returns the number of completed cycles.
func (m *Mode) CycleNum() int64 { return m.cycleNum }

// Toggle flips the autonomous mode state and returns the new state.
func (m *Mode) Toggle() bool {
	m.enabled = !m.enabled
	slog.Info("autonomous: toggled", "enabled", m.enabled)
	return m.enabled
}

// RunCycleResult contains the outcome of a single autonomous cycle.
type RunCycleResult struct {
	Cycle      int64
	Task       string
	PlanSteps  int
	Score      int
	Accepted   bool
	Committed  bool
	CommitHash string
	Error      string
}

// RunCycle executes one autonomous improvement cycle:
//  1. Verifies autonomous mode is enabled
//  2. Queries the CodeAgent to generate a fix plan
//  3. Executes the plan in the sandbox
//  4. Evaluates the result through the evolution engine
//  5. If accepted and score is good, commits the changes to Git
func (m *Mode) RunCycle(
	task agent.Task,
	codeAgent *agent.CodeAgent,
	sandboxExec *agent.SandboxExecutor,
	agentLoop *evolution.AgentLoop,
	projectRoot string,
	semanticRisk float64,
	semanticReasons []string,
) RunCycleResult {
	if !m.enabled {
		return RunCycleResult{Error: "autonomous mode is disabled"}
	}

	m.cycleNum++
	cycle := m.cycleNum

	slog.Info("autonomous: starting cycle", "cycle", cycle, "task", task.Goal)

	// Step 1: Generate plan via CodeAgent
	candidateID := generateID()
	ctx := agent.Context{FilePath: task.Target}
	ctx.Enrich(task.Target, 200)
	resp, err := codeAgent.Solve(task, ctx)
	if err != nil {
		slog.Warn("autonomous: Solve failed", "cycle", cycle, "err", err)
		return RunCycleResult{Cycle: cycle, Task: task.Goal, Error: fmt.Sprintf("Solve: %v", err)}
	}

	slog.Info("autonomous: plan generated", "cycle", cycle, "steps", len(resp.Steps))

	// Step 2: Execute in sandbox
	sandboxRes := sandboxExec.ExecuteWithSandbox(resp)
	if sandboxRes.Error != nil {
		slog.Warn("autonomous: sandbox error", "cycle", cycle, "err", sandboxRes.Error)
		return RunCycleResult{
			Cycle: cycle, Task: task.Goal, PlanSteps: len(resp.Steps),
			Error: fmt.Sprintf("sandbox: %v", sandboxRes.Error),
		}
	}

	// Step 3: Compute candidate score from sandbox results
	candidateScore := computeSandboxScore(sandboxRes)

	// Step 4: Build candidate and evaluate
	candidate := evolution.BuildAgentCandidate(
		candidateID,
		task.Target,
		serializePlan(resp),
		codeAgent.MinConfidence(),
		agentLoop.SelectArm(),
	)

	result := agentLoop.Evaluate(candidate, candidateScore, 1.0, semanticRisk, semanticReasons)

	slog.Info("autonomous: evaluation complete",
		"cycle", cycle,
		"accepted", result.Accepted,
		"score", result.CycleResult.Score,
		"reward", result.Reward,
	)

	if !result.Accepted {
		slog.Info("autonomous: candidate rejected, skipping commit", "cycle", cycle, "reason", result.Reason)
		return RunCycleResult{
			Cycle: cycle, Task: task.Goal, PlanSteps: len(resp.Steps),
			Score: candidateScore, Accepted: false,
			Error: "rejected: " + result.Reason,
		}
	}

	// Step 5: Apply to real project (copy from sandbox) and commit.
	committed, commitHash, applyErr := m.applyAndCommit(resp, projectRoot)
	if applyErr != nil {
		slog.Warn("autonomous: apply/commit failed", "cycle", cycle, "err", applyErr)
		return RunCycleResult{
			Cycle: cycle, Task: task.Goal, PlanSteps: len(resp.Steps),
			Score: candidateScore, Accepted: true,
			Error: fmt.Sprintf("apply/commit: %v", applyErr),
		}
	}

	slog.Info("autonomous: cycle complete", "cycle", cycle, "committed", committed, "hash", commitHash)
	return RunCycleResult{
		Cycle: cycle, Task: task.Goal, PlanSteps: len(resp.Steps),
		Score: candidateScore, Accepted: true, Committed: committed,
		CommitHash: commitHash,
	}
}

// applyAndCommit copies changed files from sandbox to real project and commits.
func (m *Mode) applyAndCommit(plan agent.Plan, projectRoot string) (committed bool, hash string, err error) {
	// Collect files to add from plan.
	var filesToAdd []string
	for _, step := range plan.Steps {
		if step.Kind == agent.ActionWriteFile && step.Target != "" {
			filesToAdd = append(filesToAdd, step.Target)
		}
	}

	if len(filesToAdd) == 0 {
		return false, "", errors.New("no files to commit")
	}

	// Stage files.
	for _, f := range filesToAdd {
		if err := gitAdd(projectRoot, f); err != nil {
			return false, "", fmt.Errorf("git add %s: %w", f, err)
		}
	}

	// Commit with a descriptive message.
	msg := fmt.Sprintf("autonomous: apply cycle %d — %s", m.cycleNum, describePlan(plan))
	hash, err = gitCommit(projectRoot, msg)
	if err != nil {
		return false, "", fmt.Errorf("git commit: %w", err)
	}

	return true, hash, nil
}

// gitAdd runs `git add` on a single file.
func gitAdd(repoRoot, filePath string) error {
	// Make path relative to repoRoot for the add command.
	rel, err := filepath.Rel(repoRoot, filePath)
	if err != nil {
		rel = filePath
	}
	cmd := exec.Command("git", "-C", repoRoot, "add", "--", rel)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", cmd.String(), string(out))
	}
	return nil
}

// gitCommit runs `git commit -m <msg>` and returns the commit hash.
func gitCommit(repoRoot, msg string) (string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "commit", "-m", msg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Check if there's nothing to commit.
		if strings.Contains(string(out), "nothing to commit") {
			return "", nil
		}
		return "", fmt.Errorf("%s: %s", cmd.String(), string(out))
	}

	// Get the commit hash.
	hashCmd := exec.Command("git", "-C", repoRoot, "rev-parse", "HEAD")
	hashOut, err := hashCmd.Output()
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(hashOut)), nil
}

// computeSandboxScore converts sandbox results into an evolution score.
func computeSandboxScore(res agent.SandboxResult) int {
	score := 0
	if res.Error == nil && !strings.Contains(res.BuildOutput, "error") {
		score += 30
	}
	if res.Passed {
		score += 50
	} else if res.TestsPassed > 0 {
		total := res.TestsPassed + res.TestsFailed
		if total > 0 {
			score += 50 * res.TestsPassed / total
		}
	}
	if !strings.Contains(res.BuildOutput, "warning") && !strings.Contains(res.TestOutput, "warning") {
		score += 20
	}
	return score
}

// serializePlan converts a Plan to a string representation.
func serializePlan(plan agent.Plan) string {
	var parts []string
	for i, step := range plan.Steps {
		parts = append(parts, fmt.Sprintf("step%d: %s -> %s", i, step.Kind, step.Target))
	}
	return strings.Join(parts, "; ")
}

// describePlan returns a short one-line description of a plan.
func describePlan(plan agent.Plan) string {
	var targets []string
	for _, step := range plan.Steps {
		if step.Kind == agent.ActionWriteFile && step.Target != "" {
			rel := filepath.Base(step.Target)
			targets = append(targets, rel)
		}
	}
	if len(targets) == 0 {
		return "no files"
	}
	if len(targets) == 1 {
		return targets[0]
	}
	return fmt.Sprintf("%s (+%d more)", targets[0], len(targets)-1)
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}