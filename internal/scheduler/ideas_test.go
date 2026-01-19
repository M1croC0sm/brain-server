package scheduler

import (
	"testing"
)

func TestSlugifyTitle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Simple Title", "simple-title"},
		{"UPPERCASE", "uppercase"},
		{"with_underscores", "with-underscores"},
		{"with spaces and CAPS", "with-spaces-and-caps"},
		{"Special!@#$%Characters", "specialcharacters"},
		{"Multiple---Hyphens", "multiple-hyphens"},
		{"  Leading/Trailing  ", "leadingtrailing"},
		{"", "idea"}, // default for empty
		{"This is a very long title that should be truncated to fit within the limit", "this-is-a-very-long-title-that-should-be-truncate"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugifyTitle(tt.input)
			if got != tt.want {
				t.Errorf("slugifyTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSlugifyTitleLength(t *testing.T) {
	// Very long input
	input := "This is an extremely long title that definitely exceeds the maximum allowed length and should be truncated"
	result := slugifyTitle(input)

	if len(result) > 50 {
		t.Errorf("slugifyTitle() returned string of length %d, want at most 50", len(result))
	}

	// Should not end with hyphen
	if len(result) > 0 && result[len(result)-1] == '-' {
		t.Errorf("slugifyTitle() result ends with hyphen: %q", result)
	}
}

func TestNewIdeaExpander(t *testing.T) {
	// This is a basic test - would need mocked dependencies for full testing
	expander := NewIdeaExpander(nil, nil)
	if expander == nil {
		t.Error("NewIdeaExpander() returned nil")
	}
}
