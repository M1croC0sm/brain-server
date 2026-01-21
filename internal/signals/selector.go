package signals

// Eligibility thresholds (based on WINDOW COUNTS, not signal weights)
const (
	// Daily letter eligibility: need at least 1 capture in 24h window
	MinDailyCaptures = 1

	// Weekly letter eligibility: need at least 3 captures in 7d window
	MinWeeklyCaptures = 3

	// Theme eligibility: need at least 2 supporting evidence points
	MinThemeEvidence = 2
)

// Countermoves - library of gentle weekly reframes
// Map from detected theme type to countermove suggestion
var Countermoves = map[string]string{
	// Theme-specific countermoves
	"scattered_attention":   "Consider picking one thread to pull this week",
	"definition_friction":   "Those pending clarifications might be worth a quiet moment",
	"health_focus":          "The body's been talking - maybe it has something to teach",
	"project_progress":      "Good momentum on projects - what would make next week even better?",
	"term_repeat":           "Something's been on your mind - worth exploring deeper?",

	// Fallback countermoves for common patterns
	"high_volume":           "Lots of captures lately - anything connecting them?",
	"low_volume":            "Quiet week - sometimes that's exactly what's needed",
	"projects_dominant":     "Projects taking center stage - is that where you want your energy?",
	"health_dominant":       "Health's been a theme - listening to signals from the body",
	"life_dominant":         "Life stuff accumulating - any patterns worth noticing?",
	"ideas_dominant":        "Ideas flowing - which ones have legs?",

	// Generic fallback
	"default":               "What would make next week feel complete?",
}

// IsDailyEligible checks if there's enough window evidence for a daily letter
func IsDailyEligible(profile *DayProfile) bool {
	return profile.CaptureCount >= MinDailyCaptures
}

// IsWeeklyEligible checks if there's enough window evidence for a weekly letter
func IsWeeklyEligible(profile *WeekProfile) bool {
	return profile.CaptureCount >= MinWeeklyCaptures
}

// SelectTheme picks the best theme from candidates, or nil for silence
// Selection priority:
// 1. Highest evidence count
// 2. Prefer friction/stalled over term_repeat (more actionable)
// 3. Return nil if no candidate has sufficient evidence
func SelectTheme(candidates []ThemeCandidate) *ThemeCandidate {
	if len(candidates) == 0 {
		return nil
	}

	// Candidates are already sorted by evidence count descending
	best := candidates[0]

	// Check minimum evidence threshold
	if best.Evidence < MinThemeEvidence {
		return nil
	}

	// If there are ties, prefer actionable themes
	actionablePriority := map[string]int{
		"friction":      3, // Most actionable
		"stalled":       2,
		"project_focus": 1,
		"health_focus":  1,
		"scattered":     1,
		"term_repeat":   0, // Least actionable (just observation)
	}

	for i := 1; i < len(candidates); i++ {
		c := candidates[i]
		if c.Evidence < best.Evidence {
			break // No more ties
		}
		// Same evidence count - compare actionability
		if actionablePriority[c.SourceType] > actionablePriority[best.SourceType] {
			best = c
		}
	}

	return &best
}

// SelectDailyAction picks the best concrete action for a daily letter
// Priority: project_next > pending_clarify > countermove > none
func SelectDailyAction(profile *DayProfile) *NextAction {
	// 1. Check for project with next action
	for _, pa := range profile.ProjectActivity {
		if pa.HasNextAction && pa.NextAction != "" {
			return &NextAction{
				Text:       pa.NextAction,
				Source:     "project_next",
				ProjectRef: pa.Name,
			}
		}
	}

	// 2. Check for pending clarifications
	if profile.PendingCount > 0 {
		return &NextAction{
			Text:   "You have pending clarifications to review",
			Source: "pending_clarify",
		}
	}

	// 3. Theme-based countermove (if theme selected)
	if profile.SelectedTheme != nil {
		countermove, ok := Countermoves[profile.SelectedTheme.SourceType]
		if ok {
			return &NextAction{
				Text:   countermove,
				Source: "countermove",
			}
		}
	}

	// 4. No action - letter will be observation only
	return nil
}

// SelectWeeklyCountermove picks a countermove for the weekly letter
func SelectWeeklyCountermove(profile *WeekProfile) string {
	// 1. Theme-based countermove
	if profile.SelectedTheme != nil {
		if cm, ok := Countermoves[profile.SelectedTheme.SourceType]; ok {
			return cm
		}
	}

	// 2. Volume-based countermove
	if profile.CaptureCount >= 20 {
		return Countermoves["high_volume"]
	}
	if profile.CaptureCount <= 5 {
		return Countermoves["low_volume"]
	}

	// 3. Category-based countermove
	categoryLabel := GetCategoryMixLabel(profile.CountsByCategory)
	switch categoryLabel {
	case CategoryMixLabels["projects_dominant"]:
		return Countermoves["projects_dominant"]
	case CategoryMixLabels["health_dominant"]:
		return Countermoves["health_dominant"]
	case CategoryMixLabels["life_dominant"]:
		return Countermoves["life_dominant"]
	case CategoryMixLabels["ideas_dominant"]:
		return Countermoves["ideas_dominant"]
	}

	// 4. Default fallback
	return Countermoves["default"]
}

// ApplyThemeSelection sets SelectedTheme on a DayProfile
func ApplyThemeSelection(profile *DayProfile) {
	profile.SelectedTheme = SelectTheme(profile.ThemeCandidates)
	profile.BestNextAction = SelectDailyAction(profile)
}

// ApplyWeeklyThemeSelection sets SelectedTheme on a WeekProfile
func ApplyWeeklyThemeSelection(profile *WeekProfile) {
	profile.SelectedTheme = SelectTheme(profile.ThemeCandidates)
}
