package evolution

import (
    "math"
    "math/rand"
)

// UCBSelector implements a bandit using the UCB1 algorithm.
type UCBSelector struct {
    arms      map[string]float64   // total reward per arm
    counts    map[string]int       // number of pulls per arm
    names     []string
    rng       *rand.Rand
}

func NewUCBSelector(seed int64) *UCBSelector {
    return &UCBSelector{
        arms:   make(map[string]float64),
        counts: make(map[string]int),
        names:  make([]string, 0),
        rng:    rand.New(rand.NewSource(seed)),
    }
}

// AddArm introduces a new selectable arm.
func (u *UCBSelector) AddArm(name string) {
    if _, exists := u.arms[name]; !exists {
        u.arms[name] = 0.0
        u.counts[name] = 0
        u.names = append(u.names, name)
    }
}

// Update gives a reward to the arm that was selected.
func (u *UCBSelector) Update(name string, reward float64) {
    if _, ok := u.arms[name]; ok {
        u.arms[name] += reward
        u.counts[name]++
    }
}

// Select picks the arm with the highest UCB value.
func (u *UCBSelector) Select() string {
	if len(u.names) == 0 {
		return ""
	}
	totalPulls := 0
	for _, c := range u.counts {
		totalPulls += c
	}
	// If any arm has not been pulled yet, pick one randomly (exploration)
	unpulled := make([]string, 0)
	for _, name := range u.names {
		if u.counts[name] == 0 {
			unpulled = append(unpulled, name)
		}
	}
	if len(unpulled) > 0 {
		return unpulled[u.rng.Intn(len(unpulled))]
	}
	// UCB1 calculation
	bestArm := u.names[0]
	bestValue := -1.0
	for _, name := range u.names {
		avgReward := u.arms[name] / float64(u.counts[name])
		exploration := math.Sqrt(2 * math.Log(float64(totalPulls)) / float64(u.counts[name]))
		ucb := avgReward + exploration
		if ucb > bestValue {
			bestValue = ucb
			bestArm = name
		}
	}
	return bestArm
}

// Names returns all arm names.
func (u *UCBSelector) Names() []string {
	return u.names
}

// ListArms returns all arm names (implements Selector interface).
func (u *UCBSelector) ListArms() []string {
	return u.names
}

// Arms returns the arm rewards map.
func (u *UCBSelector) Arms() map[string]float64 {
	return u.arms
}

// Counts returns the arm pull counts map.
func (u *UCBSelector) Counts() map[string]int {
	return u.counts
}
