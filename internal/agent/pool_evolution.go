package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"pads-v3/internal/policy/evolution"
	"pads-v3/internal/semantic/memory"
)

// EvolutionConnectorPool runs N agents in parallel and returns the best result.
// It wraps AgentPool and provides the same SuggestAndEvaluate interface as
// EvolutionConnector, but with multi-agent competition via UCB.
type EvolutionConnectorPool struct {
	pool        *AgentPool
	projectRoot string
	semMem      *memory.SemanticMemory
	semMemOnce  sync.Once
	semMemErr   error
	mu          sync.RWMutex
}

// NewEvolutionConnectorPool creates a multi-agent connector with n parallel agents.
// Each agent has its own UCB arm so the pool learns which strategy works best.
// The first SuggestAndEvaluate call triggers the shared semantic memory index.
func NewEvolutionConnectorPool(n int, projectRoot string) *EvolutionConnectorPool {
	ec := &EvolutionConnectorPool{projectRoot: projectRoot}
	semMemGetter := func() *memory.SemanticMemory {
		ec.semMemOnce.Do(func() {
			ec.semMem, ec.semMemErr = memory.New(ec.projectRoot, ec.projectRoot+"/.pads/semantic")
			if ec.semMemErr == nil {
				ec.semMemErr = ec.semMem.IncrementallyIndex()
			}
			if ec.semMemErr != nil {
				log.Printf("[EvolutionConnectorPool] semantic memory init failed: %v", ec.semMemErr)
				ec.semMem = nil
			}
		})
		return ec.semMem
	}
	ec.pool = NewAgentPool(n, projectRoot, semMemGetter)
	return ec
}

// SuggestAndEvaluate runs all N agents in parallel and returns the best result.
// The best result is determined by highest score, with tie-breaking on fewest
// semantic reasons (simpler fix preferred).
// This is the main entry point — compatible with EvolutionConnector.SuggestAndEvaluate.
func (ec *EvolutionConnectorPool) SuggestAndEvaluate(task Task, ctx Context) (*evolution.AgentResult, error) {
	if task.Kind != TaskFixBroken {
		return nil, fmt.Errorf("EvolutionConnectorPool only handles TaskFixBroken, got %s", task.Kind)
	}

	results, err := ec.pool.RunAll(context.Background(), task, ctx)
	if err != nil {
		return nil, fmt.Errorf("pool.RunAll: %w", err)
	}

	// Filter out nil results.
	var valid []*evolution.AgentResult
	for _, r := range results {
		if r != nil {
			valid = append(valid, r)
		}
	}
	if len(valid) == 0 {
		return nil, fmt.Errorf("no agent produced a result")
	}

	// Pick best by score, tie-break by fewer reasons.
	best := valid[0]
	for _, r := range valid[1:] {
		if r.Score > best.Score || (r.Score == best.Score && len(r.SemanticReasons) < len(best.SemanticReasons)) {
			best = r
		}
	}

	log.Printf("[EvolutionConnectorPool] best result: arm=%s score=%d accepted=%v reasons=%d",
		best.UCBArm, best.Score, best.Accepted, len(best.SemanticReasons))

	return best, nil
}

// GetPoolStats returns UCB statistics for all agents in the pool.
func (ec *EvolutionConnectorPool) GetPoolStats() map[string]evolution.UCBArmStats {
	return ec.pool.PoolStats()
}

// PoolSize returns the number of agents in the pool.
func (ec *EvolutionConnectorPool) PoolSize() int {
	return ec.pool.Len()
}

// AllResults returns all individual agent results from the last RunAll.
// Useful for debugging or displaying per-agent outcomes.
func (ec *EvolutionConnectorPool) AllResults() []*evolution.AgentResult {
	results := make([]*evolution.AgentResult, ec.pool.Len())
	for i := 0; i < ec.pool.Len(); i++ {
		ec.pool.agents[i].mu.RLock()
		results[i] = ec.pool.agents[i].LastResult
		ec.pool.agents[i].mu.RUnlock()
	}
	return results
}

// BestArm returns the UCB arm (strategy) with the highest average reward so far.
// Returns "" if no arm has been evaluated yet.
func (ec *EvolutionConnectorPool) BestArm() string {
	stats := ec.GetPoolStats()
	var bestArm string
	var bestAvg float64 = -1
	for arm, stat := range stats {
		if stat.PullCount == 0 {
			continue
		}
		avg := stat.TotalReward / float64(stat.PullCount)
		if avg > bestAvg {
			bestAvg = avg
			bestArm = arm
		}
	}
	return bestArm
}

// FilterAgentsByPrefix returns agents whose strategy starts with the given prefix.
// Useful for testing or selective inspection.
func (ec *EvolutionConnectorPool) FilterAgentsByPrefix(prefix string) []*PooledAgent {
	var matching []*PooledAgent
	for _, a := range ec.pool.agents {
		if strings.HasPrefix(a.Strategy, prefix) {
			matching = append(matching, a)
		}
	}
	return matching
}