package main

import (
	"flag"
	"fmt"
	"os"

	"pads-v3/internal/policy/evolution"
)

func main() {
	fileFlag := flag.String("file", "evolution.log", "Chemin du fichier d'événements (JSON lignes)")
	fullFlag := flag.Bool("full", false, "Afficher l'historique complet (chaque étape)")
	stepFlag := flag.Bool("step", false, "Mode pas-à-pas (appuyez sur Entrée pour continuer)")
	stateAtFlag := flag.Int("state-at", -1, "Afficher l'état du système à la séquence N")
	flag.Parse()

	store := evolution.NewEventStore(*fileFlag)
	events, err := store.LoadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erreur chargement événements : %v\n", err)
		os.Exit(1)
	}

	if len(events) == 0 {
		fmt.Println("Aucun événement trouvé.")
		return
	}

	engine := evolution.NewReplayEngine(events)

	// Rebuild the final state
	finalState := engine.Rebuild()

	if *stateAtFlag >= 0 {
		// Replay only up to the requested sequence
		if *stateAtFlag == 0 {
			printState(0, evolution.SystemState{})
			return
		}
		for _, ev := range events {
			if ev.Sequence == *stateAtFlag {
				partialEngine := evolution.NewReplayEngine(events[:ev.Sequence])
				state := partialEngine.Rebuild()
				printState(ev.Sequence, state)
				return
			}
		}
		fmt.Printf("Séquence %d introuvable (max %d).\n", *stateAtFlag, len(events))
		return
	}

	if *fullFlag || *stepFlag {
		// Sequential replay with progressive display
		for i := range events {
			// Rebuild up to and including event i
			partialEngine := evolution.NewReplayEngine(events[:i+1])
			state := partialEngine.Rebuild()
			printState(events[i].Sequence, state)
			if *stepFlag && i < len(events)-1 {
				fmt.Print("-- Appuyez sur Entrée pour l'étape suivante --")
				fmt.Scanln()
			}
		}
	} else {
		// Default behavior: display the final state
		fmt.Printf("État final après %d événements :\n", len(events))
		printState(finalState.Sequence, finalState)
	}
}

// RebuildState creates a ReplayEngine from the given events and returns its final state.
// Exported to allow testing from main_test.go.
func RebuildState(events []evolution.Event) evolution.SystemState {
	engine := evolution.NewReplayEngine(events)
	return engine.Rebuild()
}

func printState(seq int, s evolution.SystemState) {
	fmt.Printf("=== Séquence %d ===\n", seq)
	fmt.Printf("Mode: %s\n", s.Mode)
	fmt.Printf("Bandit: bras=%v seed=%d\n", s.Bandit.Arms, s.Bandit.Seed)
	fmt.Printf("Gate: fenêtre longue=%v seuil=%.2f varianceSeuil=%.2f\n",
		s.Gate.LongWindow, s.Gate.Threshold, s.Gate.VarianceThresh)
	fmt.Printf("Détecteur: fenêtre=%v\n", s.DetectorWindow)
	fmt.Println()
}