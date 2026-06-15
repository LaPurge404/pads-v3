package evolution

import (
    "math/rand"
    "time"
)

type Bandit struct {
    arms map[string]float64
    rng  *rand.Rand
}

func NewBandit() *Bandit {
    return &Bandit{
        arms: make(map[string]float64),
        rng:  rand.New(rand.NewSource(time.Now().UnixNano())),
    }
}

// NewBanditWithSeed crée un bandit reproductible (indispensable pour le replay).
func NewBanditWithSeed(seed int64) *Bandit {
    return &Bandit{
        arms: make(map[string]float64),
        rng:  rand.New(rand.NewSource(seed)),
    }
}

func (b *Bandit) AddArm(name string) {
    b.arms[name] = 1.0
}

func (b *Bandit) Update(name string, reward float64) {
    b.arms[name] += reward
}

func (b *Bandit) Select() string {
    if len(b.arms) == 0 {
        return ""
    }
    // Choix purement aléatoire pour l'exploration (seedé donc déterministe)
    keys := make([]string, 0, len(b.arms))
    for k := range b.arms {
        keys = append(keys, k)
    }
    idx := b.rng.Intn(len(keys))
    return keys[idx]
}
