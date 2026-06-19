package ci

import (
	"testing"

	"pads-v3/internal/event"
	"pads-v3/internal/trace"
)

func TestScheduler_ReplayDeterminism(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := tmpDir + "/ci.wal"

	// Run 1
	cacheDir1 := tmpDir + "/cache1"
	wal1, err := NewWAL(walPath)
	if err != nil {
		t.Fatal(err)
	}
	sched1 := &Scheduler{
		Cache:       NewCache(cacheDir1),
		Artifacts:   NewArtifactStore(nil, tmpDir+"/artifacts1", wal1),
		WAL:         wal1,
		MaxParallel: 1,
	}
	jobs := map[string]Job{
		"test": {
			ID: "test",
			Steps: []Step{
				{ID: "step1", Run: "echo hello"},
			},
		},
	}
	if err := sched1.Run(jobs); err != nil {
		t.Fatal(err)
	}
	wal1.Close()

	events1, err := trace.ReadWALFile(walPath)
	if err != nil {
		t.Fatal(err)
	}

	// Run 2 (fresh everything)
	cacheDir2 := tmpDir + "/cache2"
	walPath2 := tmpDir + "/ci2.wal"
	wal2, err := NewWAL(walPath2)
	if err != nil {
		t.Fatal(err)
	}
	sched2 := &Scheduler{
		Cache:       NewCache(cacheDir2),
		Artifacts:   NewArtifactStore(nil, tmpDir+"/artifacts2", wal2),
		WAL:         wal2,
		MaxParallel: 1,
	}
	if err := sched2.Run(jobs); err != nil {
		t.Fatal(err)
	}
	wal2.Close()

	events2, err := trace.ReadWALFile(walPath2)
	if err != nil {
		t.Fatal(err)
	}

	if len(events1) != len(events2) {
		t.Logf("Run 1 events: %d, Run 2 events: %d", len(events1), len(events2))
		printDiff(t, events1, events2)
		t.Fatalf("WAL lengths differ")
	}

	for i := range events1 {
		if events1[i].Type != events2[i].Type ||
			events1[i].JobID != events2[i].JobID ||
			events1[i].StepID != events2[i].StepID {
			t.Fatalf("Event %d differs:\n  Run1: %+v\n  Run2: %+v", i, events1[i], events2[i])
		}
	}
	t.Logf("Determinism OK: %d identical events", len(events1))
}

func printDiff(t *testing.T, e1, e2 []event.CanonicalEvent) {
	t.Helper()
	min := len(e1)
	if len(e2) < min {
		min = len(e2)
	}
	for i := 0; i < min; i++ {
		if e1[i].Type != e2[i].Type || e1[i].JobID != e2[i].JobID || e1[i].StepID != e2[i].StepID {
			t.Logf("Diff at index %d:", i)
			t.Logf("  Run1: %+v", e1[i])
			t.Logf("  Run2: %+v", e2[i])
			return
		}
	}
	if len(e1) > len(e2) {
		t.Logf("Extra event in Run1 at index %d: %+v", len(e2), e1[len(e2)])
	} else if len(e2) > len(e1) {
		t.Logf("Extra event in Run2 at index %d: %+v", len(e1), e2[len(e1)])
	}
}
