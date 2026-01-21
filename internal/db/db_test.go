package db

import (
	"os"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) (*DB, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "brain-db-test-*.db")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	tmpFile.Close()

	db, err := Open(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("opening database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
	}

	return db, cleanup
}

func TestLogCapture(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	err := db.LogCapture("cap_123", "wolf", "note", "test content", "Ideas", "filed", 0.85)
	if err != nil {
		t.Fatalf("logging capture: %v", err)
	}

	// Verify it was logged
	captures, err := db.GetRecentCaptures("wolf", time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("getting captures: %v", err)
	}

	if len(captures) != 1 {
		t.Errorf("expected 1 capture, got %d", len(captures))
	}

	if captures[0].CaptureID != "cap_123" {
		t.Errorf("expected capture_id cap_123, got %s", captures[0].CaptureID)
	}
}

func TestPendingClarifications(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Add pending
	err := db.AddPending("cap_456", "wolf", "test text", `["Ideas","Projects"]`, time.Now().Format(time.RFC3339), "test-device")
	if err != nil {
		t.Fatalf("adding pending: %v", err)
	}

	// Get pending
	pending, err := db.GetPending("wolf")
	if err != nil {
		t.Fatalf("getting pending: %v", err)
	}

	if len(pending) != 1 {
		t.Errorf("expected 1 pending, got %d", len(pending))
	}

	// Get by ID
	p, err := db.GetPendingByID("cap_456")
	if err != nil {
		t.Fatalf("getting pending by ID: %v", err)
	}

	if p == nil {
		t.Fatal("expected pending item, got nil")
	}

	if p.RawText != "test text" {
		t.Errorf("expected raw_text 'test text', got %s", p.RawText)
	}

	// Resolve
	resolved, err := db.ResolvePending("cap_456", "Ideas")
	if err != nil {
		t.Fatalf("resolving pending: %v", err)
	}

	if !resolved {
		t.Error("expected resolution to succeed")
	}

	// Should be empty now
	pending, _ = db.GetPending("wolf")
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after resolve, got %d", len(pending))
	}
}

func TestPendingExpiration(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Manually insert an expired pending
	_, err := db.conn.Exec(`
		INSERT INTO pending_clarifications (capture_id, actor, raw_text, choices, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "cap_expired", "wolf", "old text", `["Ideas"]`,
		time.Now().Add(-48*time.Hour).Format(time.RFC3339),
		time.Now().Add(-24*time.Hour).Format(time.RFC3339))
	if err != nil {
		t.Fatalf("inserting expired: %v", err)
	}

	// Expire should mark it
	expired, err := db.ExpirePending()
	if err != nil {
		t.Fatalf("expiring pending: %v", err)
	}

	if len(expired) != 1 {
		t.Errorf("expected 1 expired, got %d", len(expired))
	}
}

func TestLetters(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Save letter
	err := db.SaveLetter("let_123", "daily", "2024-01-15", "/path/to/letter.md")
	if err != nil {
		t.Fatalf("saving letter: %v", err)
	}

	// Get letters
	letters, err := db.GetLetters("", "daily", nil)
	if err != nil {
		t.Fatalf("getting letters: %v", err)
	}

	if len(letters) != 1 {
		t.Errorf("expected 1 letter, got %d", len(letters))
	}

	if letters[0].ForDate != "2024-01-15" {
		t.Errorf("expected for_date 2024-01-15, got %s", letters[0].ForDate)
	}
}

func TestDuplicateCaptureID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// First insert should succeed
	err := db.LogCapture("cap_dup", "wolf", "note", "content", "", "received", 0)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Second insert with same ID should fail
	err = db.LogCapture("cap_dup", "wolf", "note", "content 2", "", "received", 0)
	if err == nil {
		t.Error("expected error on duplicate capture_id")
	}
}
