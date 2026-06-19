package certification

import (
	"crypto/sha256"
	"fmt"
	"os"

	"pads-v3/internal/ci"
)

// Certifier replays a WAL and produces a Certificate.
type Certifier struct {
	ReplayVerifier *ci.ReplayVerifier
	Jobs           map[string]ci.Job
	Snapshot       ci.CacheSnapshot
}

// Certify re-exécute le WAL, compare les hashes et génère un certificat.
func (c *Certifier) Certify(walPath string) (*Certificate, error) {
	// 1. Compute WAL hash
	walHash, err := computeFileHash(walPath)
	if err != nil {
		return nil, fmt.Errorf("certify: compute wal hash: %w", err)
	}

	// 2. Replay and get the replay WAL path
	replayWalPath := walPath + ".cert"
	replayWal, err := ci.NewWAL(replayWalPath)
	if err != nil {
		return nil, fmt.Errorf("certify: create replay wal: %w", err)
	}
	defer replayWal.Close()

	sched := &ci.Scheduler{
		Cache:       c.ReplayVerifier.Cache,
		Artifacts:   ci.NewArtifactStore(nil, "", replayWal),
		WAL:         replayWal,
		MaxParallel: 1,
	}
	// Replay using the same jobs
	if err := sched.Run(c.Jobs); err != nil {
		return nil, fmt.Errorf("certify: replay execution: %w", err)
	}
	replayWal.Close()

	// 3. Compute replay hash
	replayHash, err := computeFileHash(replayWalPath)
	if err != nil {
		return nil, fmt.Errorf("certify: compute replay hash: %w", err)
	}

	// 4. Compare hashes
	deterministic := walHash == replayHash

	return &Certificate{
		RunID:            fmt.Sprintf("run-%x", sha256.Sum256([]byte(walPath))),
		WALHash:          walHash,
		ReplayHash:       replayHash,
		Deterministic:    deterministic,
		SchedulerVersion: "v0.6",
	}, nil
}

func computeFileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h), nil
}
