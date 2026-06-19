package chaos

import (
	"os"
	"testing"
)

func TestChaosEngine_Delay(t *testing.T) {
	engine := &Engine{
		Mode:   ModeSilent,
		Faults: []Fault{&DelayFault{MaxDelayMs: 10}},
	}
	err := engine.Inject(&Context{JobID: "j", StepID: "s"})
	if err != nil {
		t.Logf("Chaos injected (expected in some runs): %v", err)
	}
}

func TestChaosEngine_KillWorker(t *testing.T) {
	engine := &Engine{
		Mode:   ModeFull, // use Full mode to guarantee fault application
		Faults: []Fault{&KillWorkerFault{}},
	}
	err := engine.Inject(&Context{JobID: "j", StepID: "s"})
	if err == nil {
		t.Error("expected kill fault to return an error")
	}
}

func TestChaosEngine_CorruptWAL(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := tmpDir + "/wal.log"

	// Create the file first
	if _, err := os.Create(walPath); err != nil {
		t.Fatal(err)
	}

	engine := &Engine{
		Mode:   ModeFull, // use Full mode to guarantee fault application
		Faults: []Fault{&CorruptWALFault{}},
	}
	err := engine.Inject(&Context{WALPath: walPath})
	if err != nil {
		t.Fatal(err)
	}
	// Verify file was corrupted
	data, err := os.ReadFile(walPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{corrupted_event:true}\n" {
		t.Errorf("unexpected WAL content: %s", string(data))
	}
}
