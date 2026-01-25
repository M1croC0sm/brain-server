package vault

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Note represents a note to be written to the vault
type Note struct {
	ID         string
	Created    time.Time
	Category   string
	Confidence float64
	Actor      string
	DeviceID   string
	Tags       []string
	Title      string
	Content    string
}

// Vault handles all file operations for the vault
type Vault struct {
	basePath   string
	ledgerLock sync.Mutex // Protects ledger JSONL writes from race conditions
	logLock    sync.Mutex // Protects capture log JSONL writes from race conditions
}

// NewVault creates a new Vault instance
func NewVault(basePath string) *Vault {
	return &Vault{basePath: basePath}
}

// BasePath returns the vault base path
func (v *Vault) BasePath() string {
	return v.basePath
}

// WriteNote writes a note to the appropriate category folder
func (v *Vault) WriteNote(note Note) (string, error) {
	// Build filename: 2024-01-15-title-slug.md
	dateStr := note.Created.Format("2006-01-02")
	slug := slugify(note.Title)
	filename := fmt.Sprintf("%s-%s.md", dateStr, slug)

	// Build path: Vault/{Category}/{filename}
	relPath := filepath.Join(note.Category, filename)
	fullPath := filepath.Join(v.basePath, relPath)

	// Build content with YAML frontmatter
	content := v.buildNoteContent(note)

	if err := WriteFileAtomic(fullPath, []byte(content)); err != nil {
		return "", fmt.Errorf("writing note: %w", err)
	}

	return relPath, nil
}

func (v *Vault) buildNoteContent(note Note) string {
	var sb strings.Builder

	// YAML frontmatter
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("id: %s\n", note.ID))
	sb.WriteString(fmt.Sprintf("created: %s\n", note.Created.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("category: %s\n", strings.ToLower(note.Category)))
	sb.WriteString(fmt.Sprintf("confidence: %.2f\n", note.Confidence))
	sb.WriteString(fmt.Sprintf("actor: %s\n", note.Actor))
	sb.WriteString(fmt.Sprintf("device: %s\n", note.DeviceID))

	if len(note.Tags) > 0 {
		sb.WriteString("tags:\n")
		for _, tag := range note.Tags {
			sb.WriteString(fmt.Sprintf("  - %s\n", tag))
		}
	} else {
		sb.WriteString("tags: []\n")
	}

	sb.WriteString("---\n\n")

	// Content
	sb.WriteString(note.Content)
	sb.WriteString("\n")

	return sb.String()
}

// GetNotePath returns the full path for a note
func (v *Vault) GetNotePath(category, filename string) string {
	return filepath.Join(v.basePath, category, filename)
}

// CategoryPath returns the path to a category folder
func (v *Vault) CategoryPath(category string) string {
	return filepath.Join(v.basePath, category)
}

// slugify converts a title to a URL-friendly slug
func slugify(s string) string {
	// Convert to lowercase
	s = strings.ToLower(s)

	// Replace spaces and underscores with hyphens
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	// Remove non-alphanumeric characters except hyphens
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	s = reg.ReplaceAllString(s, "")

	// Replace multiple hyphens with single hyphen
	reg = regexp.MustCompile(`-+`)
	s = reg.ReplaceAllString(s, "-")

	// Trim hyphens from start and end
	s = strings.Trim(s, "-")

	// Limit length
	if len(s) > 50 {
		s = s[:50]
		// Don't end with a hyphen
		s = strings.TrimRight(s, "-")
	}

	// Default if empty
	if s == "" {
		s = "note"
	}

	return s
}

// WriteRawJournalCapture writes a journal capture to the Raw/ folder for narrator processing
func (v *Vault) WriteRawJournalCapture(note Note) (string, error) {
	// Build filename: YYYY-MM-DD_HHMMSS_cap_xxxxx.md
	dateStr := note.Created.Format("2006-01-02")
	timeStr := note.Created.Format("150405")
	filename := fmt.Sprintf("%s_%s_%s.md", dateStr, timeStr, note.ID)

	// Build path: Vault/Journal/Raw/{filename}
	relPath := filepath.Join("Journal", "Raw", filename)
	fullPath := filepath.Join(v.basePath, relPath)

	// Build content with YAML frontmatter (narrator format)
	content := v.buildRawJournalContent(note)

	if err := WriteFileAtomic(fullPath, []byte(content)); err != nil {
		return "", fmt.Errorf("writing raw journal: %w", err)
	}

	return relPath, nil
}

func (v *Vault) buildRawJournalContent(note Note) string {
	var sb strings.Builder

	// YAML frontmatter (narrator format)
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("id: %s\n", note.ID))
	sb.WriteString(fmt.Sprintf("created: %s\n", note.Created.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("actor: %s\n", note.Actor))
	sb.WriteString(fmt.Sprintf("device: %s\n", note.DeviceID))
	sb.WriteString("---\n\n")

	// Content
	sb.WriteString(note.Content)
	sb.WriteString("\n")

	return sb.String()
}
