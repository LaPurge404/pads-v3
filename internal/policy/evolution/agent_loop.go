package evolution

import (
	"fmt"
	"time"
)

// AgentCandidate represents a code modification proposed by an agent.
// It wraps the agent's suggestion with metadata needed for evolution evaluation.
type AgentCandidate struct {
	// Unique identifier for this suggestion
	ID string
	// Target file path
	TargetFile string
	// The proposed code change (full content or patch)
	Patch string
	// LLM confidence in the suggestion (0-1)
	Confidence float64
	// Number of retries attempted by the agent
	Retries int
	// Agent strategy used (for UCB learning)
	Strategy string
	// Timestamp when the candidate was created
	CreatedAt time.Time
	// ModImpact encodes the semantic impact of the change (0.0-1.0).
	// Used to weight the score: signature changes on public APIs → high impact.
	ModImpact float64
}

// AgentResult represents the outcome of evaluating an AgentCandidate.
type AgentResult struct {
	CandidateID    string
	Score          int // evolution score after evaluation
	Accepted       bool
	CycleResult    CycleResult
	StabilityScore float64
	Reason         string
	UCBArm         string
	Reward         float64
	// Semantic analysis of the modification (populated by the agent bridge)
	SemanticRisk    float64  // 0.0 (safe) → 1.0 (high risk public API change)
	SemanticReasons []string // human-readable risk factors
}

// BuildAgentCandidate creates a new AgentCandidate from agent output.
func BuildAgentCandidate(id, targetFile, patch string, confidence float64, strategy string) *AgentCandidate {
	return &AgentCandidate{
		ID:         id,
		TargetFile: targetFile,
		Patch:      patch,
		Confidence: confidence,
		Retries:    0,
		Strategy:   strategy,
		CreatedAt:  time.Now(),
	}
}

// Score returns the raw confidence score (0-100) for this candidate.
func (ac *AgentCandidate) Score() int {
	return int(ac.Confidence * 100)
}

// WeightedScore returns a risk-adjusted score (0-130) for evolution ranking.
// High-impact public API changes get a lower effective score (risk-averse).
// Low-impact internal refactorings get a slightly higher score.
func (ac *AgentCandidate) WeightedScore() int {
	base := ac.Confidence * 100
	// ModImpact 0.0 = minimal change (comment, import) → bonus
	// ModImpact 0.5 = medium change (internal body) → neutral
	// ModImpact 1.0 = high-risk change (exported signature) → penalty
	multiplier := 1.0 - ac.ModImpact*0.5 + 0.5 // range: 0.5x (max risk) to 1.5x (min risk, but capped at 1.3)
	if multiplier < 0.5 {
		multiplier = 0.5
	}
	if multiplier > 1.3 {
		multiplier = 1.3
	}
	return int(base * multiplier)
}

// ToQueueEvent converts an AgentCandidate to a QueueEvent for the evolution queue.
// It uses WeightedScore() so high-risk public API changes are ranked lower.
func (ac *AgentCandidate) ToQueueEvent(currentScore int, weight float64, mode Mode) QueueEvent {
	return QueueEvent{
		ID:        ac.ID,
		Type:      "agent_" + string(mode), // tag as agent-driven
		Candidate: ac.WeightedScore(),      // risk-adjusted score
		Current:   currentScore,
		Weight:    weight * ac.Confidence, // boost weight based on confidence
		Mode:      mode,
		Metadata: map[string]string{
			"strategy":   ac.Strategy,
			"confidence": fmt.Sprintf("%.2f", ac.Confidence),
			"target":     ac.TargetFile,
			"mod_impact": fmt.Sprintf("%.2f", ac.ModImpact),
		},
	}
}

// computeScore converts agent metrics into an evolution-compatible score.
// Score = confidence * 100 (normalized to 0-100)
func (ac *AgentCandidate) computeScore() int {
	return int(ac.Confidence * 100)
}

// AgentLoop integrates CodeAgent with the evolution engine.
// It evaluates agent suggestions through the full evolution pipeline.
type AgentLoop struct {
	loop     *SafeEvolutionLoopV3
	selector *UCBSelector
	rewarder Rewarder
}

// NewAgentLoop creates a new AgentLoop with required dependencies.
func NewAgentLoop(
	loop *SafeEvolutionLoopV3,
	selector *UCBSelector,
	rewarder Rewarder,
) *AgentLoop {
	return &AgentLoop{
		loop:     loop,
		selector: selector,
		rewarder: rewarder,
	}
}

// Evaluate runs the agent candidate through the evolution engine and updates UCB.
// semanticRisk and semanticReasons come from the codeanalysis/semantic package and
// are used to populate AgentResult for richer feedback to the caller.
func (al *AgentLoop) Evaluate(candidate *AgentCandidate, currentScore int, weight float64, semanticRisk float64, semanticReasons []string) AgentResult {
	// Convert to evolution candidate
	evCandidate := Candidate{Score: candidate.Score()}
	evCurrent := Candidate{Score: currentScore}

	// Select arm based on current exploration strategy
	arm := al.selector.Select()

	// Update candidate with selected strategy
	candidate.Strategy = arm

	// Run evolution evaluation
	accepted, err := al.loop.Evolve(evCandidate, evCurrent, weight)
	if err != nil {
		return AgentResult{
			CandidateID:     candidate.ID,
			Accepted:        false,
			Reason:          err.Error(),
			UCBArm:          arm,
			SemanticRisk:    semanticRisk,
			SemanticReasons: semanticReasons,
		}
	}

	// Compute reward for UCB
	stabilityBefore := float64(currentScore)
	stabilityAfter := al.loop.StabilityScore()
	reward := al.rewarder.ComputeReward(stabilityBefore, stabilityAfter, accepted)

	// Update UCB selector with the outcome
	al.selector.Update(arm, reward)

	// Get current stability score
	stabilityScore := al.loop.StabilityScore()

	return AgentResult{
		CandidateID:     candidate.ID,
		Score:           int(stabilityScore),
		Accepted:        accepted,
		StabilityScore:  stabilityScore,
		Reason:          BuildReason(accepted, candidate.Score(), currentScore, stabilityAfter-stabilityBefore),
		UCBArm:          arm,
		Reward:          reward,
		SemanticRisk:    semanticRisk,
		SemanticReasons: semanticReasons,
	}
}

// SelectArm returns the currently selected UCB arm for the next agent strategy.
func (al *AgentLoop) SelectArm() string {
	return al.selector.Select()
}

// AddArm registers a new strategy arm for UCB selection.
func (al *AgentLoop) AddArm(name string) {
	al.selector.AddArm(name)
}

// UCBStats returns current UCB statistics for all arms.
func (al *AgentLoop) UCBStats() map[string]UCBArmStats {
	stats := make(map[string]UCBArmStats)
	for _, name := range al.selector.Names() {
		arms := al.selector.Arms()
		counts := al.selector.Counts()
		avg := 0.0
		if counts[name] > 0 {
			avg = arms[name] / float64(counts[name])
		}
		stats[name] = UCBArmStats{
			TotalReward: arms[name],
			PullCount:   counts[name],
			AvgReward:   avg,
		}
	}
	return stats
}

// UCBArmStats holds statistics for a single UCB arm.
type UCBArmStats struct {
	TotalReward float64
	PullCount   int
	AvgReward   float64
}
