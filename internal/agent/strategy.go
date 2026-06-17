package agent

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
)

// AgentStrategy defines how the CodeAgent formulates prompts and approaches tasks.
// UCB selects the best strategy based on historical rewards.
type AgentStrategy struct {
	Name        string  // "conservative", "aggressive", "test-first", "minimal-change"
	Temperature float64 // LLM temperature (0-1)
	SystemPrompt string  // System prompt for this strategy
	Description  string  // Human-readable description
}

// StrategyRegistry holds all available agent strategies.
type StrategyRegistry struct {
	strategies map[string]*AgentStrategy
	rng        *rand.Rand
}

// DefaultStrategies returns the standard set of agent strategies.
func DefaultStrategies() *StrategyRegistry {
	registry := &StrategyRegistry{
		strategies: make(map[string]*AgentStrategy),
		rng:        rand.New(rand.NewSource(42)),
	}

	registry.register(AgentStrategy{
		Name:        "conservative",
		Temperature: 0.1,
		Description: "Minimal, safe changes. Prefers small patches over large rewrites.",
		SystemPrompt: `You are a CONSERVATIVE code repair agent. Your priority is SAFETY and MINIMALISM.

Rules:
- Make the SMALLEST change possible to fix the issue
- NEVER introduce new features or refactor unrelated code
- Prefer adding nil checks, error handling, and guard clauses
- If unsure, leave it as-is rather than guessing
- Output only the exact code change needed`,
	})

	registry.register(AgentStrategy{
		Name:        "aggressive",
		Temperature: 0.7,
		Description: "Bold refactoring when simple fixes won't work.",
		SystemPrompt: `You are an AGGRESSIVE code improvement agent. You are NOT afraid to refactor.

Rules:
- If a simple fix is insufficient, refactor boldly
- Extract duplicated logic into helpers
- Improve naming and structure even if not strictly required
- Consider performance implications
- Don't shy away from multi-file changes if needed`,
	})

	registry.register(AgentStrategy{
		Name:        "test-first",
		Temperature: 0.3,
		Description: "Writes tests first, then fixes to make tests pass.",
		SystemPrompt: `You are a TEST-FIRST code repair agent. Tests are your source of truth.

Rules:
- Analyze the failing test to understand EXACTLY what is expected
- Write a minimal failing test case that reproduces the bug
- Then fix the code to make all tests pass
- Never break existing passing tests
- If you must change test code, add tests but never remove passing ones`,
	})

	registry.register(AgentStrategy{
		Name:        "minimal-change",
		Temperature: 0.15,
		Description: "Ultra-minimal changes - only what's strictly necessary.",
		SystemPrompt: `You are a MINIMAL-CHANGE agent. Every character you add is a risk.

Rules:
- Only change exactly what is broken
- Add nothing: no comments, no formatting, no improvements
- Preserve original code style exactly
- If the file has 4-space indent, you MUST use 4-space indent
- Zero tolerance for introducing new code patterns`,
	})

	return registry
}

// register adds a strategy to the registry.
func (r *StrategyRegistry) register(s AgentStrategy) {
	r.strategies[s.Name] = &s
}

// Get returns a strategy by name, or nil if not found.
func (r *StrategyRegistry) Get(name string) *AgentStrategy {
	return r.strategies[name]
}

// Names returns all registered strategy names.
func (r *StrategyRegistry) Names() []string {
	names := make([]string, 0, len(r.strategies))
	for name := range r.strategies {
		names = append(names, name)
	}
	return names
}

// Random returns a random strategy (for initial exploration).
func (r *StrategyRegistry) Random() *AgentStrategy {
	names := r.Names()
	name := names[r.rng.Intn(len(names))]
	return r.strategies[name]
}

// SelectByTemperature selects strategies probabilistically based on temperature.
// Hotter = more likely to pick aggressive; colder = more conservative.
func (r *StrategyRegistry) SelectByTemperature(temp float64) *AgentStrategy {
	// Simple deterministic selection based on temp ranges
	switch {
	case temp < 0.2:
		return r.strategies["conservative"]
	case temp < 0.4:
		return r.strategies["minimal-change"]
	case temp < 0.6:
		return r.strategies["test-first"]
	default:
		return r.strategies["aggressive"]
	}
}

// BuildPromptForStrategy builds a CodePrompt tailored to the given strategy.
func BuildPromptForStrategy(task Task, ctx Context, strategy *AgentStrategy) CodePrompt {
	var constraints strings.Builder
	constraints.WriteString("- " + strategy.Description + "\n")

	var context strings.Builder
	if ctx.FilePath != "" {
		context.WriteString(fmt.Sprintf("File: %s\n", ctx.FilePath))
	}
	if ctx.PackagePath != "" {
		context.WriteString(fmt.Sprintf("Package: %s\n", ctx.PackagePath))
	}
	if ctx.NodeID != "" {
		context.WriteString(fmt.Sprintf("Node ID: %s\n", ctx.NodeID))
	}

	// Add recent L2 events if available
	if len(ctx.L2Events) > 0 {
		context.WriteString("\nRecent events:\n")
		for i, ev := range ctx.L2Events {
			if i >= 3 {
				break
			}
			context.WriteString(fmt.Sprintf("  - %s: %s\n", ev.EventType, ev.Payload))
		}
	}

	return CodePrompt{
		Task:       task.Goal,
		FilePath:   task.Target,
		Language:   detectLanguage(task.Target),
		Context:    context.String(),
		Constraints: constraints.String(),
	}
}

// ApplyStrategyToLLM applies a strategy's temperature to the LLM client.
// This is a no-op in current implementation but allows future per-request temp control.
func ApplyStrategyToLLM(llm LLMClient, strategy *AgentStrategy) LLMClient {
	// Currently temperature is per-call in GenerateCode
	// Future: wrap client to inject temperature per request
	return llm
}

// StrategyForUCBArm returns the AgentStrategy matching the UCB arm name.
// Falls back to "conservative" if not found.
func StrategyForUCBArm(arm string) *AgentStrategy {
	registry := DefaultStrategies()
	if s := registry.Get(arm); s != nil {
		return s
	}
	return registry.Get("conservative")
}

// StrategyAdapter wraps a CodeAgent to use UCB-selected strategies.
type StrategyAdapter struct {
	Agent       *CodeAgent
	Registry    *StrategyRegistry
	Selector    StrategySelector
}

// StrategySelector chooses which strategy to use based on UCB state.
type StrategySelector interface {
	Select() string // Returns strategy name
	Update(name string, reward float64)
}

// NewStrategyAdapter creates a CodeAgent adapter that uses UCB-selected strategies.
func NewStrategyAdapter(selector StrategySelector) *StrategyAdapter {
	return &StrategyAdapter{
		Agent:       NewCodeAgentDefault(),
		Registry:    DefaultStrategies(),
		Selector:    selector,
	}
}

// SolveWithStrategy solves a task using the UCB-selected strategy.
func (sa *StrategyAdapter) SolveWithStrategy(task Task, ctx Context) (*CodeResponse, *AgentStrategy, error) {
	// Select strategy via UCB
	strategyName := sa.Selector.Select()
	strategy := sa.Registry.Get(strategyName)
	if strategy == nil {
		strategy = sa.Registry.Get("conservative")
	}

	// Build prompt with strategy
	prompt := BuildPromptForStrategy(task, ctx, strategy)

	// Query LLM
	resp, err := sa.Agent.llm.GenerateCode(context.Background(), prompt)
	if err != nil {
		return nil, strategy, err
	}

	return resp, strategy, nil
}

// UpdateStrategyReward updates the UCB selector with the outcome.
func (sa *StrategyAdapter) UpdateStrategyReward(reward float64) {
	strategyName := sa.Selector.Select()
	sa.Selector.Update(strategyName, reward)
}

// GetStrategyStats returns statistics for all strategies.
func (sa *StrategyAdapter) GetStrategyStats() map[string]StrategyStats {
	stats := make(map[string]StrategyStats)
	for _, name := range sa.Registry.Names() {
		s := sa.Registry.Get(name)
		stats[name] = StrategyStats{
			Name:        s.Name,
			Description: s.Description,
			Temperature: s.Temperature,
		}
	}
	return stats
}

// StrategyStats holds statistics for a strategy.
type StrategyStats struct {
	Name        string
	Description string
	Temperature float64
	TotalReward float64
	PullCount   int
	AvgReward   float64
}