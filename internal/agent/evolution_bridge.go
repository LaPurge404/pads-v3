package agent

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"pads-v3/internal/policy/evolution"
)

// EvolutionConnector connects the CodeAgent to the evolution engine.
// It transforms agent suggestions into evolution candidates and evaluates them.
type EvolutionConnector struct {
	codeAgent     *CodeAgent
	sandboxExec   *SandboxExecutor
	agentLoop     *evolution.AgentLoop
	currentScore  int
}

// NewEvolutionConnector creates a new connector with all dependencies.
func NewEvolutionConnector(
	codeAgent *CodeAgent,
	sandboxExec *SandboxExecutor,
	agentLoop *evolution.AgentLoop,
	currentScore int,
) *EvolutionConnector {
	return &EvolutionConnector{
		codeAgent:    codeAgent,
		sandboxExec:  sandboxExec,
		agentLoop:    agentLoop,
		currentScore: currentScore,
	}
}

// SuggestAndEvaluate is the main entry point:
// 1. CodeAgent generates a fix for the task
// 2. Sandbox tests the fix
// 3. Evolution engine evaluates and decides
// 4. UCB selector learns from the outcome
func (ec *EvolutionConnector) SuggestAndEvaluate(task Task, ctx Context) (*evolution.AgentResult, error) {
	// Generate a unique ID for this candidate
	candidateID := generateID()

	// Step 1: Get suggestion from CodeAgent
	resp, err := ec.codeAgent.Solve(task, ctx)
	if err != nil {
		return nil, fmt.Errorf("CodeAgent.Solve: %w", err)
	}

	log.Printf("EvolutionConnector: candidate=%s LLM confidence=%.2f", candidateID, ec.codeAgent.minConfidence)

	// Step 2: Run in sandbox
	sandboxRes := ec.sandboxExec.ExecuteWithSandbox(resp)

	if sandboxRes.Error != nil {
		log.Printf("EvolutionConnector: sandbox error: %v", sandboxRes.Error)
		// Even sandbox errors are learning events
		ec.agentLoop.Evaluate(
			evolution.BuildAgentCandidate(candidateID, task.Target, "", ec.codeAgent.minConfidence, "sandbox_error"),
			ec.currentScore,
			1.0,
		)
		return nil, fmt.Errorf("sandbox error: %w", sandboxRes.Error)
	}

	// Step 3: Build agent candidate with score
	candidate := evolution.BuildAgentCandidate(
		candidateID,
		task.Target,
		serializePlan(resp),
		ec.codeAgent.minConfidence,
		ec.agentLoop.SelectArm(), // Use current UCB-selected strategy
	)

	// Step 4: Evaluate with evolution engine
	result := ec.agentLoop.Evaluate(candidate, ec.currentScore, 1.0)

	log.Printf("EvolutionConnector: candidate=%s accepted=%v stability=%.2f reward=%.2f",
		candidateID, result.Accepted, result.StabilityScore, result.Reward)

	// Update current score for next iteration
	if result.Accepted {
		ec.currentScore = result.CycleResult.Score
	}

	return &result, nil
}

// UpdateScore allows external callers to update the current score.
func (ec *EvolutionConnector) UpdateScore(score int) {
	ec.currentScore = score
}

// GetUCBStats returns current UCB statistics.
func (ec *EvolutionConnector) GetUCBStats() map[string]evolution.UCBArmStats {
	return ec.agentLoop.UCBStats()
}

// generateID creates a unique ID for a candidate.
func generateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback : pseudo-random basé sur le temps si rand échoue
		b[0] = byte(time.Now().UnixNano() & 0xff)
		b[1] = byte((time.Now().UnixNano() >> 8) & 0xff)
	}
	return hex.EncodeToString(b)
}

// serializePlan converts a Plan to a string representation.
func serializePlan(plan Plan) string {
	var parts []string
	for i, step := range plan.Steps {
		parts = append(parts, fmt.Sprintf("step%d: %s -> %s", i, step.Kind, step.Target))
	}
	return strings.Join(parts, "; ")
}

// CodeAgentForEvolution wraps a CodeAgent with evolution engine integration.
type CodeAgentForEvolution struct {
	CodeAgent    *CodeAgent
	SandboxExec  *SandboxExecutor
	AgentLoop    *evolution.AgentLoop
	CurrentScore int
	ProjectRoot  string
}

// NewCodeAgentForEvolution creates a fully integrated agent.
func NewCodeAgentForEvolution(llm LLMClient, projectRoot string, agentLoop *evolution.AgentLoop) *CodeAgentForEvolution {
	return &CodeAgentForEvolution{
		CodeAgent:    NewCodeAgent(llm),
		SandboxExec:  NewSandboxExecutor(projectRoot, true),
		AgentLoop:    agentLoop,
		CurrentScore: 50, // Default starting score
		ProjectRoot:  projectRoot,
	}
}

// NewCodeAgentForEvolutionDefault creates a fully integrated agent with Nvidia LLM (default).
func NewCodeAgentForEvolutionDefault(projectRoot string, agentLoop *evolution.AgentLoop) *CodeAgentForEvolution {
	return NewCodeAgentForEvolution(NewDefaultLLMClient(), projectRoot, agentLoop)
}

// RunTask evaluates a task through the full pipeline.
func (cae *CodeAgentForEvolution) RunTask(task Task, ctx Context) (*evolution.AgentResult, error) {
	// Generate suggestion
	resp, err := cae.CodeAgent.Solve(task, ctx)
	if err != nil {
		return nil, err
	}

	// Test in sandbox
	sandboxRes := cae.SandboxExec.ExecuteWithSandbox(resp)
	if sandboxRes.Error != nil {
		return nil, sandboxRes.Error
	}

	// Compute candidate score from sandbox results
	candidateScore := computeSandboxScore(sandboxRes)

	// Create agent candidate
	candidate := evolution.BuildAgentCandidate(
		generateID(),
		task.Target,
		serializePlan(resp),
		cae.CodeAgent.minConfidence,
		cae.AgentLoop.SelectArm(),
	)

	// Evaluate through evolution engine
	result := cae.AgentLoop.Evaluate(candidate, candidateScore, 1.0)

	// Update score if accepted
	if result.Accepted {
		cae.CurrentScore = result.CycleResult.Score
	}

	return &result, nil
}

// computeSandboxScore converts sandbox results into an evolution score.
func computeSandboxScore(res SandboxResult) int {
	score := 0

	if res.Error == nil && !strings.Contains(res.BuildOutput, "error") {
		score += 30 // Build success
	}

	if res.Passed {
		score += 50 // All tests passing
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

// FixBrokenNode is a convenience method that finds and fixes a broken node.
func (cae *CodeAgentForEvolution) FixBrokenNode(ctx Context) (*evolution.AgentResult, error) {
	task := Task{
		Kind:   TaskFixBroken,
		Target: ctx.FilePath,
		Goal:   fmt.Sprintf("Fix broken node in %s", ctx.FilePath),
	}
	return cae.RunTask(task, ctx)
}