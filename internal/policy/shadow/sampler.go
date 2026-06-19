package shadow

import "pads-v3/internal/policy"

// Sampler performs intelligent sampling of recent inputs to avoid CPU explosion.
type Sampler struct {
	MaxSamples int
}

// NewSampler creates a sampler with the given maximum number of samples.
func NewSampler(maxSamples int) *Sampler {
	return &Sampler{MaxSamples: maxSamples}
}

// Sample returns at most MaxSamples inputs, uniformly distributed.
func (s *Sampler) Sample(inputs []policy.GateInput) []policy.GateInput {
	if len(inputs) <= s.MaxSamples {
		return inputs
	}
	out := make([]policy.GateInput, 0, s.MaxSamples)
	step := float64(len(inputs)) / float64(s.MaxSamples)
	for i := 0; len(out) < s.MaxSamples; i++ {
		idx := int(float64(i) * step)
		if idx >= len(inputs) {
			break
		}
		out = append(out, inputs[idx])
	}
	return out
}
