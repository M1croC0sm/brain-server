package narrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StateManager handles loading and saving journal state atomically
type StateManager struct {
	metaPath string
}

// NewStateManager creates a state manager for the given journal path
func NewStateManager(journalPath string) *StateManager {
	return &StateManager{
		metaPath: filepath.Join(journalPath, "_meta"),
	}
}

// EnsureDirectories creates the required directory structure
func (sm *StateManager) EnsureDirectories(journalPath string) error {
	dirs := []string{
		filepath.Join(journalPath, "Raw"),
		filepath.Join(journalPath, "Daily"),
		filepath.Join(journalPath, "_meta"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// LoadState reads the current journal state from disk
// Returns a zero-value state if the file doesn't exist
func (sm *StateManager) LoadState() (JournalState, error) {
	statePath := filepath.Join(sm.metaPath, "journal_state.json")

	data, err := os.ReadFile(statePath)
	if os.IsNotExist(err) {
		// Return empty state for first run
		return JournalState{
			DayStatus: "open",
		}, nil
	}
	if err != nil {
		return JournalState{}, fmt.Errorf("failed to read state file: %w", err)
	}

	var state JournalState
	if err := json.Unmarshal(data, &state); err != nil {
		return JournalState{}, fmt.Errorf("failed to parse state file: %w", err)
	}

	return state, nil
}

// SaveState writes the journal state atomically (write to temp, then rename)
func (sm *StateManager) SaveState(state JournalState) error {
	statePath := filepath.Join(sm.metaPath, "journal_state.json")
	tempPath := statePath + ".tmp"

	// Update the last update timestamp
	state.LastUpdateAt = time.Now()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write to temp file first
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp state file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, statePath); err != nil {
		os.Remove(tempPath) // Clean up temp file on failure
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	return nil
}

// AppendMapping adds a narration mapping entry to the audit log
func (sm *StateManager) AppendMapping(mapping NarrationMapping) error {
	mapPath := filepath.Join(sm.metaPath, "journal_map.jsonl")

	data, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("failed to marshal mapping: %w", err)
	}

	// Open file in append mode, create if doesn't exist
	f, err := os.OpenFile(mapPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open mapping file: %w", err)
	}
	defer f.Close()

	// Write JSON line with newline
	if _, err := f.WriteString(string(data) + "\n"); err != nil {
		return fmt.Errorf("failed to write mapping: %w", err)
	}

	return nil
}

// GetLastProcessedTimestamp returns the timestamp of the last processed raw file
func (sm *StateManager) GetLastProcessedTimestamp(state JournalState) time.Time {
	return state.LastProcessedTS
}
