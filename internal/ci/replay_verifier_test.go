package ci

import (
	"testing"
)

func TestReplayVerifier_Identical(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := tmpDir + "/ci.wal"

	// First run: capture snapshot
	wal1, _ := NewWAL(walPath)
	sched1 := &Scheduler{
		Cache:       NewCache(tmpDir + "/cache"),
		Artifacts:   NewArtifactStore(nil, tmpDir+"/artifacts", wal1),
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
	snapshot, err := sched1.RunWithSnapshot(jobs)
	if err != nil {
		t.Fatal(err)
	}
	wal1.Close()

	// Replay with the SAME snapshot
	rv := &ReplayVerifier{
		Cache:        NewCache(tmpDir + "/cache_replay"),
		ArtifactsDir: tmpDir, // base directory for replay artifacts
	}
	result, err := rv.Replay(walPath, jobs, snapshot)
	if err != nil {
		t.Fatal(err)
	}

	if !result.OK {
		if result.FirstDiff >= 0 {
			t.Errorf("Divergence at index %d:", result.FirstDiff)
			t.Logf("Original: %+v", result.OriginalSeq[result.FirstDiff])
			t.Logf("Replay:   %+v", result.ReplaySeq[result.FirstDiff])
		} else {
			t.Errorf("Length mismatch: original %d, replay %d", result.OriginalLen, result.ReplayLen)
		}
	} else {
		t.Logf("Replay OK: %d events identical", result.OriginalLen)
	}
}
