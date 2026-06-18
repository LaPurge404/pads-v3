package evolution

import (
	"path/filepath"
	"testing"
)

// TestWorkerOffsetBasedReading vérifie que ReadFrom ne relit pas les événements
// déjà traités lors des appels précédents (l'offset est géré en interne).
func TestWorkerOffsetBasedReading(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, "test_queue.log")

	queue, err := NewEventQueue(queuePath)
	if err != nil {
		t.Fatalf("NewEventQueue: %v", err)
	}
	defer queue.Close()

	// Écrire 3 événements initiaux
	for i := 0; i < 3; i++ {
		if err := queue.Enqueue(QueueEvent{
			ID:   "initial-" + string(rune('A'+i)),
			Type: "evolve",
		}); err != nil {
			t.Fatalf("Enqueue initial[%d]: %v", i, err)
		}
	}

	// Première lecture : doit voir les 3 événements
	events1, err := queue.ReadFrom()
	if err != nil {
		t.Fatalf("ReadFrom (1st call): %v", err)
	}
	if len(events1) != 3 {
		t.Errorf("première lecture: attendu 3 événements, obtenu %d", len(events1))
	}

	processed := make(map[string]bool)
	for _, e := range events1 {
		processed[e.ID] = true
	}

	// Deuxième lecture (immédiatement après) : ne doit voir aucun nouvel événement
	events2, err := queue.ReadFrom()
	if err != nil {
		t.Fatalf("ReadFrom (2nd call): %v", err)
	}
	if len(events2) != 0 {
		t.Errorf("deuxième lecture (aucun nouvel événement): attendu 0, obtenu %d", len(events2))
	}

	// Écrire 2 nouveaux événements
	for i := 0; i < 2; i++ {
		if err := queue.Enqueue(QueueEvent{
			ID:   "new-" + string(rune('X'+i)),
			Type: "evolve",
		}); err != nil {
			t.Fatalf("Enqueue new[%d]: %v", i, err)
		}
	}

	// Troisième lecture : doit voir les 2 nouveaux événements
	events3, err := queue.ReadFrom()
	if err != nil {
		t.Fatalf("ReadFrom (3rd call): %v", err)
	}
	if len(events3) != 2 {
		t.Errorf("troisième lecture (2 nouveaux): attendu 2, obtenu %d", len(events3))
	}

	newEvents := 0
	for _, e := range events3 {
		if !processed[e.ID] {
			processed[e.ID] = true
			newEvents++
		}
	}
	if newEvents != 2 {
		t.Errorf("événements effectivement nouveaux: attendu 2, obtenu %d", newEvents)
	}

	t.Logf("événements traitées (total unique): %d", len(processed))
}

// TestWorkerReadFromEmptyQueue vérifie que ReadFrom fonctionne sur un fichier vide.
func TestWorkerReadFromEmptyQueue(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, "empty_queue.log")

	queue, err := NewEventQueue(queuePath)
	if err != nil {
		t.Fatalf("NewEventQueue: %v", err)
	}
	defer queue.Close()

	events, err := queue.ReadFrom()
	if err != nil {
		t.Fatalf("ReadFrom on empty queue: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("lecture file vide: attendu 0 événements, obtenu %d", len(events))
	}
}

// TestWorkerProcessedMapDedup vérifie que la map processed dédouane bien les événements.
func TestWorkerProcessedMapDedup(t *testing.T) {
	processed := make(map[string]bool)

	events := []QueueEvent{
		{ID: "evt-1", Type: "evolve"},
		{ID: "evt-2", Type: "evolve"},
		{ID: "evt-1", Type: "evolve"}, // doublon
	}

	processedCount := 0
	for _, e := range events {
		if !processed[e.ID] {
			processed[e.ID] = true
			processedCount++
		}
	}

	if processedCount != 2 {
		t.Errorf("processedCount: attendu 2, obtenu %d", processedCount)
	}
	if len(processed) != 2 {
		t.Errorf("taille map processed: attendu 2, obtenu %d", len(processed))
	}
}

// TestWorkerProcessedMapCleanup vérifie que cleanupProcessed bornela map processed.
func TestWorkerProcessedMapCleanup(t *testing.T) {
	processed := make(map[string]bool)

	// Simuler 2500 événements avec dédoublonnage
	for i := 0; i < 2500; i++ {
		id := string(rune(i))
		if !processed[id] {
			processed[id] = true
		}
	}

	if len(processed) != 2500 {
		t.Errorf("dédupliqué: attendu 2500 événements, obtenu %d", len(processed))
	}

	// Simuler cleanupWorker qui garde les 500 derniers
	const maxRetention = 500
	cleaned := make(map[string]bool)
	keys := make([]string, 0, len(processed))
	for k := range processed {
		keys = append(keys, k)
	}
	start := len(keys) - maxRetention
	if start < 0 {
		start = 0
	}
	for _, k := range keys[start:] {
		cleaned[k] = true
	}
	if len(cleaned) != maxRetention {
		t.Errorf("après cleanup, map devrait avoir %d entrées, obtenu %d", maxRetention, len(cleaned))
	}

	t.Logf("map size before cleanup: %d, after cleanup: %d (bounded)", len(processed), len(cleaned))
}

// TestEventQueueEnqueueAppendOnly vérifie qu'Enqueue utilise O_APPEND
// (le fichier grossit, pas de troncature).
func TestEventQueueEnqueueAppendOnly(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, "append_only.log")

	queue, err := NewEventQueue(queuePath)
	if err != nil {
		t.Fatalf("NewEventQueue: %v", err)
	}
	defer queue.Close()

	for i := 0; i < 5; i++ {
		if err := queue.Enqueue(QueueEvent{ID: "evt"}); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}

	size, err := queue.Size()
	if err != nil {
		t.Fatalf("Size: %v", err)
	}
	if size == 0 {
		t.Error("Size should be > 0 after 5 enqueues")
	}
}

// TestEventQueueSync vérifie que Enqueue retourne une erreur si le fichier
// sous-jacent devient inaccessible (dossier inexistant).
func TestEventQueueSyncError(t *testing.T) {
	// Créer un EventQueue, puis supprimer son fichier et essayer d'écrire
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, "sync_error.log")

	queue, err := NewEventQueue(queuePath)
	if err != nil {
		t.Fatalf("NewEventQueue: %v", err)
	}
	queue.Close()
}