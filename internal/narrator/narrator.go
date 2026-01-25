package narrator

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"
)

// Narrator orchestrates the journal narration process
type Narrator struct {
	config   NarrationConfig
	state    *StateManager
	scanner  *Scanner
	pipeline *Pipeline
	writer   *Writer
}

// New creates a new Narrator instance
func New(llm LLMClient, config NarrationConfig) (*Narrator, error) {
	journalPath := filepath.Join(config.VaultPath, config.JournalPath)

	// Initialize state manager and ensure directories exist
	stateMgr := NewStateManager(journalPath)
	if err := stateMgr.EnsureDirectories(journalPath); err != nil {
		return nil, fmt.Errorf("failed to ensure directories: %w", err)
	}

	return &Narrator{
		config:   config,
		state:    stateMgr,
		scanner:  NewScanner(journalPath),
		pipeline: NewPipeline(llm, config.Model, config.MaxRetries),
		writer:   NewWriter(journalPath),
	}, nil
}

// UpdateResult contains the result of an update operation
type UpdateResult struct {
	ProcessedCount int
	DaysUpdated    []string
	Errors         []string
}

// Update processes all unprocessed raw entries and updates daily files
// This is the main entry point called by the API endpoint
func (n *Narrator) Update(ctx context.Context) (*UpdateResult, error) {
	result := &UpdateResult{}

	// Load current state
	state, err := n.state.LoadState()
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	// Scan for unprocessed entries
	entries, err := n.scanner.ScanUnprocessed(state.LastProcessedTS)
	if err != nil {
		return nil, fmt.Errorf("failed to scan raw files: %w", err)
	}

	if len(entries) == 0 {
		log.Println("narrator: no new entries to process")
		return result, nil
	}

	log.Printf("narrator: found %d unprocessed entries", len(entries))

	// Group entries by date
	byDate := GroupByDate(entries)
	dates := GetUniqueDates(entries)

	// Process each day's entries
	for _, date := range dates {
		dayEntries := byDate[date]

		// Process in batches if needed
		for i := 0; i < len(dayEntries); i += n.config.BatchSize {
			end := i + n.config.BatchSize
			if end > len(dayEntries) {
				end = len(dayEntries)
			}
			batch := dayEntries[i:end]

			if err := n.processBatch(ctx, date, batch, &state); err != nil {
				errMsg := fmt.Sprintf("failed to process batch for %s: %v", date, err)
				log.Printf("narrator: %s", errMsg)
				result.Errors = append(result.Errors, errMsg)
				continue
			}

			result.ProcessedCount += len(batch)
		}

		result.DaysUpdated = append(result.DaysUpdated, date)
	}

	// Save final state
	if err := n.state.SaveState(state); err != nil {
		return result, fmt.Errorf("failed to save state: %w", err)
	}

	log.Printf("narrator: processed %d entries across %d days", result.ProcessedCount, len(result.DaysUpdated))
	return result, nil
}

// processBatch handles a single batch of entries for a day
func (n *Narrator) processBatch(ctx context.Context, date string, entries []RawEntry, state *JournalState) error {
	// Run the 3-step pipeline
	pipelineResult, err := n.pipeline.Process(ctx, entries)
	if err != nil {
		return fmt.Errorf("pipeline failed: %w", err)
	}

	// Append to daily file
	if err := n.writer.AppendToDaily(date, pipelineResult.NarratedText); err != nil {
		return fmt.Errorf("failed to write to daily file: %w", err)
	}

	// Log the mapping for audit trail
	mapping := NarrationMapping{
		Day:            date,
		GeneratedAt:    time.Now().Format(time.RFC3339),
		RawFiles:       pipelineResult.RawFiles,
		Model:          n.config.Model,
		VerifierPassed: pipelineResult.Verified,
	}
	if err := n.state.AppendMapping(mapping); err != nil {
		log.Printf("narrator: warning - failed to append mapping: %v", err)
	}

	// Update state with last processed entry
	lastEntry := entries[len(entries)-1]
	state.LastProcessedRaw = lastEntry.Filename
	state.LastProcessedTS = lastEntry.Created
	state.CurrentDay = date

	return nil
}

// NightlyClose is called by the nightly job to close the current day
func (n *Narrator) NightlyClose(ctx context.Context) error {
	// Get current date in configured timezone
	now := time.Now().In(n.config.Timezone)
	today := now.Format("2006-01-02")

	log.Printf("narrator: nightly close for %s", today)

	// First, run a final update to catch any remaining entries
	if _, err := n.Update(ctx); err != nil {
		log.Printf("narrator: warning - update before close failed: %v", err)
	}

	// Close the day
	if err := n.writer.CloseDay(today); err != nil {
		return fmt.Errorf("failed to close day: %w", err)
	}

	// Update state
	state, err := n.state.LoadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	state.DayStatus = "closed"
	state.LastNightRunAt = now

	if err := n.state.SaveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	log.Printf("narrator: day %s closed successfully", today)
	return nil
}

// Status returns the current state of the narrator
func (n *Narrator) Status() (JournalState, error) {
	return n.state.LoadState()
}

// GetJournalPath returns the full path to the Journal folder
func (n *Narrator) GetJournalPath() string {
	return filepath.Join(n.config.VaultPath, n.config.JournalPath)
}
