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

// Weekly report prompt - mental landscape focus
const weeklyReportPrompt = `You are generating a weekly mental landscape report. This summarizes how someone's mind was working over the past week based on their captured thoughts.

%s

YOUR TASK:
Analyze the week's mental activity. Focus on:
- What ideas emerged and whether any connect
- Which projects got attention (or didn't)
- Health/Life/Spirituality signals (body and mind indicators)
- Patterns in thinking or focus shifts

VOICE REQUIREMENTS (CRITICAL):
- STRICTLY THIRD PERSON: Write as an observer describing someone else
- Use phrases like: "The mind was occupied with...", "Attention went to...", "There was focus on..."
- NEVER use: "you", "your", "yourself", "I", "we", "our"
- NEVER give advice phrased as commands: "Continue doing X", "Try to Y", "Focus on Z"
- NEXT WEEK section should be an observation/suggestion, not a directive

ACCURACY REQUIREMENTS:
- ONLY reference what's explicitly in the data above
- Quote or closely paraphrase actual captures when citing evidence
- If a pattern isn't clearly supported by the data, don't mention it
- Note absent categories (e.g., "No Life captures this week")

FORBIDDEN:
- Money, spending, budgets, financial matters
- Words: "journey", "growth mindset", "self-care", "boundaries", "embrace"
- Inventing or extrapolating beyond what's captured
- Second-person advice or directives

OUTPUT FORMAT (exactly this structure):
THIS WEEK: [2-3 sentences on what dominated mental activity - third person]

PATTERNS:
- [Pattern 1 with specific evidence from captures]
- [Pattern 2 with specific evidence from captures]
- [Pattern 3 if clearly supported]

SHIFTS: [What changed mid-week, or "No significant shifts detected"]

NEXT WEEK: [One observation about what might warrant attention - phrased as "X could be worth revisiting" not "revisit X"]

Generate the report now:`

// Silence messages
const (
	silenceDaily  = "INSIGHT: No clear patterns yet.\nACTION: Capture a few thoughts today and check back tomorrow."
	silenceWeekly = "THIS WEEK: Quiet week with minimal mental capture activity.\n\nPATTERNS:\n- Insufficient data for pattern detection\n\nSHIFTS: No shifts detected.\n\nNEXT WEEK: Resume capturing thoughts to build a clearer picture."
)

// LetterGenerator generates daily and weekly reports using trend analysis
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
	// 1. Build trend data from last 7 days (all categories for daily)
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
	response = cleanDailyResponse(response)

	return response, nil
}

// GenerateWeeklyLetter generates a weekly mental landscape report
func (g *LetterGenerator) GenerateWeeklyLetter(ctx context.Context, actor string, weekStart time.Time) (string, error) {
	// 1. Build trend data EXCLUDING Financial, Tasks, Journal
	trend, err := signals.BuildWeeklyTrendData(g.database, actor, weekStart)
	if err != nil {
		return "", fmt.Errorf("building weekly trend data: %w", err)
	}

	// 2. Check eligibility
	totalCaptures := 0
	for _, day := range trend.Days {
		totalCaptures += day.CaptureCount
	}

	if totalCaptures < 3 {
		return silenceWeekly, nil
	}

	// 3. Format context for LLM (weekly-specific format)
	trendContext := signals.FormatWeeklyContext(trend)
	prompt := fmt.Sprintf(weeklyReportPrompt, trendContext)

	// 4. Generate report
	response, err := g.llm.GenerateText(ctx, prompt, true)
	if err != nil {
		return "", fmt.Errorf("generating weekly report: %w", err)
	}

	// 5. Clean response
	response = cleanWeeklyResponse(response)

	return response, nil
}

// cleanDailyResponse ensures the daily response follows the expected format
func cleanDailyResponse(response string) string {
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

// cleanWeeklyResponse ensures the weekly response follows the expected format
func cleanWeeklyResponse(response string) string {
	response = strings.TrimSpace(response)

	// Normalize section headers
	response = strings.Replace(response, "this week:", "THIS WEEK:", 1)
	response = strings.Replace(response, "This week:", "THIS WEEK:", 1)
	response = strings.Replace(response, "This Week:", "THIS WEEK:", 1)
	response = strings.Replace(response, "patterns:", "PATTERNS:", 1)
	response = strings.Replace(response, "Patterns:", "PATTERNS:", 1)
	response = strings.Replace(response, "shifts:", "SHIFTS:", 1)
	response = strings.Replace(response, "Shifts:", "SHIFTS:", 1)
	response = strings.Replace(response, "next week:", "NEXT WEEK:", 1)
	response = strings.Replace(response, "Next week:", "NEXT WEEK:", 1)
	response = strings.Replace(response, "Next Week:", "NEXT WEEK:", 1)

	return response
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
