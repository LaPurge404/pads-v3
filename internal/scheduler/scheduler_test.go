package scheduler

import (
	"os"
	"testing"
	"time"

	"pads-v3/internal/storage"
)

func TestScheduler_StartStop(t *testing.T) {
	tmp, _ := os.CreateTemp("", "pads-sched-*.db")
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)

	db, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	s := New(db, 100*time.Millisecond)
	if s == nil {
		t.Fatal("scheduler is nil")
	}

	go s.Start()
	time.Sleep(200 * time.Millisecond)
	s.Stop()
	t.Log("scheduler started and stopped cleanly")
}
