package main

import (
"flag"
"fmt"
"log"
"os"
"os/signal"
"strings"

"pads-v3/internal/compiler"
"pads-v3/internal/engine"
"pads-v3/internal/storage"
)

func main() {
dbPath := flag.String("db", "pads.db", "path to SQLite database")
ingestDir := flag.String("ingest", "", "directory of Go files to ingest")
flag.Parse()

// Ouvrir la base de données
db, err := storage.Open(*dbPath)
if err != nil {
log.Fatalf("open db: %v", err)
}
defer db.Close()

// Ingestion initiale si un répertoire est spécifié
if *ingestDir != "" {
log.Printf("Ingesting Go files from %s...", *ingestDir)
entries, err := os.ReadDir(*ingestDir)
if err != nil {
log.Fatalf("read dir: %v", err)
}
for _, entry := range entries {
if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
path := fmt.Sprintf("%s/%s", *ingestDir, entry.Name())
res, err := compiler.IngestFile(db, path)
if err != nil {
log.Printf("ingest %s: %v", path, err)
continue
}
log.Printf("ingested %s: %d nodes, %d edges", path, res.NodesAdded, res.EdgesAdded)
}
}
}

// Lancer le moteur d'exécution une première fois
log.Println("Running engine...")
if err := engine.RunOnce(db); err != nil {
log.Printf("engine: %v", err)
}

// Afficher l'état final
rows, err := db.Query(`SELECT node_id, state FROM graph_state ORDER BY node_id`)
if err != nil {
log.Fatalf("query state: %v", err)
}
defer rows.Close()
fmt.Println("\n=== Final State (L3) ===")
for rows.Next() {
var id, state string
rows.Scan(&id, &state)
fmt.Printf("  %s -> %s\n", id, state)
}

// Attendre un signal pour arrêter
log.Println("PADS is running. Press Ctrl+C to stop.")
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, os.Interrupt)
<-sigCh
log.Println("Shutting down.")
}
