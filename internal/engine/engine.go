package engine

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"

    "pads-v3/internal/reducer"
    "pads-v3/internal/storage"
)

// RunOnce finds all nodes not yet in graph_state, executes go test for their files,
// generates OS_EXEC_RESULT events, and runs the reducer to update L3.
func RunOnce(db *storage.DB) error {
    // 1. Find all nodes not yet tracked in graph_state
    rows, err := db.Query(`
        SELECT id, file_path FROM nodes
        WHERE id NOT IN (SELECT node_id FROM graph_state)
    `)
    if err != nil {
        return fmt.Errorf("engine: query untracked nodes: %w", err)
    }
    defer rows.Close()

    // Group node IDs by file path
    fileMap := make(map[string][]string)
    for rows.Next() {
        var nodeID, filePath string
        if err := rows.Scan(&nodeID, &filePath); err != nil {
            return fmt.Errorf("engine: scan: %w", err)
        }
        fileMap[filePath] = append(fileMap[filePath], nodeID)
    }
    rows.Close()

    if len(fileMap) == 0 {
        return nil // nothing to test
    }

    // 2. For each file, run go test from the module root
    for filePath, nodeIDs := range fileMap {
        exitCode, stderr, err := runGoTest(filePath)
        if err != nil {
            exitCode = 1
            stderr = err.Error()
        }

        // 3. Generate event
        sanitized := strings.ReplaceAll(filePath, "/", "_")
        eventID := fmt.Sprintf("engine-%s", sanitized)

        _, err = db.InsertEvent(eventID, "OS_EXEC_RESULT", filePath, exitCode)
        if err != nil {
            return fmt.Errorf("engine: insert event: %w", err)
        }
        for _, nid := range nodeIDs {
            db.InsertEventNode(eventID, nid)
        }

        if stderr != "" {
            fmt.Printf("engine: %s stderr: %s\n", filePath, strings.TrimSpace(stderr))
        }
    }

    // 4. Run reducer to update L3
    _, err = reducer.RunReductionLoop(db)
    if err != nil {
        return fmt.Errorf("engine: reducer: %w", err)
    }

    return nil
}

// runGoTest finds the Go module root for the given file and runs go test ./... from there.
func runGoTest(filePath string) (int, string, error) {
    dir := filepath.Dir(filePath)
    moduleRoot, err := findModuleRoot(dir)
    if err != nil {
        // Fallback: use the directory as is, go test will likely fail
        moduleRoot = dir
    }

    cmd := exec.Command("go", "test", "./...")
    cmd.Dir = moduleRoot
    output, err := cmd.CombinedOutput()
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            return exitErr.ExitCode(), string(output), nil
        }
        return 1, string(output), err
    }
    return 0, "", nil
}

// findModuleRoot walks up the directory tree until it finds a go.mod file.
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
