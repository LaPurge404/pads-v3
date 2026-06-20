package memory

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"pads-v3/internal/codeanalysis/semantic"
)

const dbFileName = "semantic_memory.db"

// DB wraps the SQLite connection for the semantic memory index.
type DB struct {
	SQL *sql.DB
}

// Open opens (or creates) the semantic memory SQLite database.
func Open(dir string) (*DB, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("memory.Open mkdir: %w", err)
	}
	path := filepath.Join(dir, dbFileName)
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("memory.Open sql.Open: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = FULL",
		"PRAGMA busy_timeout = 5000",
	}
	for _, p := range pragmas {
		if _, err := sqlDB.Exec(p); err != nil {
			sqlDB.Close()
			return nil, fmt.Errorf("memory.Open pragma %q: %w", p, err)
		}
	}
	if _, err := sqlDB.Exec(schemaDDL); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("memory.Open schema: %w", err)
	}
	return &DB{SQL: sqlDB}, nil
}

// Close closes the database connection.
func (db *DB) Close() error { return db.SQL.Close() }

// WithTransaction runs fn inside a write transaction.
func (db *DB) WithTransaction(fn func(tx *sql.Tx) error) error {
	tx, err := db.SQL.Begin()
	if err != nil {
		return fmt.Errorf("WithTransaction begin: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// SemanticMemory is the main index. It is safe for concurrent use.
type SemanticMemory struct {
	db          *DB
	analyzer    *semantic.Analyzer
	projectRoot string // absolute path for reliable prefix stripping
	mu          sync.RWMutex
}

// New creates a SemanticMemory for the given project root.
// The index is loaded from dir (created if empty) and is ready to query.
func New(projectRoot, dir string) (*SemanticMemory, error) {
	db, err := Open(dir)
	if err != nil {
		return nil, err
	}
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("New abs: %w", err)
	}
	return &SemanticMemory{
		db:          db,
		analyzer:    semantic.NewAnalyzer(projectRoot),
		projectRoot: absRoot,
	}, nil
}

// Close releases all resources held by the memory index.
func (m *SemanticMemory) Close() error { return m.db.Close() }

// Symbol represents a symbol stored in the persistent index.
type Symbol struct {
	ID        string
	Name      string
	Kind      string
	Package   string
	FilePath  string
	Line      int
	Exported  bool
	Signature string
	IsTest    bool
}

// fileHash computes the SHA-256 hex of a file's contents.
func fileHash(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h), nil
}

// rowID returns the deterministic primary key for a symbol.
// Format: "package:file_path:name"
func rowID(pkg, filePath, name string) string {
	return pkg + ":" + filePath + ":" + name
}

// toSymbolRow converts a semantic.Symbol into an indexed row.
// projectRoot is used to strip the prefix and derive the short package name.
func toSymbolRow(sym semantic.Symbol, projectRoot string) (rowIDStr, name, kind, pkg, filePath, lineStr, signature string, exported, isTest int) {
	// Position is set as: fmt.Sprintf("%s:%d", pos.Filename, pos.Line)
	// Find the LAST colon to handle file paths that may contain colons.
	lastColon := strings.LastIndexByte(sym.Position, ':')
	var line int
	if lastColon >= 0 {
		fmt.Sscanf(sym.Position[lastColon+1:], "%d", &line)
	}
	fullFilePath := sym.Position[:lastColon]

	// Derive short package name from the relative path under projectRoot.
	relPath := strings.TrimPrefix(fullFilePath, projectRoot)
	relPath = strings.TrimPrefix(relPath, string(filepath.Separator))
	shortPkg := filepath.Dir(relPath)
	if shortPkg == "." {
		shortPkg = ""
	}
	if shortPkg == "" {
		shortPkg = filepath.Base(filepath.Dir(fullFilePath))
	}

	switch sym.Kind {
	case semantic.KindFunc:
		kind = "func"
	case semantic.KindMethod:
		kind = "method"
	case semantic.KindType:
		kind = "type"
	case semantic.KindVar:
		kind = "var"
	case semantic.KindConst:
		kind = "const"
	default:
		kind = "unknown"
	}

	return rowID(shortPkg, fullFilePath, sym.Name),
		sym.Name, kind, shortPkg, fullFilePath,
		fmt.Sprintf("%d", line), sym.Signature,
		boolToInt(sym.Exported), boolToInt(sym.IsTest)
}

func boolToInt(b bool) int { return map[bool]int{true: 1, false: 0}[b] }

// resolveAndInsertCall resolves a callee name to a symbol_id and inserts
// the call edge into the call_index table.
func resolveAndInsertCall(tx *sql.Tx, callerID, callerPkg, callerFile, calleeName string) {
	cleanName := calleeName
	if idx := strings.LastIndex(calleeName, "."); idx >= 0 {
		cleanName = calleeName[idx+1:]
	}

	var calleeID string
	// Prefer same-package match
	qr := tx.QueryRow(
		`SELECT symbol_id FROM symbol_index WHERE name = ? AND package = ? LIMIT 1`,
		cleanName, callerPkg,
	)
	if err := qr.Scan(&calleeID); err != nil {
		// Fall back to any file with this name
		qr = tx.QueryRow(`SELECT symbol_id FROM symbol_index WHERE name = ? LIMIT 1`, cleanName)
		if err := qr.Scan(&calleeID); err != nil {
			return
		}
	}
	if calleeID != callerID {
		tx.Exec(`INSERT OR IGNORE INTO call_index (caller_id, callee_id) VALUES (?, ?)`, callerID, calleeID)
	}
}

// IncrementallyIndex walks all .go files under projectRoot and updates
// the index for files whose content hash has changed since the last run.
// It only re-indexes stale files, so it is suitable to call on every
// modification without paying the full project cost.
//
// The write lock is held only for the DB transaction; the file walk
// (I/O-heavy) runs without any lock so reads are not blocked.
func (m *SemanticMemory) IncrementallyIndex() error {
	// Phase 1: collect files and hashes without holding any lock.
	var goFiles []string
	if err := filepath.Walk(m.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			goFiles = append(goFiles, path)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("IncrementallyIndex walk: %w", err)
	}

	// Phase 2: index DB writes under a single write lock (brief, no I/O).
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().Unix()

	return m.db.WithTransaction(func(tx *sql.Tx) error {
		upsertSym := `INSERT OR REPLACE INTO symbol_index
			(symbol_id, name, kind, package, file_path, line, exported, signature, is_test)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
		upsertFile := `INSERT OR REPLACE INTO file_index (file_path, file_hash, indexed_at) VALUES (?, ?, ?)`

		// Two-pass indexing: insert ALL symbols first (so callees are always resolvable),
		// then insert call edges. This guarantees callees are in symbol_index before
		// resolveAndInsertCall queries for them.
		for _, filePath := range goFiles {
			hash, err := fileHash(filePath)
			if err != nil {
				continue
			}

			var storedHash string
			if row := tx.QueryRow(`SELECT file_hash FROM file_index WHERE file_path = ?`, filePath); row.Scan(&storedHash) == nil && storedHash == hash {
				continue // unchanged
			}

			sum, err := m.analyzer.AnalyzeFile(filePath)
			if err != nil {
				continue
			}

			// Pass 1: collect all symbols and call edges in memory
			type symData struct {
				rowID, name, kind, pkg, fp, line, sig string
				exp, ist                              int
			}
			type callData struct{ callerID, calleeName, callerPkg string }

			var syms []symData
			var calls []callData

			for _, s := range sum.Symbols {
				rid, name, kind, pkg, fp, line, sig, exp, ist := toSymbolRow(s, m.projectRoot)
				syms = append(syms, symData{rid, name, kind, pkg, fp, line, sig, exp, ist})
				symID := rowID(pkg, filePath, s.Name)
				for _, calleeName := range s.Callees {
					calls = append(calls, callData{symID, calleeName, pkg})
				}
			}

			// Pass 2: upsert all symbols
			for _, s := range syms {
				if _, err := tx.Exec(upsertSym, s.rowID, s.name, s.kind, s.pkg, s.fp, s.line, s.exp, s.sig, s.ist); err != nil {
					return fmt.Errorf("upsert symbol %s: %w", s.rowID, err)
				}
				// Delete old call edges for this symbol
				if _, err := tx.Exec(`DELETE FROM call_index WHERE caller_id = ? OR callee_id = ?`, s.rowID, s.rowID); err != nil {
					return fmt.Errorf("delete calls for %s: %w", s.rowID, err)
				}
			}

			// Pass 3: insert call edges (all symbols are now in the DB)
			for _, c := range calls {
				resolveAndInsertCall(tx, c.callerID, c.callerPkg, filePath, c.calleeName)
			}

			// Update file index
			if _, err := tx.Exec(upsertFile, filePath, hash, now); err != nil {
				return fmt.Errorf("upsert file %s: %w", filePath, err)
			}
		}
		return nil
	})
}

// IndexFile indexes a single file and updates its entry in the file index.
// Call this after a file is modified to update only the changed file.
func (m *SemanticMemory) IndexFile(filePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	hash, err := fileHash(filePath)
	if err != nil {
		return fmt.Errorf("IndexFile hash: %w", err)
	}
	now := time.Now().Unix()

	sum, err := m.analyzer.AnalyzeFile(filePath)
	if err != nil {
		return fmt.Errorf("IndexFile analyze: %w", err)
	}

	return m.db.WithTransaction(func(tx *sql.Tx) error {
		upsertSym := `INSERT OR REPLACE INTO symbol_index
			(symbol_id, name, kind, package, file_path, line, exported, signature, is_test)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
		upsertFile := `INSERT OR REPLACE INTO file_index (file_path, file_hash, indexed_at) VALUES (?, ?, ?)`

		// Two-pass: collect all symbols first, then build call edges.
		type symData struct {
			rowID, name, kind, pkg, fp, line, sig string
			exp, ist                              int
		}
		type callData struct{ callerID, calleeName, callerPkg string }

		var syms []symData
		var calls []callData

		for _, s := range sum.Symbols {
			rid, name, kind, pkg, fp, line, sig, exp, ist := toSymbolRow(s, m.projectRoot)
			syms = append(syms, symData{rid, name, kind, pkg, fp, line, sig, exp, ist})
			symID := rowID(pkg, filePath, s.Name)
			for _, calleeName := range s.Callees {
				calls = append(calls, callData{symID, calleeName, pkg})
			}
		}

		// Pass 2: upsert all symbols
		for _, s := range syms {
			if _, err := tx.Exec(upsertSym, s.rowID, s.name, s.kind, s.pkg, s.fp, s.line, s.exp, s.sig, s.ist); err != nil {
				return fmt.Errorf("upsert symbol %s: %w", s.rowID, err)
			}
			if _, err := tx.Exec(`DELETE FROM call_index WHERE caller_id = ? OR callee_id = ?`, s.rowID, s.rowID); err != nil {
				return fmt.Errorf("delete calls: %w", err)
			}
		}

		// Pass 3: insert call edges (all symbols now in the DB)
		for _, c := range calls {
			resolveAndInsertCall(tx, c.callerID, c.callerPkg, filePath, c.calleeName)
		}

		if _, err := tx.Exec(upsertFile, filePath, hash, now); err != nil {
			return fmt.Errorf("upsert file: %w", err)
		}
		return nil
	})
}

// SymbolCount returns the total number of indexed symbols.
func (m *SemanticMemory) SymbolCount() (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var n int
	return n, m.db.SQL.QueryRow(`SELECT COUNT(*) FROM symbol_index`).Scan(&n)
}

// PackageCount returns the number of distinct packages in the index.
func (m *SemanticMemory) PackageCount() (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var n int
	return n, m.db.SQL.QueryRow(`SELECT COUNT(DISTINCT package) FROM symbol_index`).Scan(&n)
}
