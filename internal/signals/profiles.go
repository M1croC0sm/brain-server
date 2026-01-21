package signals

import (
	"fmt"
	"time"

	"github.com/mrwolf/brain-server/internal/db"
)

// DayProfile - built primarily from WindowEvidence, signals only for tie-breaks
type DayProfile struct {
	Date string

	// PRIMARY: from WindowEvidence (24h)
	CaptureCount     int
	CountsByCategory map[string]int
	TopTermsInWindow []TermCount
	ProjectActivity  []ProjectActivity
	PendingCount     int
	TemporalShape    string // "steady", "clustered", "scattered"

	// SECONDARY: from signals table (for breadcrumbs/tie-breaks only)
	LongTermTendencies []WeightedTerm

	// DERIVED: from theme detection on window evidence
	ThemeCandidates []ThemeCandidate
	SelectedTheme   *ThemeCandidate // nil = silence letter

	// ACTION: best concrete next step (not always countermove)
	BestNextAction *NextAction // nil = no action / countermove fallback
}

// WeekProfile - built from 7 days of window evidence
type WeekProfile struct {
	WeekID string // "2026-W03"

	// PRIMARY: from WindowEvidence (7d)
	CaptureCount     int
	CountsByCategory map[string]int
	TopTermsInWindow []TermCount
	ProjectActivity  []ProjectActivity

	// DERIVED
	ThemeCandidates []ThemeCandidate
	SelectedTheme   *ThemeCandidate
}

// WeightedTerm represents a term with its decayed weight from signals table
type WeightedTerm struct {
	Term   string
	Weight float64
}

// NextAction represents the best concrete next step for a letter
type NextAction struct {
	Text       string
	Source     string // "project_next", "pending_clarify", "countermove"
	ProjectRef string // if from a project
}

// CategoryMixLabels - fixed strings for deterministic phrasing
var CategoryMixLabels = map[string]string{
	"projects_dominant": "mostly Projects",
	"health_dominant":   "mostly Health",
	"life_dominant":     "mostly Life",
	"ideas_dominant":    "mostly Ideas",
	"health_life_mix":   "Health and Life",
	"projects_health":   "Projects and Health",
	"mixed":             "mixed activity",
	"light":             "light capture day",
}

// GetCategoryMixLabel returns a fixed string based on category proportions
func GetCategoryMixLabel(counts map[string]int) string {
	if len(counts) == 0 {
		return CategoryMixLabels["light"]
	}

	total := 0
	for _, count := range counts {
		total += count
	}

	if total < 3 {
		return CategoryMixLabels["light"]
	}

	// Find dominant category
	var maxCat string
	var maxCount int
	for cat, count := range counts {
		if count > maxCount {
			maxCat = cat
			maxCount = count
		}
	}

	// Check if dominant (>50% of total)
	if float64(maxCount)/float64(total) > 0.5 {
		switch maxCat {
		case "Projects":
			return CategoryMixLabels["projects_dominant"]
		case "Health":
			return CategoryMixLabels["health_dominant"]
		case "Life":
			return CategoryMixLabels["life_dominant"]
		case "Ideas":
			return CategoryMixLabels["ideas_dominant"]
		}
	}

	// Check for common mixes
	healthCount := counts["Health"]
	lifeCount := counts["Life"]
	projectCount := counts["Projects"]

	if healthCount > 0 && lifeCount > 0 && healthCount+lifeCount > total/2 {
		return CategoryMixLabels["health_life_mix"]
	}
	if projectCount > 0 && healthCount > 0 && projectCount+healthCount > total/2 {
		return CategoryMixLabels["projects_health"]
	}

	return CategoryMixLabels["mixed"]
}

// BuildDayProfile constructs a day profile from database
// Window evidence is PRIMARY, signals are SECONDARY
func BuildDayProfile(database *db.DB, actor string, date time.Time) (*DayProfile, error) {
	profile := &DayProfile{
		Date:             date.Format("2006-01-02"),
		CountsByCategory: make(map[string]int),
	}

	// 1. Get captures in 24h window
	since := date.Add(-24 * time.Hour)
	captures, err := database.GetRecentCaptures(actor, since)
	if err != nil {
		return nil, err
	}
	profile.CaptureCount = len(captures)

	// 2. Get pending clarifications
	pending, err := database.GetPending(actor)
	if err != nil {
		return nil, err
	}
	profile.PendingCount = len(pending)

	// 3. Build WindowEvidence from captures
	evidence := BuildWindowEvidence(captures, profile.PendingCount)
	profile.CountsByCategory = evidence.CategoryCounts
	profile.TopTermsInWindow = GetTopTermsFromEvidence(evidence, 5)
	profile.ProjectActivity = evidence.ProjectActivity
	profile.TemporalShape = DetectTemporalShape(evidence.Timestamps)

	// 4. Detect themes from window evidence
	profile.ThemeCandidates = DetectThemes(evidence)

	// 5. Optionally get long-term signals for tie-breaks
	signals, err := database.GetTopSignals("term", 10)
	if err == nil {
		for _, s := range signals {
			// Strip "term:" prefix if present
			term := s.Key
			if len(term) > 5 && term[:5] == "term:" {
				term = term[5:]
			}
			profile.LongTermTendencies = append(profile.LongTermTendencies, WeightedTerm{
				Term:   term,
				Weight: s.Weight,
			})
		}
	}

	return profile, nil
}

// BuildWeekProfile constructs a week profile from database
// 95% window evidence, signals barely used
func BuildWeekProfile(database *db.DB, actor string, weekStart time.Time) (*WeekProfile, error) {
	_, week := weekStart.ISOWeek()
	profile := &WeekProfile{
		WeekID:           weekStart.Format("2006") + "-W" + padWeek(week),
		CountsByCategory: make(map[string]int),
	}

	// 1. Get captures in 7d window
	since := weekStart.Add(-7 * 24 * time.Hour)
	captures, err := database.GetRecentCaptures(actor, since)
	if err != nil {
		return nil, err
	}
	profile.CaptureCount = len(captures)

	// 2. Get pending for friction detection
	pending, err := database.GetPending(actor)
	if err != nil {
		return nil, err
	}

	// 3. Build WindowEvidence from captures
	evidence := BuildWindowEvidence(captures, len(pending))
	profile.CountsByCategory = evidence.CategoryCounts
	profile.TopTermsInWindow = GetTopTermsFromEvidence(evidence, 5)
	profile.ProjectActivity = evidence.ProjectActivity

	// 4. Detect themes from window evidence
	profile.ThemeCandidates = DetectThemes(evidence)

	return profile, nil
}

func padWeek(week int) string {
	return fmt.Sprintf("%02d", week)
}
