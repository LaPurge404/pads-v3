package ci

import (
"crypto/sha256"
"fmt"
"os"
"path/filepath"

"pads-v3/internal/storage"
)

// ArtifactStore persists artifacts to disk and logs them as L2 events.
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

// Save writes the artifact content to disk and emits an L2 event.
func (a ArtifactStore) Save(jobID, stepID, content string) error {
key := fmt.Sprintf("%s-%s-%d", jobID, stepID, os.Getpid())
path := filepath.Join(a.Root, key+".artifact")

if err := os.WriteFile(path, []byte(content), 0644); err != nil {
return err
}

// Compute a stable content hash for audit
sum := sha256.Sum256([]byte(content))
contentHash := fmt.Sprintf("%x", sum)

if a.WAL != nil {
_, _ = a.WAL.Append(EventRecord{
Type:    "CI_ARTIFACT",
JobID:   jobID,
StepID:  stepID,
Status:  "CREATED",
Payload: fmt.Sprintf(`{"artifact_id":"%s","sha256":"%s"}`, key, contentHash),
})
}

if a.DB != nil {
_, err := a.DB.InsertEvent(
fmt.Sprintf("artifact-%s", key),
"CI_ARTIFACT",
fmt.Sprintf(`{"artifact_id":"%s","sha256":"%s"}`, key, contentHash),
0,
)
return err
}

return nil
}
