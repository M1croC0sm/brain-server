# Brain Server - Development Summary

## Overview
Go-based backend server for the 2ndBrain capture system. Handles thought capture, classification, signal tracking, and automated letter generation.

## Architecture

### Core Components
- **API** (`internal/api/`) - Chi router with auth middleware, capture/clarify/letters endpoints
- **Database** (`internal/db/`) - SQLite with captures, signals, and letters tables
- **Vault** (`internal/vault/`) - Markdown file storage for letters and captures
- **LLM** (`internal/llm/`) - Ollama integration for classification and letter generation
- **Scheduler** (`internal/scheduler/`) - gocron-based job scheduling
- **Signals** (`internal/signals/`) - Signal extraction, decay, and letter generation logic

### Signal-Based Letter Generation System

The letter system uses **window evidence as primary input** (95%) with signals only for tie-breaks (5%).

**Key Design Principles:**
- No hallucination - only reference what user actually captured
- Deterministic where possible - fixed phrases for category mixes
- Theme detection from window evidence, not long-term memory
- Silence letters when no clear theme (countermove only)

**Signal Types & Decay:**
- Terms: 3-day half-life (fast decay)
- Categories: 7-day half-life (medium decay)
- Projects: 30-day half-life (slow decay)

**Daily Letter Flow:**
1. Build DayProfile from 24h window evidence
2. Detect themes from captures (project momentum, health focus, etc.)
3. Select best theme or fall back to silence/countermove
4. Generate letter via LLM with strict constraints
5. Validate output (forbidden terms, length, no greetings/signoffs)

**Weekly Letter Flow:**
1. Build WeekProfile from 7-day window evidence
2. Detect weekly themes and patterns
3. Generate reflective letter via LLM

### Scheduler Timing (4am wake-up)
- **03:45** - Signal decay job
- **03:50** - Daily letter generation
- **03:50 Sunday** - Weekly letter generation

### Test Endpoints
- `POST /api/v1/test/daily?actor=wolf` - Trigger daily letter now
- `POST /api/v1/test/weekly?actor=wolf` - Trigger weekly letter now

## Files Modified/Created

### Signal Layer (`internal/signals/`)
- `signals.go` - Signal types, decay logic, extraction
- `evidence.go` - WindowEvidence building from captures
- `themes.go` - Theme detection and selection
- `profiles.go` - DayProfile and WeekProfile builders
- `validator.go` - Letter validation and sanitization
- `signals_test.go` - 44 tests covering all signal logic

### Scheduler (`internal/scheduler/`)
- `scheduler.go` - Job scheduling, letter generation orchestration
- `letters.go` - LetterGenerator with LLM prompts

### API (`internal/api/`)
- `handlers.go` - Added LetterGenerator interface and test endpoints
- `routes.go` - Added test routes, returns handlers for wiring

### Main (`cmd/brain-server/`)
- `main.go` - Wires scheduler to handlers for test endpoints

## Configuration

Required environment variables:
```
BRAIN_PORT=8080
BRAIN_DB_PATH=./brain.db
BRAIN_VAULT_PATH=./vault
BRAIN_OLLAMA_URL=http://localhost:11434
BRAIN_OLLAMA_MODEL=qwen2.5:3b
BRAIN_OLLAMA_MODEL_HEAVY=qwen2.5:14b-instruct
BRAIN_TIMEZONE=America/Denver
BRAIN_TOKEN_WOLF=<token>
```

## Testing

Run signal layer tests:
```bash
go test ./internal/signals/... -v
```

Test letter generation manually:
```bash
curl -X POST "http://localhost:8080/api/v1/test/daily?actor=wolf" \
  -H "Authorization: Bearer $BRAIN_TOKEN_WOLF"
```

## Git Commits

1. `00a8e56` - Signal-based letter generation system
2. `6db79e4` - Comprehensive tests for signal layer
3. `c7d657d` - Test endpoints for manual letter generation
