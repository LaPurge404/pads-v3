package ci

import (
	"crypto/sha256"
	"fmt"
	"os"
)

// ComputeDigest returns the SHA-256 hash of the raw WAL file.
func ComputeDigest(walPath string) (string, error) {
	data, err := os.ReadFile(walPath)
	if err != nil {
		return "", fmt.Errorf("read WAL: %w", err)
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum), nil
}
