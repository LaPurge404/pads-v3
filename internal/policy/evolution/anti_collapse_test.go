package evolution_test

import (
	"math"
	"testing"

	"pads-v3/internal/policy/evolution"
)

func TestAntiCollapseDetector_Variance(t *testing.T) {
	d := evolution.NewAntiCollapseDetector(5, 10.0)
	d.Add(10)
	d.Add(20)
	d.Add(30)
	// Variance devrait être > 0
	if d.Variance() == 0 {
		t.Fatal("expected non-zero variance")
	}
}

func TestAntiCollapseDetector_Stable(t *testing.T) {
	d := evolution.NewAntiCollapseDetector(5, 1.0) // seuil très bas
	d.Add(10)
	d.Add(10)
	d.Add(10)
	if !d.IsStable() {
		t.Fatal("expected stable")
	}
}

func TestAntiCollapseDetector_Oscillation(t *testing.T) {
	d := evolution.NewAntiCollapseDetector(5, 10.0)
	d.Add(10)
	d.Add(20)
	d.Add(10)
	if !d.IsOscillating() {
		t.Fatal("expected oscillation detected")
	}
}

func TestAntiCollapseDetector_StdDev(t *testing.T) {
	d := evolution.NewAntiCollapseDetector(5, 10.0)
	d.Add(10)
	d.Add(20)
	if math.Abs(d.StdDev()-5.0) > 0.001 {
		t.Fatalf("expected stddev ~5, got %f", d.StdDev())
	}
}
