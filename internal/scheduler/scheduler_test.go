package scheduler

import (
	"testing"
	"time"
)

func TestFormatCapturesSummary(t *testing.T) {
	tests := []struct {
		name     string
		captures []CaptureEntry
		wantLen  int // minimum expected length
	}{
		{
			name:     "empty captures",
			captures: []CaptureEntry{},
			wantLen:  0,
		},
		{
			name: "single capture",
			captures: []CaptureEntry{
				{
					Text:      "Test capture",
					Category:  "Ideas",
					Timestamp: time.Now(),
				},
			},
			wantLen: 10, // Should have some content
		},
		{
			name: "multiple captures",
			captures: []CaptureEntry{
				{
					Text:      "First capture",
					Category:  "Ideas",
					Timestamp: time.Now(),
				},
				{
					Text:      "Second capture",
					Category:  "Projects",
					Timestamp: time.Now().Add(-time.Hour),
				},
			},
			wantLen: 20, // Should have more content
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCapturesSummary(tt.captures)
			if len(result) < tt.wantLen {
				t.Errorf("FormatCapturesSummary() returned string of length %d, want at least %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestCaptureEntry(t *testing.T) {
	entry := CaptureEntry{
		Text:      "Test text",
		Category:  "Ideas",
		Timestamp: time.Now(),
	}

	if entry.Text != "Test text" {
		t.Errorf("CaptureEntry.Text = %q, want %q", entry.Text, "Test text")
	}
	if entry.Category != "Ideas" {
		t.Errorf("CaptureEntry.Category = %q, want %q", entry.Category, "Ideas")
	}
}
