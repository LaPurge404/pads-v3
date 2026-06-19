//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	readyFile := os.Args[1]
	port := os.Args[2]
	hits := 0

	// Signal readiness.
	f, err := os.Create(readyFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create ready file: %v\n", err)
		os.Exit(1)
	}
	f.Close()

	http.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		hits++
		fmt.Fprintf(os.Stderr, "hit %d\n", hits)
		time.Sleep(50 * time.Millisecond)
		if hits < 3 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{
					"content": `{"patch":"// ok after retries","explanation":"ok","confidence":0.9,"warnings":[]}`,
				}},
			},
		})
	})

	fmt.Fprintf(os.Stderr, "listening on %s\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		fmt.Fprintf(os.Stderr, "server exit: %v\n", err)
	}
}
