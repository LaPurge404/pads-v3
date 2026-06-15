package evolution

func IsSafe(candidateScore, currentScore float64, confidence float64) bool {

// hard safety gate
if confidence < 0.3 {
return false
}

// no regression allowed unless high confidence
if candidateScore < currentScore && confidence < 0.8 {
return false
}

return true
}
