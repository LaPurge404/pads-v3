package memory

import (
	"database/sql"
	"fmt"
	"strings"
)

// CallersOf returns all symbols that call the symbol identified by name.
// If pkg is non-empty, the search is scoped to that package.
func (m *SemanticMemory) CallersOf(name, pkg string) ([]Symbol, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var rows *sql.Rows
	var err error

	if pkg != "" {
		rows, err = m.db.SQL.Query(`
			SELECT DISTINCT s.symbol_id, s.name, s.kind, s.package, s.file_path,
			       s.line, s.exported, s.signature, s.is_test
			FROM call_index ci
			JOIN symbol_index s ON s.symbol_id = ci.caller_id
			WHERE ci.callee_id IN (
				SELECT symbol_id FROM symbol_index WHERE name = ? AND package = ?
			)`, name, pkg)
	} else {
		rows, err = m.db.SQL.Query(`
			SELECT DISTINCT s.symbol_id, s.name, s.kind, s.package, s.file_path,
			       s.line, s.exported, s.signature, s.is_test
			FROM call_index ci
			JOIN symbol_index s ON s.symbol_id = ci.caller_id
			WHERE ci.callee_id IN (
				SELECT symbol_id FROM symbol_index WHERE name = ?
			)`, name)
	}
	if err != nil {
		return nil, fmt.Errorf("CallersOf: %w", err)
	}
	defer rows.Close()

	return scanSymbols(rows)
}

// CalleesOf returns all symbols called by the symbol identified by name.
// If pkg is non-empty, the search is scoped to that package.
func (m *SemanticMemory) CalleesOf(name, pkg string) ([]Symbol, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var rows *sql.Rows
	var err error

	if pkg != "" {
		rows, err = m.db.SQL.Query(`
			SELECT s.symbol_id, s.name, s.kind, s.package, s.file_path,
			       s.line, s.exported, s.signature, s.is_test
			FROM call_index ci
			JOIN symbol_index s ON s.symbol_id = ci.callee_id
			WHERE ci.caller_id IN (
				SELECT symbol_id FROM symbol_index WHERE name = ? AND package = ?
			)`, name, pkg)
	} else {
		rows, err = m.db.SQL.Query(`
			SELECT s.symbol_id, s.name, s.kind, s.package, s.file_path,
			       s.line, s.exported, s.signature, s.is_test
			FROM call_index ci
			JOIN symbol_index s ON s.symbol_id = ci.callee_id
			WHERE ci.caller_id IN (
				SELECT symbol_id FROM symbol_index WHERE name = ?
			)`, name)
	}
	if err != nil {
		return nil, fmt.Errorf("CalleesOf: %w", err)
	}
	defer rows.Close()

	return scanSymbols(rows)
}

// SymbolByName looks up a symbol by exact name. If pkg is non-empty,
// the result is scoped to that package; otherwise the first match is returned.
func (m *SemanticMemory) SymbolByName(name, pkg string) (*Symbol, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var row *sql.Row
	if pkg != "" {
		row = m.db.SQL.QueryRow(`
			SELECT symbol_id, name, kind, package, file_path, line, exported, signature, is_test
			FROM symbol_index WHERE name = ? AND package = ? LIMIT 1`, name, pkg)
	} else {
		row = m.db.SQL.QueryRow(`
			SELECT symbol_id, name, kind, package, file_path, line, exported, signature, is_test
			FROM symbol_index WHERE name = ? LIMIT 1`, name)
	}
	return scanSymbolRow(row)
}

// ExportedSymbols returns all exported (public) symbols.
// If kind is non-empty ("func", "type", "var", "const"), results are filtered.
func (m *SemanticMemory) ExportedSymbols(kind string) ([]Symbol, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var rows *sql.Rows
	var err error
	if kind != "" {
		rows, err = m.db.SQL.Query(`
			SELECT symbol_id, name, kind, package, file_path, line, exported, signature, is_test
			FROM symbol_index WHERE exported = 1 AND kind = ?
			ORDER BY package, name`, kind)
	} else {
		rows, err = m.db.SQL.Query(`
			SELECT symbol_id, name, kind, package, file_path, line, exported, signature, is_test
			FROM symbol_index WHERE exported = 1
			ORDER BY package, name`)
	}
	if err != nil {
		return nil, fmt.Errorf("ExportedSymbols: %w", err)
	}
	defer rows.Close()
	return scanSymbols(rows)
}

// SymbolsInFile returns all symbols defined in the given file.
func (m *SemanticMemory) SymbolsInFile(filePath string) ([]Symbol, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.SQL.Query(`
		SELECT symbol_id, name, kind, package, file_path, line, exported, signature, is_test
		FROM symbol_index WHERE file_path = ? ORDER BY line`, filePath)
	if err != nil {
		return nil, fmt.Errorf("SymbolsInFile: %w", err)
	}
	defer rows.Close()
	return scanSymbols(rows)
}

// SymbolsInPackage returns all symbols in the given package.
func (m *SemanticMemory) SymbolsInPackage(pkg string) ([]Symbol, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.SQL.Query(`
		SELECT symbol_id, name, kind, package, file_path, line, exported, signature, is_test
		FROM symbol_index WHERE package = ? ORDER BY file_path, line`, pkg)
	if err != nil {
		return nil, fmt.Errorf("SymbolsInPackage: %w", err)
	}
	defer rows.Close()
	return scanSymbols(rows)
}

// PublicAPISurface returns the full public API surface of the project:
// all exported functions, types, and constants grouped by package.
func (m *SemanticMemory) PublicAPISurface() (map[string][]Symbol, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.SQL.Query(`
		SELECT symbol_id, name, kind, package, file_path, line, exported, signature, is_test
		FROM symbol_index
		WHERE exported = 1 AND is_test = 0
		ORDER BY package, kind, name`)
	if err != nil {
		return nil, fmt.Errorf("PublicAPISurface: %w", err)
	}
	defer rows.Close()

	surface := make(map[string][]Symbol)
	for rows.Next() {
		var s Symbol
		if err := rows.Scan(&s.ID, &s.Name, &s.Kind, &s.Package, &s.FilePath,
			&s.Line, &s.Exported, &s.Signature, &s.IsTest); err != nil {
			return nil, fmt.Errorf("PublicAPISurface scan: %w", err)
		}
		surface[s.Package] = append(surface[s.Package], s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("PublicAPISurface rows.Err: %w", err)
	}
	return surface, nil
}

// SymbolImpact returns the impact analysis for a symbol:
// number of direct callers and number of transitive callers.
// This is useful for evaluating the risk of modifying a symbol.
func (m *SemanticMemory) SymbolImpact(name, pkg string) (direct, transitive int, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Direct callers
	if pkg != "" {
		err = m.db.SQL.QueryRow(`
			SELECT COUNT(DISTINCT caller_id) FROM call_index
			WHERE callee_id = (SELECT symbol_id FROM symbol_index WHERE name = ? AND package = ?)`,
			name, pkg).Scan(&direct)
	} else {
		err = m.db.SQL.QueryRow(`
			SELECT COUNT(DISTINCT caller_id) FROM call_index
			WHERE callee_id = (SELECT symbol_id FROM symbol_index WHERE name = ? LIMIT 1)`,
			name).Scan(&direct)
	}
	if err != nil {
		return 0, 0, fmt.Errorf("SymbolImpact direct: %w", err)
	}

	// Transitive: callers of my callers (2 hops)
	var t int
	row := m.db.SQL.QueryRow(`
		SELECT COUNT(DISTINCT ci2.caller_id) FROM call_index ci1
		JOIN call_index ci2 ON ci2.callee_id = ci1.caller_id
		WHERE ci1.callee_id = (SELECT symbol_id FROM symbol_index WHERE name = ? AND package = ? LIMIT 1)
		  AND ci2.caller_id != ci1.callee_id`, name, pkg)
	if err := row.Scan(&t); err != nil && err != sql.ErrNoRows {
		return direct, 0, fmt.Errorf("SymbolImpact transitive: %w", err)
	}
	transitive = t
	return direct, transitive, nil
}

// scanSymbols scans multiple rows into a slice of Symbol.
func scanSymbols(rows *sql.Rows) ([]Symbol, error) {
	var symbols []Symbol
	for rows.Next() {
		var s Symbol
		if err := rows.Scan(&s.ID, &s.Name, &s.Kind, &s.Package, &s.FilePath,
			&s.Line, &s.Exported, &s.Signature, &s.IsTest); err != nil {
			return nil, fmt.Errorf("scanSymbols: %w", err)
		}
		symbols = append(symbols, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scanSymbols rows.Err: %w", err)
	}
	return symbols, nil
}

// scanSymbolRow scans a single row into a *Symbol.
func scanSymbolRow(row *sql.Row) (*Symbol, error) {
	var s Symbol
	err := row.Scan(&s.ID, &s.Name, &s.Kind, &s.Package, &s.FilePath,
		&s.Line, &s.Exported, &s.Signature, &s.IsTest)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scanSymbolRow: %w", err)
	}
	return &s, nil
}

// QueryContext returns a human-readable summary of the semantic knowledge
// about a target file and its surrounding code. This is designed to be
// injected into a CodeAgent prompt to give the LLM rich context before
// it generates a plan.
//
// It covers:
//   - Symbols defined in the file (exported and internal)
//   - Callers of exported symbols (regression risk)
//   - Callees of exported symbols (blast radius)
//   - Overall symbol impact (direct + transitive callers)
//
// If semMem is nil, returns an empty string (no context available).
func (m *SemanticMemory) QueryContext(targetFile string) string {
	if m == nil {
		return ""
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Gather symbols in the file
	syms, _ := m.symbolsInFile(targetFile)
	if len(syms) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Semantic Context\n")

	exported := make([]Symbol, 0, len(syms))
	for _, s := range syms {
		if s.Exported {
			exported = append(exported, s)
		}
	}

	// Summarize what's exported from this file.
	if len(exported) > 0 {
		b.WriteString("Exported API:\n")
		for _, s := range exported {
			b.WriteString(fmt.Sprintf("  [%s] %s %s — defined at line %d\n", s.Kind, s.Name, s.Signature, s.Line))
		}
		b.WriteString("\n")
	}

	// For each exported symbol, summarize callers and callees.
	for _, s := range exported {
		// Callers (who depends on this?)
		if callers, err := m.callersOf(s.Name, s.Package); err == nil && len(callers) > 0 {
			b.WriteString(fmt.Sprintf("Callers of %s (%d direct):\n", s.Name, len(callers)))
			for _, c := range callers {
				b.WriteString(fmt.Sprintf("  - %s.%s (line %d, %s)\n", c.Package, c.Name, c.Line, c.Kind))
			}
			b.WriteString("\n")
		}

		// Callees (what does this depend on?)
		if callees, err := m.calleesOf(s.Name, s.Package); err == nil && len(callees) > 0 {
			b.WriteString(fmt.Sprintf("Callees of %s (%d direct):\n", s.Name, len(callees)))
			for _, c := range callees {
				if c.Exported {
					b.WriteString(fmt.Sprintf("  - %s.%s (exported %s)\n", c.Package, c.Name, c.Kind))
				}
			}
			b.WriteString("\n")
		}

		// Impact summary
		if direct, transitive, err := m.symbolImpact(s.Name, s.Package); err == nil && (direct > 0 || transitive > 0) {
			b.WriteString(fmt.Sprintf("Impact: %d direct, %d transitive callers\n\n", direct, transitive))
		}
	}

	return b.String()
}

// callersOf is the internal read-only version of CallersOf.
func (m *SemanticMemory) callersOf(name, pkg string) ([]Symbol, error) {
	var rows *sql.Rows
	var err error
	if pkg != "" {
		rows, err = m.db.SQL.Query(`
			SELECT DISTINCT s.symbol_id, s.name, s.kind, s.package, s.file_path,
			       s.line, s.exported, s.signature, s.is_test
			FROM call_index ci
			JOIN symbol_index s ON s.symbol_id = ci.caller_id
			WHERE ci.callee_id IN (
				SELECT symbol_id FROM symbol_index WHERE name = ? AND package = ?
			)`, name, pkg)
	} else {
		rows, err = m.db.SQL.Query(`
			SELECT DISTINCT s.symbol_id, s.name, s.kind, s.package, s.file_path,
			       s.line, s.exported, s.signature, s.is_test
			FROM call_index ci
			JOIN symbol_index s ON s.symbol_id = ci.caller_id
			WHERE ci.callee_id IN (
				SELECT symbol_id FROM symbol_index WHERE name = ?
			)`, name)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSymbols(rows)
}

// calleesOf is the internal read-only version of CalleesOf.
func (m *SemanticMemory) calleesOf(name, pkg string) ([]Symbol, error) {
	var rows *sql.Rows
	var err error
	if pkg != "" {
		rows, err = m.db.SQL.Query(`
			SELECT s.symbol_id, s.name, s.kind, s.package, s.file_path,
			       s.line, s.exported, s.signature, s.is_test
			FROM call_index ci
			JOIN symbol_index s ON s.symbol_id = ci.callee_id
			WHERE ci.caller_id IN (
				SELECT symbol_id FROM symbol_index WHERE name = ? AND package = ?
			)`, name, pkg)
	} else {
		rows, err = m.db.SQL.Query(`
			SELECT s.symbol_id, s.name, s.kind, s.package, s.file_path,
			       s.line, s.exported, s.signature, s.is_test
			FROM call_index ci
			JOIN symbol_index s ON s.symbol_id = ci.callee_id
			WHERE ci.caller_id IN (
				SELECT symbol_id FROM symbol_index WHERE name = ?
			)`, name)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSymbols(rows)
}

// symbolImpact is the internal read-only version of SymbolImpact.
func (m *SemanticMemory) symbolImpact(name, pkg string) (direct, transitive int, err error) {
	if err = m.db.SQL.QueryRow(`
		SELECT COUNT(DISTINCT caller_id) FROM call_index
		WHERE callee_id = (SELECT symbol_id FROM symbol_index WHERE name = ? AND package = ? LIMIT 1)`,
		name, pkg).Scan(&direct); err != nil {
		return 0, 0, err
	}
	var t int
	row := m.db.SQL.QueryRow(`
		SELECT COUNT(DISTINCT ci2.caller_id) FROM call_index ci1
		JOIN call_index ci2 ON ci2.callee_id = ci1.caller_id
		WHERE ci1.callee_id = (SELECT symbol_id FROM symbol_index WHERE name = ? AND package = ? LIMIT 1)
		  AND ci2.caller_id != ci1.callee_id`, name, pkg)
	if err := row.Scan(&t); err != nil && err != sql.ErrNoRows {
		return direct, 0, err
	}
	return direct, t, nil
}

// symbolsInFile is the internal read-only version of SymbolsInFile.
func (m *SemanticMemory) symbolsInFile(filePath string) ([]Symbol, error) {
	rows, err := m.db.SQL.Query(`
		SELECT symbol_id, name, kind, package, file_path, line, exported, signature, is_test
		FROM symbol_index WHERE file_path = ? ORDER BY line`, filePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSymbols(rows)
}
