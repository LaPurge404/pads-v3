package change

import (
"crypto/sha256"
"encoding/hex"
"encoding/json"
"fmt"
"math"
"time"

"pads-v3/internal/policy"
"pads-v3/internal/policy/shadow"
)

// Validator checks whether a candidate config should be applied.
type Validator struct {
Evaluator     *shadow.ShadowEvaluator
Cache         *ProposalCache
MinConfidence float64
MinGainRatio  float64
}

func NewValidator(factory shadow.EngineFactory) *Validator {
return &Validator{
Evaluator:     shadow.New(factory),
Cache:         NewProposalCache(),
MinConfidence: 0.40,
MinGainRatio:  0.10,
}
}

// Validate compares candidate vs current config on the same recent inputs.
// It returns a fully annotated proposal.
func (v *Validator) Validate(
current policy.TunedConfig,
candidate policy.TunedConfig,
recentInputs []policy.GateInput,
currentEngine *policy.Engine,
) (PolicyChangeProposal, error) {
if v == nil || v.Evaluator == nil || v.Cache == nil {
return PolicyChangeProposal{}, fmt.Errorf("validator not initialized")
}

key := v.cacheKey(current, candidate, recentInputs)
if cached, ok := v.Cache.Get(key); ok {
return cached, nil
}

candAvg, currAvg, abAccept := v.Evaluator.Evaluate(candidate, current, recentInputs, currentEngine)
confidence := v.computeConfidence(candAvg, currAvg, recentInputs)

gainRatio := 0.0
if currAvg > 0 {
gainRatio = (candAvg - currAvg) / currAvg
}

accepted := abAccept && gainRatio >= v.MinGainRatio && confidence >= v.MinConfidence

reason := "candidate rejected"
switch {
case accepted:
reason = "candidate accepted after shadow validation"
case !abAccept:
reason = "candidate rejected by A/B gate"
case gainRatio < v.MinGainRatio:
reason = "candidate rejected: gain too small"
case confidence < v.MinConfidence:
reason = "candidate rejected: confidence too low"
}

proposal := PolicyChangeProposal{
ID:             key,
FromConfig:     current,
ToConfig:       candidate,
CandidateScore: candAvg,
CurrentScore:   currAvg,
Confidence:     confidence,
Accepted:       accepted,
Reason:         reason,
CreatedAt:      time.Now().UTC(),
}

v.Cache.Set(key, proposal)
return proposal, nil
}

func (v *Validator) computeConfidence(candidateAvg, currentAvg float64, recentInputs []policy.GateInput) float64 {
if currentAvg <= 0 {
return 0
}

gain := candidateAvg - currentAvg
if gain <= 0 {
return 0
}

// Confidence is based on relative gain, dampened by sample size.
conf := gain / currentAvg
if len(recentInputs) > 0 {
conf = conf / math.Log2(float64(len(recentInputs)+2))
}

if conf < 0 {
return 0
}
if conf > 1 {
return 1
}
return conf
}

func (v *Validator) cacheKey(current, candidate policy.TunedConfig, recentInputs []policy.GateInput) string {
h := sha256.New()

b, _ := json.Marshal(current)
h.Write(b)

b, _ = json.Marshal(candidate)
h.Write(b)

for _, in := range recentInputs {
b, _ = json.Marshal(in)
h.Write(b)
}

return hex.EncodeToString(h.Sum(nil))
}
