package certification

import (
	"path/filepath"
	"testing"

	"pads-v3/internal/ci"
)

func TestCertifier_DeterministicRun(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "ci.wal")

	// Create a reference WAL via a scheduler run
	wal, _ := ci.NewWAL(walPath)
	sched := &ci.Scheduler{
		Cache:       ci.NewCache(tmpDir + "/cache"),
		Artifacts:   ci.NewArtifactStore(nil, tmpDir+"/artifacts", wal),
		WAL:         wal,
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
	snapshot, err := sched.RunWithSnapshot(jobs)
	if err != nil {
		t.Fatal(err)
	}
	wal.Close()

	// Certify
	certifier := &Certifier{
		ReplayVerifier: &ci.ReplayVerifier{
			Cache:        ci.NewCache(tmpDir + "/replay_cache"),
			ArtifactsDir: tmpDir + "/replay_artifacts",
		},
		Jobs:     jobs,
		Snapshot: snapshot,
	}
	cert, err := certifier.Certify(walPath)
	if err != nil {
		t.Fatal(err)
	}

	if !cert.Deterministic {
		t.Errorf("expected deterministic run, got non-deterministic")
	}
	t.Logf("Certificate: %+v", cert)
}
