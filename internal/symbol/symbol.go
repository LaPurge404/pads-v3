package symbol

import (
    "database/sql"
    "fmt"
    "sort"
    "strings"

    "pads-v3/internal/storage"
)

type Kind string

const (
    KindFunc    Kind = "func"
    KindMethod  Kind = "method"
    KindType    Kind = "type"
    KindVar     Kind = "var"
    KindUnknown Kind = "unknown"
)

type Symbol struct {
    Package  string
    Receiver string
    Name     string
    Kind     Kind
    NodeID   string
}

type SymbolTable struct {
    index map[string]map[string][]Symbol
}

func BuildSymbolTable(db *storage.DB) (*SymbolTable, error) {
    rows, err := db.Query(`SELECT id, type FROM nodes ORDER BY id`)
    if err != nil {
        return nil, fmt.Errorf("BuildSymbolTable: %w", err)
    }
    defer rows.Close()

    st := &SymbolTable{
        index: make(map[string]map[string][]Symbol),
    }

    for rows.Next() {
        var id, typ string
        if err := rows.Scan(&id, &typ); err != nil {
            return nil, fmt.Errorf("BuildSymbolTable: scan: %w", err)
        }
        sym := parseNodeID(id, typ)
        if sym == nil {
            continue
        }
        st.addSymbol(*sym)
    }

    for _, pkgMap := range st.index {
        for name, syms := range pkgMap {
            sort.Slice(syms, func(i, j int) bool { return syms[i].NodeID < syms[j].NodeID })
            pkgMap[name] = syms
        }
    }

    return st, nil
}

func parseNodeID(id, typ string) *Symbol {
    parts := strings.SplitN(id, ".", 3)
    if len(parts) < 2 {
        return nil
    }

    sym := &Symbol{
        Package: parts[0],
        NodeID:  id,
    }

    switch typ {
    case "func":
        if len(parts) == 3 {
            sym.Kind = KindMethod
            sym.Receiver = parts[1]
            sym.Name = parts[2]
        } else {
            sym.Kind = KindFunc
            sym.Name = parts[1]
        }
    case "struct", "interface":
        sym.Kind = KindType
        sym.Name = parts[1]
    default:
        sym.Kind = KindUnknown
        sym.Name = parts[1]
    }
    return sym
}

func (st *SymbolTable) addSymbol(sym Symbol) {
    if st.index[sym.Package] == nil {
        st.index[sym.Package] = make(map[string][]Symbol)
    }
    st.index[sym.Package][sym.Name] = append(st.index[sym.Package][sym.Name], sym)
}

func (st *SymbolTable) Resolve(pkg, identifier string) string {
    pkgMap, ok := st.index[pkg]
    if !ok {
        return ""
    }

    syms, ok := pkgMap[identifier]
    if !ok || len(syms) == 0 {
        return ""
    }

    for _, s := range syms {
        if s.Kind == KindFunc || s.Kind == KindType {
            return s.NodeID
        }
    }

    for _, s := range syms {
        if s.Kind == KindMethod {
            return s.NodeID
        }
    }

    return ""
}

func (st *SymbolTable) ResolveCalls(db *storage.DB) (int, error) {
    rows, err := db.Query(`
        SELECT rowid, source, target FROM edges
        WHERE relation = 'CALLS' AND target LIKE 'unresolved:%'
        ORDER BY source, target
    `)
    if err != nil {
        return 0, fmt.Errorf("ResolveCalls: %w", err)
    }
    defer rows.Close()

    type replacement struct {
        rowid      int
        source     string
        resolvedID string
    }
    var replacements []replacement

    for rows.Next() {
        var rowid int
        var s, t string
        if err := rows.Scan(&rowid, &s, &t); err != nil {
            return 0, fmt.Errorf("ResolveCalls: scan: %w", err)
        }
        identifier := strings.TrimPrefix(t, "unresolved:")
        pkg := strings.SplitN(s, ".", 2)[0]
        resolvedID := st.Resolve(pkg, identifier)
        if resolvedID != "" {
            replacements = append(replacements, replacement{rowid: rowid, source: s, resolvedID: resolvedID})
        }
    }
    rows.Close()

    if len(replacements) == 0 {
        return 0, nil
    }

    err = db.WithTransaction(func(tx *sql.Tx) error {
        for _, r := range replacements {
            if _, err := tx.Exec(`DELETE FROM edges WHERE rowid = ?`, r.rowid); err != nil {
                return err
            }
            if _, err := tx.Exec(
                `INSERT OR IGNORE INTO edges (source, target, relation) VALUES (?, ?, 'CALLS')`,
                r.source, r.resolvedID,
            ); err != nil {
                return err
            }
        }
        return nil
    })
    if err != nil {
        return 0, err
    }
    return len(replacements), nil
}
