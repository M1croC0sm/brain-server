package narrator

import (
	"context"
	"fmt"

	"github.com/mrwolf/brain-server/internal/llm"
)

// BrainServerAdapter adapts the existing brain-server LLM client to the narrator interface
type BrainServerAdapter struct {
	client *llm.Client
}

// NewBrainServerAdapter creates an adapter for the brain-server LLM client
func NewBrainServerAdapter(client *llm.Client) *BrainServerAdapter {
	return &BrainServerAdapter{client: client}
}

// Generate implements LLMClient interface
// Combines system and prompt since the brain-server client doesn't have separate system support
func (a *BrainServerAdapter) Generate(ctx context.Context, model, system, prompt string) (string, error) {
	// Combine system prompt with user prompt
	// The model parameter is ignored since brain-server client uses configured models
	fullPrompt := prompt
	if system != "" {
		fullPrompt = fmt.Sprintf("%s\n\n%s", system, prompt)
	}

	// Use heavy model (14b) for narrator tasks since they need good reasoning
	return a.client.GenerateText(ctx, fullPrompt, true)
}
