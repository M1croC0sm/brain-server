package scheduler

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mrwolf/brain-server/internal/llm"
	"github.com/mrwolf/brain-server/internal/vault"
)

const ideaExpanderPrompt = `Expand on this idea with questions and angles to explore.

Idea: "%s"
Category context: %s

Generate:
- 3-5 probing questions about this idea
- 2-3 potential applications or directions
- 1-2 potential challenges or considerations

Do NOT search the web. Use only reasoning.
Output as markdown with headers.`

// IdeaExpander generates research files for new ideas
type IdeaExpander struct {
	llm   *llm.Client
	vault *vault.Vault
}

// NewIdeaExpander creates a new idea expander
func NewIdeaExpander(client *llm.Client, v *vault.Vault) *IdeaExpander {
	return &IdeaExpander{
		llm:   client,
		vault: v,
	}
}

// ExpandIdea generates research content for an idea
func (e *IdeaExpander) ExpandIdea(ctx context.Context, ideaText, title, category string) (string, error) {
	prompt := fmt.Sprintf(ideaExpanderPrompt, ideaText, category)

	response, err := e.llm.GenerateText(ctx, prompt, true) // Use heavy model
	if err != nil {
		return "", fmt.Errorf("generating idea expansion: %w", err)
	}

	return response, nil
}

// WriteResearchFile writes the expanded research to the vault
func (e *IdeaExpander) WriteResearchFile(ideaID, title, content string) (string, error) {
	// Path: Research/Ideas/{date}-{title}-research.md
	dateStr := time.Now().Format("2006-01-02")
	slug := slugifyTitle(title)
	filename := fmt.Sprintf("%s-%s-research.md", dateStr, slug)
	relPath := filepath.Join("Research", "Ideas", filename)

	fullContent := fmt.Sprintf(`---
id: %s_research
source_idea: %s
created: %s
---

# Research: %s

%s
`, ideaID, ideaID, time.Now().UTC().Format(time.RFC3339), title, content)

	if err := vault.WriteFileAtomic(filepath.Join(e.vault.BasePath(), relPath), []byte(fullContent)); err != nil {
		return "", err
	}

	return relPath, nil
}

func slugifyTitle(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	// Keep only alphanumeric and hyphens
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}

	// Collapse multiple hyphens
	out := result.String()
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	out = strings.Trim(out, "-")

	if len(out) > 50 {
		out = out[:50]
		out = strings.TrimRight(out, "-")
	}

	if out == "" {
		out = "idea"
	}

	return out
}
