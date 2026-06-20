package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCounterAdd verifies that a counter increments correctly and Value() is readable.
func TestCounterAdd(t *testing.T) {
	c := NewCounter("test_counter", "a test counter")
	c.Add(1)
	c.Add(2)
	c.Add(3)

	if got := c.Value(); got != 6 {
		t.Errorf("counter.Value() = %d, want 6", got)
	}
}

// TestCounterAddNegative verifies that Add accepts negative deltas.
func TestCounterAddNegative(t *testing.T) {
	c := NewCounter("test_counter_neg", "negative delta test")
	c.Add(10)
	c.Add(-3)
	if got := c.Value(); got != 7 {
		t.Errorf("counter.Value() = %d, want 7", got)
	}
}

// TestCounterString verifies that String() implements expvar.Var.
func TestCounterString(t *testing.T) {
	c := NewCounter("test_counter_str", "string test")
	c.Add(42)
	if got := c.String(); got != "42" {
		t.Errorf("counter.String() = %q, want %q", got, "42")
	}
}

// TestPredefinedCounters verifies the 5 package-level counters exist and are readable.
func TestPredefinedCounters(t *testing.T) {
	counters := []*Counter{
		EvolutionCycles,
		UCBUpdates,
		SandboxRuns,
		AutonomousCycles,
		WorkerErrors,
	}
	names := []string{
		"evolution_cycles_total",
		"ucb_updates_total",
		"sandbox_runs_total",
		"autonomous_cycles_total",
		"worker_errors_total",
	}

	for i, c := range counters {
		if c == nil {
			t.Errorf("predefined counter[%d] (%s) is nil", i, names[i])
			continue
		}
		// Value() must not panic.
		_ = c.Value()
		// Increment to confirm atomic operations work.
		c.Add(1)
		if c.Value() != 1 {
			t.Errorf("%s after Add(1) = %d, want 1", names[i], c.Value())
		}
	}
}

// TestPrometheusHandlerOutput verifies the handler returns Prometheus text format.
func TestPrometheusHandlerOutput(t *testing.T) {
	// Reset counters before test to have predictable output.
	EvolutionCycles.value = 0
	UCBUpdates.value = 0
	SandboxRuns.value = 0
	AutonomousCycles.value = 0
	WorkerErrors.value = 0

	EvolutionCycles.Add(5)
	SandboxRuns.Add(3)

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	PrometheusHandler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()

	// Content-Type must be Prometheus text format.
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}

	// Must contain TYPE and HELP lines for each known counter.
	for _, name := range []string{
		"evolution_cycles_total",
		"sandbox_runs_total",
		"autonomous_cycles_total",
	} {
		if !strings.Contains(body, "# HELP "+name) {
			t.Errorf("missing # HELP %s in output", name)
		}
		if !strings.Contains(body, "# TYPE "+name+" counter") {
			t.Errorf("missing # TYPE %s counter in output", name)
		}
	}

	// Counter values must appear as plain integers.
	if !strings.Contains(body, "evolution_cycles_total 5") {
		t.Errorf("missing expected metric value 'evolution_cycles_total 5' in output body")
	}
	if !strings.Contains(body, "sandbox_runs_total 3") {
		t.Errorf("missing expected metric value 'sandbox_runs_total 3' in output body")
	}

	// Version metric must be present.
	if !strings.Contains(body, "pads_version") {
		t.Error("missing pads_version metric")
	}
}

// TestPrometheusHandlerEmpty verifies the handler works when all counters are zero.
func TestPrometheusHandlerEmpty(t *testing.T) {
	EvolutionCycles.value = 0
	UCBUpdates.value = 0
	SandboxRuns.value = 0
	AutonomousCycles.value = 0
	WorkerErrors.value = 0

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	PrometheusHandler()(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	// Zeros must appear as integer 0 (not "0.0" or missing).
	if !strings.Contains(body, "evolution_cycles_total 0") {
		t.Errorf("missing zero counter in body: %s", body)
	}
}

// TestPrometheusHandlerMultipleIncrements verifies values accumulate correctly.
func TestPrometheusHandlerMultipleIncrements(t *testing.T) {
	EvolutionCycles.value = 0
	UCBUpdates.value = 0
	SandboxRuns.value = 0
	AutonomousCycles.value = 0
	WorkerErrors.value = 0

	EvolutionCycles.Add(1)
	EvolutionCycles.Add(1)
	WorkerErrors.Add(7)

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	PrometheusHandler()(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "evolution_cycles_total 2") {
		t.Errorf("expected evolution_cycles_total 2, got body: %s", body)
	}
	if !strings.Contains(body, "worker_errors_total 7") {
		t.Errorf("expected worker_errors_total 7, got body: %s", body)
	}
}