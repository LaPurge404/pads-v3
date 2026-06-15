package shadow

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"

    "pads-v3/internal/policy"
)

// EngineFactory creates a new Engine for a given configuration.
type EngineFactory func(cfg policy.TunedConfig) *policy.Engine

// ShadowEvaluator safely evaluates a candidate configuration against the current one.
type ShadowEvaluator struct {
    EngineFactory EngineFactory
    Cache         *ShadowCache
    Sampler       *Sampler
    ABGate        *ABGate
}

// New creates a production-ready ShadowEvaluator.
func New(factory EngineFactory) *ShadowEvaluator {
    return &ShadowEvaluator{
        EngineFactory: factory,
        Cache:         NewShadowCache(),
        Sampler:       NewSampler(10),
        ABGate:        NewABGate(0.5, 0.005),
    }
}

// Evaluate tests a candidate configuration against the current one using recent inputs.
func (s *ShadowEvaluator) Evaluate(
    candidate policy.TunedConfig,
    current policy.TunedConfig,
    recentInputs []policy.GateInput,
    currentEngine *policy.Engine,
) (candidateAvg, currentAvg float64, accept bool) {
    if len(recentInputs) == 0 {
        return 0, 0, false
    }

    cacheKey := s.cacheKey(candidate, recentInputs)
    if cached, ok := s.Cache.Get(cacheKey); ok {
        return cached.Candidate, cached.Current, cached.Candidate > cached.Current
    }

    sampled := s.Sampler.Sample(recentInputs)
    candidateEngine := s.EngineFactory(candidate)

    var candSum, currSum float64
    for _, in := range sampled {
        candSum += candidateEngine.Evaluate(in.Gates, in.Cert, in.Chaos).Score
        currSum += currentEngine.Evaluate(in.Gates, in.Cert, in.Chaos).Score
    }

    n := float64(len(sampled))
    candidateAvg = candSum / n
    currentAvg = currSum / n

    s.Cache.Set(cacheKey, ShadowResult{
        Candidate: candidateAvg,
        Current:   currentAvg,
    })

    accept = s.ABGate.Accept(candidateAvg, currentAvg)
    return candidateAvg, currentAvg, accept
}

func (s *ShadowEvaluator) cacheKey(cfg policy.TunedConfig, inputs []policy.GateInput) string {
    h := sha256.New()
    b, _ := json.Marshal(cfg)
    h.Write(b)
    for _, in := range inputs {
        b, _ := json.Marshal(in)
        h.Write(b)
    }
    return hex.EncodeToString(h.Sum(nil))
}
