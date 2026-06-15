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

// Variance calcule la variance de la fenêtre.
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

// StdDev retourne l'écart-type (racine carrée de la variance).
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

// IsStable vérifie que la variance est sous le seuil.
func (d *AntiCollapseDetector) IsStable() bool {
    return d.Variance() < d.varianceThreshold
}

// IsOscillating détecte un changement de signe de la pente, avec une amplitude minimale.
func (d *AntiCollapseDetector) IsOscillating() bool {
    if len(d.window) < 3 {
        return false
    }
    for i := 2; i < len(d.window); i++ {
        diff1 := d.window[i] - d.window[i-1]
        diff2 := d.window[i-1] - d.window[i-2]
        // Changement de signe ET amplitude significative (au moins 1% du score moyen)
        if (diff1*diff2 < 0) && math.Abs(diff1) > 0.01*d.mean() {
            return true
        }
    }
    return false
}
