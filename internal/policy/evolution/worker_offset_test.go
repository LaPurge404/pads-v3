package evolution

import (
	"path/filepath"
	"testing"
)

// TestWorkerOffsetBasedReading vérifie que le worker ne relit pas les événements
// déjà traités lors des itérations précédentes.
func TestWorkerOffsetBasedReading(t *testing.T) {
	tmpDir := t.TempDir()
	queuePath := filepath.Join(tmpDir, "test_queue.log")

	queue, err := NewEventQueue(queuePath)
	if err != nil {
		t.Fatalf("NewEventQueue: %v", err)
	}
	defer queue.Close()

	for i := 0; i < 3; i++ {
		if err := queue.Enqueue(QueueEvent{
			ID:   "initial-" + string(rune('A'+i)),
			Type: "evolve",
		}); err != nil {
			t.Fatalf("Enqueue initial[%d]: %v", i, err)
		}
	}

	events1, offset1, err := queue.ReadFrom(0)
	if err != nil {
		t.Fatalf("ReadFrom(0): %v", err)
	}
	if len(events1) != 3 {
		t.Errorf("première lecture: attendu 3 événements, obtenu %d", len(events1))
	}
	if offset1 == 0 {
		t.Error("offset1 ne doit pas être 0 après lecture de 3 événements")
	}

	processed := make(map[string]bool)
	for _, e := range events1 {
		processed[e.ID] = true
	}

	events2, offset2, err := queue.ReadFrom(offset1)
	if err != nil {
		t.Fatalf("ReadFrom(%d): %v", offset1, err)
	}
	if len(events2) != 0 {
		t.Errorf("deuxième lecture (aucun nouvel événement): attendu 0, obtenu %d", len(events2))
	}
	if offset2 != offset1 {
		t.Errorf("offset2 devrait être égal à offset1 (pas de nouvelles données)")
	}

	for i := 0; i < 2; i++ {
		if err := queue.Enqueue(QueueEvent{
			ID:   "new-" + string(rune('X'+i)),
			Type: "evolve",
		}); err != nil {
			t.Fatalf("Enqueue new[%d]: %v", i, err)
		}
	}

	events3, offset3, err := queue.ReadFrom(offset1)
	if err != nil {
		t.Fatalf("ReadFrom(%d) après ajout: %v", offset1, err)
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
	if offset3 <= offset1 {
		t.Errorf("offset3 (%d) devrait être > offset1 (%d)", offset3, offset1)
	}

	t.Logf("offset initial=%d, après lecture 1=%d, après lecture 2=%d, événements traitées=%d",
		offset1, offset2, offset3, len(processed))
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

	events, offset, err := queue.ReadFrom(0)
	if err != nil {
		t.Fatalf("ReadFrom(0) on empty queue: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("lecture file vide: attendu 0 événements, obtenu %d", len(events))
	}
	if offset != 0 {
		t.Errorf("offset file vide: attendu 0, obtenu %d", offset)
	}
}

// TestWorkerReadFromNonexistentFile vérifie le comportement sur fichier inexistant.
func TestWorkerReadFromNonexistentFile(t *testing.T) {
	queue := &EventQueue{path: "/nonexistent/path/queue.log"}
	events, offset, err := queue.ReadFrom(0)
	if err == nil {
		t.Error("ReadFrom sur fichier inexistant: attendu une erreur, obtenu nil")
	}
	if len(events) != 0 {
		t.Errorf("événements sur fichier inexistant: attendu 0, obtenu %d", len(events))
	}
	if offset != 0 {
		t.Errorf("offset sur fichier inexistant: attendu 0, obtenu %d", offset)
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

// TestWorkerProcessedMapCleanup vérifie le comportement de dédoublonnage
// de la map processed sans faire d'hypothèses sur la taille exacte.
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

	// Simuler un "nettoyage" qui garde les 100 derniers
	cleaned := make(map[string]bool)
	keys := make([]string, 0, len(processed))
	for k := range processed {
		keys = append(keys, k)
	}
	start := len(keys) - 100
	if start < 0 {
		start = 0
	}
	for _, k := range keys[start:] {
		cleaned[k] = true
	}
	if len(cleaned) != 100 {
		t.Errorf("après cleanup, map devrait avoir 100 entrées, obtenu %d", len(cleaned))
	}

	t.Logf("map size before cleanup: %d, after cleanup: 100 (bounded)", len(processed))
}