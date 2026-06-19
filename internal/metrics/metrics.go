// Package metrics provides Prometheus-compatible metrics via expvar.
package metrics

import (
	"expvar"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"
)

// Counter is an atomic increment-only 64-bit integer metric.
type Counter struct {
	name   string
	help   string
	value  int64
	pubVar *expvar.Int
}

// NewCounter creates a named counter with a help string and publishes it via expvar.
func NewCounter(name, help string) *Counter {
	c := &Counter{name: name, help: help}
	c.pubVar = expvar.NewInt(name)
	return c
}

// Add increments the counter by delta.
func (c *Counter) Add(delta int64) {
	atomic.AddInt64(&c.value, delta)
	c.pubVar.Add(delta)
}

// Value returns the current count.
func (c *Counter) Value() int64 {
	return atomic.LoadInt64(&c.value)
}

// String implements expvar.Var (used only by expvar.Handler/debug/vars).
func (c *Counter) String() string {
	return fmt.Sprintf("%d", c.Value())
}

// --- Pre-defined counters (package-level, lazily initialized) ---

var (
	EvolutionCycles   = newCounter("evolution_cycles_total", "Total number of evolution cycles run")
	UCBUpdates        = newCounter("ucb_updates_total", "Total number of UCB selector reward updates")
	SandboxRuns       = newCounter("sandbox_runs_total", "Total number of sandbox executions")
	AutonomousCycles  = newCounter("autonomous_cycles_total", "Total number of autonomous mode cycles")
	WorkerErrors      = newCounter("worker_errors_total", "Total number of worker process errors")
)

// newCounter is a package-level helper for the pre-defined counters.
func newCounter(name, help string) *Counter { return NewCounter(name, help) }

// PrometheusHandler returns an HTTP handler that outputs all metrics
// in Prometheus text exposition format. Public (no auth).
func PrometheusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		// Collect all expvar metrics and filter those that look like Prometheus counters.
		// We also include our known counters directly to guarantee they appear.
		metrics := []promLine{
			{name: "evolution_cycles_total", value: EvolutionCycles.Value()},
			{name: "ucb_updates_total", value: UCBUpdates.Value()},
			{name: "sandbox_runs_total", value: SandboxRuns.Value()},
			{name: "autonomous_cycles_total", value: AutonomousCycles.Value()},
			{name: "worker_errors_total", value: WorkerErrors.Value()},
		}

		// Gather any other expvar counters added by third-party packages.
		expvar.Do(func(kv expvar.KeyValue) {
			switch v := kv.Value.(type) {
			case *expvar.Int:
				// Skip our own counters (already listed above).
				switch kv.Key {
				case "evolution_cycles_total", "ucb_updates_total",
					"sandbox_runs_total", "autonomous_cycles_total", "worker_errors_total":
					return
				}
				metrics = append(metrics, promLine{name: kv.Key, value: v.Value()})
			}
		})

		sort.Slice(metrics, func(i, j int) bool {
			return strings.Compare(metrics[i].name, metrics[j].name) < 0
		})

		// Build output.
		var b strings.Builder
		b.WriteString("# HELP pads_version PADS version info\n")
		b.WriteString("# TYPE pads_version gauge\n")
		b.WriteString("pads_version{version=\"0.9.0\"} 1\n\n")

		for _, m := range metrics {
			b.WriteString(fmt.Sprintf("# HELP %s\n", m.name))
			b.WriteString(fmt.Sprintf("# TYPE %s counter\n", m.name))
			b.WriteString(fmt.Sprintf("%s %d\n\n", m.name, m.value))
		}

		w.Write([]byte(b.String()))
	}
}

type promLine struct {
	name  string
	value int64
}