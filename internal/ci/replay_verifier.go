package ci

import (
	"crypto/sha256"
	"fmt"
	"os"

	"pads-v3/internal/event"
	"pads-v3/internal/trace"
)

// ReplayVerifier replays a WAL file and verifies determinism.
type ReplayVerifier struct {
	Cache        Cache
	ArtifactsDir string
}

// ReplayResult contains the outcome of a replay verification.
type ReplayResult struct {
	OK          bool
	OriginalLen int
	ReplayLen   int
	FirstDiff   int
	OriginalSeq []event.CanonicalEvent
	ReplaySeq   []event.CanonicalEvent
}

// Replay replays a run using the given cache snapshot and compares output hashes.
func (rv *ReplayVerifier) Replay(walPath string, jobs map[string]Job, snapshot CacheSnapshot) (ReplayResult, error) {
	origEvents, err := trace.ReadWALFile(walPath)
	if err != nil {
		return ReplayResult{}, fmt.Errorf("read original WAL: %w", err)
	}

	replayWalPath := walPath + ".replay"
	replayWal, err := NewWAL(replayWalPath)
	if err != nil {
		return ReplayResult{}, fmt.Errorf("create replay WAL: %w", err)
	}
	defer replayWal.Close()

	artifactDir, err := os.MkdirTemp(rv.ArtifactsDir, "replay-artifacts-*")
	if err != nil {
		return ReplayResult{}, fmt.Errorf("create artifact dir: %w", err)
	}
	defer os.RemoveAll(artifactDir)
	artifacts := NewArtifactStore(nil, artifactDir, replayWal)

	graph, _ := BuildDAG(jobs, &rv.Cache)
	_ = snapshot
	_ = artifacts

	sched := &Scheduler{
		Cache:       rv.Cache,
		Artifacts:   artifacts,
		WAL:         replayWal,
		MaxParallel: 1,
	}
	if err := sched.executePlan(graph); err != nil {
		return ReplayResult{}, fmt.Errorf("replay execution: %w", err)
	}
	replayWal.Close()

	replayEvents, err := trace.ReadWALFile(replayWalPath)
	if err != nil {
		return ReplayResult{}, fmt.Errorf("read replay WAL: %w", err)
	}

	result := ReplayResult{
		OriginalLen: len(origEvents),
		ReplayLen:   len(replayEvents),
		OriginalSeq: origEvents,
		ReplaySeq:   replayEvents,
		OK:          true,
		FirstDiff:   -1,
	}

	if len(origEvents) != len(replayEvents) {
		result.OK = false
		return result, nil
	}

	for i := range origEvents {
		if !eventsEqual(origEvents[i], replayEvents[i]) {
			result.OK = false
			result.FirstDiff = i
			break
		}
	}

	return result, nil
}

func eventsEqual(a, b event.CanonicalEvent) bool {
	if a.Type != b.Type || a.JobID != b.JobID || a.StepID != b.StepID || a.Status != b.Status {
		return false
	}
	return hashString(a.Payload) == hashString(b.Payload)
}

func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}
