package shadow

import "testing"

func TestABGate_Accept(t *testing.T) {
    // MinDelta=0.5, MinGain=0.005 (0.5%)
    gate := NewABGate(0.5, 0.005)

    // delta=1.0, gain=1/90 ≈ 1.1% → accepted
    if !gate.Accept(91.0, 90.0) {
        t.Error("should accept 91.0 vs 90.0")
    }

    // delta=0.4 < 0.5 → rejected
    if gate.Accept(90.4, 90.0) {
        t.Error("should not accept 90.4 vs 90.0 (delta too small)")
    }

    // Worse candidate → rejected
    if gate.Accept(85.0, 90.0) {
        t.Error("should not accept worse candidate")
    }
}

func TestABGate_ZeroCurrent(t *testing.T) {
    gate := NewABGate(0.5, 0.005)
    if gate.Accept(10.0, 0.0) {
        t.Error("should not accept when current is zero")
    }
}
