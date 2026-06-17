package evolution_test

import (
	"os"
	"testing"

	"pads-v3/internal/policy/evolution"
)

func TestEventStore_AppendAndLoad(t *testing.T) {
	store := evolution.NewEventStore(t.TempDir() + "/events.log")
	ev := evolution.Event{
		Sequence:       1,
		CandidateScore: 88,
		CurrentScore:   44,
		Weight:         1.0,
		Mode:           evolution.ModeStable,
		BanditSeed:     0,
	}
	err := store.Append(ev)
	if err != nil {
		t.Fatal(err)
	}

	events, err := store.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].CandidateScore != 88 {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestEventStore_CorruptedLineResilience(t *testing.T) {
	// Create a log file with mixed valid and corrupted lines
	// Note: Go json.Marshal uses exact field names (CandidateScore, not candidate_score)
	dir := t.TempDir()
	logPath := dir + "/corrupted_events.log"

	lines := []string{
		`{"Sequence":1,"CandidateScore":80,"CurrentScore":40,"Weight":1.0,"Mode":"stable","BanditSeed":0}`,
		`INVALID_CORRUPTED_JSON_LINE{{{`,
		`{"Sequence":2,"CandidateScore":85,"CurrentScore":80,"Weight":1.0,"Mode":"stable","BanditSeed":0}`,
		`not valid json at all`,
		`{"Sequence":3,"CandidateScore":90,"CurrentScore":85,"Weight":1.0,"Mode":"bandit","BanditSeed":0}`,
		``,
		`{"Sequence":0,"CandidateScore":0,"CurrentScore":0,"Weight":0,"Mode":"","BanditSeed":0}`, // empty but valid JSON
		`{"Sequence":4,"CandidateScore":95,"CurrentScore":90,"Weight":0.5,"Mode":"stable","BanditSeed":0}`,
	}

	f, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range lines {
		f.WriteString(line + "\n")
	}
	f.Close()

	// Load should skip only lines with JSON parse errors
	store := evolution.NewEventStore(logPath)
	events, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll should not return error on corrupted log: %v", err)
	}

	// Should have 5 events: 4 with data + 1 empty-but-valid
	if len(events) != 5 {
		t.Fatalf("expected 5 events (4 valid + 1 empty object), got %d: %+v", len(events), events)
	}

	// Verify sequences are in order (0 from {} comes after 3, before 4)
	expectedSeqs := []int{1, 2, 3, 0, 4}
	for i, ev := range events {
		if ev.Sequence != expectedSeqs[i] {
			t.Errorf("event[%d] sequence: expected %d, got %d", i, expectedSeqs[i], ev.Sequence)
		}
	}

	// Verify specific scores
	if events[0].CandidateScore != 80 {
		t.Errorf("event[0] CandidateScore: expected 80, got %d", events[0].CandidateScore)
	}
	if events[2].Mode != evolution.ModeBandit {
		t.Errorf("event[2] Mode: expected ModeBandit, got %v", events[2].Mode)
	}
	if events[4].CandidateScore != 95 {
		t.Errorf("event[4] CandidateScore: expected 95, got %d", events[4].CandidateScore)
	}

	// Verify invalid JSON lines (INVALID_CORRUPTED_JSON_LINE, "not valid json", empty) were skipped
	// These would cause parse errors, not captured in events
	for _, ev := range events {
		if ev.CandidateScore == 0 && ev.Sequence != 0 {
			t.Errorf("found event with CandidateScore 0 but non-zero sequence %d - possible missed parse error", ev.Sequence)
		}
	}
}

func TestEventStore_EmptyCorruptedFile(t *testing.T) {
	dir := t.TempDir()
	logPath := dir + "/empty_corrupted.log"

	// Create file with only corrupted content
	f, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("INVALID_JSON\n{{{{BROKEN}}\n")
	f.Close()

	store := evolution.NewEventStore(logPath)
	events, err := store.LoadAll()

	// Should return empty slice, no error
	if err != nil {
		t.Fatalf("LoadAll should not error on fully corrupted file: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events from corrupted file, got %d", len(events))
	}
}

func TestEventStore_AppendAfterCorruption(t *testing.T) {
	dir := t.TempDir()
	logPath := dir + "/append_after_corrupt.log"

	// Write corrupted content
	os.WriteFile(logPath, []byte("INVALID_LINE\n"), 0644)

	store := evolution.NewEventStore(logPath)

	// Append a valid event
	ev := evolution.Event{
		Sequence:       1,
		CandidateScore: 100,
		CurrentScore:   50,
		Weight:         1.0,
		Mode:           evolution.ModeStable,
	}
	if err := store.Append(ev); err != nil {
		t.Fatal(err)
	}

	// Load should return the valid appended event
	events, err := store.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event after append, got %d", len(events))
	}
	if events[0].CandidateScore != 100 {
		t.Errorf("expected CandidateScore 100, got %d", events[0].CandidateScore)
	}
}
