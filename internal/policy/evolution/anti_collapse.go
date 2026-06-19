package evolution

import "math"

type AntiCollapseDetector struct {
	window            []float64
	size              int
	varianceThreshold float64
}

func NewAntiCollapseDetector(windowSize int, varianceThreshold float64) *AntiCollapseDetector {
	return &AntiCollapseDetector{
		window:            make([]float64, 0, windowSize),
		size:              windowSize,
		varianceThreshold: varianceThreshold,
	}
}

func (d *AntiCollapseDetector) Add(score float64) {
	d.window = append(d.window, score)
	if len(d.window) > d.size {
		d.window = d.window[1:]
	}
}

// Variance computes the variance of the window.
func (d *AntiCollapseDetector) Variance() float64 {
	if len(d.window) < 2 {
		return 0
	}
	mean := d.mean()
	sumSquares := 0.0
	for _, v := range d.window {
		diff := v - mean
		sumSquares += diff * diff
	}
	return sumSquares / float64(len(d.window))
}

// StdDev returns the standard deviation (square root of variance).
func (d *AntiCollapseDetector) StdDev() float64 {
	return math.Sqrt(d.Variance())
}

func (d *AntiCollapseDetector) mean() float64 {
	if len(d.window) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range d.window {
		sum += v
	}
	return sum / float64(len(d.window))
}

// IsStable checks that the variance is below the threshold.
func (d *AntiCollapseDetector) IsStable() bool {
	return d.Variance() < d.varianceThreshold
}

// IsOscillating detects a sign change in the slope, with a minimum amplitude.
func (d *AntiCollapseDetector) IsOscillating() bool {
	if len(d.window) < 3 {
		return false
	}
	for i := 2; i < len(d.window); i++ {
		diff1 := d.window[i] - d.window[i-1]
		diff2 := d.window[i-1] - d.window[i-2]
		// Sign change AND significant amplitude (at least 1% of the average score)
		if (diff1*diff2 < 0) && math.Abs(diff1) > 0.01*d.mean() {
			return true
		}
	}
	return false
}
