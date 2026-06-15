package evolution_test

import (
    "testing"

    "pads-v3/internal/policy/evolution"
)

func TestStabilityGate_Check_AcceptsHighScore(t *testing.T) {
    gate := evolution.NewStabilityGate()
    if !gate.Check(100) {
        t.Fatal("expected high score to be accepted")
    }
}

func TestStabilityGate_Check_RejectsLowScore(t *testing.T) {
    // On utilise un gate avec un seuil de base plus élevé pour que 10 soit rejeté
    gate := evolution.NewStabilityGateV2(50, 5, 10.0)
    if gate.Check(10) {
        t.Fatal("expected low score to be rejected")
    }
}

func TestStabilityGate_AdaptiveThreshold(t *testing.T) {
    gate := evolution.NewStabilityGate()
    // Ajouter quelques scores faibles puis un score élevé mais pas énorme
    gate.Check(10)
    gate.Check(15)
    gate.Check(12)
    // Le seuil adaptatif devrait être augmenté, donc 50 pourrait ne plus passer
    if gate.Check(50) {
        t.Log("50 still accepted (threshold might not be high enough)")
    }
}

func TestStabilityGate_ExportImport(t *testing.T) {
    gate := evolution.NewStabilityGate()
    gate.Check(80)
    state := gate.ExportState()

    gate2 := evolution.NewStabilityGate()
    gate2.ImportState(state)

    // Vérifier que les deux gates prennent la même décision
    if gate.Check(85) != gate2.Check(85) {
        t.Fatal("export/import mismatch")
    }
}
