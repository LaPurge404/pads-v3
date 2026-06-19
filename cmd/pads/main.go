package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"time"

	"pads-v3/internal/compiler"
	"pads-v3/internal/engine"
	"pads-v3/internal/scheduler"
	"pads-v3/internal/storage"
)

func main() {
	dbPath := flag.String("db", "pads.db", "path to SQLite database")
	ingestDir := flag.String("ingest", "", "directory of Go files to ingest")
	daemonMode := flag.Bool("daemon", false, "run in daemon mode with continuous execution")
	interval := flag.Duration("interval", 30*time.Second, "interval between engine runs in daemon mode")
	flag.Parse()

	// Open the database
	db, err := storage.Open(*dbPath)
	if err != nil {
		slog.Error("open db", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	// Initial ingestion if a directory is specified
	if *ingestDir != "" {
		slog.Info("ingesting Go files", "dir", *ingestDir)
		entries, err := os.ReadDir(*ingestDir)
		if err != nil {
			slog.Error("read dir", "dir", *ingestDir, "err", err)
			os.Exit(1)
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
				path := fmt.Sprintf("%s/%s", *ingestDir, entry.Name())
				res, err := compiler.IngestFile(db, path)
				if err != nil {
					slog.Warn("ingest file failed", "path", path, "err", err)
					continue
				}
				slog.Info("ingested file", "path", path, "nodes", res.NodesAdded, "edges", res.EdgesAdded)
			}
		}
	}

	if *daemonMode {
		// Daemon mode: continuous loop with scheduler
		slog.Info("starting daemon mode", "interval", *interval)
		sched := scheduler.New(db, *interval)
		go sched.Start()
		defer sched.Stop()

		// Wait for a signal to stop
		slog.Info("daemon running")
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		<-sigCh
		slog.Info("shutting down daemon")
	} else {
		// One-shot mode: single execution
		slog.Info("running engine once")
		if err := engine.RunOnce(db); err != nil {
			slog.Error("engine error", "err", err)
		}

		// Display the final state
		rows, err := db.Query(`SELECT node_id, state FROM graph_state ORDER BY node_id`)
		if err != nil {
			slog.Error("query state", "err", err)
			os.Exit(1)
		}
		defer rows.Close()
		fmt.Println("\n=== Final State (L3) ===")
		for rows.Next() {
			var id, state string
			rows.Scan(&id, &state)
			fmt.Printf("  %s -> %s\n", id, state)
		}
	}
}
