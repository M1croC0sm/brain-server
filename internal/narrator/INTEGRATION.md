# Integration Guide for Journal Narrator

This document describes how to integrate the narrator package into brain-server.

## 1. LLM Client Adapter

Create an adapter that implements the `narrator.LLMClient` interface using your existing LLM client.

```go
// internal/narrator/llm_adapter.go (or in your llm package)

package narrator

import (
    "context"
    "your-project/internal/llm"  // adjust import path
)

// LLMAdapter adapts your existing LLM client to the narrator interface
type LLMAdapter struct {
    client *llm.Client  // your existing LLM client type
}

func NewLLMAdapter(client *llm.Client) *LLMAdapter {
    return &LLMAdapter{client: client}
}

func (a *LLMAdapter) Generate(ctx context.Context, model, system, prompt string) (string, error) {
    // Adapt to your LLM client's API
    // Example:
    return a.client.Generate(ctx, llm.GenerateRequest{
        Model:  model,
        System: system,
        Prompt: prompt,
    })
}
```

## 2. API Handlers

Add these handlers to `internal/api/handlers.go`:

```go
// JournalUpdate triggers narration of pending raw journal entries
func (h *Handler) JournalUpdate(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    result, err := h.narrator.Update(ctx)
    if err != nil {
        h.errorResponse(w, http.StatusInternalServerError, err.Error())
        return
    }

    h.jsonResponse(w, http.StatusOK, map[string]interface{}{
        "processed": result.ProcessedCount,
        "days":      result.DaysUpdated,
        "errors":    result.Errors,
    })
}

// JournalStatus returns the current narrator state
func (h *Handler) JournalStatus(w http.ResponseWriter, r *http.Request) {
    state, err := h.narrator.Status()
    if err != nil {
        h.errorResponse(w, http.StatusInternalServerError, err.Error())
        return
    }

    h.jsonResponse(w, http.StatusOK, state)
}
```

## 3. Routes

Add these routes to `internal/api/routes.go`:

```go
// In your route setup function:
r.Post("/api/v1/journal/update", h.JournalUpdate)
r.Get("/api/v1/journal/status", h.JournalStatus)
```

## 4. Capture Flow Modification

Modify `internal/vault/notes.go` (or wherever captures are handled) to route Journal captures to `Journal/Raw/`:

```go
func (v *Vault) SaveCapture(category, content string, metadata map[string]string) error {
    // Special handling for Journal category
    if category == "Journal" {
        return v.saveRawJournalCapture(content, metadata)
    }

    // Existing logic for other categories...
}

func (v *Vault) saveRawJournalCapture(content string, metadata map[string]string) error {
    now := time.Now()

    // Generate filename: YYYY-MM-DD_HHMMSS_cap_xxxxx.md
    captureID := fmt.Sprintf("cap_%s", generateShortID())
    filename := fmt.Sprintf("%s_%s.md",
        now.Format("2006-01-02_150405"),
        captureID,
    )

    // Build frontmatter
    frontmatter := fmt.Sprintf(`---
id: %s
created: %s
actor: %s
device: %s
---

`,
        captureID,
        now.Format(time.RFC3339),
        metadata["actor"],
        metadata["device"],
    )

    // Write to Journal/Raw/
    path := filepath.Join(v.BasePath, "Journal", "Raw", filename)
    return os.WriteFile(path, []byte(frontmatter+content), 0644)
}
```

## 5. Scheduler Integration

Add to `internal/scheduler/scheduler.go`:

```go
type Scheduler struct {
    // ... existing fields
    narrator *narrator.Narrator
}

func NewScheduler(/* existing params */, narr *narrator.Narrator) *Scheduler {
    s := &Scheduler{
        // ... existing initialization
        narrator: narr,
    }
    return s
}

func (s *Scheduler) setupJobs() error {
    // ... existing jobs

    // Nightly journal close at 22:00
    _, err := s.scheduler.NewJob(
        gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(22, 0, 0))),
        gocron.NewTask(s.narrateJournal),
        gocron.WithName("journal-narrator"),
    )
    if err != nil {
        return fmt.Errorf("failed to schedule journal narrator: %w", err)
    }

    return nil
}

func (s *Scheduler) narrateJournal() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    if err := s.narrator.NightlyClose(ctx); err != nil {
        log.Printf("scheduler: journal narrator failed: %v", err)
    }
}
```

## 6. Main Initialization

In `cmd/brain-server/main.go`:

```go
import "your-project/internal/narrator"

func main() {
    // ... existing setup

    // Create LLM adapter
    llmAdapter := narrator.NewLLMAdapter(llmClient)

    // Create narrator
    narratorConfig := narrator.DefaultConfig(vaultPath)
    narr, err := narrator.New(llmAdapter, narratorConfig)
    if err != nil {
        log.Fatalf("failed to create narrator: %v", err)
    }

    // Pass to handler
    handler := api.NewHandler(/* existing params */, narr)

    // Pass to scheduler
    scheduler := scheduler.NewScheduler(/* existing params */, narr)

    // ... rest of setup
}
```

## Directory Structure After Integration

```
Vault/Journal/
├── Raw/                    # New: raw captures go here
│   └── 2026-01-24_153012_cap_abc123.md
├── Daily/                  # New: narrated daily pages
│   └── 2026-01-24.md
├── _meta/                  # New: processing state
│   ├── journal_state.json
│   └── journal_map.jsonl
└── *.md                    # Existing: old journal files (untouched)
```

## Testing

```bash
# Create test raw file
mkdir -p /path/to/Vault/Journal/Raw
cat > /path/to/Vault/Journal/Raw/2026-01-24_153012_cap_test1.md << 'EOF'
---
id: cap_test1
created: 2026-01-24T15:30:12Z
actor: wolf
device: phone
---

Had a great meeting with the team today. We discussed the new feature roadmap.
Decided to prioritize mobile improvements first.
EOF

# Trigger update via API
curl -X POST http://localhost:8080/api/v1/journal/update \
  -H "Authorization: Bearer your-token"

# Check status
curl http://localhost:8080/api/v1/journal/status \
  -H "Authorization: Bearer your-token"

# Verify output
cat /path/to/Vault/Journal/Daily/2026-01-24.md
```
