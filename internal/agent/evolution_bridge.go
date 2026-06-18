package agent

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"pads-v3/internal/codeanalysis/semantic"
	"pads-v3/internal/policy/evolution"
	"pads-v3/internal/semantic/memory"
)

// EvolutionConnector connects the CodeAgent to the evolution engine.
// It transforms agent suggestions into evolution candidates and evaluates them.
type EvolutionConnector struct {
	codeAgent    *CodeAgent
	sandboxExec  *SandboxExecutor
	agentLoop    *evolution.AgentLoop
	semMemory    *memory.SemanticMemory // lazily initialized
	semMemOnce   sync.Once
	semMemErr    error
	currentScore int
	projectRoot  string
}

// NewEvolutionConnector creates a new connector with all dependencies.
func NewEvolutionConnector(
	codeAgent *CodeAgent,
	sandboxExec *SandboxExecutor,
	agentLoop *evolution.AgentLoop,
	currentScore int,
	projectRoot string,
) *EvolutionConnector {
	return &EvolutionConnector{
		codeAgent:    codeAgent,
		sandboxExec:  sandboxExec,
		agentLoop:    agentLoop,
		currentScore: currentScore,
		projectRoot:  projectRoot,
	}
}

// getSemanticMemory lazily initializes and returns the shared SemanticMemory.
// The first call triggers the initial project index. Subsequent calls are no-ops.
func (ec *EvolutionConnector) getSemanticMemory() *memory.SemanticMemory {
	ec.semMemOnce.Do(func() {
		// Store memory dir under projectRoot so it lives alongside the project data
		ec.semMemory, ec.semMemErr = memory.New(ec.projectRoot, ec.projectRoot+"/.pads/semantic")
		if ec.semMemErr == nil {
			ec.semMemErr = ec.semMemory.IncrementallyIndex()
		}
		if ec.semMemErr != nil {
			log.Printf("[evolution_bridge] getSemanticMemory: failed to init memory: %v", ec.semMemErr)
			ec.semMemory = nil
		}
	})
	return ec.semMemory
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
				0.0,      // semantic risk unknown
				[]string{"sandbox execution failed"},
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

	// Step 3b: Run lazy semantic analysis on the target file (if available)
	semanticRisk, semanticReasons, modImpact := runSemanticAnalysis(ec.projectRoot, task.Target, resp, ec.getSemanticMemory())
	candidate.ModImpact = modImpact

	// Step 4: Evaluate with evolution engine
	result := ec.agentLoop.Evaluate(candidate, ec.currentScore, 1.0, semanticRisk, semanticReasons)

	log.Printf("EvolutionConnector: candidate=%s accepted=%v stability=%.2f reward=%.2f semantic_risk=%.2f",
		candidateID, result.Accepted, result.StabilityScore, result.Reward, result.SemanticRisk)

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
	semMemory    *memory.SemanticMemory
	semMemOnce   sync.Once
	semMemErr    error
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

// getSemanticMemory lazily initializes and returns the shared SemanticMemory.
func (cae *CodeAgentForEvolution) getSemanticMemory() *memory.SemanticMemory {
	cae.semMemOnce.Do(func() {
		cae.semMemory, cae.semMemErr = memory.New(cae.ProjectRoot, cae.ProjectRoot+"/.pads/semantic")
		if cae.semMemErr == nil {
			cae.semMemErr = cae.semMemory.IncrementallyIndex()
		}
		if cae.semMemErr != nil {
			log.Printf("[CodeAgentForEvolution] getSemanticMemory: failed to init memory: %v", cae.semMemErr)
			cae.semMemory = nil
		}
	})
	return cae.semMemory
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

	// Lazy semantic analysis on the first file touched by the plan
	semanticRisk, semanticReasons, modImpact := runSemanticAnalysis(cae.ProjectRoot, task.Target, resp, cae.getSemanticMemory())
	candidate.ModImpact = modImpact

	// Evaluate through evolution engine
	result := cae.AgentLoop.Evaluate(candidate, candidateScore, 1.0, semanticRisk, semanticReasons)

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

// runSemanticAnalysis extracts the first file target from the plan and runs
// the semantic analyzer on it. Returns (risk 0-1, reasons, modImpact 0-1).
// If semMem is non-nil, it also queries the global call graph (CallersOf,
// CalleesOf, SymbolImpact) to compute a richer risk score.
// It is lazy — only runs when a target file is present and is a Go file.
// This avoids the overhead of AST parsing when the agent only runs commands.
func runSemanticAnalysis(projectRoot, targetFile string, plan Plan, semMem *memory.SemanticMemory) (risk float64, reasons []string, modImpact float64) {
	// Use the explicit target if available, otherwise fall back to first write action
	filePath := targetFile
	if filePath == "" {
		for _, step := range plan.Steps {
			if step.Kind == ActionWriteFile && step.Target != "" {
				filePath = step.Target
				break
			}
		}
	}
	if filePath == "" {
		return 0.0, []string{"no target file identified"}, 0.0
	}
	if !strings.HasSuffix(filePath, ".go") {
		return 0.0, []string{"non-Go file, skipped"}, 0.0
	}

	analyzer := semantic.NewAnalyzer(projectRoot)
	sum, err := analyzer.AnalyzeFile(filePath)
	if err != nil {
		reasons = []string{fmt.Sprintf("semantic analysis failed: %v", err)}
		return 0.5, reasons, 0.5
	}

	// RiskScore (0-1) is our primary risk metric
	risk = sum.RiskScore
	reasons = sum.RiskReasons

	// Global call graph queries — enrich risk for every exported symbol found.
	if semMem != nil && len(sum.Symbols) > 0 {
		for _, sym := range sum.Symbols {
			if !sym.Exported {
				continue
			}
			symRisk := 0.0

			// 1. External callers (regression risk): changing this breaks callers.
			if callers, err := semMem.CallersOf(sym.Name, ""); err == nil && callers != nil {
				n := len(callers)
				if n > 0 {
					reasons = append(reasons, fmt.Sprintf("exported symbol %q has %d external caller(s)", sym.Name, n))
					symRisk += float64(min(n, 10)) * 0.05
				}
			}

			// 2. Callees (blast radius): this symbol calls N others — high N means
			//    wide downstream impact if its logic changes.
			if callees, err := semMem.CalleesOf(sym.Name, ""); err == nil && callees != nil {
				n := len(callees)
				if n > 3 {
					reasons = append(reasons, fmt.Sprintf("exported symbol %q calls %d other symbols (wide blast radius)", sym.Name, n))
					symRisk += float64(min(n-3, 7)) * 0.02
				}
			}

			// 3. Full impact: direct + transitive callers via SymbolImpact.
			if direct, transitive, err := semMem.SymbolImpact(sym.Name, ""); err == nil && (direct > 0 || transitive > 0) {
				reasons = append(reasons, fmt.Sprintf("symbol %q has %d direct and %d transitive callers", sym.Name, direct, transitive))
				symRisk += float64(min(transitive, 5)) * 0.03
				// Use SymbolImpact as modImpact — reflects how widely used this symbol is.
				symbolImpactScore := float64(direct+transitive) / 10.0
				modImpact = max(modImpact, min(1.0, symbolImpactScore))
			}

			risk = min(1.0, risk+symRisk)
		}
	}

	// Fallback modImpact when no SemanticMemory available.
	if modImpact == 0 {
		modImpact = risk
	}
	return risk, reasons, modImpact
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