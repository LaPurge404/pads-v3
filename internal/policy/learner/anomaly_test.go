package learner

import (
	"testing"
)

func TestAnomalyDetector_Normal(t *testing.T) {
	d := NewAnomalyDetector()

	// Feed consistent scores
	for i := 0; i < 10; i++ {
		d.UpdateEMA(85.0, "PASS", false)
	}

	isAnomaly, reason := d.IsAnomaly(88.0)
	if isAnomaly {
		t.Errorf("expected no anomaly, got: %s", reason)
	}
}

func TestAnomalyDetector_Spike(t *testing.T) {
	d := NewAnomalyDetector()

	// Feed high scores
	for i := 0; i < 10; i++ {
		d.UpdateEMA(90.0, "PASS", false)
	}

	// Sudden drop
	d.UpdateEMA(40.0, "BLOCK", true)

	isAnomaly, reason := d.IsAnomaly(40.0)
	if !isAnomaly {
		t.Error("expected anomaly for sudden drop")
	} else {
		t.Logf("Anomaly detected: %s", reason)
	}
}

func TestAnomalyDetector_EMA(t *testing.T) {
	d := NewAnomalyDetector()
	d.UpdateEMA(100.0, "PASS", false)
	d.UpdateEMA(80.0, "PASS", false)
	d.UpdateEMA(90.0, "PASS", false)

	// EMA should be between 80 and 100
	ema := d.EMA()
	if ema < 80 || ema > 100 {
		t.Errorf("EMA out of range: %.2f", ema)
	}
}
