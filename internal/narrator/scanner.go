package narrator

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Scanner finds and parses raw journal files
type Scanner struct {
	rawPath string
}

// NewScanner creates a scanner for the given raw journal path
func NewScanner(journalPath string) *Scanner {
	return &Scanner{
		rawPath: filepath.Join(journalPath, "Raw"),
	}
}

// filenamePattern matches: YYYY-MM-DD_HHMMSS_cap_xxxxx.md
var filenamePattern = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})_(\d{6})_(.+)\.md$`)

// ScanUnprocessed finds all raw files newer than the last processed timestamp
// Returns entries sorted by creation time (oldest first)
func (s *Scanner) ScanUnprocessed(lastProcessedTS time.Time) ([]RawEntry, error) {
	entries, err := os.ReadDir(s.rawPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No raw folder yet, nothing to process
		}
		return nil, fmt.Errorf("failed to read raw directory: %w", err)
	}

	var rawEntries []RawEntry

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		rawEntry, err := s.parseRawFile(entry.Name())
		if err != nil {
			// Log warning but continue with other files
			fmt.Printf("Warning: skipping invalid raw file %s: %v\n", entry.Name(), err)
			continue
		}

		// Filter: only include files newer than last processed
		if rawEntry.Created.After(lastProcessedTS) {
			rawEntries = append(rawEntries, rawEntry)
		}
	}

	// Sort by creation time (oldest first for chronological processing)
	sort.Slice(rawEntries, func(i, j int) bool {
		return rawEntries[i].Created.Before(rawEntries[j].Created)
	})

	return rawEntries, nil
}

// ScanByDate returns all raw entries for a specific date
func (s *Scanner) ScanByDate(date string) ([]RawEntry, error) {
	entries, err := os.ReadDir(s.rawPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read raw directory: %w", err)
	}

	var rawEntries []RawEntry

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		// Quick filename check before full parse
		if !strings.HasPrefix(entry.Name(), date) {
			continue
		}

		rawEntry, err := s.parseRawFile(entry.Name())
		if err != nil {
			continue
		}

		if rawEntry.DayDate == date {
			rawEntries = append(rawEntries, rawEntry)
		}
	}

	// Sort by creation time
	sort.Slice(rawEntries, func(i, j int) bool {
		return rawEntries[i].Created.Before(rawEntries[j].Created)
	})

	return rawEntries, nil
}

// parseRawFile reads and parses a raw journal file
func (s *Scanner) parseRawFile(filename string) (RawEntry, error) {
	// Extract date from filename
	matches := filenamePattern.FindStringSubmatch(filename)
	if matches == nil {
		return RawEntry{}, fmt.Errorf("filename doesn't match expected pattern")
	}

	dayDate := matches[1]
	timeStr := matches[2]
	captureID := matches[3]

	// Parse timestamp from filename
	tsStr := dayDate + "_" + timeStr
	created, err := time.ParseInLocation("2006-01-02_150405", tsStr, time.Local)
	if err != nil {
		return RawEntry{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	// Read file content
	filePath := filepath.Join(s.rawPath, filename)
	content, frontmatter, err := s.readFileWithFrontmatter(filePath)
	if err != nil {
		return RawEntry{}, fmt.Errorf("failed to read file: %w", err)
	}

	entry := RawEntry{
		Filename: filename,
		ID:       captureID,
		Created:  created,
		DayDate:  dayDate,
		Content:  content,
	}

	// Extract frontmatter fields if present
	if id, ok := frontmatter["id"]; ok {
		entry.ID = id
	}
	if actor, ok := frontmatter["actor"]; ok {
		entry.Actor = actor
	}
	if device, ok := frontmatter["device"]; ok {
		entry.Device = device
	}
	if createdStr, ok := frontmatter["created"]; ok {
		if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
			entry.Created = t
		}
	}

	return entry, nil
}

// readFileWithFrontmatter reads a markdown file and separates frontmatter from content
func (s *Scanner) readFileWithFrontmatter(path string) (content string, frontmatter map[string]string, err error) {
	file, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer file.Close()

	frontmatter = make(map[string]string)
	var contentBuilder strings.Builder
	scanner := bufio.NewScanner(file)

	// Check for frontmatter delimiter
	inFrontmatter := false
	frontmatterDone := false
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// First line: check for frontmatter start
		if lineNum == 1 && line == "---" {
			inFrontmatter = true
			continue
		}

		// End of frontmatter
		if inFrontmatter && line == "---" {
			inFrontmatter = false
			frontmatterDone = true
			continue
		}

		// Parse frontmatter
		if inFrontmatter {
			if idx := strings.Index(line, ":"); idx > 0 {
				key := strings.TrimSpace(line[:idx])
				value := strings.TrimSpace(line[idx+1:])
				frontmatter[key] = value
			}
			continue
		}

		// Regular content (skip leading empty lines after frontmatter)
		if frontmatterDone && contentBuilder.Len() == 0 && line == "" {
			continue
		}

		if contentBuilder.Len() > 0 {
			contentBuilder.WriteString("\n")
		}
		contentBuilder.WriteString(line)
	}

	if err := scanner.Err(); err != nil {
		return "", nil, err
	}

	return contentBuilder.String(), frontmatter, nil
}

// GroupByDate groups raw entries by their date
func GroupByDate(entries []RawEntry) map[string][]RawEntry {
	grouped := make(map[string][]RawEntry)
	for _, entry := range entries {
		grouped[entry.DayDate] = append(grouped[entry.DayDate], entry)
	}
	return grouped
}

// GetUniqueDates returns sorted unique dates from entries
func GetUniqueDates(entries []RawEntry) []string {
	dateSet := make(map[string]bool)
	for _, entry := range entries {
		dateSet[entry.DayDate] = true
	}

	dates := make([]string, 0, len(dateSet))
	for date := range dateSet {
		dates = append(dates, date)
	}
	sort.Strings(dates)
	return dates
}
