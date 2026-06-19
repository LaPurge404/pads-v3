package learner

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"pads-v3/internal/policy"
	"pads-v3/internal/policy/wal"
)

type Learner struct {
	mu           sync.Mutex
	LearningRate float64
	MinSamples   int
	MinInterval  time.Duration
	lastLearn    time.Time
	Detector     *AnomalyDetector
}

func NewLearner() *Learner {
	return &Learner{
		LearningRate: 0.1,
		MinSamples:   5,
		MinInterval:  30 * time.Second,
		Detector:     NewAnomalyDetector(),
	}
}

func (l *Learner) ShouldLearn() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.lastLearn.IsZero() {
		return true
	}
	return time.Since(l.lastLearn) >= l.MinInterval
}

func (l *Learner) LearnFromWAL(walDB *wal.PolicyWAL, currentConfig TunedConfig) (*TunedConfig, *LearningReport, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.shouldLearnLocked() {
		return nil, nil, fmt.Errorf("learner: too soon since last learn")
	}

	events, err := walDB.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("read WAL: %w", err)
	}
	if len(events) < l.MinSamples {
		return nil, nil, fmt.Errorf("not enough samples: %d < %d", len(events), l.MinSamples)
	}

	l.Detector.Replay(events)
	traces := wal.LoadTraces(events)
	tuned, report, err := l.learn(traces, currentConfig)
	if err == nil {
		l.lastLearn = time.Now()
	}
	return tuned, report, err
}

func (l *Learner) shouldLearnLocked() bool {
	if l.lastLearn.IsZero() {
		return true
	}
	return time.Since(l.lastLearn) >= l.MinInterval
}

func (l *Learner) learn(traces []policy.PolicyTrace, currentConfig TunedConfig) (*TunedConfig, *LearningReport, error) {
	if len(traces) < l.MinSamples {
		return nil, nil, fmt.Errorf("not enough samples: %d < %d", len(traces), l.MinSamples)
	}

	report := &LearningReport{}
	tuned := &TunedConfig{
		GateWeights:    copyIntMap(currentConfig.GateWeights),
		ChaosPenalties: copyFloatMap(currentConfig.ChaosPenalties),
		ThresholdPass:  currentConfig.ThresholdPass,
		ThresholdWarn:  currentConfig.ThresholdWarn,
		ThresholdFail:  currentConfig.ThresholdFail,
		HardFailGates:  copyBoolMap(currentConfig.HardFailGates),
	}
	if tuned.ThresholdPass == 0 {
		tuned.ThresholdPass = 90
	}
	if tuned.ThresholdWarn == 0 {
		tuned.ThresholdWarn = 70
	}
	if tuned.ThresholdFail == 0 {
		tuned.ThresholdFail = 50
	}

	gateFailCount := make(map[string]int)
	chaosActivation := 0
	chaosModes := make(map[string]int)
	totalRuns := len(traces)

	for _, trace := range traces {
		for _, step := range trace.Steps {
			if step.Stage == "GATES" {
				for _, gateName := range step.FailedGates {
					gateFailCount[gateName]++
				}
			}
			if step.Stage == "CHAOS" {
				chaosActivation++
				chaosModes[step.Source]++
			}
		}
	}

	for gateName, currentWeight := range currentConfig.GateWeights {
		failRate := float64(gateFailCount[gateName]) / float64(totalRuns)
		if failRate > 0.3 {
			newWeight := int(float64(currentWeight) * (1 + l.LearningRate))
			if newWeight > 50 {
				newWeight = 50
			}
			if newWeight != currentWeight {
				tuned.GateWeights[gateName] = newWeight
				report.Adjustments = append(report.Adjustments, Adjustment{
					Parameter: "gate_weight",
					Target:    gateName,
					OldValue:  float64(currentWeight),
					NewValue:  float64(newWeight),
					Reason:    fmt.Sprintf("high failure rate: %.2f", failRate),
				})
			}
		}
	}

	if chaosActivation > 0 {
		for mode, count := range chaosModes {
			activationRate := float64(count) / float64(totalRuns)
			if activationRate > 0.2 {
				penalty, ok := currentConfig.ChaosPenalties[mode]
				if !ok {
					penalty = 0
				}
				newPenalty := penalty * (1 + l.LearningRate)
				if newPenalty > 50 {
					newPenalty = 50
				}
				if newPenalty != penalty {
					tuned.ChaosPenalties[mode] = newPenalty
					report.Adjustments = append(report.Adjustments, Adjustment{
						Parameter: "chaos_penalty",
						Target:    mode,
						OldValue:  penalty,
						NewValue:  newPenalty,
						Reason:    fmt.Sprintf("high activation rate: %.2f", activationRate),
					})
				}
			}
		}
	}

	blockCount := 0
	for _, trace := range traces {
		if trace.FinalStatus == policy.StatusBlock {
			blockCount++
		}
	}
	report.AnomalyScore = float64(blockCount) / float64(totalRuns)
	report.Confidence = 1.0 - report.AnomalyScore

	var scores []float64
	for _, trace := range traces {
		scores = append(scores, trace.FinalScore)
	}
	sort.Float64s(scores)
	if len(scores) > 0 {
		median := scores[len(scores)/2]
		if median < 60 {
			tuned.ThresholdPass = 85
			tuned.ThresholdWarn = 65
			tuned.ThresholdFail = 45
			report.Adjustments = append(report.Adjustments, Adjustment{
				Parameter: "thresholds",
				Target:    "all",
				OldValue:  90,
				NewValue:  85,
				Reason:    fmt.Sprintf("median score low: %.1f", median),
			})
		}
	}

	return tuned, report, nil
}

func (l *Learner) Learn(traces []policy.PolicyTrace, currentConfig TunedConfig) (*TunedConfig, *LearningReport, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.learn(traces, currentConfig)
}

func copyIntMap(m map[string]int) map[string]int {
	cp := make(map[string]int)
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

func copyFloatMap(m map[string]float64) map[string]float64 {
	cp := make(map[string]float64)
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

func copyBoolMap(m map[string]bool) map[string]bool {
	cp := make(map[string]bool)
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
