package memory

// schemaDDL defines the SQLite schema for the semantic symbol index.
// Key design decisions:
//   - symbol_id is deterministic (package + file_path + name) for stable
//     IDs across re-indexing so incremental updates work correctly
//   - call_index tracks caller→callee name pairs resolved at query time
//   - file_index tracks last indexed hash to skip files that haven't changed
const schemaDDL = `
CREATE TABLE IF NOT EXISTS symbol_index (
    symbol_id    TEXT PRIMARY KEY,  -- "pkg:file_path:name" deterministic key
    name         TEXT    NOT NULL,
    kind         TEXT    NOT NULL,  -- func|method|type|var|const
    package      TEXT    NOT NULL,  -- fully qualified package path
    file_path    TEXT    NOT NULL,
    line         INTEGER NOT NULL DEFAULT 0,
    exported     INTEGER NOT NULL DEFAULT 0,  -- boolean
    signature    TEXT    NOT NULL DEFAULT '',
    is_test      INTEGER NOT NULL DEFAULT 0   -- boolean
);

CREATE TABLE IF NOT EXISTS call_index (
    caller_id  TEXT NOT NULL,  -- symbol_id of caller
    callee_id  TEXT NOT NULL,  -- symbol_id of callee
    PRIMARY KEY (caller_id, callee_id)
);

CREATE TABLE IF NOT EXISTS file_index (
    file_path  TEXT PRIMARY KEY,
    file_hash  TEXT NOT NULL DEFAULT '',  -- SHA-256 of file contents
    indexed_at INTEGER NOT NULL DEFAULT 0  -- unix timestamp
);

-- Indexes for query performance
CREATE INDEX IF NOT EXISTS idx_symbol_name     ON symbol_index(name);
CREATE INDEX IF NOT EXISTS idx_symbol_package ON symbol_index(package);
CREATE INDEX IF NOT EXISTS idx_symbol_exported ON symbol_index(exported);
CREATE INDEX IF NOT EXISTS idx_symbol_kind    ON symbol_index(kind);
CREATE INDEX IF NOT EXISTS idx_call_callee    ON call_index(callee_id);
`
