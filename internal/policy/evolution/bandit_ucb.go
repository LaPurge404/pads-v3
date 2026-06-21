package evolution

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"sync"
	"time"
)

// ucbState is the JSON-serializable form of UCBSelector for persistence.
type ucbState struct {
	Arms   map[string]float64 `json:"arms"`
	Counts map[string]int     `json:"counts"`
	Names  []string           `json:"names"`
}

// UCBSelector implements a bandit using the UCB1 algorithm.
type UCBSelector struct {
	arms        map[string]float64 // total reward per arm
	counts      map[string]int     // number of pulls per arm
	names       []string
	rng         *rand.Rand
	persistPath string        // if non-empty, auto-save is enabled
	saveMu      sync.Mutex    // protects pendingSave
	saveTimer   *time.Timer   // debounced save timer
	stopSave    chan struct{} // closed when auto-save goroutine should exit
	saveWg      sync.WaitGroup
	mu          sync.Mutex // protects arms/counts/names for thread safety
}

// NewUCBSelector creates a new UCBSelector and optionally restores its state
// from path. If path exists it is loaded; otherwise a fresh selector is created.
func NewUCBSelector(seed int64, persistPath ...string) *UCBSelector {
	u := &UCBSelector{
		arms:     make(map[string]float64),
		counts:   make(map[string]int),
		names:    make([]string, 0),
		rng:      rand.New(rand.NewSource(seed)),
		stopSave: make(chan struct{}),
	}
	if len(persistPath) > 0 && persistPath[0] != "" {
		u.persistPath = persistPath[0]
		if err := u.Load(persistPath[0]); err == nil {
			// state restored successfully
		}
	}
	return u
}

// AddArm introduces a new selectable arm.
func (u *UCBSelector) AddArm(name string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if _, exists := u.arms[name]; !exists {
		u.arms[name] = 0.0
		u.counts[name] = 0
		u.names = append(u.names, name)
	}
}

// Update gives a reward to the arm that was selected.
func (u *UCBSelector) Update(name string, reward float64) {
	u.mu.Lock()
	if _, ok := u.arms[name]; ok {
		u.arms[name] += reward
		u.counts[name]++
	}
	u.mu.Unlock()
	u.scheduleSave()
}

// Save persists the current arms/counts to the given path as JSON.
// It is safe to call concurrently.
func (u *UCBSelector) Save(path string) error {
	u.mu.Lock()
	state := ucbState{
		Arms:   make(map[string]float64, len(u.arms)),
		Counts: make(map[string]int, len(u.counts)),
		Names:  make([]string, len(u.names)),
	}
	for k, v := range u.arms {
		state.Arms[k] = v
	}
	for k, v := range u.counts {
		state.Counts[k] = v
	}
	copy(state.Names, u.names)
	u.mu.Unlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Load restores arms/counts/names from a JSON file previously saved with Save.
func (u *UCBSelector) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var state ucbState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	u.mu.Lock()
	u.arms = state.Arms
	u.counts = state.Counts
	// Rebuild names slice preserving order
	u.names = make([]string, 0, len(state.Names))
	for _, n := range state.Names {
		u.names = append(u.names, n)
	}
	u.mu.Unlock()
	return nil
}

// EnableAutoSave starts a background goroutine that saves the state to path
// at most once per interval (debounced). Multiple calls are idempotent.
func (u *UCBSelector) EnableAutoSave(interval time.Duration) {
	if u.persistPath == "" {
		return
	}
	u.saveWg.Add(1)
	go func() {
		defer u.saveWg.Done()
		for {
			select {
			case <-u.stopSave:
				return
			case <-time.After(interval):
				u.saveMu.Lock()
				if u.saveTimer != nil {
					u.Save(u.persistPath)
				}
				u.saveTimer = nil
				u.saveMu.Unlock()
			}
		}
	}()
}

// scheduleSave resets the debounce timer. Safe to call from multiple goroutines.
func (u *UCBSelector) scheduleSave() {
	if u.persistPath == "" {
		return
	}
	u.saveMu.Lock()
	defer u.saveMu.Unlock()
	if u.saveTimer != nil {
		u.saveTimer.Reset(30 * time.Second)
	} else {
		u.saveTimer = time.NewTimer(30 * time.Second)
	}
}

// Stop stops the auto-save goroutine and performs a final save.
func (u *UCBSelector) Stop() {
	u.saveMu.Lock()
	if u.saveTimer != nil {
		u.saveTimer.Stop()
		u.saveTimer = nil
	}
	u.saveMu.Unlock()
	close(u.stopSave)
	u.saveWg.Wait()
	if u.persistPath != "" {
		u.Save(u.persistPath)
	}
}

// Select picks the arm with the highest UCB value.
func (u *UCBSelector) Select() string {
	u.mu.Lock()
	defer u.mu.Unlock()
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
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.names
}

// ListArms returns all arm names (implements Selector interface).
func (u *UCBSelector) ListArms() []string {
	return u.Names()
}

// Arms returns the arm rewards map (copy to avoid external mutation).
func (u *UCBSelector) Arms() map[string]float64 {
	u.mu.Lock()
	defer u.mu.Unlock()
	m := make(map[string]float64, len(u.arms))
	for k, v := range u.arms {
		m[k] = v
	}
	return m
}

// Counts returns the arm pull counts map (copy to avoid external mutation).
func (u *UCBSelector) Counts() map[string]int {
	u.mu.Lock()
	defer u.mu.Unlock()
	m := make(map[string]int, len(u.counts))
	for k, v := range u.counts {
		m[k] = v
	}
	return m
}
