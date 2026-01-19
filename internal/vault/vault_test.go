package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteNote(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "vault-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	v := NewVault(tmpDir)

	note := Note{
		ID:         "cap_test123",
		Created:    time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC),
		Category:   "Ideas",
		Confidence: 0.85,
		Actor:      "wolf",
		DeviceID:   "phone_123",
		Tags:       []string{"creative", "tech"},
		Title:      "My Great Idea",
		Content:    "This is the content of my idea.",
	}

	relPath, err := v.WriteNote(note)
	if err != nil {
		t.Fatalf("writing note: %v", err)
	}

	// Verify file was created
	fullPath := filepath.Join(tmpDir, relPath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("reading note: %v", err)
	}

	// Check content
	str := string(content)
	if !strings.Contains(str, "id: cap_test123") {
		t.Error("missing id in frontmatter")
	}
	if !strings.Contains(str, "category: ideas") {
		t.Error("missing category in frontmatter")
	}
	if !strings.Contains(str, "confidence: 0.85") {
		t.Error("missing confidence in frontmatter")
	}
	if !strings.Contains(str, "This is the content of my idea.") {
		t.Error("missing content")
	}

	// Check filename
	expectedPath := "Ideas/2024-01-15-my-great-idea.md"
	if relPath != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, relPath)
	}
}

func TestWriteTransaction(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vault-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	v := NewVault(tmpDir)

	txn := NewTransaction(
		"txn_test123",
		"wolf",
		"phone_123",
		"spent 46 quid at tesco",
		45.99,
		"GBP",
		"Tesco",
		"groceries",
		"weekly shop",
		0.92,
	)

	relPath, err := v.WriteTransaction(txn)
	if err != nil {
		t.Fatalf("writing transaction: %v", err)
	}

	// Verify file was created
	fullPath := filepath.Join(tmpDir, relPath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("reading ledger: %v", err)
	}

	str := string(content)
	if !strings.Contains(str, `"id":"txn_test123"`) {
		t.Error("missing id in transaction")
	}
	if !strings.Contains(str, `"amount":45.99`) {
		t.Error("missing amount in transaction")
	}
	if !strings.Contains(str, `"merchant":"Tesco"`) {
		t.Error("missing merchant in transaction")
	}

	// Check path
	expectedPath := "Financial/Ledger/transactions_wolf.jsonl"
	if relPath != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, relPath)
	}
}

func TestLogCapture(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vault-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	v := NewVault(tmpDir)

	entry := NewCaptureLog(
		"cap_test123",
		"wolf",
		"note",
		"my idea about something",
		"Ideas",
		"filed",
		"phone_123",
		0.85,
	)

	if err := v.LogCapture(entry); err != nil {
		t.Fatalf("logging capture: %v", err)
	}

	// Verify file was created
	fullPath := filepath.Join(tmpDir, "Log", "captures.jsonl")
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("reading log: %v", err)
	}

	str := string(content)
	if !strings.Contains(str, `"id":"cap_test123"`) {
		t.Error("missing id in log")
	}
	if !strings.Contains(str, `"routed_to":"Ideas"`) {
		t.Error("missing routed_to in log")
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"My Great Idea", "my-great-idea"},
		{"Test_with_underscores", "test-with-underscores"},
		{"Special!@#Characters", "specialcharacters"},
		{"Multiple   Spaces", "multiple-spaces"},
		{"", "note"},
		{"---", "note"},
	}

	for _, tc := range tests {
		result := slugify(tc.input)
		if result != tc.expected {
			t.Errorf("slugify(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}
