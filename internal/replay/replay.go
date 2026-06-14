package replay

import (
    "crypto/sha256"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "time"

    "pads-v3/internal/storage"
)

// Result holds the outcome of replaying the snapshot.
type Result struct {
    ExitCode   int
    Output     string
    DurationMs int64
}

// ReplayEngine re-executes a run from a snapshot in a hermetic workspace.
type ReplayEngine struct{}

// Run replays the given snapshot in a temporary module, emits events, and returns the result.
func (r *ReplayEngine) Run(db *storage.DB, snapshot *RunSnapshot) (Result, error) {
    start := time.Now()

    output, exitCode, err := runHermeticWorkspace(snapshot.Files)
    if err != nil {
        return Result{ExitCode: 1, Output: err.Error(), DurationMs: time.Since(start).Milliseconds()}, err
    }

    // Generate a deterministic event ID based on the snapshot hash
    snapshotHash := hashFiles(snapshot.Files)
    eventID := fmt.Sprintf("replay-%x", snapshotHash)

    payload := fmt.Sprintf(`{"files":%d,"output":"%s"}`, len(snapshot.Files), output)
    _, err = db.InsertEvent(eventID, "REPLAY_RESULT", payload, exitCode)
    if err != nil {
        return Result{ExitCode: exitCode, Output: output, DurationMs: time.Since(start).Milliseconds()}, fmt.Errorf("replay: insert event: %w", err)
    }

    // Associate all nodes from the snapshot with this event
    for _, f := range snapshot.Files {
        for _, nid := range f.Nodes {
            if err := db.InsertEventNode(eventID, nid); err != nil {
                return Result{ExitCode: exitCode, Output: output, DurationMs: time.Since(start).Milliseconds()}, fmt.Errorf("replay: insert event node: %w", err)
            }
        }
    }

    return Result{ExitCode: exitCode, Output: output, DurationMs: time.Since(start).Milliseconds()}, nil
}

// runHermeticWorkspace creates a temporary Go module from the snapshot files,
// executes go test in a CI-grade isolated environment, and returns output and exit code.
func runHermeticWorkspace(files []FileSnapshot) (string, int, error) {
    tmpDir, err := os.MkdirTemp("", "pads-replay-*")
    if err != nil {
        return "", 1, fmt.Errorf("mkdirtemp: %w", err)
    }
    defer os.RemoveAll(tmpDir)

    // go.mod
    goMod := filepath.Join(tmpDir, "go.mod")
    if err := os.WriteFile(goMod, []byte("module testmod\n\ngo 1.24\n"), 0644); err != nil {
        return "", 1, fmt.Errorf("write go.mod: %w", err)
    }

    // Write snapshot files
    for _, f := range files {
        target := filepath.Join(tmpDir, filepath.Base(f.FilePath))
        if err := os.WriteFile(target, []byte(f.Content), 0644); err != nil {
            return "", 1, fmt.Errorf("write %s: %w", target, err)
        }
    }

    // CI-grade isolated environment with runtime-detected architecture
    env := append(os.Environ(),
        "GOOS="+runtime.GOOS,
        "GOARCH="+runtime.GOARCH,
        "CGO_ENABLED=0",
        "GOFLAGS=-mod=readonly",
        "GOCACHE="+filepath.Join(tmpDir, ".gocache"),
        "GOMODCACHE="+filepath.Join(tmpDir, ".gomodcache"),
        "GOPATH="+filepath.Join(tmpDir, ".gopath"),
    )

    cmd := exec.Command("go", "test", "-count=1", "./...")
    cmd.Dir = tmpDir
    cmd.Env = env

    out, err := cmd.CombinedOutput()
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            return string(out), exitErr.ExitCode(), nil
        }
        return string(out), 1, err
    }

    return string(out), 0, nil
}

// hashFiles computes a deterministic hash of the snapshot files.
func hashFiles(files []FileSnapshot) [32]byte {
    h := sha256.New()
    for _, f := range files {
        h.Write([]byte(f.FilePath))
        h.Write([]byte(f.Content))
    }
    var sum [32]byte
    h.Sum(sum[:0])
    return sum
}
