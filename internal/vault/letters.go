package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Letter represents a daily or weekly letter
type Letter struct {
	ID      string
	Type    string // "daily" or "weekly"
	ForDate string // "2024-01-15" or "2024-W03"
	Actor   string
	Content string
}

// WriteLetter writes a letter to the appropriate folder
func (v *Vault) WriteLetter(letter Letter) (string, error) {
	// Path: Vault/Letters/{Daily|Weekly}/{date}.md
	var subdir string
	var filename string

	switch letter.Type {
	case "daily":
		subdir = "Daily"
		filename = letter.ForDate + ".md"
	case "weekly":
		subdir = "Weekly"
		filename = letter.ForDate + ".md"
	default:
		return "", fmt.Errorf("unknown letter type: %s", letter.Type)
	}

	relPath := filepath.Join("Letters", subdir, filename)
	fullPath := filepath.Join(v.basePath, relPath)

	// Build content
	content := v.buildLetterContent(letter)

	if err := WriteFileAtomic(fullPath, []byte(content)); err != nil {
		return "", fmt.Errorf("writing letter: %w", err)
	}

	return relPath, nil
}

func (v *Vault) buildLetterContent(letter Letter) string {
	return fmt.Sprintf("---\nid: %s\ntype: %s\nfor_date: %s\nactor: %s\ncreated: %s\n---\n\n%s\n",
		letter.ID,
		letter.Type,
		letter.ForDate,
		letter.Actor,
		time.Now().UTC().Format(time.RFC3339),
		letter.Content,
	)
}

// ReadLetter reads a letter file and returns its content
func (v *Vault) ReadLetter(letterType, forDate string) (string, error) {
	var subdir string
	switch letterType {
	case "daily":
		subdir = "Daily"
	case "weekly":
		subdir = "Weekly"
	default:
		return "", fmt.Errorf("unknown letter type: %s", letterType)
	}

	fullPath := filepath.Join(v.basePath, "Letters", subdir, forDate+".md")

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("reading letter: %w", err)
	}

	return string(content), nil
}

// GetLatestDailyLetter returns the most recent daily letter path
func (v *Vault) GetLatestDailyLetter() (string, error) {
	dir := filepath.Join(v.basePath, "Letters", "Daily")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	if len(entries) == 0 {
		return "", nil
	}

	// Files are named by date, so last alphabetically is most recent
	var latest string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".md" {
			latest = e.Name()
		}
	}

	if latest == "" {
		return "", nil
	}

	return filepath.Join(dir, latest), nil
}
