package narrator

import (
	"fmt"
	"strings"
)

// Prompt templates for the 3-step LLM pipeline

const claimExtractionPrompt = `You are a precise fact extractor. Your job is to extract ONLY explicit claims from journal text.

RULES:
1. Extract only facts that are explicitly stated in the text
2. Do NOT infer emotions, motivations, or causes unless explicitly stated
3. Do NOT add any information not present in the source
4. Each claim must have a supporting quote from the source text
5. Keep claims factual and objective

INPUT TEXT:
%s

OUTPUT FORMAT (JSON):
{
  "claims": [
    {"fact": "The explicit fact here", "quote": "The exact supporting quote from text"},
    ...
  ]
}

Extract all explicit claims now:`

const narrationPrompt = `You are a skilled journal narrator. Transform these factual claims into engaging first-person journal paragraphs.

RULES:
1. Write in first-person voice (I, me, my)
2. Be conversational and natural, as if writing in a personal journal
3. Do NOT add any facts not present in the claims
4. Do NOT include specific dates or times
5. Do NOT invent details, emotions, or context not supported by claims
6. Connect related claims into flowing paragraphs
7. Keep it concise: 1-4 paragraphs maximum
8. Maintain the emotional tone implied by the facts without embellishment

CLAIMS TO NARRATE:
%s

Write the journal entry now:`

const strictNarrationPrompt = `You are a precise journal narrator. Transform these claims into first-person paragraphs with STRICT adherence to the source material.

CRITICAL RULES:
1. Every sentence must be directly supported by a claim
2. Use ONLY the facts provided - add nothing
3. Write in first-person (I, me, my)
4. No dates, times, or temporal markers
5. No invented emotions or reactions
6. Keep it factual and brief

CLAIMS:
%s

PREVIOUS ATTEMPT FAILED VERIFICATION. Issues found:
%s

Write a more faithful version now:`

const verificationPrompt = `You are a fact-checker. Compare the narrated text against the source claims and identify any unsupported statements.

CLAIMS (source of truth):
%s

NARRATED TEXT (to verify):
%s

TASK:
1. Check each sentence in the narrated text
2. Verify it is supported by one or more claims
3. Flag any sentences that add information not in the claims

OUTPUT FORMAT (JSON):
{
  "passed": true/false,
  "unsupported_claims": ["sentence 1 that has no support", "sentence 2...", ...],
  "feedback": "Brief explanation of issues if any"
}

Verify now:`

// BuildClaimExtractionPrompt creates the prompt for step 1
func BuildClaimExtractionPrompt(entries []RawEntry) string {
	var texts []string
	for i, entry := range entries {
		texts = append(texts, fmt.Sprintf("--- Entry %d ---\n%s", i+1, entry.Content))
	}
	combinedText := strings.Join(texts, "\n\n")
	return fmt.Sprintf(claimExtractionPrompt, combinedText)
}

// BuildNarrationPrompt creates the prompt for step 2
func BuildNarrationPrompt(claims ClaimSet) string {
	var claimTexts []string
	for i, claim := range claims.Claims {
		claimTexts = append(claimTexts, fmt.Sprintf("%d. %s", i+1, claim.Fact))
	}
	return fmt.Sprintf(narrationPrompt, strings.Join(claimTexts, "\n"))
}

// BuildStrictNarrationPrompt creates a stricter prompt for retry attempts
func BuildStrictNarrationPrompt(claims ClaimSet, feedback string) string {
	var claimTexts []string
	for i, claim := range claims.Claims {
		claimTexts = append(claimTexts, fmt.Sprintf("%d. %s", i+1, claim.Fact))
	}
	return fmt.Sprintf(strictNarrationPrompt, strings.Join(claimTexts, "\n"), feedback)
}

// BuildVerificationPrompt creates the prompt for step 3
func BuildVerificationPrompt(claims ClaimSet, narratedText string) string {
	var claimTexts []string
	for i, claim := range claims.Claims {
		claimTexts = append(claimTexts, fmt.Sprintf("%d. %s (quote: \"%s\")", i+1, claim.Fact, claim.Quote))
	}
	return fmt.Sprintf(verificationPrompt, strings.Join(claimTexts, "\n"), narratedText)
}

// SystemPrompt provides context for the LLM
const SystemPrompt = `You are a journal assistant helping to narrate personal journal entries.
You value accuracy and faithfulness to the source material above all else.
You never invent details or embellish facts.
You write in a warm but precise first-person voice.`
