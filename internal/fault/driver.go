package fault

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"math/rand"
	"time"

	"modernc.org/sqlite"
)

// FaultConfig defines the failure injection parameters.
type FaultConfig struct {
	ErrorRate     float64
	BusyRate      float64
	IORate        float64
	LatencyMin    time.Duration
	LatencyMax    time.Duration
	WriteFailRate float64
}

type injectableDriver struct {
	realDriver driver.Driver
	cfg        FaultConfig
	rng        *rand.Rand
}

func (d *injectableDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.realDriver.Open(name)
	if err != nil {
		return nil, err
	}
	return &injectableConn{
		conn: conn,
		cfg:  d.cfg,
		rng:  d.rng,
	}, nil
}

type injectableConn struct {
	conn driver.Conn
	cfg  FaultConfig
	rng  *rand.Rand
}

func (c *injectableConn) injectLatency() {
	if c.cfg.LatencyMax > 0 {
		sleep := c.cfg.LatencyMin + time.Duration(c.rng.Int63n(int64(c.cfg.LatencyMax-c.cfg.LatencyMin)))
		time.Sleep(sleep)
	}
}

func (c *injectableConn) shouldError(rate float64) bool {
	return c.rng.Float64() < rate
}

func (c *injectableConn) Prepare(query string) (driver.Stmt, error) {
	c.injectLatency()
	if c.shouldError(c.cfg.ErrorRate) {
		return nil, fmt.Errorf("fault injected: prepare error")
	}
	stmt, err := c.conn.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &injectableStmt{stmt: stmt, cfg: c.cfg, rng: c.rng}, nil
}

func (c *injectableConn) Close() error {
	return c.conn.Close()
}

func (c *injectableConn) Begin() (driver.Tx, error) {
	c.injectLatency()
	if c.shouldError(c.cfg.ErrorRate) {
		return nil, fmt.Errorf("fault injected: begin error")
	}
	// Prefer context-aware BeginTx when the underlying driver supports it.
	tx, _ := c.BeginTx(context.Background(), nil)
	return tx, nil
}

// BeginTx implements driver.ConnBeginTx.
// It delegates to the underlying connection if it supports Tx.
func (c *injectableConn) BeginTx(ctx context.Context, opts *driver.TxOptions) (driver.Tx, error) {
	c.injectLatency()
	if c.shouldError(c.cfg.ErrorRate) {
		return nil, fmt.Errorf("fault injected: begin error")
	}
	if tx, ok := c.conn.(driver.ConnBeginTx); ok {
		if opts == nil {
			zeroOpts := driver.TxOptions{}
			return tx.BeginTx(ctx, zeroOpts)
		}
		return tx.BeginTx(ctx, *opts)
	}
	// Fallback for drivers that do not implement ConnBeginTx (deprecated but required for interface completeness).
	return c.conn.Begin()
}

type injectableStmt struct {
	stmt driver.Stmt
	cfg  FaultConfig
	rng  *rand.Rand
}

func (s *injectableStmt) Close() error {
	return s.stmt.Close()
}

func (s *injectableStmt) NumInput() int {
	return s.stmt.NumInput()
}

func (s *injectableStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.injectLatency()
	if s.shouldError(s.cfg.ErrorRate) {
		return nil, fmt.Errorf("fault injected: exec error")
	}
	if s.shouldError(s.cfg.WriteFailRate) {
		return nil, fmt.Errorf("fault injected: write failure")
	}
	if ec, ok := s.stmt.(driver.StmtExecContext); ok {
		return ec.ExecContext(context.Background(), namedValuesFromValues(args))
	}
	return s.stmt.Exec(args)
}

func (s *injectableStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.injectLatency()
	if s.shouldError(s.cfg.ErrorRate) {
		return nil, fmt.Errorf("fault injected: query error")
	}
	if s.shouldError(s.cfg.IORate) {
		return nil, fmt.Errorf("fault injected: IO error")
	}
	if qc, ok := s.stmt.(driver.StmtQueryContext); ok {
		return qc.QueryContext(context.Background(), namedValuesFromValues(args))
	}
	return s.stmt.Query(args)
}

// ExecContext implements driver.StmtExecContext.
func (s *injectableStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	s.injectLatency()
	if s.shouldError(s.cfg.ErrorRate) {
		return nil, fmt.Errorf("fault injected: exec error")
	}
	if s.shouldError(s.cfg.WriteFailRate) {
		return nil, fmt.Errorf("fault injected: write failure")
	}
	if ec, ok := s.stmt.(driver.StmtExecContext); ok {
		return ec.ExecContext(ctx, args)
	}
	// Fallback: convert NamedValue to Value
	vals := make([]driver.Value, len(args))
	for i, a := range args {
		vals[i] = a.Value
	}
	return s.stmt.Exec(vals)
}

// QueryContext implements driver.StmtQueryContext.
func (s *injectableStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	s.injectLatency()
	if s.shouldError(s.cfg.ErrorRate) {
		return nil, fmt.Errorf("fault injected: query error")
	}
	if s.shouldError(s.cfg.IORate) {
		return nil, fmt.Errorf("fault injected: IO error")
	}
	if qc, ok := s.stmt.(driver.StmtQueryContext); ok {
		return qc.QueryContext(ctx, args)
	}
	// Fallback: convert NamedValue to Value
	vals := make([]driver.Value, len(args))
	for i, a := range args {
		vals[i] = a.Value
	}
	return s.stmt.Query(vals)
}

func (s *injectableStmt) injectLatency() {
	if s.cfg.LatencyMax > 0 {
		sleep := s.cfg.LatencyMin + time.Duration(s.rng.Int63n(int64(s.cfg.LatencyMax-s.cfg.LatencyMin)))
		time.Sleep(sleep)
	}
}

func (s *injectableStmt) shouldError(rate float64) bool {
	return s.rng.Float64() < rate
}

// OpenFaultDB opens a database with fault injection enabled.
func OpenFaultDB(dbPath string, cfg FaultConfig) (*sql.DB, error) {
	driverName := fmt.Sprintf("fault-%d", rand.Int63())
	sql.Register(driverName, &injectableDriver{
		realDriver: &sqlite.Driver{},
		cfg:        cfg,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
	})
	return sql.Open(driverName, dbPath)
}

// namedValuesFromValues converts []driver.Value to []driver.NamedValue.
// This is needed when forwarding legacy Exec/Query calls to context-aware methods.
func namedValuesFromValues(vals []driver.Value) []driver.NamedValue {
	out := make([]driver.NamedValue, len(vals))
	for i, v := range vals {
		out[i] = driver.NamedValue{Ordinal: i + 1, Value: v}
	}
	return out
}