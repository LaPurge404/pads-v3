package replay

import (
    "fmt"
    "os"
    "sort"

    "pads-v3/internal/storage"
)

// RunSnapshot captures the complete state of a run for replay.
type RunSnapshot struct {
    Files []FileSnapshot
}

// FileSnapshot is a frozen view of a file and its nodes, including content.
type FileSnapshot struct {
    FilePath string
    FileHash string
    Content  string   // file content at snapshot time
    Nodes    []string
}

// CaptureSnapshot builds a sorted, immutable snapshot from the current database.
// It reads the file content from disk to enable hermetic replay.
func CaptureSnapshot(db *storage.DB) (*RunSnapshot, error) {
    rows, err := db.Query(`SELECT id, file_path, file_hash FROM nodes`)
    if err != nil {
        return nil, fmt.Errorf("capture snapshot: %w", err)
    }
    defer rows.Close()

    grouped := make(map[string]*FileSnapshot)
    for rows.Next() {
        var id, path, hash string
        if err := rows.Scan(&id, &path, &hash); err != nil {
            return nil, fmt.Errorf("capture snapshot: scan: %w", err)
        }
        if _, ok := grouped[path]; !ok {
            content, err := os.ReadFile(path)
            if err != nil {
                content = []byte{}
            }
            grouped[path] = &FileSnapshot{
                FilePath: path,
                FileHash: hash,
                Content:  string(content),
            }
        }
        grouped[path].Nodes = append(grouped[path].Nodes, id)
    }

    // Deterministic ordering
    for _, fs := range grouped {
        sort.Strings(fs.Nodes)
    }

    snapshot := &RunSnapshot{}
    for _, v := range grouped {
        snapshot.Files = append(snapshot.Files, *v)
    }
    sort.Slice(snapshot.Files, func(i, j int) bool {
        return snapshot.Files[i].FilePath < snapshot.Files[j].FilePath
    })

    return snapshot, nil
}
