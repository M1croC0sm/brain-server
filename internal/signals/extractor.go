package signals

import (
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mrwolf/brain-server/internal/db"
)

// WindowEvidence contains actual data from the time window (NOT from signals table)
// This is the PRIMARY input for letter generation
type WindowEvidence struct {
	Captures        []db.CaptureRecord // raw captures in window
	TermCounts      map[string]int     // term → count in window
	CategoryCounts  map[string]int     // category → count in window
	ProjectActivity []ProjectActivity  // project mentions in window
	PendingCount    int                // clarifications pending in window
	Timestamps      []time.Time        // for temporal shape detection
}

// ProjectActivity tracks project mentions within the window
type ProjectActivity struct {
	Name          string
	MentionCount  int
	LastMention   time.Time
	HasNextAction bool
	NextAction    string // if extractable
}

// ThemeCandidate represents a detected theme from window evidence
type ThemeCandidate struct {
	Name       string
	Evidence   int    // count of supporting events in window
	SourceType string // "term_repeat", "friction", "stalled", "health_focus", "project_focus"
}

// tokenize splits text into lowercase words, removing punctuation
var wordRegex = regexp.MustCompile(`[a-zA-Z]+`)

// ExtractTerms extracts terms from text, lowercase, remove stopwords, return top N by frequency
func ExtractTerms(text string, maxTerms int) []string {
	words := wordRegex.FindAllString(strings.ToLower(text), -1)

	// Count non-stopword terms
	counts := make(map[string]int)
	for _, word := range words {
		if len(word) < 3 {
			continue // Skip very short words
		}
		if IsStopword(word) {
			continue
		}
		counts[word]++
	}

	// Sort by count descending
	type termCount struct {
		term  string
		count int
	}
	var sorted []termCount
	for term, count := range counts {
		sorted = append(sorted, termCount{term, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	// Take top N
	var result []string
	for i := 0; i < len(sorted) && i < maxTerms; i++ {
		result = append(result, sorted[i].term)
	}
	return result
}

// BuildWindowEvidence extracts evidence from captures in the time window
func BuildWindowEvidence(captures []db.CaptureRecord, pendingCount int) *WindowEvidence {
	evidence := &WindowEvidence{
		Captures:       captures,
		TermCounts:     make(map[string]int),
		CategoryCounts: make(map[string]int),
		PendingCount:   pendingCount,
	}

	projectMentions := make(map[string]*ProjectActivity)

	for _, c := range captures {
		// Extract terms and count them
		terms := ExtractTerms(c.RawText, 10)
		for _, term := range terms {
			evidence.TermCounts[term]++
		}

		// Count categories
		if c.RoutedTo != "" {
			evidence.CategoryCounts[c.RoutedTo]++
		}

		// Track timestamps
		evidence.Timestamps = append(evidence.Timestamps, c.CreatedAt)

		// Track project activity (if category is Projects)
		if c.RoutedTo == "Projects" {
			// Use first significant term as project identifier
			projectName := "unnamed"
			if len(terms) > 0 {
				projectName = terms[0]
			}

			if pa, exists := projectMentions[projectName]; exists {
				pa.MentionCount++
				if c.CreatedAt.After(pa.LastMention) {
					pa.LastMention = c.CreatedAt
				}
			} else {
				projectMentions[projectName] = &ProjectActivity{
					Name:         projectName,
					MentionCount: 1,
					LastMention:  c.CreatedAt,
				}
			}
		}
	}

	// Convert project map to slice
	for _, pa := range projectMentions {
		evidence.ProjectActivity = append(evidence.ProjectActivity, *pa)
	}

	// Sort projects by mention count
	sort.Slice(evidence.ProjectActivity, func(i, j int) bool {
		return evidence.ProjectActivity[i].MentionCount > evidence.ProjectActivity[j].MentionCount
	})

	return evidence
}

// DetectThemes performs rule-based theme detection FROM WINDOW EVIDENCE (not signals)
// Rules applied to actual evidence:
//   - term count >= 3 in window → theme candidate (term_repeat)
//   - pending count > 3 → theme:definition_friction
//   - project mentioned but no activity in 7d → theme:stalled_momentum
//   - health captures >= 3 in window → theme:health_focus
func DetectThemes(evidence *WindowEvidence) []ThemeCandidate {
	var candidates []ThemeCandidate

	// Rule 1: Repeated terms (count >= 3)
	for term, count := range evidence.TermCounts {
		if count >= 3 {
			candidates = append(candidates, ThemeCandidate{
				Name:       term + "_focus",
				Evidence:   count,
				SourceType: "term_repeat",
			})
		}
	}

	// Rule 2: Definition friction (pending clarifications > 3)
	if evidence.PendingCount > 3 {
		candidates = append(candidates, ThemeCandidate{
			Name:       "definition_friction",
			Evidence:   evidence.PendingCount,
			SourceType: "friction",
		})
	}

	// Rule 3: Health focus (health captures >= 3)
	if healthCount, ok := evidence.CategoryCounts["Health"]; ok && healthCount >= 3 {
		candidates = append(candidates, ThemeCandidate{
			Name:       "health_focus",
			Evidence:   healthCount,
			SourceType: "health_focus",
		})
	}

	// Rule 4: Project focus (projects captures >= 2)
	if projectCount, ok := evidence.CategoryCounts["Projects"]; ok && projectCount >= 2 {
		candidates = append(candidates, ThemeCandidate{
			Name:       "project_progress",
			Evidence:   projectCount,
			SourceType: "project_focus",
		})
	}

	// Rule 5: Scattered attention (many categories with low counts)
	categoryCount := len(evidence.CategoryCounts)
	if categoryCount >= 4 {
		// Many different categories, no dominant one
		maxCount := 0
		for _, count := range evidence.CategoryCounts {
			if count > maxCount {
				maxCount = count
			}
		}
		// If max category has < 40% of total, attention is scattered
		total := 0
		for _, count := range evidence.CategoryCounts {
			total += count
		}
		if total > 0 && float64(maxCount)/float64(total) < 0.4 {
			candidates = append(candidates, ThemeCandidate{
				Name:       "scattered_attention",
				Evidence:   categoryCount,
				SourceType: "scattered",
			})
		}
	}

	// Sort by evidence count descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Evidence > candidates[j].Evidence
	})

	return candidates
}

// DetectTemporalShape analyzes capture timestamps
// Returns: "steady", "clustered", or "scattered"
func DetectTemporalShape(timestamps []time.Time) string {
	if len(timestamps) < 3 {
		return "scattered" // Too few captures to determine pattern
	}

	// Sort timestamps
	sorted := make([]time.Time, len(timestamps))
	copy(sorted, timestamps)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Before(sorted[j])
	})

	// Check for clustering: 70%+ in a 2-hour window
	total := len(sorted)
	for i := 0; i < total; i++ {
		windowEnd := sorted[i].Add(2 * time.Hour)
		inWindow := 0
		for j := i; j < total && sorted[j].Before(windowEnd); j++ {
			inWindow++
		}
		if float64(inWindow)/float64(total) >= 0.7 {
			return "clustered"
		}
	}

	// Check for steady: reasonable gaps between captures
	// Calculate average gap
	var totalGap time.Duration
	for i := 1; i < len(sorted); i++ {
		totalGap += sorted[i].Sub(sorted[i-1])
	}
	avgGap := totalGap / time.Duration(len(sorted)-1)

	// Check variance
	var variance float64
	for i := 1; i < len(sorted); i++ {
		gap := sorted[i].Sub(sorted[i-1])
		diff := float64(gap - avgGap)
		variance += diff * diff
	}
	variance /= float64(len(sorted) - 1)

	// If variance is low relative to average, it's steady
	// Coefficient of variation < 1.0 indicates relative steadiness
	avgGapSeconds := avgGap.Seconds()
	if avgGapSeconds > 0 {
		cv := (variance / (avgGapSeconds * avgGapSeconds))
		if cv < 1.0 {
			return "steady"
		}
	}

	return "scattered"
}

// TermCount represents a term with its count in window
type TermCount struct {
	Term  string
	Count int
}

// GetTopTermsFromEvidence returns top N terms by count from window evidence
func GetTopTermsFromEvidence(evidence *WindowEvidence, n int) []TermCount {
	var terms []TermCount
	for term, count := range evidence.TermCounts {
		terms = append(terms, TermCount{Term: term, Count: count})
	}

	sort.Slice(terms, func(i, j int) bool {
		return terms[i].Count > terms[j].Count
	})

	if len(terms) > n {
		return terms[:n]
	}
	return terms
}
