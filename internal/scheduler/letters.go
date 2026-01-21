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

// Daily letter prompt - system provides all context, LLM articulates
const dailyLetterPrompt = `You are writing a brief daily letter. The system has already analyzed today's activity.

CONTEXT (pre-computed by system):
- Date: %s
- Capture count: %d
- Activity mix: %s
- Top focus areas: %s
- Temporal shape: %s
%s%s

CONSTRAINTS:
- Write 2-3 SHORT sentences maximum
- Start directly, no greeting
- End directly, no signoff
- Warm but not saccharine
- NEVER mention: money, spending, budgets, costs, prices, purchases, $, dollars
- NEVER use: "journey", "growth mindset", "self-care", "boundaries", "space for"
- Do not invent details not provided above
- If action is provided, include it naturally; if not, observation only

Write the letter now:`

// Weekly letter prompt - system provides themes and countermove
const weeklyLetterPrompt = `You are writing a weekly reflection. The system has analyzed the week.

CONTEXT (pre-computed by system):
- Week: %s
- Capture count: %d
- Activity mix: %s
- Top themes: %s
- Projects active: %s
%s
COUNTERMOVE TO INCLUDE: %s

CONSTRAINTS:
- Write ONE paragraph only (3-4 sentences)
- Include the countermove naturally, not as a command
- Honest, not falsely positive
- NEVER mention: money, spending, budgets, costs, prices, purchases, $, dollars
- NEVER use: "journey", "growth mindset", "self-care", "boundaries", "space for"
- Do not invent details not provided

Write the letter now:`

// Silence letter - used when no theme selected
const silenceLetter = "Nothing pressing today. Carry on."

// LetterGenerator generates daily and weekly letters using signal profiles
type LetterGenerator struct {
	llm      *llm.Client
	database *db.DB
}

// NewLetterGenerator creates a new letter generator
func NewLetterGenerator(client *llm.Client, database *db.DB) *LetterGenerator {
	return &LetterGenerator{llm: client, database: database}
}

// GenerateDailyLetter generates a daily letter using the signal layer
func (g *LetterGenerator) GenerateDailyLetter(ctx context.Context, actor string, date time.Time) (string, error) {
	// 1. Build profile from window evidence
	profile, err := signals.BuildDayProfile(g.database, actor, date)
	if err != nil {
		return "", fmt.Errorf("building day profile: %w", err)
	}

	// 2. Check eligibility
	if !signals.IsDailyEligible(profile) {
		return silenceLetter, nil
	}

	// 3. Apply theme and action selection
	signals.ApplyThemeSelection(profile)

	// 4. Check if silence is appropriate (no theme selected)
	if profile.SelectedTheme == nil && profile.BestNextAction == nil {
		return silenceLetter, nil
	}

	// 5. Format context for LLM
	prompt := formatDailyPrompt(profile)

	// 6. Generate letter
	response, err := g.llm.GenerateText(ctx, prompt, true)
	if err != nil {
		return "", fmt.Errorf("generating daily letter: %w", err)
	}

	return response, nil
}

// GenerateWeeklyLetter generates a weekly letter using the signal layer
func (g *LetterGenerator) GenerateWeeklyLetter(ctx context.Context, actor string, weekStart time.Time) (string, error) {
	// 1. Build profile from window evidence
	profile, err := signals.BuildWeekProfile(g.database, actor, weekStart)
	if err != nil {
		return "", fmt.Errorf("building week profile: %w", err)
	}

	// 2. Check eligibility
	if !signals.IsWeeklyEligible(profile) {
		return "Quiet week. Sometimes that's exactly what's needed.", nil
	}

	// 3. Apply theme selection
	signals.ApplyWeeklyThemeSelection(profile)

	// 4. Select countermove
	countermove := signals.SelectWeeklyCountermove(profile)

	// 5. Format context for LLM
	prompt := formatWeeklyPrompt(profile, countermove)

	// 6. Generate letter
	response, err := g.llm.GenerateText(ctx, prompt, true)
	if err != nil {
		return "", fmt.Errorf("generating weekly letter: %w", err)
	}

	return response, nil
}

// formatDailyPrompt builds the prompt from a day profile
func formatDailyPrompt(profile *DayProfile) string {
	// Activity mix
	activityMix := signals.GetCategoryMixLabel(profile.CountsByCategory)

	// Top terms
	var topTerms []string
	for _, tc := range profile.TopTermsInWindow {
		topTerms = append(topTerms, tc.Term)
	}
	topTermsStr := "none detected"
	if len(topTerms) > 0 {
		topTermsStr = strings.Join(topTerms, ", ")
	}

	// Theme line (if selected)
	themeLine := ""
	if profile.SelectedTheme != nil {
		themeLine = fmt.Sprintf("- Detected theme: %s (evidence: %d)\n",
			profile.SelectedTheme.Name, profile.SelectedTheme.Evidence)
	}

	// Action line (if selected)
	actionLine := ""
	if profile.BestNextAction != nil {
		actionLine = fmt.Sprintf("- Suggested action: %s\n", profile.BestNextAction.Text)
	}

	return fmt.Sprintf(dailyLetterPrompt,
		profile.Date,
		profile.CaptureCount,
		activityMix,
		topTermsStr,
		profile.TemporalShape,
		themeLine,
		actionLine,
	)
}

// formatWeeklyPrompt builds the prompt from a week profile
func formatWeeklyPrompt(profile *WeekProfile, countermove string) string {
	// Activity mix
	activityMix := signals.GetCategoryMixLabel(profile.CountsByCategory)

	// Top themes
	var themes []string
	for _, tc := range profile.ThemeCandidates {
		if len(themes) >= 3 {
			break
		}
		themes = append(themes, tc.Name)
	}
	themesStr := "no clear themes"
	if len(themes) > 0 {
		themesStr = strings.Join(themes, ", ")
	}

	// Projects
	var projects []string
	for _, pa := range profile.ProjectActivity {
		if len(projects) >= 3 {
			break
		}
		projects = append(projects, fmt.Sprintf("%s (%d)", pa.Name, pa.MentionCount))
	}
	projectsStr := "none"
	if len(projects) > 0 {
		projectsStr = strings.Join(projects, ", ")
	}

	// Selected theme line
	themeLine := ""
	if profile.SelectedTheme != nil {
		themeLine = fmt.Sprintf("- Primary theme: %s\n", profile.SelectedTheme.Name)
	}

	return fmt.Sprintf(weeklyLetterPrompt,
		profile.WeekID,
		profile.CaptureCount,
		activityMix,
		themesStr,
		projectsStr,
		themeLine,
		countermove,
	)
}

// DayProfile alias for use in this package
type DayProfile = signals.DayProfile

// WeekProfile alias for use in this package
type WeekProfile = signals.WeekProfile

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

// CaptureEntry represents a capture for summarization (legacy)
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
