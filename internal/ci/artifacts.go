package ci

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"pads-v3/internal/event"
	"pads-v3/internal/storage"
)

// ArtifactStore persists artifacts to disk and logs them as events.
type ArtifactStore struct {
	DB   *storage.DB
	Root string
	WAL  *WAL
}

// NewArtifactStore creates a new ArtifactStore.
func NewArtifactStore(db *storage.DB, root string, wal *WAL) ArtifactStore {
	_ = os.MkdirAll(root, 0755)
	return ArtifactStore{DB: db, Root: root, WAL: wal}
}

// SaveEvent writes the artifact content to disk and returns the CanonicalEvent to be recorded.
// It does NOT write to the WAL directly.
func (a ArtifactStore) SaveEvent(jobID, stepID, content string) event.CanonicalEvent {
	key := fmt.Sprintf("%s-%s-%d", jobID, stepID, os.Getpid())
	path := filepath.Join(a.Root, key+".artifact")
	_ = os.WriteFile(path, []byte(content), 0644)

	sum := sha256.Sum256([]byte(content))
	contentHash := fmt.Sprintf("%x", sum)

	return event.CanonicalEvent{
		Type:    "CI_ARTIFACT",
		JobID:   jobID,
		StepID:  stepID,
		Status:  "CREATED",
		Payload: fmt.Sprintf(`{"artifact_id":"%s","sha256":"%s"}`, key, contentHash),
	}
}

// Save writes the artifact content to disk and records the event in the WAL.
func (a ArtifactStore) Save(jobID, stepID, content string) error {
	ev := a.SaveEvent(jobID, stepID, content)
	if a.WAL != nil {
		if err := a.WAL.AppendCanonical(ev); err != nil {
			return err
		}
	}
	if a.DB != nil {
		_, err := a.DB.InsertEvent(
			fmt.Sprintf("artifact-%s", jobID),
			"CI_ARTIFACT",
			ev.Payload,
			0,
		)
		return err
	}
	return nil
}
