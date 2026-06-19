package adaptive

import (
	"testing"

	"pads-v3/internal/ci"
)

func TestAdaptiveLoopV2_Convergence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create reference WAL (oracle)
	walRefPath := tmpDir + "/oracle.wal"
	walRef, _ := ci.NewWAL(walRefPath)
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

	// Create test scheduler with identical configuration
	walTestPath := tmpDir + "/test.wal"
	walTest, _ := ci.NewWAL(walTestPath)
	schedTest := &ci.Scheduler{
		Cache:       ci.NewCache(tmpDir + "/cache_test"),
		Artifacts:   ci.NewArtifactStore(nil, tmpDir+"/artifacts_test", walTest),
		WAL:         walTest,
		MaxParallel: 1,
	}

	jobsCopy := map[string]ci.Job{
		"test": {
			ID: "test",
			Steps: []ci.Step{
				{ID: "step1", Run: "echo hello"},
			},
		},
	}

	loop := &AdaptiveLoopV2{
		Scheduler:     schedTest,
		OracleWALPath: walRefPath,
		MaxRetries:    3,
		MutationCtx: &MutationContext{
			Jobs: &jobsCopy,
		},
	}

	// Run the adaptive loop; it should converge immediately since the jobs are identical.
	if err := loop.Run(); err != nil {
		t.Fatalf("adaptive loop v2 failed: %v", err)
	}
	t.Log("Adaptive loop v2 converged successfully")
}
