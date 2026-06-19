package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"pads-v3/internal/policy/evolution"
	"pads-v3/internal/semantic/memory"
)

// DefaultAgentStrategies are the UCB arm names for the default 3-agent pool.
var DefaultAgentStrategies = []string{"greedy", "exploratory", "conservative"}

// PooledAgent is a single agent in the pool, with its own CodeAgent, sandbox,
// evolution loop, and UCB selector.
type PooledAgent struct {
	ID          string
	Strategy    string // UCB arm name
	CodeAgent   *CodeAgent
	SandboxExec *SandboxExecutor
	Loop        *evolution.AgentLoop
	LastResult  *evolution.AgentResult
	mu          sync.RWMutex
}

// AgentPool manages N parallel CodeAgents that compete via UCB.
// Each agent has its own UCB arm so the pool learns which strategy works best.
type AgentPool struct {
	agents         []*PooledAgent
	sharedLoop     *evolution.SafeEvolutionLoopV3
	sharedRewarder evolution.Rewarder
	semMemGetter   func() *memory.SemanticMemory // lazily initialized shared memory
	poolMu         sync.RWMutex
}

// NewAgentPool creates a pool of n agents, each with a distinct strategy.
// All agents share the same SafeEvolutionLoopV3 so evolution state is shared,
// but each has its own UCB selector to track per-strategy performance.
// The semMemGetter is called lazily on first use to get the shared SemanticMemory.
func NewAgentPool(n int, projectRoot string, semMemGetter func() *memory.SemanticMemory) *AgentPool {
	if n < 1 {
		n = 1
	}
	if n > 8 {
		n = 8 // hard cap to avoid resource exhaustion
	}

	// Nil semMemGetter is safe — returns nil (semantic analysis skipped).
	if semMemGetter == nil {
		semMemGetter = func() *memory.SemanticMemory { return nil }
	}

	// Use default strategies if not enough custom ones.
	strategies := DefaultAgentStrategies
	for len(strategies) < n {
		strategies = append(strategies, "strategy_"+string(rune('A'+len(strategies))))
	}

	// Shared evolution loop — all agents write to the same WAL/state.
	sharedLoop := evolution.NewSafeEvolutionLoopV3Minimal(evolution.ModeBandit, nil)
	rewarder := evolution.DeltaRewarder{}

	agents := make([]*PooledAgent, 0, n)
	for i := 0; i < n; i++ {
		strategy := strategies[i]
		codeAgent := NewCodeAgent(NewDefaultLLMClient())
		sandboxExec := NewSandboxExecutor(projectRoot, true)
		persistPath := fmt.Sprintf(".pads/ucb_%s.json", strategy)
		selector := evolution.NewUCBSelector(int64(i)+time.Now().UnixNano(), persistPath)
		selector.AddArm(strategy)
		selector.EnableAutoSave(30 * time.Second)

		agentLoop := evolution.NewAgentLoop(sharedLoop, selector, rewarder)
		agents = append(agents, &PooledAgent{
			ID:          strategy,
			Strategy:    strategy,
			CodeAgent:   codeAgent,
			SandboxExec: sandboxExec,
			Loop:        agentLoop,
		})
	}

	return &AgentPool{
		agents:         agents,
		sharedLoop:     sharedLoop,
		sharedRewarder: rewarder,
		semMemGetter:   semMemGetter,
	}
}

// RunAll runs all agents in parallel for the given task and returns all results.
// Each agent independently generates a candidate, runs it in its sandbox,
// evaluates through its own AgentLoop (which shares the SafeEvolutionLoopV3),
// and returns its result. Results are ordered by agent index (deterministic).
func (ap *AgentPool) RunAll(ctx context.Context, task Task, ctxContext Context) ([]*evolution.AgentResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	type result struct {
		idx    int
		result *evolution.AgentResult
		err    error
	}

	resCh := make(chan result, len(ap.agents))
	var wg sync.WaitGroup

	for i, agent := range ap.agents {
		agent := agent // capture loop var
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			r := result{idx: idx}

			semMem := ap.semMemGetter()

			// Enrich context: inject source code and semantic context.
			agentCtx := ctxContext
			agentCtx.SemMem = semMem
			agentCtx.Enrich(task.Target, 200)

			// Step 1: generate plan
			plan, err := agent.CodeAgent.Solve(task, agentCtx)
			if err != nil {
				r.err = err
				r.result = &evolution.AgentResult{
					UCBArm: agent.Strategy,
					Reason: "CodeAgent.Solve failed: " + err.Error(),
				}
				slog.Warn("agent Solve failed", "strategy", agent.Strategy, "error", err)
				resCh <- r
				return
			}

			// Step 2: sandbox execution
			sandboxRes := agent.SandboxExec.ExecuteWithSandbox(plan)
			if sandboxRes.Error != nil {
				// Even failures are learning events.
				r.err = sandboxRes.Error
				evResult := agent.Loop.Evaluate(
					evolution.BuildAgentCandidate(agent.Strategy+"_"+task.Target, task.Target, SerializePlan(plan), 0, agent.Strategy),
					50, // default starting score
					1.0,
					0.5, // semantic risk unknown
					[]string{"sandbox error: " + sandboxRes.Error.Error()},
				)
				evResult.Reason = "sandbox_error: " + sandboxRes.Error.Error()
				r.result = &evResult
				slog.Warn("agent sandbox failed", "strategy", agent.Strategy, "error", sandboxRes.Error)
				resCh <- r
				return
			}

			// Step 3: semantic analysis (uses shared SemanticMemory if available)
			semanticRisk, semanticReasons, modImpact := runSemanticAnalysis(ap.agents[0].SandboxExec.ProjectRoot(), task.Target, plan, semMem)

			// Step 4: build candidate and evaluate
			candidate := evolution.BuildAgentCandidate(
				agent.Strategy+"_"+task.Target,
				task.Target,
				SerializePlan(plan),
				agent.CodeAgent.MinConfidence(),
				agent.Strategy,
			)
			candidate.ModImpact = modImpact

			evResult := agent.Loop.Evaluate(candidate, 50, 1.0, semanticRisk, semanticReasons)
			r.result = &evResult
			resCh <- r
		}(i)
	}

	go func() {
		wg.Wait()
		close(resCh)
	}()

	// Collect results maintaining original order.
	results := make([]*evolution.AgentResult, len(ap.agents))
	for res := range resCh {
		results[res.idx] = res.result
	}

	// Update LastResult for each agent.
	for i, r := range results {
		ap.agents[i].mu.Lock()
		ap.agents[i].LastResult = r
		ap.agents[i].mu.Unlock()
	}

	return results, nil
}

// BestResult returns the agent result with the highest score from the last RunAll.
// If scores are equal, prefers the result with fewer semantic reasons (simpler fix).
// Returns nil if RunAll has never been called.
func (ap *AgentPool) BestResult() *evolution.AgentResult {
	ap.poolMu.RLock()
	defer ap.poolMu.RUnlock()

	var best *evolution.AgentResult
	for _, agent := range ap.agents {
		agent.mu.RLock()
		r := agent.LastResult
		agent.mu.RUnlock()
		if r == nil {
			continue
		}
		if best == nil || r.Score > best.Score ||
			(r.Score == best.Score && len(r.SemanticReasons) < len(best.SemanticReasons)) {
			best = r
		}
	}
	return best
}

// PoolStats returns UCB statistics for all agents in the pool.
func (ap *AgentPool) PoolStats() map[string]evolution.UCBArmStats {
	stats := make(map[string]evolution.UCBArmStats)
	for _, agent := range ap.agents {
		agent.mu.RLock()
		ucbStats := agent.Loop.UCBStats()
		agent.mu.RUnlock()
		stats[agent.Strategy] = ucbStats[agent.Strategy]
	}
	return stats
}

// Len returns the number of agents in the pool.
func (ap *AgentPool) Len() int {
	return len(ap.agents)
}

// AgentAt returns the agent at index i. Panics if i is out of range.
func (ap *AgentPool) AgentAt(i int) *PooledAgent {
	return ap.agents[i]
}
