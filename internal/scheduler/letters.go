package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mrwolf/brain-server/internal/db"
	"github.com/mrwolf/brain-server/internal/llm"
	"github.com/mrwolf/brain-server/internal/signals"
)

// Enhanced daily report prompt - includes actual content and 7-day trends
const dailyReportPrompt = `You are generating a brief daily report for a personal life capture system.

%s

YOUR TASK:
Look at the actual captures and trends above. Identify ONE meaningful pattern or direction that the person should be aware of. This could be:
- Something they keep coming back to (recurring themes)
- A shift in focus they may not have noticed  
- An imbalance worth addressing
- Something that went quiet that might need attention

CONSTRAINTS:
- Be specific - reference actual captures/themes you see above
- Be honest - if there's no clear pattern, say so briefly
- NEVER mention: money amounts, spending, budgets, prices, purchases, dollars
- NEVER use: "journey", "growth mindset", "self-care", "boundaries", "embrace", "space for"
- No greeting or signoff
- No generic advice like "take time to reflect"

OUTPUT FORMAT (exactly this structure):
INSIGHT: [One sentence describing the pattern or direction you notice - be specific]
ACTION: [One concrete, specific thing to do today - not vague advice]

Generate the report now:`

// Weekly letter prompt - system provides themes and countermove
const weeklyLetterPrompt = `You are writing a weekly reflection. The system has analyzed the week.

%s

COUNTERMOVE TO CONSIDER: %s

CONSTRAINTS:
- Write ONE paragraph only (3-4 sentences)
- Include the countermove naturally, not as a command
- Honest, not falsely positive
- NEVER mention: money, spending, budgets, costs, prices, purchases, $, dollars
- NEVER use: "journey", "growth mindset", "self-care", "boundaries", "space for"
- Reference specific things from the week above

Write the letter now:`

// Silence messages
const (
	silenceDaily  = "INSIGHT: No clear patterns yet.\nACTION: Capture a few thoughts today and check back tomorrow."
	silenceWeekly = "Quiet week. Sometimes that's exactly what's needed."
)

// LetterGenerator generates daily and weekly letters using trend analysis
type LetterGenerator struct {
	llm      *llm.Client
	database *db.DB
}

// NewLetterGenerator creates a new letter generator
func NewLetterGenerator(client *llm.Client, database *db.DB) *LetterGenerator {
	return &LetterGenerator{llm: client, database: database}
}

// GenerateDailyLetter generates an enhanced daily report using 7-day trend data
func (g *LetterGenerator) GenerateDailyLetter(ctx context.Context, actor string, date time.Time) (string, error) {
	// 1. Build trend data from last 7 days
	trend, err := signals.BuildTrendData(g.database, actor, date)
	if err != nil {
		return "", fmt.Errorf("building trend data: %w", err)
	}

	// 2. Check if there's enough data
	totalCaptures := 0
	for _, day := range trend.Days {
		totalCaptures += day.CaptureCount
	}

	if totalCaptures == 0 {
		return silenceDaily, nil
	}

	if totalCaptures < 3 {
		return "INSIGHT: Light week so far - not enough data for patterns.\nACTION: Keep capturing thoughts and check back in a day or two.", nil
	}

	// 3. Format context for LLM
	trendContext := signals.FormatTrendContext(trend)
	prompt := fmt.Sprintf(dailyReportPrompt, trendContext)

	// 4. Generate report
	response, err := g.llm.GenerateText(ctx, prompt, true)
	if err != nil {
		return "", fmt.Errorf("generating daily report: %w", err)
	}

	// 5. Validate and clean response
	response = cleanReportResponse(response)

	return response, nil
}

// GenerateWeeklyLetter generates a weekly letter using trend data
func (g *LetterGenerator) GenerateWeeklyLetter(ctx context.Context, actor string, weekStart time.Time) (string, error) {
	// 1. Build trend data
	trend, err := signals.BuildTrendData(g.database, actor, weekStart)
	if err != nil {
		return "", fmt.Errorf("building trend data: %w", err)
	}

	// 2. Check eligibility
	totalCaptures := 0
	for _, day := range trend.Days {
		totalCaptures += day.CaptureCount
	}

	if totalCaptures < 5 {
		return silenceWeekly, nil
	}

	// 3. Select countermove based on trends
	countermove := selectCountermove(trend)

	// 4. Format context for LLM
	trendContext := signals.FormatTrendContext(trend)
	prompt := fmt.Sprintf(weeklyLetterPrompt, trendContext, countermove)

	// 5. Generate letter
	response, err := g.llm.GenerateText(ctx, prompt, true)
	if err != nil {
		return "", fmt.Errorf("generating weekly letter: %w", err)
	}

	return strings.TrimSpace(response), nil
}

// selectCountermove picks an appropriate countermove based on trend data
func selectCountermove(trend *signals.TrendData) string {
	// Check for momentum shifts first
	if len(trend.MomentumShifts) > 0 {
		return "Something changed mid-week - worth asking why"
	}

	// Check category trends
	for cat, direction := range trend.CategoryTrend {
		if direction == "↓ declining" {
			return fmt.Sprintf("%s dropped off - intentional or oversight?", cat)
		}
		if direction == "↑ increasing" && (cat == "Health" || cat == "Life") {
			return fmt.Sprintf("%s is asking for attention - what does it need?", cat)
		}
	}

	// Check for recurring themes
	if len(trend.RecurringTerms) > 0 {
		return fmt.Sprintf("You keep coming back to '%s' - worth exploring deeper?", trend.RecurringTerms[0])
	}

	// Default based on dominant theme
	switch trend.DominantTheme {
	case "quiet week":
		return "Quiet doesn't mean empty - what's brewing under the surface?"
	default:
		return "What would make next week feel complete?"
	}
}

// cleanReportResponse ensures the response follows the expected format
func cleanReportResponse(response string) string {
	response = strings.TrimSpace(response)

	// Check if it has the expected format
	hasInsight := strings.Contains(strings.ToUpper(response), "INSIGHT:")
	hasAction := strings.Contains(strings.ToUpper(response), "ACTION:")

	if hasInsight && hasAction {
		// Normalize case
		response = strings.Replace(response, "insight:", "INSIGHT:", 1)
		response = strings.Replace(response, "Insight:", "INSIGHT:", 1)
		response = strings.Replace(response, "action:", "ACTION:", 1)
		response = strings.Replace(response, "Action:", "ACTION:", 1)
		return response
	}

	// If LLM didn't follow format, try to parse it
	lines := strings.Split(response, "\n")
	if len(lines) >= 2 {
		// Assume first non-empty line is insight, find action
		var insight, action string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if insight == "" {
				insight = line
			} else {
				action = line
				break
			}
		}
		if insight != "" && action != "" {
			return fmt.Sprintf("INSIGHT: %s\nACTION: %s", insight, action)
		}
	}

	// Last resort: use response as insight, add generic action
	return fmt.Sprintf("INSIGHT: %s\nACTION: Pick one thing from today's captures and take a small step on it.", response)
}

// Legacy support: CaptureEntry for backward compatibility
type CaptureEntry struct {
	Text      string
	Category  string
	Timestamp time.Time
}

// Legacy support: FormatCapturesSummary for backward compatibility
// DEPRECATED: Use GenerateDailyLetter with database instead
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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
