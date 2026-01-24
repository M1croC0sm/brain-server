package signals

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mrwolf/brain-server/internal/db"
)

// Categories to exclude from weekly mental landscape report
var WeeklyExcludeCategories = map[string]bool{
	"Financial": true,
	"Tasks":     true,
	"Journal":   true,
}

// DaySummary holds captures grouped by day with summaries
type DaySummary struct {
	Date               string
	DayOfWeek          string
	CaptureCount       int
	CapturesByCategory map[string][]string // category -> list of truncated texts
	TopTerms           []string
}

// TrendData holds multi-day trend analysis
type TrendData struct {
	Days           []DaySummary      // Last 7 days, most recent first
	CategoryTrend  map[string]string // category -> "↑ increasing", "↓ declining", "→ steady"
	RecurringTerms []string          // Terms appearing 3+ days
	MomentumShifts []string          // Notable changes: "Projects went quiet since Tuesday"
	DominantTheme  string            // Overall theme across the week
}

// BuildTrendData analyzes captures over the past 7 days (all categories)
func BuildTrendData(database *db.DB, actor string, now time.Time) (*TrendData, error) {
	return buildTrendDataWithFilter(database, actor, now, nil)
}

// BuildWeeklyTrendData analyzes captures excluding Financial/Tasks/Journal
func BuildWeeklyTrendData(database *db.DB, actor string, now time.Time) (*TrendData, error) {
	return buildTrendDataWithFilter(database, actor, now, WeeklyExcludeCategories)
}

// buildTrendDataWithFilter analyzes captures with optional category exclusion
func buildTrendDataWithFilter(database *db.DB, actor string, now time.Time, excludeCategories map[string]bool) (*TrendData, error) {
	trend := &TrendData{
		CategoryTrend: make(map[string]string),
	}

	// Get captures for last 7 days
	since := now.Add(-7 * 24 * time.Hour)
	captures, err := database.GetRecentCaptures(actor, since)
	if err != nil {
		return nil, err
	}

	// Group captures by day
	dayMap := make(map[string]*DaySummary)
	termDays := make(map[string]map[string]bool) // term -> set of dates it appeared

	for _, c := range captures {
		category := c.RoutedTo
		if category == "" {
			category = "Uncategorized"
		}

		// Skip excluded categories if filter is set
		if excludeCategories != nil && excludeCategories[category] {
			continue
		}

		dateStr := c.CreatedAt.Format("2006-01-02")

		if _, exists := dayMap[dateStr]; !exists {
			dayMap[dateStr] = &DaySummary{
				Date:               dateStr,
				DayOfWeek:          c.CreatedAt.Weekday().String()[:3],
				CapturesByCategory: make(map[string][]string),
			}
		}

		day := dayMap[dateStr]
		day.CaptureCount++

		// Add truncated text to category
		summary := truncateText(c.RawText, 60)
		day.CapturesByCategory[category] = append(day.CapturesByCategory[category], summary)

		// Track terms across days
		terms := ExtractTerms(c.RawText, 5)
		for _, term := range terms {
			if termDays[term] == nil {
				termDays[term] = make(map[string]bool)
			}
			termDays[term][dateStr] = true
		}
	}

	// Sort days most recent first
	var dates []string
	for d := range dayMap {
		dates = append(dates, d)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	for _, d := range dates {
		trend.Days = append(trend.Days, *dayMap[d])
	}

	// Find recurring terms (3+ days)
	for term, days := range termDays {
		if len(days) >= 3 {
			trend.RecurringTerms = append(trend.RecurringTerms, term)
		}
	}

	// Analyze category trends
	trend.CategoryTrend, trend.MomentumShifts = analyzeCategoryTrends(trend.Days)

	// Determine dominant theme
	trend.DominantTheme = detectDominantTheme(trend)

	return trend, nil
}

// analyzeCategoryTrends compares category activity across days
func analyzeCategoryTrends(days []DaySummary) (map[string]string, []string) {
	trends := make(map[string]string)
	var shifts []string

	if len(days) < 2 {
		return trends, shifts
	}

	// Get all categories
	allCats := make(map[string]bool)
	for _, day := range days {
		for cat := range day.CapturesByCategory {
			allCats[cat] = true
		}
	}

	// Analyze each category
	for cat := range allCats {
		// Get counts for first half vs second half of the week
		var recentCount, olderCount int
		midpoint := len(days) / 2
		if midpoint == 0 {
			midpoint = 1
		}

		for i, day := range days {
			count := len(day.CapturesByCategory[cat])
			if i < midpoint {
				recentCount += count
			} else {
				olderCount += count
			}
		}

		// Determine trend
		if recentCount > olderCount*2 && recentCount >= 3 {
			trends[cat] = "↑ increasing"
		} else if olderCount > recentCount*2 && olderCount >= 3 {
			trends[cat] = "↓ declining"
			// Check for silence - was active, now quiet
			if recentCount == 0 && olderCount >= 2 {
				lastActiveDay := ""
				for i := len(days) - 1; i >= 0; i-- {
					if len(days[i].CapturesByCategory[cat]) > 0 {
						lastActiveDay = days[i].DayOfWeek
						break
					}
				}
				if lastActiveDay != "" {
					shifts = append(shifts, fmt.Sprintf("%s went quiet after %s", cat, lastActiveDay))
				}
			}
		} else {
			trends[cat] = "→ steady"
		}
	}

	return trends, shifts
}

// detectDominantTheme determines the overall theme of the week
func detectDominantTheme(trend *TrendData) string {
	// Count total by category
	catTotals := make(map[string]int)
	totalCaptures := 0

	for _, day := range trend.Days {
		for cat, captures := range day.CapturesByCategory {
			catTotals[cat] += len(captures)
			totalCaptures += len(captures)
		}
	}

	if totalCaptures == 0 {
		return "quiet week"
	}

	// Find dominant category
	var maxCat string
	var maxCount int
	for cat, count := range catTotals {
		if count > maxCount {
			maxCat = cat
			maxCount = count
		}
	}

	// Check if truly dominant (>40%)
	if float64(maxCount)/float64(totalCaptures) > 0.4 {
		return fmt.Sprintf("%s-focused", strings.ToLower(maxCat))
	}

	// Check for recurring terms as theme
	if len(trend.RecurringTerms) > 0 {
		return fmt.Sprintf("recurring focus on %s", trend.RecurringTerms[0])
	}

	return "mixed focus"
}

// FormatTrendContext creates the context string for the daily LLM prompt
func FormatTrendContext(trend *TrendData) string {
	var sb strings.Builder

	// Recent days with actual content
	sb.WriteString("RECENT ACTIVITY:\n")
	for i, day := range trend.Days {
		if i >= 3 { // Only show last 3 days in detail
			break
		}
		sb.WriteString(fmt.Sprintf("\n%s (%s) - %d captures:\n", day.DayOfWeek, day.Date, day.CaptureCount))

		for cat, texts := range day.CapturesByCategory {
			if len(texts) > 3 {
				texts = texts[:3] // Limit to 3 per category
			}
			sb.WriteString(fmt.Sprintf("  %s: %s\n", cat, strings.Join(quoteStrings(texts), ", ")))
		}
	}

	// 7-day trends
	sb.WriteString("\n7-DAY TRENDS:\n")
	for cat, direction := range trend.CategoryTrend {
		if direction != "→ steady" { // Only show interesting trends
			sb.WriteString(fmt.Sprintf("  %s: %s\n", cat, direction))
		}
	}

	// Momentum shifts
	if len(trend.MomentumShifts) > 0 {
		sb.WriteString("\nNOTABLE SHIFTS:\n")
		for _, shift := range trend.MomentumShifts {
			sb.WriteString(fmt.Sprintf("  - %s\n", shift))
		}
	}

	// Recurring terms
	if len(trend.RecurringTerms) > 0 {
		terms := trend.RecurringTerms
		if len(terms) > 5 {
			terms = terms[:5]
		}
		sb.WriteString(fmt.Sprintf("\nRECURRING THEMES (3+ days): %s\n", strings.Join(terms, ", ")))
	}

	sb.WriteString(fmt.Sprintf("\nOVERALL: %s\n", trend.DominantTheme))

	return sb.String()
}

// FormatWeeklyContext creates the context string for the weekly LLM prompt
// Focuses on mental landscape: Ideas, Projects, Health, Life, Spirituality
func FormatWeeklyContext(trend *TrendData) string {
	var sb strings.Builder

	// Count totals by category
	catTotals := make(map[string]int)
	allCaptures := make(map[string][]string) // category -> all capture texts

	for _, day := range trend.Days {
		for cat, texts := range day.CapturesByCategory {
			catTotals[cat] += len(texts)
			allCaptures[cat] = append(allCaptures[cat], texts...)
		}
	}

	totalCaptures := 0
	for _, count := range catTotals {
		totalCaptures += count
	}

	sb.WriteString(fmt.Sprintf("WEEK OVERVIEW: %d captures across %d days\n\n", totalCaptures, len(trend.Days)))

	// Ideas section
	if ideas, ok := allCaptures["Ideas"]; ok && len(ideas) > 0 {
		sb.WriteString(fmt.Sprintf("IDEAS (%d):\n", len(ideas)))
		for _, idea := range ideas {
			sb.WriteString(fmt.Sprintf("  - %s\n", idea))
		}
		sb.WriteString("\n")
	}

	// Projects section
	if projects, ok := allCaptures["Projects"]; ok && len(projects) > 0 {
		sb.WriteString(fmt.Sprintf("PROJECTS (%d):\n", len(projects)))
		for _, proj := range projects {
			sb.WriteString(fmt.Sprintf("  - %s\n", proj))
		}
		sb.WriteString("\n")
	}

	// Health section
	if health, ok := allCaptures["Health"]; ok && len(health) > 0 {
		sb.WriteString(fmt.Sprintf("HEALTH (%d):\n", len(health)))
		for _, h := range health {
			sb.WriteString(fmt.Sprintf("  - %s\n", h))
		}
		sb.WriteString("\n")
	}

	// Life section
	if life, ok := allCaptures["Life"]; ok && len(life) > 0 {
		sb.WriteString(fmt.Sprintf("LIFE (%d):\n", len(life)))
		for _, l := range life {
			sb.WriteString(fmt.Sprintf("  - %s\n", l))
		}
		sb.WriteString("\n")
	}

	// Spirituality section
	if spirit, ok := allCaptures["Spirituality"]; ok && len(spirit) > 0 {
		sb.WriteString(fmt.Sprintf("SPIRITUALITY (%d):\n", len(spirit)))
		for _, s := range spirit {
			sb.WriteString(fmt.Sprintf("  - %s\n", s))
		}
		sb.WriteString("\n")
	}

	// Category trends
	sb.WriteString("CATEGORY MOMENTUM:\n")
	for cat, direction := range trend.CategoryTrend {
		sb.WriteString(fmt.Sprintf("  %s: %s\n", cat, direction))
	}

	// Momentum shifts
	if len(trend.MomentumShifts) > 0 {
		sb.WriteString("\nSHIFTS DETECTED:\n")
		for _, shift := range trend.MomentumShifts {
			sb.WriteString(fmt.Sprintf("  - %s\n", shift))
		}
	}

	// Recurring terms
	if len(trend.RecurringTerms) > 0 {
		terms := trend.RecurringTerms
		if len(terms) > 5 {
			terms = terms[:5]
		}
		sb.WriteString(fmt.Sprintf("\nRECURRING TERMS: %s\n", strings.Join(terms, ", ")))
	}

	// Absent categories
	var absent []string
	for _, cat := range []string{"Ideas", "Projects", "Health", "Life", "Spirituality"} {
		if catTotals[cat] == 0 {
			absent = append(absent, cat)
		}
	}
	if len(absent) > 0 {
		sb.WriteString(fmt.Sprintf("\nNO CAPTURES: %s\n", strings.Join(absent, ", ")))
	}

	return sb.String()
}

func truncateText(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func quoteStrings(ss []string) []string {
	quoted := make([]string, len(ss))
	for i, s := range ss {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return quoted
}
