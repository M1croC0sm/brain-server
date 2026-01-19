-- Pending clarifications queue
CREATE TABLE IF NOT EXISTS pending_clarifications (
    capture_id TEXT PRIMARY KEY,
    actor TEXT NOT NULL,
    raw_text TEXT NOT NULL,
    choices TEXT NOT NULL,  -- JSON array
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    resolved_at TEXT,
    destination TEXT
);

-- Capture log (backup, for debugging)
CREATE TABLE IF NOT EXISTS capture_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    capture_id TEXT UNIQUE NOT NULL,
    actor TEXT NOT NULL,
    mode TEXT NOT NULL,
    raw_text TEXT NOT NULL,
    routed_to TEXT,
    confidence REAL,
    status TEXT NOT NULL,
    created_at TEXT NOT NULL
);

-- Letter tracking
CREATE TABLE IF NOT EXISTS letters (
    letter_id TEXT PRIMARY KEY,
    type TEXT NOT NULL,  -- daily, weekly
    for_date TEXT NOT NULL,
    created_at TEXT NOT NULL,
    file_path TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_pending_actor ON pending_clarifications(actor);
CREATE INDEX IF NOT EXISTS idx_pending_expires ON pending_clarifications(expires_at);
CREATE INDEX IF NOT EXISTS idx_letters_date ON letters(for_date);
