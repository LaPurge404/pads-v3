package adaptive

import (
	"testing"

	"pads-v3/internal/ci"
)

func TestAdaptiveLoop_Convergence(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := tmpDir + "/ci.wal"

	// First, run a reference execution to obtain the expected digest.
	walRef, _ := ci.NewWAL(walPath)
	schedRef := &ci.Scheduler{
		Cache:       ci.NewCache(tmpDir + "/cache_ref"),
		Artifacts:   ci.NewArtifactStore(nil, tmpDir+"/artifacts_ref", walRef),
		WAL:         walRef,
		MaxParallel: 1,
	}
	jobs := map[string]ci.Job{
		"test": {
			ID: "test",
			Steps: []ci.Step{
				{ID: "step1", Run: "echo hello"},
			},
		},
	}
	schedRef.Run(jobs)
	walRef.Close()

	refDigest, err := ci.ComputeDigest(walPath)
	if err != nil {
		t.Fatal(err)
	}

	// Now run the adaptive loop with a fresh scheduler and the reference digest.
	walTestPath := tmpDir + "/ci_test.wal"
	walTest, _ := ci.NewWAL(walTestPath)
	schedTest := &ci.Scheduler{
		Cache:       ci.NewCache(tmpDir + "/cache_test"),
		Artifacts:   ci.NewArtifactStore(nil, tmpDir+"/artifacts_test", walTest),
		WAL:         walTest,
		MaxParallel: 1,
	}

	loop := &Loop{
		Scheduler:       schedTest,
		Oracle:          &ci.ReplayVerifier{Cache: ci.NewCache(tmpDir + "/oracle_cache"), ArtifactsDir: tmpDir + "/oracle_artifacts"},
		MaxRetries:      2,
		ReferenceDigest: refDigest,
	}

	// Run the adaptive loop; it should converge immediately since the jobs are identical.
	err = loop.Run(jobs)
	if err != nil {
		t.Fatalf("adaptive loop failed: %v", err)
	}
	t.Logf("adaptive loop converged successfully")
}
