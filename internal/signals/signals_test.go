package signals

import (
	"math"
	"testing"
	"time"

	"github.com/mrwolf/brain-server/internal/db"
)

// ============== Decay Tests ==============

func TestDecayWeight(t *testing.T) {
	tests := []struct {
		name         string
		oldWeight    float64
		daysSince    float64
		signalType   string
		everDominant bool
		wantApprox   float64
		tolerance    float64
	}{
		{
			name:       "term decays 50% in 3 days",
			oldWeight:  1.0,
			daysSince:  3.0,
			signalType: "term",
			wantApprox: 0.5,
			tolerance:  0.05,
		},
		{
			name:       "category decays 50% in 7 days",
			oldWeight:  1.0,
			daysSince:  7.0,
			signalType: "category",
			wantApprox: 0.5,
			tolerance:  0.05,
		},
		{
			name:       "project decays 50% in 30 days",
			oldWeight:  1.0,
			daysSince:  30.0,
			signalType: "project",
			wantApprox: 0.5,
			tolerance:  0.05,
		},
		{
			name:         "dominant project has floor",
			oldWeight:    0.1,
			daysSince:    100.0,
			signalType:   "project",
			everDominant: true,
			wantApprox:   FloorProject, // should not go below floor
			tolerance:    0.001,
		},
		{
			name:       "zero days means no decay",
			oldWeight:  5.0,
			daysSince:  0.0,
			signalType: "term",
			wantApprox: 5.0,
			tolerance:  0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DecayWeight(tt.oldWeight, tt.daysSince, tt.signalType, tt.everDominant)
			if math.Abs(got-tt.wantApprox) > tt.tolerance {
				t.Errorf("DecayWeight() = %v, want ~%v (±%v)", got, tt.wantApprox, tt.tolerance)
			}
		})
	}
}

func TestGetHalfLife(t *testing.T) {
	if getHalfLife("term") != HalfLifeTerm {
		t.Errorf("expected term half-life %v, got %v", HalfLifeTerm, getHalfLife("term"))
	}
	if getHalfLife("category") != HalfLifeCategory {
		t.Errorf("expected category half-life %v, got %v", HalfLifeCategory, getHalfLife("category"))
	}
	if getHalfLife("project") != HalfLifeProject {
		t.Errorf("expected project half-life %v, got %v", HalfLifeProject, getHalfLife("project"))
	}
	// Unknown type should default to term
	if getHalfLife("unknown") != HalfLifeTerm {
		t.Errorf("expected unknown type to default to term half-life")
	}
}

// ============== Extractor Tests ==============

func TestExtractTerms(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		maxTerms  int
		wantCount int
		wantTerms []string // expected terms in result
	}{
		{
			name:      "basic extraction",
			text:      "The project meeting went well. Project progress is good.",
			maxTerms:  3,
			wantCount: 3,
			wantTerms: []string{"project"}, // "project" appears twice
		},
		{
			name:      "removes stopwords",
			text:      "I am going to the store with some people",
			maxTerms:  5,
			wantCount: 1,
			wantTerms: []string{"store"},
		},
		{
			name:      "handles empty text",
			text:      "",
			maxTerms:  5,
			wantCount: 0,
		},
		{
			name:      "short words filtered",
			text:      "a an it is go on by up",
			maxTerms:  5,
			wantCount: 0, // all are stopwords or < 3 chars
		},
		{
			name:      "respects maxTerms",
			text:      "apple banana cherry date elderberry fig grape",
			maxTerms:  3,
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTerms(tt.text, tt.maxTerms)
			if len(got) != tt.wantCount {
				t.Errorf("ExtractTerms() returned %d terms, want %d", len(got), tt.wantCount)
			}
			for _, want := range tt.wantTerms {
				found := false
				for _, term := range got {
					if term == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ExtractTerms() missing expected term %q in %v", want, got)
				}
			}
		})
	}
}

func TestBuildWindowEvidence(t *testing.T) {
	now := time.Now()
	captures := []db.CaptureRecord{
		{RawText: "working on brain project today", RoutedTo: "Projects", CreatedAt: now},
		{RawText: "brain server needs more work", RoutedTo: "Projects", CreatedAt: now},
		{RawText: "feeling tired after workout", RoutedTo: "Health", CreatedAt: now},
	}

	evidence := BuildWindowEvidence(captures, 2)

	if len(evidence.Captures) != 3 {
		t.Errorf("expected 3 captures, got %d", len(evidence.Captures))
	}

	if evidence.PendingCount != 2 {
		t.Errorf("expected pending count 2, got %d", evidence.PendingCount)
	}

	if evidence.CategoryCounts["Projects"] != 2 {
		t.Errorf("expected 2 Projects, got %d", evidence.CategoryCounts["Projects"])
	}

	if evidence.CategoryCounts["Health"] != 1 {
		t.Errorf("expected 1 Health, got %d", evidence.CategoryCounts["Health"])
	}

	// Should have tracked "brain" as a repeated term
	if evidence.TermCounts["brain"] < 2 {
		t.Errorf("expected 'brain' term count >= 2, got %d", evidence.TermCounts["brain"])
	}
}

func TestDetectThemes(t *testing.T) {
	tests := []struct {
		name       string
		evidence   *WindowEvidence
		wantThemes []string // theme names to find
	}{
		{
			name: "detects term repeat theme",
			evidence: &WindowEvidence{
				TermCounts: map[string]int{"focus": 5, "random": 1},
			},
			wantThemes: []string{"focus_focus"},
		},
		{
			name: "detects definition friction",
			evidence: &WindowEvidence{
				PendingCount: 5,
				TermCounts:   map[string]int{},
			},
			wantThemes: []string{"definition_friction"},
		},
		{
			name: "detects health focus",
			evidence: &WindowEvidence{
				CategoryCounts: map[string]int{"Health": 4},
				TermCounts:     map[string]int{},
			},
			wantThemes: []string{"health_focus"},
		},
		{
			name: "detects project focus",
			evidence: &WindowEvidence{
				CategoryCounts: map[string]int{"Projects": 3},
				TermCounts:     map[string]int{},
			},
			wantThemes: []string{"project_progress"},
		},
		{
			name: "detects scattered attention",
			evidence: &WindowEvidence{
				CategoryCounts: map[string]int{
					"Health":   2,
					"Projects": 2,
					"Life":     2,
					"Ideas":    2,
				},
				TermCounts: map[string]int{},
			},
			wantThemes: []string{"scattered_attention"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			themes := DetectThemes(tt.evidence)
			for _, want := range tt.wantThemes {
				found := false
				for _, theme := range themes {
					if theme.Name == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("DetectThemes() missing expected theme %q", want)
				}
			}
		})
	}
}

func TestDetectTemporalShape(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		timestamps []time.Time
		want       string
	}{
		{
			name:       "too few captures",
			timestamps: []time.Time{now, now.Add(1 * time.Hour)},
			want:       "scattered",
		},
		{
			name: "clustered captures",
			timestamps: []time.Time{
				now,
				now.Add(10 * time.Minute),
				now.Add(20 * time.Minute),
				now.Add(30 * time.Minute),
			},
			want: "clustered",
		},
		{
			name: "steady captures",
			timestamps: []time.Time{
				now,
				now.Add(4 * time.Hour),
				now.Add(8 * time.Hour),
				now.Add(12 * time.Hour),
			},
			want: "steady",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectTemporalShape(tt.timestamps)
			if got != tt.want {
				t.Errorf("DetectTemporalShape() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ============== Selector Tests ==============

func TestIsDailyEligible(t *testing.T) {
	tests := []struct {
		name    string
		profile *DayProfile
		want    bool
	}{
		{
			name:    "eligible with 1 capture",
			profile: &DayProfile{CaptureCount: 1},
			want:    true,
		},
		{
			name:    "eligible with many captures",
			profile: &DayProfile{CaptureCount: 10},
			want:    true,
		},
		{
			name:    "not eligible with 0 captures",
			profile: &DayProfile{CaptureCount: 0},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDailyEligible(tt.profile)
			if got != tt.want {
				t.Errorf("IsDailyEligible() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsWeeklyEligible(t *testing.T) {
	tests := []struct {
		name    string
		profile *WeekProfile
		want    bool
	}{
		{
			name:    "eligible with 3 captures",
			profile: &WeekProfile{CaptureCount: 3},
			want:    true,
		},
		{
			name:    "not eligible with 2 captures",
			profile: &WeekProfile{CaptureCount: 2},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsWeeklyEligible(tt.profile)
			if got != tt.want {
				t.Errorf("IsWeeklyEligible() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectTheme(t *testing.T) {
	tests := []struct {
		name       string
		candidates []ThemeCandidate
		wantNil    bool
		wantName   string
	}{
		{
			name:       "nil for empty candidates",
			candidates: []ThemeCandidate{},
			wantNil:    true,
		},
		{
			name: "nil for insufficient evidence",
			candidates: []ThemeCandidate{
				{Name: "weak", Evidence: 1, SourceType: "term_repeat"},
			},
			wantNil: true,
		},
		{
			name: "selects highest evidence",
			candidates: []ThemeCandidate{
				{Name: "strong", Evidence: 5, SourceType: "term_repeat"},
				{Name: "weak", Evidence: 2, SourceType: "term_repeat"},
			},
			wantName: "strong",
		},
		{
			name: "prefers actionable on tie",
			candidates: []ThemeCandidate{
				{Name: "term_theme", Evidence: 3, SourceType: "term_repeat"},
				{Name: "friction_theme", Evidence: 3, SourceType: "friction"},
			},
			wantName: "friction_theme", // friction is more actionable
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SelectTheme(tt.candidates)
			if tt.wantNil {
				if got != nil {
					t.Errorf("SelectTheme() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Errorf("SelectTheme() = nil, want %q", tt.wantName)
				return
			}
			if got.Name != tt.wantName {
				t.Errorf("SelectTheme() = %q, want %q", got.Name, tt.wantName)
			}
		})
	}
}

func TestSelectDailyAction(t *testing.T) {
	tests := []struct {
		name       string
		profile    *DayProfile
		wantSource string
	}{
		{
			name: "project next action takes priority",
			profile: &DayProfile{
				ProjectActivity: []ProjectActivity{
					{Name: "brain", HasNextAction: true, NextAction: "deploy v2"},
				},
				PendingCount: 1,
			},
			wantSource: "project_next",
		},
		{
			name: "pending clarify second priority",
			profile: &DayProfile{
				ProjectActivity: []ProjectActivity{},
				PendingCount:    5,
			},
			wantSource: "pending_clarify",
		},
		{
			name: "countermove from theme",
			profile: &DayProfile{
				ProjectActivity: []ProjectActivity{},
				PendingCount:    0,
				SelectedTheme:   &ThemeCandidate{SourceType: "health_focus"},
			},
			wantSource: "countermove",
		},
		{
			name: "nil when no action available",
			profile: &DayProfile{
				ProjectActivity: []ProjectActivity{},
				PendingCount:    0,
				SelectedTheme:   nil,
			},
			wantSource: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SelectDailyAction(tt.profile)
			if tt.wantSource == "" {
				if got != nil {
					t.Errorf("SelectDailyAction() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Errorf("SelectDailyAction() = nil, want source %q", tt.wantSource)
				return
			}
			if got.Source != tt.wantSource {
				t.Errorf("SelectDailyAction().Source = %q, want %q", got.Source, tt.wantSource)
			}
		})
	}
}

// ============== Validator Tests ==============

func TestValidateLetter(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		isDaily   bool
		wantValid bool
	}{
		{
			name:      "valid letter",
			text:      "Yesterday was productive. You worked on the brain project.",
			isDaily:   true,
			wantValid: true,
		},
		{
			name:      "too short",
			text:      "Hi",
			isDaily:   true,
			wantValid: false,
		},
		{
			name:      "contains money term",
			text:      "You spent money on the project",
			isDaily:   true,
			wantValid: false,
		},
		{
			name:      "contains therapy-speak",
			text:      "This is your journey of self-care",
			isDaily:   true,
			wantValid: false,
		},
		{
			name:      "contains currency",
			text:      "The project cost $500 to build",
			isDaily:   true,
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateLetter(tt.text, tt.isDaily)
			if result.Valid != tt.wantValid {
				t.Errorf("ValidateLetter().Valid = %v, want %v (errors: %v)", result.Valid, tt.wantValid, result.Errors)
			}
		})
	}
}

func TestSanitizeLetter(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		wantChanged bool
	}{
		{
			name:        "no change needed",
			text:        "Yesterday was productive.",
			wantChanged: false,
		},
		{
			name:        "removes greeting",
			text:        "Dear friend, Yesterday was productive.",
			wantChanged: true,
		},
		{
			name:        "removes signoff",
			text:        "Yesterday was productive.\n\nSincerely,",
			wantChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, changed := SanitizeLetter(tt.text)
			if changed != tt.wantChanged {
				t.Errorf("SanitizeLetter() changed = %v, want %v", changed, tt.wantChanged)
			}
		})
	}
}

func TestValidateProfile(t *testing.T) {
	tests := []struct {
		name      string
		profile   *DayProfile
		wantValid bool
	}{
		{
			name:      "nil profile invalid",
			profile:   nil,
			wantValid: false,
		},
		{
			name:      "missing date invalid",
			profile:   &DayProfile{Date: "", CaptureCount: 1},
			wantValid: false,
		},
		{
			name:      "negative capture count invalid",
			profile:   &DayProfile{Date: "2026-01-21", CaptureCount: -1},
			wantValid: false,
		},
		{
			name:      "valid profile",
			profile:   &DayProfile{Date: "2026-01-21", CaptureCount: 5},
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateProfile(tt.profile)
			if result.Valid != tt.wantValid {
				t.Errorf("ValidateProfile().Valid = %v, want %v", result.Valid, tt.wantValid)
			}
		})
	}
}

// ============== Profile Tests ==============

func TestGetCategoryMixLabel(t *testing.T) {
	tests := []struct {
		name   string
		counts map[string]int
		want   string
	}{
		{
			name:   "empty counts",
			counts: map[string]int{},
			want:   "light capture day",
		},
		{
			name:   "few captures",
			counts: map[string]int{"Projects": 2},
			want:   "light capture day",
		},
		{
			name:   "projects dominant",
			counts: map[string]int{"Projects": 5, "Health": 1},
			want:   "mostly Projects",
		},
		{
			name:   "health dominant",
			counts: map[string]int{"Health": 6, "Life": 2},
			want:   "mostly Health",
		},
		{
			name:   "mixed activity",
			counts: map[string]int{"Projects": 3, "Health": 2, "Life": 2, "Ideas": 3},
			want:   "mixed activity",
		},
		{
			name:   "health and life mix",
			counts: map[string]int{"Health": 3, "Life": 3, "Projects": 1},
			want:   "Health and Life",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetCategoryMixLabel(tt.counts)
			if got != tt.want {
				t.Errorf("GetCategoryMixLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ============== Stopwords Tests ==============

func TestIsStopword(t *testing.T) {
	stopwords := []string{"the", "is", "and", "to", "of", "in", "for"}
	for _, word := range stopwords {
		if !IsStopword(word) {
			t.Errorf("IsStopword(%q) = false, want true", word)
		}
	}

	nonStopwords := []string{"project", "brain", "health", "exercise"}
	for _, word := range nonStopwords {
		if IsStopword(word) {
			t.Errorf("IsStopword(%q) = true, want false", word)
		}
	}
}
