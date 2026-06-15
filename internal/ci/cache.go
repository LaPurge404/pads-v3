package ci

import (
    "crypto/sha256"
    "encoding/hex"
    "os"
    "path/filepath"
    "sort"
)

type Cache struct {
    Root string
}

func NewCache(root string) Cache {
    _ = os.MkdirAll(root, 0755)
    return Cache{Root: root}
}

// Key generates a fully deterministic cache key.
// It includes job ID, step ID, step run command, matrix hash, and input.
func (c Cache) Key(jobID string, step Step, input string, matrixHash string) string {
    h := sha256.New()
    h.Write([]byte(jobID))
    h.Write([]byte(step.ID))
    h.Write([]byte(step.Run))
    h.Write([]byte(matrixHash))
    h.Write([]byte(input))
    return hex.EncodeToString(h.Sum(nil))
}

func (c Cache) Hit(key string) (string, bool) {
    path := filepath.Join(c.Root, key+".cache")
    b, err := os.ReadFile(path)
    if err != nil {
        return "", false
    }
    return string(b), true
}

func (c Cache) Store(key string, output string) error {
    path := filepath.Join(c.Root, key+".cache")
    return os.WriteFile(path, []byte(output), 0644)
}

// flatten converts matrix vars into a deterministic string.
func flatten(vars map[string]string) string {
    keys := make([]string, 0, len(vars))
    for k := range vars {
        keys = append(keys, k)
    }
    sort.Strings(keys)

    out := ""
    for _, k := range keys {
        out += k + "=" + vars[k] + ";"
    }
    return out
}

// cloneMap creates a shallow copy of a map.
func cloneMap(m map[string]string) map[string]string {
    cp := make(map[string]string)
    for k, v := range m {
        cp[k] = v
    }
    return cp
}
