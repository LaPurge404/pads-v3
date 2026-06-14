package chaos

import (
    "os"
    "testing"
    "time"

    "pads-v3/internal/fault"
    "pads-v3/internal/storage"
)

func TestFaultInjectedConvergence(t *testing.T) {
    path := createTempDB(t)
    defer os.Remove(path)

    // Phase 1 : créer le schéma et insérer l'événement de test sans injection de pannes
    dbClean, err := storage.Open(path)
    if err != nil {
        t.Fatal(err)
    }
    _, err = dbClean.InsertEvent("evt-1", "OS_EXEC_RESULT", "test", 0)
    if err != nil {
        dbClean.Close()
        t.Fatal(err)
    }
    err = dbClean.InsertEventNode("evt-1", "main.A")
    if err != nil {
        dbClean.Close()
        t.Fatal(err)
    }
    dbClean.Close()

    // Phase 2 : ouvrir avec le driver d'injection de pannes
    cfg := fault.FaultConfig{
        ErrorRate:     0.1,
        BusyRate:      0.05,
        IORate:        0.1,
        LatencyMin:    10 * time.Millisecond,
        LatencyMax:    50 * time.Millisecond,
        WriteFailRate: 0.2,
    }

    dbFault, err := fault.OpenFaultDB(path, cfg)
    if err != nil {
        t.Fatal(err)
    }
    defer dbFault.Close()

    // Tenter la réduction en présence de pannes, jusqu'à convergence
    var state string
    for i := 0; i < 10; i++ {
        dbFault.QueryRow(`SELECT state FROM graph_state WHERE node_id = 'main.A'`).Scan(&state)
        if state == "STABLE" {
            break
        }
        _, err := dbFault.Exec(`
            DELETE FROM graph_state;
            INSERT OR REPLACE INTO graph_state (node_id, state, last_event_id, last_exit_code)
            SELECT en.node_id, CASE WHEN e.exit_code = 0 THEN 'STABLE' ELSE 'BROKEN' END, e.event_id, e.exit_code
            FROM events e JOIN event_nodes en ON e.event_id = en.event_id
            WHERE e.event_id = 'evt-1'
        `)
        if err != nil {
            t.Logf("reduction attempt %d failed: %v", i, err)
        }
    }

    if state != "STABLE" {
        t.Errorf("system did not converge to STABLE after 10 attempts, got '%s'", state)
    }
}
