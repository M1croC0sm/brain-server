package classifier

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mrwolf/brain-server/internal/llm"
	"github.com/mrwolf/brain-server/internal/models"
)

const classifierPrompt = `You are a personal note classifier. Classify the following capture into exactly one category.

Categories:
- Ideas: Creative thoughts, concepts, "what if" musings, inventions
- Projects: Actionable items with multiple steps, goals, tasks
- Financial: Money, transactions, purchases, bills (handled separately)
- Health: Body, mind, medical, fitness, wellness
- Life: Emotions, relationships, events, reflections, state of being

Capture: "%s"
Actor: %s
Timestamp: %s

Respond in JSON:
{
  "category": "Ideas|Projects|Financial|Health|Life",
  "confidence": 0.0-1.0,
  "title": "short descriptive title",
  "cleaned_text": "the capture, cleaned up and formatted",
  "tags": ["optional", "tags"]
}`

const transactionPrompt = `Parse this purchase/transaction from natural speech.

Input: "%s"
Actor: %s

Extract:
{
  "amount": number,
  "currency": "GBP|USD|EUR",
  "merchant": "store/vendor name",
  "label": "category like groceries, transport, etc",
  "notes": "any additional context",
  "confidence": 0.0-1.0
}

If you can't parse it reliably, set confidence below 0.5.`

// Classifier routes captures using LLM
type Classifier struct {
	client             *llm.Client
	confidenceThreshold float64
}

// NewClassifier creates a new classifier
func NewClassifier(client *llm.Client, threshold float64) *Classifier {
	return &Classifier{
		client:             client,
		confidenceThreshold: threshold,
	}
}

// Result is the classification result
type Result struct {
	Category    string
	Confidence  float64
	Title       string
	CleanedText string
	Tags        []string
	NeedsReview bool
	Choices     []string
	ParseError  bool // True if LLM response couldn't be parsed
}

// Classify classifies a capture text
func (c *Classifier) Classify(ctx context.Context, text, actor string, timestamp time.Time) (*Result, error) {
	prompt := fmt.Sprintf(classifierPrompt, text, actor, timestamp.Format(time.RFC3339))

	response, err := c.client.Generate(ctx, prompt, false)
	if err != nil {
		return nil, fmt.Errorf("generating classification: %w", err)
	}

	// Parse response
	var parsed models.ClassifierResult
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		// Return a parse error result instead of failing completely
		return &Result{
			ParseError:  true,
			NeedsReview: true,
			Choices:     suggestChoices(""),
		}, nil
	}

	// Validate category
	validCategory := validateCategory(parsed.Category)
	if validCategory == "" {
		// Invalid category is also a parse error
		return &Result{
			ParseError:  true,
			NeedsReview: true,
			Choices:     suggestChoices(""),
		}, nil
	}

	result := &Result{
		Category:    validCategory,
		Confidence:  parsed.Confidence,
		Title:       parsed.Title,
		CleanedText: parsed.CleanedText,
		Tags:        parsed.Tags,
	}

	// Check if confidence is below threshold
	if parsed.Confidence < c.confidenceThreshold {
		result.NeedsReview = true
		result.Choices = suggestChoices(parsed.Category)
	}

	return result, nil
}

// TransactionResult is the parsed transaction
type TransactionResult struct {
	Amount     float64
	Currency   string
	Merchant   string
	Label      string
	Notes      string
	Confidence float64
}

// ParseTransaction parses a purchase/transaction text
func (c *Classifier) ParseTransaction(ctx context.Context, text, actor string) (*TransactionResult, error) {
	prompt := fmt.Sprintf(transactionPrompt, text, actor)

	response, err := c.client.Generate(ctx, prompt, false)
	if err != nil {
		return nil, fmt.Errorf("generating transaction parse: %w", err)
	}

	var parsed models.TransactionResult
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		return nil, fmt.Errorf("parsing transaction response: %w (response: %s)", err, response)
	}

	return &TransactionResult{
		Amount:     parsed.Amount,
		Currency:   parsed.Currency,
		Merchant:   parsed.Merchant,
		Label:      parsed.Label,
		Notes:      parsed.Notes,
		Confidence: parsed.Confidence,
	}, nil
}

func validateCategory(cat string) string {
	normalized := strings.ToLower(strings.TrimSpace(cat))
	switch normalized {
	case "ideas":
		return models.CategoryIdeas
	case "projects":
		return models.CategoryProjects
	case "financial":
		return models.CategoryFinancial
	case "health":
		return models.CategoryHealth
	case "life":
		return models.CategoryLife
	default:
		return ""
	}
}

func suggestChoices(primaryChoice string) []string {
	allCategories := []string{
		models.CategoryIdeas,
		models.CategoryProjects,
		models.CategoryFinancial,
		models.CategoryHealth,
		models.CategoryLife,
	}

	// Put primary choice first, then others
	choices := []string{primaryChoice}
	for _, cat := range allCategories {
		if cat != primaryChoice {
			choices = append(choices, cat)
		}
	}

	// Limit to 4 choices
	if len(choices) > 4 {
		choices = choices[:4]
	}

	return choices
}
