package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/mrwolf/brain-server/internal/llm"
)

const dailyLetterPrompt = `You are writing a brief daily letter for %s.

Recent captures from the last 24 hours:
%s

Write a letter that:
- Contains at most ONE actionable suggestion, or none if nothing is pressing
- Contains at most ONE observation or reflection
- Is warm but not saccharine
- Never mentions spending or money
- Fits in 2-3 short paragraphs
- Starts without greeting, ends without signoff

If there's nothing meaningful to say, just write: "Nothing pressing today. Carry on."`

const weeklyLetterPrompt = `You are writing a weekly reflection for %s.

This week's captures:
%s

Write a short letter (one paragraph) that:
- Identifies one theme from the week
- Suggests one countermove or adjustment
- Never mentions spending or money
- Is honest, not falsely positive`

// LetterGenerator generates daily and weekly letters
type LetterGenerator struct {
	llm *llm.Client
}

// NewLetterGenerator creates a new letter generator
func NewLetterGenerator(client *llm.Client) *LetterGenerator {
	return &LetterGenerator{llm: client}
}

// GenerateDailyLetter generates a daily letter for an actor
func (g *LetterGenerator) GenerateDailyLetter(ctx context.Context, actor string, capturesSummary string) (string, error) {
	prompt := fmt.Sprintf(dailyLetterPrompt, actor, capturesSummary)

	response, err := g.llm.GenerateText(ctx, prompt, true) // Use heavy model
	if err != nil {
		return "", fmt.Errorf("generating daily letter: %w", err)
	}

	return response, nil
}

// GenerateWeeklyLetter generates a weekly letter for an actor
func (g *LetterGenerator) GenerateWeeklyLetter(ctx context.Context, actor string, weekSummary string) (string, error) {
	prompt := fmt.Sprintf(weeklyLetterPrompt, actor, weekSummary)

	response, err := g.llm.GenerateText(ctx, prompt, true) // Use heavy model
	if err != nil {
		return "", fmt.Errorf("generating weekly letter: %w", err)
	}

	return response, nil
}

// FormatCapturesSummary formats captures for the prompt
func FormatCapturesSummary(captures []CaptureEntry) string {
	if len(captures) == 0 {
		return "No captures recorded."
	}

	var summary string
	for _, c := range captures {
		summary += fmt.Sprintf("- [%s] %s: %s\n", c.Category, c.Timestamp.Format("15:04"), truncate(c.Text, 100))
	}
	return summary
}

// CaptureEntry represents a capture for summarization
type CaptureEntry struct {
	Text      string
	Category  string
	Timestamp time.Time
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
