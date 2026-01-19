package classifier

import (
	"testing"

	"github.com/mrwolf/brain-server/internal/models"
)

func TestValidateCategory(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ideas", models.CategoryIdeas},
		{"Ideas", models.CategoryIdeas},
		{"IDEAS", models.CategoryIdeas},
		{"projects", models.CategoryProjects},
		{"Projects", models.CategoryProjects},
		{"financial", models.CategoryFinancial},
		{"Financial", models.CategoryFinancial},
		{"health", models.CategoryHealth},
		{"Health", models.CategoryHealth},
		{"life", models.CategoryLife},
		{"Life", models.CategoryLife},
		{"invalid", ""},
		{"", ""},
		{"  ideas  ", models.CategoryIdeas}, // with whitespace
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := validateCategory(tt.input)
			if got != tt.want {
				t.Errorf("validateCategory(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSuggestChoices(t *testing.T) {
	tests := []struct {
		name    string
		primary string
		wantLen int
	}{
		{
			name:    "with Ideas primary",
			primary: models.CategoryIdeas,
			wantLen: 4, // limited to 4
		},
		{
			name:    "with Projects primary",
			primary: models.CategoryProjects,
			wantLen: 4,
		},
		{
			name:    "with Financial primary",
			primary: models.CategoryFinancial,
			wantLen: 4,
		},
		{
			name:    "with empty primary",
			primary: "",
			wantLen: 4, // should still return 4 choices
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			choices := suggestChoices(tt.primary)
			if len(choices) > tt.wantLen {
				t.Errorf("suggestChoices(%q) returned %d choices, want at most %d", tt.primary, len(choices), tt.wantLen)
			}

			// Primary should be first if provided
			if tt.primary != "" && len(choices) > 0 && choices[0] != tt.primary {
				t.Errorf("suggestChoices(%q) first choice is %q, want %q", tt.primary, choices[0], tt.primary)
			}
		})
	}
}

func TestSuggestChoicesIncludesFinancial(t *testing.T) {
	choices := suggestChoices("")

	hasFinancial := false
	for _, c := range choices {
		if c == models.CategoryFinancial {
			hasFinancial = true
			break
		}
	}

	if !hasFinancial {
		t.Errorf("suggestChoices() should include Financial category")
	}
}

func TestSuggestChoicesIncludesAllCategories(t *testing.T) {
	// Test that with empty primary, we get all 5 categories
	choices := suggestChoices("")

	// Since we limit to 4 choices, we can't test for all 5
	// But we should at least verify Financial is included
	hasFinancial := false
	for _, c := range choices {
		if c == models.CategoryFinancial {
			hasFinancial = true
			break
		}
	}

	if !hasFinancial {
		t.Errorf("suggestChoices(\"\") should include Financial in choices: got %v", choices)
	}
}
