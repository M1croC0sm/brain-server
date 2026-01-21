package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const schema = `
-- Pending clarifications queue
CREATE TABLE IF NOT EXISTS pending_clarifications (
    capture_id TEXT PRIMARY KEY,
    actor TEXT NOT NULL,
    raw_text TEXT NOT NULL,
    choices TEXT NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    resolved_at TEXT,
    destination TEXT,
    original_ts TEXT,
    device_id TEXT
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
    type TEXT NOT NULL,
    for_date TEXT NOT NULL,
    created_at TEXT NOT NULL,
    file_path TEXT NOT NULL
);

-- Transaction history
CREATE TABLE IF NOT EXISTS transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    txn_id TEXT UNIQUE NOT NULL,
    capture_id TEXT,
    actor TEXT NOT NULL,
    amount REAL NOT NULL,
    currency TEXT NOT NULL,
    merchant TEXT NOT NULL,
    label TEXT,
    notes TEXT,
    confidence REAL,
    raw_text TEXT,
    device_id TEXT,
    created_at TEXT NOT NULL
);

-- Scheduler job tracking per actor
CREATE TABLE IF NOT EXISTS scheduler_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    actor TEXT NOT NULL,
    job_type TEXT NOT NULL,
    status TEXT NOT NULL,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    error_message TEXT
);

-- Signal layer for letter generation
-- Tracks long-term tendencies; letters use window evidence primarily
CREATE TABLE IF NOT EXISTS signals (
    key TEXT PRIMARY KEY,           -- e.g. "term:sleep", "project:trip_cave", "cat:Health"
    type TEXT NOT NULL,             -- "term", "project", "category"
    weight REAL NOT NULL DEFAULT 0,
    last_updated TEXT NOT NULL,
    created_at TEXT NOT NULL,
    ever_dominant INTEGER DEFAULT 0 -- floor flag for PROJECTS ONLY
);

CREATE INDEX IF NOT EXISTS idx_pending_actor ON pending_clarifications(actor);
CREATE INDEX IF NOT EXISTS idx_pending_expires ON pending_clarifications(expires_at);
CREATE INDEX IF NOT EXISTS idx_letters_date ON letters(for_date);
CREATE INDEX IF NOT EXISTS idx_transactions_actor ON transactions(actor);
CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(created_at);
CREATE INDEX IF NOT EXISTS idx_scheduler_actor ON scheduler_runs(actor, job_type);
CREATE INDEX IF NOT EXISTS idx_signals_type_weight ON signals(type, weight DESC);
`

type DB struct {
	conn *sql.DB
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}

func (db *DB) migrate() error {
	_, err := db.conn.Exec(schema)
	if err != nil {
		return fmt.Errorf("executing migration: %w", err)
	}
	return nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

// LogCapture logs a capture to the database
func (db *DB) LogCapture(captureID, actor, mode, rawText, routedTo, status string, confidence float64) error {
	_, err := db.conn.Exec(`
		INSERT INTO capture_log (capture_id, actor, mode, raw_text, routed_to, confidence, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, captureID, actor, mode, rawText, routedTo, confidence, status, time.Now().UTC().Format(time.RFC3339))
	return err
}

// AddPending adds a capture to the pending clarifications queue
func (db *DB) AddPending(captureID, actor, rawText, choices, originalTS, deviceID string) error {
	now := time.Now().UTC()
	expires := now.Add(24 * time.Hour)
	_, err := db.conn.Exec(`
		INSERT INTO pending_clarifications (capture_id, actor, raw_text, choices, created_at, expires_at, original_ts, device_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, captureID, actor, rawText, choices, now.Format(time.RFC3339), expires.Format(time.RFC3339), originalTS, deviceID)
	return err
}

// GetPending returns all pending clarifications for an actor
func (db *DB) GetPending(actor string) ([]PendingClarification, error) {
	rows, err := db.conn.Query(`
		SELECT capture_id, raw_text, choices, expires_at
		FROM pending_clarifications
		WHERE actor = ? AND resolved_at IS NULL AND expires_at > ?
		ORDER BY created_at ASC
	`, actor, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pending []PendingClarification
	for rows.Next() {
		var p PendingClarification
		var expiresStr string
		if err := rows.Scan(&p.CaptureID, &p.RawText, &p.Choices, &expiresStr); err != nil {
			return nil, err
		}
		p.ExpiresAt, _ = time.Parse(time.RFC3339, expiresStr)
		pending = append(pending, p)
	}
	return pending, rows.Err()
}

// ResolvePending marks a pending clarification as resolved
func (db *DB) ResolvePending(captureID, destination string) (bool, error) {
	result, err := db.conn.Exec(`
		UPDATE pending_clarifications
		SET resolved_at = ?, destination = ?
		WHERE capture_id = ? AND resolved_at IS NULL AND expires_at > ?
	`, time.Now().UTC().Format(time.RFC3339), destination, captureID, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

// GetPendingByID returns a single pending clarification
func (db *DB) GetPendingByID(captureID string) (*PendingClarification, error) {
	var p PendingClarification
	var expiresStr string
	var originalTSStr, deviceID sql.NullString
	err := db.conn.QueryRow(`
		SELECT capture_id, actor, raw_text, choices, expires_at, original_ts, device_id
		FROM pending_clarifications
		WHERE capture_id = ? AND resolved_at IS NULL
	`, captureID).Scan(&p.CaptureID, &p.Actor, &p.RawText, &p.Choices, &expiresStr, &originalTSStr, &deviceID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.ExpiresAt, _ = time.Parse(time.RFC3339, expiresStr)
	if originalTSStr.Valid {
		p.OriginalTS, _ = time.Parse(time.RFC3339, originalTSStr.String)
	}
	if deviceID.Valid {
		p.DeviceID = deviceID.String
	}
	return &p, nil
}

// ExpiredCapture holds info about a capture that expired
type ExpiredCapture struct {
	CaptureID string
	Actor     string
	RawText   string
}

// ExpirePending marks expired clarifications and returns info about them
func (db *DB) ExpirePending() ([]ExpiredCapture, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	// First get the captures that will be expired
	rows, err := db.conn.Query(`
		SELECT capture_id, actor, raw_text
		FROM pending_clarifications
		WHERE resolved_at IS NULL AND expires_at <= ?
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expired []ExpiredCapture
	for rows.Next() {
		var e ExpiredCapture
		if err := rows.Scan(&e.CaptureID, &e.Actor, &e.RawText); err != nil {
			return nil, err
		}
		expired = append(expired, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Now mark them as expired
	_, err = db.conn.Exec(`
		UPDATE pending_clarifications
		SET resolved_at = ?, destination = 'expired'
		WHERE resolved_at IS NULL AND expires_at <= ?
	`, now, now)
	if err != nil {
		return nil, err
	}

	return expired, nil
}

// SaveLetter records a generated letter
func (db *DB) SaveLetter(letterID, letterType, forDate, filePath string) error {
	_, err := db.conn.Exec(`
		INSERT INTO letters (letter_id, type, for_date, created_at, file_path)
		VALUES (?, ?, ?, ?, ?)
	`, letterID, letterType, forDate, time.Now().UTC().Format(time.RFC3339), filePath)
	return err
}

// GetLetters returns letters optionally filtered by actor, type and date
func (db *DB) GetLetters(actor, letterType string, since *time.Time) ([]LetterRecord, error) {
	query := `SELECT letter_id, type, for_date, created_at, file_path FROM letters WHERE 1=1`
	var args []interface{}

	if actor != "" {
		// Filter by actor - letter_id contains actor name
		query += ` AND letter_id LIKE ?`
		args = append(args, "%_"+actor+"_%")
	}
	if letterType != "" && letterType != "all" {
		query += ` AND type = ?`
		args = append(args, letterType)
	}
	if since != nil {
		query += ` AND created_at >= ?`
		args = append(args, since.Format(time.RFC3339))
	}
	query += ` ORDER BY created_at DESC LIMIT 50`

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var letters []LetterRecord
	for rows.Next() {
		var l LetterRecord
		if err := rows.Scan(&l.LetterID, &l.Type, &l.ForDate, &l.CreatedAt, &l.FilePath); err != nil {
			return nil, err
		}
		letters = append(letters, l)
	}
	return letters, rows.Err()
}

type PendingClarification struct {
	CaptureID  string
	Actor      string
	RawText    string
	Choices    string // JSON array
	ExpiresAt  time.Time
	OriginalTS time.Time
	DeviceID   string
}

type LetterRecord struct {
	LetterID  string
	Type      string
	ForDate   string
	CreatedAt string
	FilePath  string
}

// CaptureRecord represents a capture from the log
type CaptureRecord struct {
	CaptureID  string
	Actor      string
	Mode       string
	RawText    string
	RoutedTo   string
	Confidence float64
	Status     string
	CreatedAt  time.Time
}

// GetRecentCaptures returns captures for an actor since a given time
// Includes all captures regardless of status for letter generation
func (db *DB) GetRecentCaptures(actor string, since time.Time) ([]CaptureRecord, error) {
	rows, err := db.conn.Query(`
		SELECT capture_id, actor, mode, raw_text, routed_to, confidence, status, created_at
		FROM capture_log
		WHERE actor = ? AND created_at >= ?
		ORDER BY created_at DESC
		LIMIT 100
	`, actor, since.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var captures []CaptureRecord
	for rows.Next() {
		var c CaptureRecord
		var createdStr string
		var routedTo sql.NullString
		if err := rows.Scan(&c.CaptureID, &c.Actor, &c.Mode, &c.RawText, &routedTo, &c.Confidence, &c.Status, &createdStr); err != nil {
			return nil, err
		}
		c.RoutedTo = routedTo.String
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
		captures = append(captures, c)
	}
	return captures, rows.Err()
}

// TransactionRecord represents a transaction from the DB
type TransactionRecord struct {
	TxnID      string
	CaptureID  string
	Actor      string
	Amount     float64
	Currency   string
	Merchant   string
	Label      string
	Notes      string
	Confidence float64
	RawText    string
	DeviceID   string
	CreatedAt  time.Time
}

// LogTransaction logs a transaction to the database
func (db *DB) LogTransaction(txnID, captureID, actor string, amount float64, currency, merchant, label, notes string, confidence float64, rawText, deviceID string) error {
	_, err := db.conn.Exec(`
		INSERT INTO transactions (txn_id, capture_id, actor, amount, currency, merchant, label, notes, confidence, raw_text, device_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, txnID, captureID, actor, amount, currency, merchant, label, notes, confidence, rawText, deviceID, time.Now().UTC().Format(time.RFC3339))
	return err
}

// GetTransactions returns transactions for an actor
func (db *DB) GetTransactions(actor string, since *time.Time, limit int) ([]TransactionRecord, error) {
	query := `SELECT txn_id, capture_id, actor, amount, currency, merchant, label, notes, confidence, raw_text, device_id, created_at
		FROM transactions WHERE actor = ?`
	args := []interface{}{actor}

	if since != nil {
		query += ` AND created_at >= ?`
		args = append(args, since.Format(time.RFC3339))
	}
	query += ` ORDER BY created_at DESC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []TransactionRecord
	for rows.Next() {
		var t TransactionRecord
		var createdStr string
		var captureID, label, notes, rawText, deviceID sql.NullString
		if err := rows.Scan(&t.TxnID, &captureID, &t.Actor, &t.Amount, &t.Currency, &t.Merchant, &label, &notes, &t.Confidence, &rawText, &deviceID, &createdStr); err != nil {
			return nil, err
		}
		t.CaptureID = captureID.String
		t.Label = label.String
		t.Notes = notes.String
		t.RawText = rawText.String
		t.DeviceID = deviceID.String
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
		transactions = append(transactions, t)
	}
	return transactions, rows.Err()
}

// SchedulerRun tracks a scheduler job execution
type SchedulerRun struct {
	ID           int64
	Actor        string
	JobType      string
	Status       string
	StartedAt    time.Time
	CompletedAt  *time.Time
	ErrorMessage string
}

// StartSchedulerRun records the start of a scheduler job
func (db *DB) StartSchedulerRun(actor, jobType string) (int64, error) {
	result, err := db.conn.Exec(`
		INSERT INTO scheduler_runs (actor, job_type, status, started_at)
		VALUES (?, ?, 'running', ?)
	`, actor, jobType, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// CompleteSchedulerRun marks a scheduler job as completed
func (db *DB) CompleteSchedulerRun(runID int64, errMsg string) error {
	status := "completed"
	if errMsg != "" {
		status = "failed"
	}
	_, err := db.conn.Exec(`
		UPDATE scheduler_runs
		SET status = ?, completed_at = ?, error_message = ?
		WHERE id = ?
	`, status, time.Now().UTC().Format(time.RFC3339), errMsg, runID)
	return err
}

// GetLastSchedulerRun returns the last run for an actor and job type
func (db *DB) GetLastSchedulerRun(actor, jobType string) (*SchedulerRun, error) {
	var run SchedulerRun
	var startedStr string
	var completedStr, errMsg sql.NullString
	err := db.conn.QueryRow(`
		SELECT id, actor, job_type, status, started_at, completed_at, error_message
		FROM scheduler_runs
		WHERE actor = ? AND job_type = ?
		ORDER BY started_at DESC
		LIMIT 1
	`, actor, jobType).Scan(&run.ID, &run.Actor, &run.JobType, &run.Status, &startedStr, &completedStr, &errMsg)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	run.StartedAt, _ = time.Parse(time.RFC3339, startedStr)
	if completedStr.Valid {
		t, _ := time.Parse(time.RFC3339, completedStr.String)
		run.CompletedAt = &t
	}
	if errMsg.Valid {
		run.ErrorMessage = errMsg.String
	}
	return &run, nil
}

// Signal represents a weighted signal for letter generation
type Signal struct {
	Key          string
	Type         string // "term", "project", "category"
	Weight       float64
	LastUpdated  time.Time
	CreatedAt    time.Time
	EverDominant bool
}

// GetSignal returns a signal by key
func (db *DB) GetSignal(key string) (*Signal, error) {
	var s Signal
	var lastUpdatedStr, createdAtStr string
	var everDominant int
	err := db.conn.QueryRow(`
		SELECT key, type, weight, last_updated, created_at, ever_dominant
		FROM signals WHERE key = ?
	`, key).Scan(&s.Key, &s.Type, &s.Weight, &lastUpdatedStr, &createdAtStr, &everDominant)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.LastUpdated, _ = time.Parse(time.RFC3339, lastUpdatedStr)
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	s.EverDominant = everDominant == 1
	return &s, nil
}

// UpsertSignal updates or inserts a signal with lazy decay then boost
// The caller is responsible for computing the decayed weight before boosting
func (db *DB) UpsertSignal(key, signalType string, weight float64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.conn.Exec(`
		INSERT INTO signals (key, type, weight, last_updated, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			weight = ?,
			last_updated = ?
	`, key, signalType, weight, now, now, weight, now)
	return err
}

// GetTopSignals returns top N signals of a given type by weight
func (db *DB) GetTopSignals(signalType string, limit int) ([]Signal, error) {
	rows, err := db.conn.Query(`
		SELECT key, type, weight, last_updated, created_at, ever_dominant
		FROM signals
		WHERE type = ?
		ORDER BY weight DESC
		LIMIT ?
	`, signalType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var signals []Signal
	for rows.Next() {
		var s Signal
		var lastUpdatedStr, createdAtStr string
		var everDominant int
		if err := rows.Scan(&s.Key, &s.Type, &s.Weight, &lastUpdatedStr, &createdAtStr, &everDominant); err != nil {
			return nil, err
		}
		s.LastUpdated, _ = time.Parse(time.RFC3339, lastUpdatedStr)
		s.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		s.EverDominant = everDominant == 1
		signals = append(signals, s)
	}
	return signals, rows.Err()
}

// GetAllSignals returns all signals for decay processing
func (db *DB) GetAllSignals() ([]Signal, error) {
	rows, err := db.conn.Query(`
		SELECT key, type, weight, last_updated, created_at, ever_dominant
		FROM signals
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var signals []Signal
	for rows.Next() {
		var s Signal
		var lastUpdatedStr, createdAtStr string
		var everDominant int
		if err := rows.Scan(&s.Key, &s.Type, &s.Weight, &lastUpdatedStr, &createdAtStr, &everDominant); err != nil {
			return nil, err
		}
		s.LastUpdated, _ = time.Parse(time.RFC3339, lastUpdatedStr)
		s.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		s.EverDominant = everDominant == 1
		signals = append(signals, s)
	}
	return signals, rows.Err()
}

// UpdateSignalWeight updates the weight of a signal (used after decay)
func (db *DB) UpdateSignalWeight(key string, weight float64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.conn.Exec(`
		UPDATE signals SET weight = ?, last_updated = ? WHERE key = ?
	`, weight, now, key)
	return err
}

// MarkDominant sets the ever_dominant flag for a signal (projects only)
func (db *DB) MarkDominant(key string) error {
	_, err := db.conn.Exec(`
		UPDATE signals SET ever_dominant = 1 WHERE key = ?
	`, key)
	return err
}

// DeleteSignal removes a signal (for cleanup of decayed-to-zero signals)
func (db *DB) DeleteSignal(key string) error {
	_, err := db.conn.Exec(`DELETE FROM signals WHERE key = ?`, key)
	return err
}
