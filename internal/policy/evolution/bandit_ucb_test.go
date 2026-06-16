package evolution_test

import (
    "testing"

    "pads-v3/internal/policy/evolution"
)

func TestUCBSelector_SelectsInitially(t *testing.T) {
    s := evolution.NewUCBSelector(42)
    s.AddArm("A")
    s.AddArm("B")
    s.AddArm("C")
    chosen := s.Select()
    if chosen == "" {
        t.Fatal("expected a selected arm")
    }
    s.Update(chosen, 1.0)
}

func TestUCBSelector_Convergence(t *testing.T) {
    s := evolution.NewUCBSelector(123)
    s.AddArm("stable")
    s.AddArm("bandit")

    // Le bras "stable" reçoit de meilleures récompenses
    for i := 0; i < 20; i++ {
        arm := s.Select()
        if arm == "stable" {
            s.Update("stable", 10.0)
        } else {
            s.Update("bandit", 1.0)
        }
    }

    // Après 20 itérations, "stable" doit être le plus sélectionné
    stableCount := 0
    for i := 0; i < 100; i++ {
        if s.Select() == "stable" {
            stableCount++
        }
    }

    if stableCount < 70 {
        t.Errorf("UCB should favor 'stable' after learning, got %d/100", stableCount)
    }
}
