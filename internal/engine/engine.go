package engine

import (
    "crypto/sha256"
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "sort"
    "strings"

    "pads-v3/internal/reducer"
    "pads-v3/internal/storage"
)

// FileSnapshot represents a frozen view of a file and its nodes.
type FileSnapshot struct {
    FilePath string
    FileHash string
    Nodes    []string
}

// ExecutionResult contains the outcome of testing one file.
type ExecutionResult struct {
    FilePath string
    Hash     string
    Status   string
    ExitCode int
    Stderr   string
    NodeIDs  []string
}

// RunOnce is the deterministic engine entry point.
func RunOnce(db *storage.DB) error {
    // Phase 1: Snapshot (pure read)
    snapshot, err := buildSnapshot(db)
    if err != nil {
        return fmt.Errorf("engine: snapshot: %w", err)
    }

    // Phase 2: Execution (pure, no DB access)
    results, err := executeSnapshot(snapshot)
    if err != nil {
        return fmt.Errorf("engine: execute: %w", err)
    }

    // Phase 3: Commit (all side effects here)
    if err := commitResults(db, results); err != nil {
        return fmt.Errorf("engine: commit: %w", err)
    }

    // Update L3 from the new events
    _, err = reducer.RunReductionLoop(db)
    if err != nil {
        return fmt.Errorf("engine: reducer: %w", err)
    }

    return nil
}

// buildSnapshot builds a sorted, immutable view of all files.
func buildSnapshot(db *storage.DB) ([]FileSnapshot, error) {
    rows, err := db.Query(`SELECT id, file_path, file_hash FROM nodes`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    grouped := map[string]*FileSnapshot{}

    for rows.Next() {
        var id, path, hash string
        if err := rows.Scan(&id, &path, &hash); err != nil {
            return nil, err
        }
        if _, ok := grouped[path]; !ok {
            grouped[path] = &FileSnapshot{
                FilePath: path,
                FileHash: hash,
            }
        }
        grouped[path].Nodes = append(grouped[path].Nodes, id)
    }

    out := make([]FileSnapshot, 0, len(grouped))
    for _, v := range grouped {
        out = append(out, *v)
    }

    sort.Slice(out, func(i, j int) bool {
        return out[i].FilePath < out[j].FilePath
    })

    return out, nil
}

// executeSnapshot runs go test for each file without touching the DB.
func executeSnapshot(files []FileSnapshot) ([]ExecutionResult, error) {
    results := make([]ExecutionResult, 0, len(files))

    for _, f := range files {
        currentHash, err := computeFileHash(f.FilePath)
        if err != nil {
            continue
        }

        status, exitCode, stderr, _ := runGoTest(f.FilePath)
        if exitCode != 0 {
            status = "FAIL"
        } else {
            status = "PASS"
        }

        results = append(results, ExecutionResult{
            FilePath: f.FilePath,
            Hash:     currentHash,
            Status:   status,
            ExitCode: exitCode,
            Stderr:   stderr,
            NodeIDs:  f.Nodes,
        })
    }

    return results, nil
}

// commitResults persists the execution results as L2 events.
func commitResults(db *storage.DB, results []ExecutionResult) error {
    for _, r := range results {
        eventID := makeEventID(r.FilePath, r.Hash, r.Status, r.ExitCode)

        // Invalidate old state for this file (kept as cleanup, not decision logic)
        if err := db.ClearGraphStateByFile(r.FilePath); err != nil {
            return err
        }

        payload := fmt.Sprintf(
            `{"file":"%s","status":"%s","stderr":"%s"}`,
            r.FilePath, r.Status, strings.TrimSpace(r.Stderr),
        )

        _, err := db.InsertEvent(eventID, "TEST_RESULT", payload, r.ExitCode)
        if err != nil {
            return err
        }

        for _, nid := range r.NodeIDs {
            if err := db.InsertEventNode(eventID, nid); err != nil {
                return err
            }
        }

        if r.Hash != "" {
            if err := db.UpdateFileHash(r.FilePath, r.Hash); err != nil {
                return err
            }
        }
    }
    return nil
}

// --- helpers (unchanged) ---

func makeEventID(filePath, fileHash, status string, exitCode int) string {
    input := fmt.Sprintf("%s|%s|%s|%d", filePath, fileHash, status, exitCode)
    sum := sha256.Sum256([]byte(input))
    return fmt.Sprintf("engine-%x", sum)
}

func computeFileHash(filePath string) (string, error) {
    f, err := os.Open(filePath)
    if err != nil {
        return "", err
    }
    defer f.Close()
    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil {
        return "", err
    }
    return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func runGoTest(filePath string) (string, int, string, error) {
    dir := filepath.Dir(filePath)
    moduleRoot, err := findModuleRoot(dir)
    if err != nil {
        moduleRoot = dir
    }

    cmd := exec.Command("go", "test", "./...")
    cmd.Dir = moduleRoot
    output, err := cmd.CombinedOutput()
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            status := "FAIL"
            if exitErr.ExitCode() == 0 {
                status = "PASS"
            }
            return status, exitErr.ExitCode(), string(output), nil
        }
        return "FAIL", 1, string(output), err
    }
    return "PASS", 0, "", nil
}

func findModuleRoot(startDir string) (string, error) {
    dir := startDir
    for {
        goModPath := filepath.Join(dir, "go.mod")
        if _, err := os.Stat(goModPath); err == nil {
            return dir, nil
        }
        parent := filepath.Dir(dir)
        if parent == dir {
            return "", fmt.Errorf("go.mod not found in any parent of %s", startDir)
        }
        dir = parent
    }
}
