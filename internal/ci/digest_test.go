package ci

import (
	"testing"
)

func TestReplayVerifier_DigestMatch(t *testing.T) {
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

	// Compute digest of the original run
	origDigest, err := ComputeDigest(walPath)
	if err != nil {
		t.Fatal(err)
	}

	// Replay with the SAME snapshot
	rv := &ReplayVerifier{
		Cache:        NewCache(tmpDir + "/cache_replay"),
		ArtifactsDir: tmpDir,
	}
	result, err := rv.Replay(walPath, jobs, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("replay failed: %+v", result)
	}

	// Compute digest of the replay
	replayWalPath := walPath + ".replay"
	replayDigest, err := ComputeDigest(replayWalPath)
	if err != nil {
		t.Fatal(err)
	}

	if origDigest != replayDigest {
		t.Errorf("Digest mismatch:\n  original: %s\n  replay:   %s", origDigest, replayDigest)
	} else {
		t.Logf("Digest match: %s", origDigest)
	}
}
