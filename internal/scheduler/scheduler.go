package scheduler

import (
	"log/slog"
	"sync"
	"time"

	"pads-v3/internal/agent"
	"pads-v3/internal/engine"
	"pads-v3/internal/storage"
)

// Scheduler runs the Execution Engine in a loop with a configurable interval.
type Scheduler struct {
	db       *storage.DB
	interval time.Duration
	running  bool
	mu       sync.Mutex
}

// New creates a new Scheduler.
func New(db *storage.DB, interval time.Duration) *Scheduler {
	return &Scheduler{
		db:       db,
		interval: interval,
	}
}

// Start begins the scheduler loop. It runs until Stop is called.
func (s *Scheduler) Start() {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	mockAgent := agent.MockAgent{}

	for s.isRunning() {
		slog.Info("scheduler: running engine cycle")
		err := engine.RunOnce(s.db)
		if err != nil {
			slog.Error("scheduler: engine error", "err", err)
		}

		slog.Info("scheduler: running agent cycle")
		err = agent.RunOnce(s.db, mockAgent)
		if err != nil {
			slog.Error("scheduler: agent error", "err", err)
		}

		time.Sleep(s.interval)
	}
}

// Stop halts the scheduler loop.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}

func (s *Scheduler) isRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
