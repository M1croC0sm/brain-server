package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WriteFileAtomic writes content to a file atomically by writing to a temp file first
// then renaming. This prevents partial writes on crash.
// Includes retry logic (up to 3 attempts with backoff).
func WriteFileAtomic(path string, content []byte) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(100*(1<<uint(attempt-1))) * time.Millisecond)
		}
		if err := writeFileAtomicOnce(path, content); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return fmt.Errorf("after 3 attempts: %w", lastErr)
}

func writeFileAtomicOnce(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	// Write to temp file in same directory (for atomic rename)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	// Clean up temp file on any error
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("syncing temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming temp file to %s: %w", path, err)
	}

	success = true
	return nil
}

// AppendLine appends a line to a file, creating it if needed
// Includes retry logic (up to 3 attempts with backoff).
func AppendLine(path string, line []byte) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(100*(1<<uint(attempt-1))) * time.Millisecond)
		}
		if err := appendLineOnce(path, line); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return fmt.Errorf("after 3 attempts: %w", lastErr)
}

func appendLineOnce(path string, line []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening file %s: %w", path, err)
	}
	defer f.Close()

	// Ensure line ends with newline
	if len(line) == 0 || line[len(line)-1] != '\n' {
		line = append(line, '\n')
	}

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("writing to file: %w", err)
	}

	if err := f.Sync(); err != nil {
		return fmt.Errorf("syncing file: %w", err)
	}

	return nil
}

// EnsureDir creates a directory if it doesn't exist
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
