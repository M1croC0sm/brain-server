package narrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// LLMClient interface for LLM interactions
// Implement this to connect to your actual LLM service
type LLMClient interface {
	Generate(ctx context.Context, model, system, prompt string) (string, error)
}

// Pipeline handles the 3-step narration process
type Pipeline struct {
	llm        LLMClient
	model      string
	maxRetries int
}

// NewPipeline creates a new narration pipeline
func NewPipeline(llm LLMClient, model string, maxRetries int) *Pipeline {
	return &Pipeline{
		llm:        llm,
		model:      model,
		maxRetries: maxRetries,
	}
}

// NarrationResult holds the output of the full pipeline
type NarrationResult struct {
	NarratedText   string
	Claims         ClaimSet
	Verified       bool
	Attempts       int
	RawFiles       []string
}

// Process runs the full 3-step pipeline on a batch of entries
func (p *Pipeline) Process(ctx context.Context, entries []RawEntry) (*NarrationResult, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("no entries to process")
	}

	// Collect filenames for audit trail
	var filenames []string
	for _, e := range entries {
		filenames = append(filenames, e.Filename)
	}

	// Step 1: Extract claims
	claims, err := p.extractClaims(ctx, entries)
	if err != nil {
		return nil, fmt.Errorf("claim extraction failed: %w", err)
	}

	if len(claims.Claims) == 0 {
		return nil, fmt.Errorf("no claims extracted from entries")
	}

	// Step 2 & 3: Narrate and verify (with retries)
	var narrated string
	var verified bool
	var feedback string
	attempts := 0

	for attempts < p.maxRetries+1 {
		attempts++

		// Step 2: Generate narration
		if attempts == 1 {
			narrated, err = p.narrate(ctx, claims)
		} else {
			narrated, err = p.narrateStrict(ctx, claims, feedback)
		}
		if err != nil {
			return nil, fmt.Errorf("narration failed (attempt %d): %w", attempts, err)
		}

		// Step 3: Verify
		result, err := p.verify(ctx, claims, narrated)
		if err != nil {
			return nil, fmt.Errorf("verification failed (attempt %d): %w", attempts, err)
		}

		if result.Passed {
			verified = true
			break
		}

		// Prepare feedback for retry
		feedback = result.Feedback
		if len(result.UnsupportedClaims) > 0 {
			feedback += "\nUnsupported statements: " + strings.Join(result.UnsupportedClaims, "; ")
		}
	}

	return &NarrationResult{
		NarratedText: narrated,
		Claims:       claims,
		Verified:     verified,
		Attempts:     attempts,
		RawFiles:     filenames,
	}, nil
}

// extractClaims runs Step 1: claim extraction
func (p *Pipeline) extractClaims(ctx context.Context, entries []RawEntry) (ClaimSet, error) {
	prompt := BuildClaimExtractionPrompt(entries)

	response, err := p.llm.Generate(ctx, p.model, SystemPrompt, prompt)
	if err != nil {
		return ClaimSet{}, err
	}

	// Parse JSON response
	claims, err := parseClaimsResponse(response)
	if err != nil {
		return ClaimSet{}, fmt.Errorf("failed to parse claims response: %w", err)
	}

	// Set date from first entry
	if len(entries) > 0 {
		claims.Date = entries[0].DayDate
	}

	return claims, nil
}

// narrate runs Step 2: first-person narration
func (p *Pipeline) narrate(ctx context.Context, claims ClaimSet) (string, error) {
	prompt := BuildNarrationPrompt(claims)

	response, err := p.llm.Generate(ctx, p.model, SystemPrompt, prompt)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(response), nil
}

// narrateStrict runs Step 2 with stricter constraints for retries
func (p *Pipeline) narrateStrict(ctx context.Context, claims ClaimSet, feedback string) (string, error) {
	prompt := BuildStrictNarrationPrompt(claims, feedback)

	response, err := p.llm.Generate(ctx, p.model, SystemPrompt, prompt)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(response), nil
}

// verify runs Step 3: verification
func (p *Pipeline) verify(ctx context.Context, claims ClaimSet, narrated string) (*VerificationResult, error) {
	prompt := BuildVerificationPrompt(claims, narrated)

	response, err := p.llm.Generate(ctx, p.model, SystemPrompt, prompt)
	if err != nil {
		return nil, err
	}

	result, err := parseVerificationResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse verification response: %w", err)
	}

	return result, nil
}

// parseClaimsResponse extracts ClaimSet from LLM JSON response
func parseClaimsResponse(response string) (ClaimSet, error) {
	// Try to extract JSON from response (LLM might include extra text)
	jsonStr := extractJSON(response)

	var claims ClaimSet
	if err := json.Unmarshal([]byte(jsonStr), &claims); err != nil {
		// Try parsing as just the claims array
		var wrapper struct {
			Claims []Claim `json:"claims"`
		}
		if err2 := json.Unmarshal([]byte(jsonStr), &wrapper); err2 != nil {
			return ClaimSet{}, fmt.Errorf("json parse error: %w (response: %s)", err, truncate(response, 200))
		}
		claims.Claims = wrapper.Claims
	}

	return claims, nil
}

// parseVerificationResponse extracts VerificationResult from LLM JSON response
func parseVerificationResponse(response string) (*VerificationResult, error) {
	jsonStr := extractJSON(response)

	var result VerificationResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("json parse error: %w (response: %s)", err, truncate(response, 200))
	}

	return &result, nil
}

// extractJSON attempts to find and extract JSON from a response that may contain extra text
func extractJSON(s string) string {
	// Find first { and last }
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")

	if start >= 0 && end > start {
		return s[start : end+1]
	}

	return s
}

// truncate limits string length for error messages
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
