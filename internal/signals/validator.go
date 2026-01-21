package signals

import (
	"regexp"
	"strings"
)

// ValidationResult contains the result of validating a letter
type ValidationResult struct {
	Valid    bool
	Warnings []string
	Errors   []string
}

// ForbiddenTerms - words/phrases that should never appear in letters
var ForbiddenTerms = []string{
	// Money/financial terms
	"money", "spending", "budget", "budgets", "cost", "costs", "price", "prices",
	"purchase", "purchases", "dollar", "dollars", "expense", "expenses",
	"afford", "affordable", "cheap", "expensive", "save", "savings",
	"invest", "investment", "financial", "finances", "bank", "banking",

	// Therapy-speak / self-help clichés
	"journey", "growth mindset", "self-care", "selfcare", "boundaries",
	"space for", "holding space", "lean into", "lean in", "honor your",
	"manifest", "manifesting", "authentic self", "best self", "true self",
	"healing journey", "inner child", "trauma response", "triggered",
	"toxic positivity", "live your truth", "speak your truth",
	"radical acceptance", "radical self-love", "self-love",
}

// CurrencyPatterns - regex patterns for currency mentions
var CurrencyPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\$\d`),               // $5, $100, etc.
	regexp.MustCompile(`\d+\s*dollars?`),     // 5 dollars, 100 dollar
	regexp.MustCompile(`\d+\s*cents?`),       // 50 cents
	regexp.MustCompile(`€\d`),                // €5
	regexp.MustCompile(`£\d`),                // £5
	regexp.MustCompile(`\d+\s*euros?`),       // 5 euros
	regexp.MustCompile(`\d+\s*pounds?`),      // 5 pounds (currency)
	regexp.MustCompile(`\d+k\b`),             // 5k, 100k (often money)
	regexp.MustCompile(`\d+\s*bucks?`),       // 5 bucks
}

// MaxDailyLength - maximum character length for daily letters
const MaxDailyLength = 500

// MaxWeeklyLength - maximum character length for weekly letters
const MaxWeeklyLength = 800

// ValidateLetter checks a generated letter for forbidden content
func ValidateLetter(text string, isDaily bool) ValidationResult {
	result := ValidationResult{Valid: true}
	lowerText := strings.ToLower(text)

	// Check forbidden terms
	for _, term := range ForbiddenTerms {
		if strings.Contains(lowerText, term) {
			result.Errors = append(result.Errors, "contains forbidden term: "+term)
			result.Valid = false
		}
	}

	// Check currency patterns
	for _, pattern := range CurrencyPatterns {
		if pattern.MatchString(text) {
			result.Errors = append(result.Errors, "contains currency reference")
			result.Valid = false
			break
		}
	}

	// Check length
	maxLen := MaxWeeklyLength
	if isDaily {
		maxLen = MaxDailyLength
	}
	if len(text) > maxLen {
		result.Warnings = append(result.Warnings, "letter exceeds recommended length")
	}

	// Check for empty or too short
	if len(strings.TrimSpace(text)) < 10 {
		result.Errors = append(result.Errors, "letter too short or empty")
		result.Valid = false
	}

	// Check for greeting patterns (should not have)
	greetingPatterns := []string{
		"dear ", "hi ", "hello ", "hey ",
		"good morning", "good evening", "good afternoon",
	}
	for _, greeting := range greetingPatterns {
		if strings.HasPrefix(lowerText, greeting) {
			result.Warnings = append(result.Warnings, "letter starts with greeting")
			break
		}
	}

	// Check for signoff patterns (should not have)
	signoffPatterns := []string{
		"sincerely", "best regards", "warm regards", "cheers",
		"take care", "yours truly", "best wishes", "warmly",
	}
	for _, signoff := range signoffPatterns {
		if strings.HasSuffix(strings.TrimSpace(lowerText), signoff) {
			result.Warnings = append(result.Warnings, "letter ends with signoff")
			break
		}
	}

	return result
}

// SanitizeLetter attempts to fix minor issues in a letter
// Returns the sanitized text and whether changes were made
func SanitizeLetter(text string) (string, bool) {
	original := text
	text = strings.TrimSpace(text)

	// Remove common greeting prefixes
	greetings := []string{
		"Dear friend,", "Dear you,", "Hi there,", "Hello,",
		"Good morning,", "Good evening,", "Good afternoon,",
	}
	for _, g := range greetings {
		if strings.HasPrefix(text, g) {
			text = strings.TrimSpace(text[len(g):])
			break
		}
	}

	// Remove common signoff suffixes
	signoffs := []string{
		"\n\nSincerely,", "\n\nBest regards,", "\n\nWarm regards,",
		"\n\nCheers,", "\n\nTake care,", "\n\nYours truly,",
		"\n\nBest wishes,", "\n\nWarmly,",
	}
	for _, s := range signoffs {
		if strings.HasSuffix(text, s) {
			text = strings.TrimSpace(text[:len(text)-len(s)])
			break
		}
	}

	return text, text != original
}

// ValidateProfile checks that a profile is valid for letter generation
func ValidateProfile(profile *DayProfile) ValidationResult {
	result := ValidationResult{Valid: true}

	if profile == nil {
		result.Errors = append(result.Errors, "profile is nil")
		result.Valid = false
		return result
	}

	if profile.Date == "" {
		result.Errors = append(result.Errors, "profile missing date")
		result.Valid = false
	}

	if profile.CaptureCount < 0 {
		result.Errors = append(result.Errors, "invalid capture count")
		result.Valid = false
	}

	return result
}

// ValidateWeekProfile checks that a week profile is valid
func ValidateWeekProfile(profile *WeekProfile) ValidationResult {
	result := ValidationResult{Valid: true}

	if profile == nil {
		result.Errors = append(result.Errors, "profile is nil")
		result.Valid = false
		return result
	}

	if profile.WeekID == "" {
		result.Errors = append(result.Errors, "profile missing week ID")
		result.Valid = false
	}

	if profile.CaptureCount < 0 {
		result.Errors = append(result.Errors, "invalid capture count")
		result.Valid = false
	}

	return result
}
