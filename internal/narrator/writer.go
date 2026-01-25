package narrator

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Writer handles writing narrated content to daily files
type Writer struct {
	dailyPath string
}

// NewWriter creates a writer for the given journal path
func NewWriter(journalPath string) *Writer {
	return &Writer{
		dailyPath: filepath.Join(journalPath, "Daily"),
	}
}

// DailyFrontmatter represents the YAML frontmatter in daily files
type DailyFrontmatter struct {
	Date      string `yaml:"date"`
	Status    string `yaml:"status"`
	UpdatedAt string `yaml:"updated_at"`
}

// AppendToDaily appends narrated text to the daily file for the given date
// Creates the file with frontmatter if it doesn't exist
func (w *Writer) AppendToDaily(date string, narratedText string) error {
	filePath := filepath.Join(w.dailyPath, date+".md")

	// Check if file exists
	exists := fileExists(filePath)

	if !exists {
		// Create new file with frontmatter
		return w.createDailyFile(filePath, date, narratedText)
	}

	// Append to existing file
	return w.appendToDailyFile(filePath, narratedText)
}

// createDailyFile creates a new daily file with frontmatter and initial content
func (w *Writer) createDailyFile(filePath, date, content string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create daily file: %w", err)
	}
	defer f.Close()

	// Write frontmatter
	now := time.Now().Format(time.RFC3339)
	frontmatter := fmt.Sprintf(`---
date: %s
status: open
updated_at: %s
---

`, date, now)

	if _, err := f.WriteString(frontmatter); err != nil {
		return fmt.Errorf("failed to write frontmatter: %w", err)
	}

	// Write content
	if _, err := f.WriteString(content + "\n"); err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}

	return nil
}

// appendToDailyFile appends content to an existing daily file and updates the frontmatter
func (w *Writer) appendToDailyFile(filePath, content string) error {
	// Read existing file
	existingContent, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read existing file: %w", err)
	}

	// Update frontmatter timestamp
	updatedContent := updateFrontmatterTimestamp(string(existingContent))

	// Append new content with separator
	updatedContent = strings.TrimRight(updatedContent, "\n") + "\n\n---\n\n" + content + "\n"

	// Write back atomically
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, []byte(updatedContent), 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// CloseDay marks a daily file as "closed" (called by nightly job)
func (w *Writer) CloseDay(date string) error {
	filePath := filepath.Join(w.dailyPath, date+".md")

	if !fileExists(filePath) {
		return nil // Nothing to close
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read daily file: %w", err)
	}

	// Update status to closed
	updatedContent := updateFrontmatterField(string(content), "status", "closed")
	updatedContent = updateFrontmatterTimestamp(updatedContent)

	// Write back atomically
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, []byte(updatedContent), 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// updateFrontmatterTimestamp updates the updated_at field in frontmatter
func updateFrontmatterTimestamp(content string) string {
	return updateFrontmatterField(content, "updated_at", time.Now().Format(time.RFC3339))
}

// updateFrontmatterField updates a specific field in YAML frontmatter
func updateFrontmatterField(content, field, value string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inFrontmatter := false
	fieldFound := false

	for i, line := range lines {
		if i == 0 && line == "---" {
			inFrontmatter = true
			result = append(result, line)
			continue
		}

		if inFrontmatter && line == "---" {
			// End of frontmatter - add field if not found
			if !fieldFound {
				result = append(result, fmt.Sprintf("%s: %s", field, value))
			}
			inFrontmatter = false
			result = append(result, line)
			continue
		}

		if inFrontmatter && strings.HasPrefix(line, field+":") {
			result = append(result, fmt.Sprintf("%s: %s", field, value))
			fieldFound = true
			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// GetDailyStatus returns the status of a daily file
func (w *Writer) GetDailyStatus(date string) (string, error) {
	filePath := filepath.Join(w.dailyPath, date+".md")

	if !fileExists(filePath) {
		return "", nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inFrontmatter := false

	for scanner.Scan() {
		line := scanner.Text()

		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			break // End of frontmatter
		}

		if inFrontmatter && strings.HasPrefix(line, "status:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "status:")), nil
		}
	}

	return "open", nil // Default status
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
