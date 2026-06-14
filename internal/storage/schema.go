package storage

const schemaDDL = `
CREATE TABLE IF NOT EXISTS nodes (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    file_path TEXT NOT NULL,
    signature_hash TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS edges (
    source TEXT NOT NULL,
    target TEXT NOT NULL,
    relation TEXT NOT NULL,
    PRIMARY KEY (source, target, relation)
);

CREATE TABLE IF NOT EXISTS events (
    sequence_id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id TEXT UNIQUE NOT NULL,
    event_type TEXT NOT NULL,
    payload TEXT,
    exit_code INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS event_nodes (
    event_id TEXT NOT NULL,
    node_id TEXT NOT NULL,
    PRIMARY KEY (event_id, node_id)
);

CREATE TABLE IF NOT EXISTS graph_state (
    node_id TEXT PRIMARY KEY,
    state TEXT DEFAULT 'UNTESTED',
    last_event_id TEXT,
    last_exit_code INTEGER,
    last_stderr_hash TEXT
);
`
