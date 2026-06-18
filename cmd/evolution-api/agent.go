package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"pads-v3/internal/policy/evolution"
)

// agentResponse is the API response for agent-based evolution.
type agentResponse struct {
	CandidateID   string  `json:"candidate_id"`
	Accepted      bool    `json:"accepted"`
	Score         int     `json:"score"`
	Confidence    float64 `json:"confidence"`
	Stability     float64 `json:"stability_score"`
	Reason        string  `json:"reason"`
	UCBArm        string  `json:"ucb_arm"`
	Reward        float64 `json:"reward"`
	SandboxPassed bool    `json:"sandbox_passed"`
}

// handleAgentEvolve is the /agent/evolve endpoint.
// It accepts a code modification from the CodeAgent and evaluates it
// through the evolution engine.
func (s *Server) handleAgentEvolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Body == nil {
		http.Error(w, "Empty body", http.StatusBadRequest)
		return
	}

	var req struct {
		TargetFile string  `json:"target_file"`
		Patch      string  `json:"patch"`
		Confidence float64 `json:"confidence"`
		Mode       string  `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.TargetFile == "" || req.Patch == "" {
		http.Error(w, "target_file and patch are required", http.StatusBadRequest)
		return
	}

	validModes := map[string]bool{"stable": true, "bandit": true, "locked": true}
	mode := evolution.ModeStable
	if req.Mode != "" && validModes[req.Mode] {
		mode = evolution.Mode(req.Mode)
	}

	// Get current score from system state
	currentScore := s.getCurrentScore()

	// Build agent candidate
	candidateID := generateAgentID()
	ac := evolution.BuildAgentCandidate(
		candidateID,
		req.TargetFile,
		req.Patch,
		req.Confidence,
		s.selector.Select(),
	)

	// Evaluate through the loop
	result, accepted, err := s.evaluateAgentCandidate(ac, currentScore, 1.0, mode)
	if err != nil {
		slog.Error("agent evolve error", "error", err)
		http.Error(w, "Evolution error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := agentResponse{
		CandidateID:   candidateID,
		Accepted:      accepted,
		Score:         result.Score,
		Confidence:    req.Confidence,
		Stability:     result.StabilityScore,
		Reason:        result.Reason,
		UCBArm:        result.UCBArm,
		Reward:        result.Reward,
		SandboxPassed: accepted,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleAgentStatus returns the current UCB statistics and agent status.
func (s *Server) handleAgentStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := make(map[string]interface{})
	arm := s.selector.Select()
	stats["selected_arm"] = arm

	// Get score from selector arms
	if ucb, ok := s.selector.(*evolution.UCBSelector); ok {
		armStats := make(map[string]map[string]interface{})
		arms := ucb.Arms()
		counts := ucb.Counts()
		for _, name := range ucb.Names() {
			armStats[name] = map[string]interface{}{
				"total_reward": arms[name],
				"pull_count":   counts[name],
				"avg_reward":   arms[name] / float64(counts[name]),
			}
		}
		stats["arms"] = armStats
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleAgentStrategies lists available agent strategies for UCB selection.
func (s *Server) handleAgentStrategies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			Strategy string `json:"strategy"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Strategy != "" {
			s.selector.AddArm(req.Strategy)
			slog.Info("added new agent strategy", "strategy", req.Strategy)
		}
	}

	// List all strategies
	arms := s.selector.ListArms()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"strategies": arms})
}

// evaluateAgentCandidate runs a candidate through the evolution loop.
func (s *Server) evaluateAgentCandidate(
	candidate *evolution.AgentCandidate,
	currentScore int,
	weight float64,
	mode evolution.Mode,
) (*evolution.AgentResult, bool, error) {
	// Create a simple loop for agent evaluation
	loop := buildAgentLoop(s.selector, mode)

	evCandidate := evolution.Candidate{Score: candidate.Score()}
	evCurrent := evolution.Candidate{Score: currentScore}

	cycleResult, accepted, err := loop.Evolve(evCandidate, evCurrent, weight)
	if err != nil {
		return nil, false, err
	}

	// Compute reward
	stabilityBefore := float64(currentScore)
	stabilityAfter := float64(cycleResult.Score)
	arm := s.selector.Select()
	reward := computeReward(stabilityBefore, stabilityAfter, accepted)
	s.selector.Update(arm, reward)

	return &evolution.AgentResult{
		CandidateID:   candidate.ID,
		Score:         cycleResult.Score,
		Accepted:      accepted,
		CycleResult:   cycleResult,
		StabilityScore: stabilityAfter,
		Reason:        evolution.BuildReason(accepted, candidate.Score(), currentScore, stabilityAfter-stabilityBefore),
		UCBArm:        arm,
		Reward:        reward,
	}, accepted, nil
}

// getCurrentScore queries the current system score.
func (s *Server) getCurrentScore() int {
	// Load events to compute current score
	events, err := s.queue.LoadAll()
	if err != nil || len(events) == 0 {
		return 50 // Default score
	}

	// Get the last accepted event's score as current
	for i := len(events) - 1; i >= 0; i-- {
		if strings.Contains(events[i].Type, "stable") || events[i].Type == "evolve" {
			return events[i].Current
		}
	}
	return 50
}

// buildAgentLoop creates an evolution loop for agent evaluation.
func buildAgentLoop(selector evolution.Selector, mode evolution.Mode) *evolution.SafeEvolutionLoopV3 {
	orch := evolution.NewOrchestrator(
		evolution.NewMultiCycleEvaluator(),
		evolution.NewStabilityGate(),
	)
	es := evolution.NewEventStore("evolution-agent.log")
	wal := evolution.NewWAL()
	detector := evolution.NewAntiCollapseDetector(5, 10.0)
	return evolution.NewSafeEvolutionLoopV3(orch, es, wal, detector, mode, selector)
}

// computeReward calculates the reward for UCB update.
func computeReward(oldStability, newStability float64, accepted bool) float64 {
	if !accepted {
		return 0
	}
	return newStability - oldStability
}

// generateAgentID creates a unique ID for an agent candidate.
func generateAgentID() string {
	b := make([]byte, 8)
	readRand(b)
	return hexEncode(b)
}

// readRand fills b with random bytes (placeholder - use crypto/rand in production).
func readRand(b []byte) {
	for i := range b {
		b[i] = byte(i*17%256 + i)
	}
}

func hexEncode(b []byte) string {
	const hexChars = "0123456789abcdef"
	result := make([]byte, len(b)*2)
	for i, v := range b {
		result[i*2] = hexChars[v>>4]
		result[i*2+1] = hexChars[v&0x0f]
	}
	return string(result)
}