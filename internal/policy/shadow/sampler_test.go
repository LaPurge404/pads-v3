package shadow

import (
    "testing"

    "pads-v3/internal/policy"
)

func TestSampler_MaxSamples(t *testing.T) {
    s := NewSampler(3)
    inputs := make([]policy.GateInput, 10)
    sampled := s.Sample(inputs)
    if len(sampled) != 3 {
        t.Errorf("expected exactly 3 samples, got %d", len(sampled))
    }
}

func TestSampler_All(t *testing.T) {
    s := NewSampler(10)
    inputs := make([]policy.GateInput, 5)
    sampled := s.Sample(inputs)
    if len(sampled) != 5 {
        t.Errorf("expected 5 samples, got %d", len(sampled))
    }
}
