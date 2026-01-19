package vault

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"
)

// CaptureLog represents a logged capture entry
type CaptureLog struct {
	ID         string  `json:"id"`
	TS         string  `json:"ts"`
	Actor      string  `json:"actor"`
	Mode       string  `json:"mode"`
	Raw        string  `json:"raw"`
	RoutedTo   string  `json:"routed_to,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	Status     string  `json:"status"`
	DeviceID   string  `json:"device"`
}

// LogCapture appends a capture entry to the log file
// Uses mutex to prevent race conditions on simultaneous writes
func (v *Vault) LogCapture(entry CaptureLog) error {
	v.logLock.Lock()
	defer v.logLock.Unlock()

	// Path: Vault/Log/captures.jsonl
	relPath := filepath.Join("Log", "captures.jsonl")
	fullPath := filepath.Join(v.basePath, relPath)

	// Marshal to JSON
	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling capture log: %w", err)
	}

	if err := AppendLine(fullPath, line); err != nil {
		return fmt.Errorf("appending capture log: %w", err)
	}

	return nil
}

// NewCaptureLog creates a capture log entry with common fields populated
func NewCaptureLog(id, actor, mode, raw, routedTo, status, deviceID string, confidence float64) CaptureLog {
	return CaptureLog{
		ID:         id,
		TS:         time.Now().UTC().Format(time.RFC3339),
		Actor:      actor,
		Mode:       mode,
		Raw:        raw,
		RoutedTo:   routedTo,
		Confidence: confidence,
		Status:     status,
		DeviceID:   deviceID,
	}
}
