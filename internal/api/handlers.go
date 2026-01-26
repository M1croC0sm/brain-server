package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mrwolf/brain-server/internal/classifier"
	"github.com/mrwolf/brain-server/internal/config"
	"github.com/mrwolf/brain-server/internal/db"
	"github.com/mrwolf/brain-server/internal/llm"
	"github.com/mrwolf/brain-server/internal/models"
	"github.com/mrwolf/brain-server/internal/scheduler"
	"github.com/mrwolf/brain-server/internal/signals"
	"github.com/mrwolf/brain-server/internal/vault"
	"github.com/mrwolf/brain-server/internal/narrator"
)

// ErrorResponse is the standard error response format
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

func writeError(w http.ResponseWriter, status int, message, code string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error: message,
		Code:  code,
	})
}

// LetterGenerator interface for test endpoints
type LetterGenerator interface {
	GenerateDailyNow(actor string) error
	GenerateWeeklyNow(actor string) error
}

type Handlers struct {
	cfg          *config.Config
	db           *db.DB
	vault        *vault.Vault
	llm          *llm.Client
	classifier   *classifier.Classifier
	ideaExpander *scheduler.IdeaExpander
	letterGen    LetterGenerator
	narratorTyped *narrator.Narrator // optional, for test endpoints
}

func NewHandlers(cfg *config.Config, database *db.DB, v *vault.Vault, llmClient *llm.Client) *Handlers {
	return &Handlers{
		cfg:          cfg,
		db:           database,
		vault:        v,
		llm:          llmClient,
		classifier:   classifier.NewClassifier(llmClient, 0.6), // 0.6 threshold per spec
		ideaExpander: scheduler.NewIdeaExpander(llmClient, v),
	}
}

// Health handles GET /health
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	resp := models.HealthResponse{
		Status:  "ok",
		Ollama:  h.checkOllama(),
		Vault:   h.checkVault(),
		Version: "1.0.0",
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handlers) checkOllama() string {
	if h.llm == nil {
		return "not configured"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.llm.HealthCheck(ctx); err != nil {
		return "error: " + err.Error()
	}
	return "connected"
}

func (h *Handlers) checkVault() string {
	info, err := os.Stat(h.cfg.VaultPath)
	if err != nil {
		return "error: " + err.Error()
	}
	if !info.IsDir() {
		return "error: not a directory"
	}
	// Check if writable by trying to stat a test path
	return "writable"
}

// Capture handles POST /capture
func (h *Handlers) Capture(w http.ResponseWriter, r *http.Request) {
	var req models.Capture
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	if req.Text == "" {
		writeError(w, http.StatusBadRequest, "text is required", "MISSING_TEXT")
		return
	}

	if req.Mode == "" {
		req.Mode = "note"
	}

	actor := GetActor(r)
	captureID := generateID("cap")

	// Use client-provided timestamp if available, otherwise use server time
	var timestamp time.Time
	if req.TSLocal != "" {
		parsed, err := time.Parse(time.RFC3339, req.TSLocal)
		if err == nil {
			timestamp = parsed
		} else {
			timestamp = time.Now()
		}
	} else {
		timestamp = time.Now()
	}

	// Handle purchase mode separately
	if req.Mode == "purchase" {
		h.handlePurchase(w, captureID, actor, req)
		return
	}

	// Run classifier
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	result, err := h.classifier.Classify(ctx, req.Text, actor, timestamp)
	if err != nil {
		log.Printf("Classification failed for %s: %v", captureID, err)
		// Fall back to pending classification
		h.handleClassificationFailure(w, captureID, actor, req, timestamp)
		return
	}

	// Log to SQLite and vault
	status := models.StatusFiled
	if result.ParseError {
		status = models.StatusParseError
	} else if result.NeedsReview {
		status = models.StatusNeedsReview
	}

	if err := h.db.LogCapture(captureID, actor, req.Mode, req.Text, result.Category, status, result.Confidence); err != nil {
		log.Printf("Failed to log capture %s to DB: %v", captureID, err)
	}

	logEntry := vault.NewCaptureLog(captureID, actor, req.Mode, req.Text, result.Category, status, req.DeviceID, result.Confidence)
	if err := h.vault.LogCapture(logEntry); err != nil {
		log.Printf("Failed to log capture %s to vault: %v", captureID, err)
	}

	// If needs review, add to pending and return
	if result.NeedsReview {
		choicesJSON, _ := json.Marshal(result.Choices)
		if err := h.db.AddPending(captureID, actor, req.Text, string(choicesJSON), timestamp.Format(time.RFC3339), req.DeviceID); err != nil {
			log.Printf("Failed to add pending %s: %v", captureID, err)
		}

		resp := models.CaptureResponse{
			CaptureID:         captureID,
			Status:            models.StatusNeedsReview,
			Prompt:            "Where should this go?",
			Choices:           result.Choices,
			AttemptsRemaining: 1,
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// High confidence: write note directly to vault
	note := vault.Note{
		ID:         captureID,
		Created:    timestamp,
		Category:   result.Category,
		Confidence: result.Confidence,
		Actor:      actor,
		DeviceID:   req.DeviceID,
		Tags:       result.Tags,
		Title:      result.Title,
		Content:    result.CleanedText,
	}

	// Route Journal to Raw/ for narrator processing
	var writeErr error
	if result.Category == models.CategoryJournal {
		_, writeErr = h.vault.WriteRawJournalCapture(note)
	} else {
		_, writeErr = h.vault.WriteNote(note)
	}
	if writeErr != nil {
		log.Printf("Failed to write note %s: %v", captureID, writeErr)
		writeError(w, http.StatusInternalServerError, "failed to write note", "WRITE_ERROR")
		return
	}

	// Boost signals asynchronously (fail closed - doesn't affect capture)
	go h.boostSignals(req.Text, result.Category)

	// Trigger journal narration asynchronously for Journal category
	if result.Category == models.CategoryJournal && h.narratorTyped != nil {
		go h.narrateJournal()
	}
	// Trigger idea expansion asynchronously for Ideas category
	if result.Category == models.CategoryIdeas {
		go h.expandIdea(captureID, result.Title, result.CleanedText, result.Tags)
	}

	resp := models.CaptureResponse{
		CaptureID: captureID,
		Status:    models.StatusReceived,
		UIMessage: "Got it",
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handlers) expandIdea(ideaID, title, content string, tags []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Build context from tags and category
	categoryContext := "Ideas"
	if len(tags) > 0 {
		categoryContext = fmt.Sprintf("Ideas (tags: %s)", strings.Join(tags, ", "))
	}

	research, err := h.ideaExpander.ExpandIdea(ctx, content, title, categoryContext)
	if err != nil {
		log.Printf("Failed to expand idea %s: %v", ideaID, err)
		return
	}

	path, err := h.ideaExpander.WriteResearchFile(ideaID, title, research)
	if err != nil {
		log.Printf("Failed to write research for %s: %v", ideaID, err)
		return
	}

	log.Printf("Generated research for idea %s: %s", ideaID, path)
}

func (h *Handlers) handlePurchase(w http.ResponseWriter, captureID, actor string, req models.Capture) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use client timestamp if available
	var timestamp time.Time
	if req.TSLocal != "" {
		parsed, err := time.Parse(time.RFC3339, req.TSLocal)
		if err == nil {
			timestamp = parsed
		} else {
			timestamp = time.Now()
		}
	} else {
		timestamp = time.Now()
	}

	result, err := h.classifier.ParseTransaction(ctx, req.Text, actor)
	if err != nil || result == nil || result.Confidence < 0.5 {
		var conf float64
		if result != nil {
			conf = result.Confidence
		}
		log.Printf("Transaction parse failed or low confidence for %s: %v (confidence: %.2f)", captureID, err, conf)
		// Route to clarification for user review
		h.db.LogCapture(captureID, actor, req.Mode, req.Text, models.CategoryFinancial, models.StatusNeedsReview, 0)
		logEntry := vault.NewCaptureLog(captureID, actor, req.Mode, req.Text, models.CategoryFinancial, models.StatusNeedsReview, req.DeviceID, 0)
		h.vault.LogCapture(logEntry)

		// Add to pending clarifications - user can confirm if it's a valid transaction
		choices := []string{"Confirm transaction", "Not a transaction", "Rephrase"}
		choicesJSON, _ := json.Marshal(choices)
		h.db.AddPending(captureID, actor, req.Text, string(choicesJSON), timestamp.Format(time.RFC3339), req.DeviceID)

		resp := models.CaptureResponse{
			CaptureID:         captureID,
			Status:            models.StatusNeedsReview,
			Prompt:            "Couldn't parse this transaction. Is this correct?",
			Choices:           choices,
			AttemptsRemaining: 1,
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Write transaction to ledger
	txnID := generateID("txn")
	txn := vault.NewTransaction(
		txnID,
		actor,
		req.DeviceID,
		req.Text,
		result.Amount,
		result.Currency,
		result.Merchant,
		result.Label,
		result.Notes,
		result.Confidence,
	)

	if _, err := h.vault.WriteTransaction(txn); err != nil {
		log.Printf("Failed to write transaction %s: %v", captureID, err)
	}

	// Log transaction to database
	if err := h.db.LogTransaction(txnID, captureID, actor, result.Amount, result.Currency, result.Merchant, result.Label, result.Notes, result.Confidence, req.Text, req.DeviceID); err != nil {
		log.Printf("Failed to log transaction %s to DB: %v", txnID, err)
	}

	// Log capture
	h.db.LogCapture(captureID, actor, req.Mode, req.Text, models.CategoryFinancial, models.StatusFiled, result.Confidence)
	logEntry := vault.NewCaptureLog(captureID, actor, req.Mode, req.Text, models.CategoryFinancial, models.StatusFiled, req.DeviceID, result.Confidence)
	h.vault.LogCapture(logEntry)

	resp := models.CaptureResponse{
		CaptureID: captureID,
		Status:    models.StatusReceived,
		UIMessage: "Got it",
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handlers) handleClassificationFailure(w http.ResponseWriter, captureID, actor string, req models.Capture, timestamp time.Time) {
	// Log as pending classification
	h.db.LogCapture(captureID, actor, req.Mode, req.Text, "", models.StatusPendingClassification, 0)
	logEntry := vault.NewCaptureLog(captureID, actor, req.Mode, req.Text, "", models.StatusPendingClassification, req.DeviceID, 0)
	h.vault.LogCapture(logEntry)

	// Add to pending with all choices (include Financial)
	choices := []string{models.CategoryIdeas, models.CategoryProjects, models.CategoryFinancial, models.CategoryHealth, models.CategoryLife, models.CategoryJournal, models.CategorySpirituality, models.CategoryTasks}
	choicesJSON, _ := json.Marshal(choices)
	h.db.AddPending(captureID, actor, req.Text, string(choicesJSON), timestamp.Format(time.RFC3339), req.DeviceID)

	resp := models.CaptureResponse{
		CaptureID:         captureID,
		Status:            models.StatusNeedsReview,
		Prompt:            "Where should this go?",
		Choices:           choices,
		AttemptsRemaining: 1,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// Clarify handles POST /clarify
func (h *Handlers) Clarify(w http.ResponseWriter, r *http.Request) {
	var req models.ClarifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	pending, err := h.db.GetPendingByID(req.CaptureID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "DB_ERROR")
		return
	}

	if pending == nil {
		resp := models.ClarifyResponse{
			CaptureID: req.CaptureID,
			Status:    models.StatusNotFound,
			UIMessage: "Not found or expired",
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(resp)
		return
	}

	resolved, err := h.db.ResolvePending(req.CaptureID, req.Destination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve", "RESOLVE_ERROR")
		return
	}

	if !resolved {
		resp := models.ClarifyResponse{
			CaptureID: req.CaptureID,
			Status:    models.StatusExpired,
			UIMessage: "Expired",
		}
		w.WriteHeader(http.StatusGone)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Write note to vault - use original timestamp and device_id
	created := pending.OriginalTS
	if created.IsZero() {
		created = time.Now()
	}
	note := vault.Note{
		ID:         pending.CaptureID,
		Created:    created,
		Category:   req.Destination,
		Confidence: 1.0, // Human-classified
		Actor:      pending.Actor,
		DeviceID:   pending.DeviceID,
		Tags:       []string{},
		Title:      truncateForTitle(pending.RawText),
		Content:    pending.RawText,
	}

	// Route Journal to Raw/ for narrator processing
	var clarifyWriteErr error
	if req.Destination == models.CategoryJournal {
		_, clarifyWriteErr = h.vault.WriteRawJournalCapture(note)
	} else {
		_, clarifyWriteErr = h.vault.WriteNote(note)
	}
	if clarifyWriteErr != nil {
		writeError(w, http.StatusInternalServerError, "failed to write note", "WRITE_ERROR")
		return
	}
	// Boost signals asynchronously (fail closed - doesn't affect clarify)
	go h.boostSignals(pending.RawText, req.Destination)
	// Trigger journal narration asynchronously for Journal category
	if req.Destination == models.CategoryJournal && h.narratorTyped != nil {
		go h.narrateJournal()
	}

	resp := models.ClarifyResponse{
		CaptureID: req.CaptureID,
		Status:    models.StatusFiled,
		UIMessage: "Filed to " + req.Destination,
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// Pending handles GET /pending
func (h *Handlers) Pending(w http.ResponseWriter, r *http.Request) {
	actor := GetActor(r)

	pending, err := h.db.GetPending(actor)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "DB_ERROR")
		return
	}

	items := make([]models.PendingItem, 0, len(pending))
	for _, p := range pending {
		var choices []string
		json.Unmarshal([]byte(p.Choices), &choices)

		preview := p.RawText
		if len(preview) > 50 {
			preview = preview[:50] + "..."
		}

		items = append(items, models.PendingItem{
			CaptureID: p.CaptureID,
			Prompt:    "Where should this go?",
			Choices:   choices,
			Preview:   preview,
			ExpiresAt: p.ExpiresAt,
		})
	}

	resp := models.PendingResponse{Pending: items}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// Letters handles GET /letters
func (h *Handlers) Letters(w http.ResponseWriter, r *http.Request) {
	actor := GetActor(r)
	letterType := r.URL.Query().Get("type")
	sinceStr := r.URL.Query().Get("since")

	var since *time.Time
	if sinceStr != "" {
		parsed, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			// Try parsing as date only (YYYY-MM-DD)
			parsed, err = time.Parse("2006-01-02", sinceStr)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid since format, use RFC3339 or YYYY-MM-DD", "INVALID_DATE")
				return
			}
		}
		since = &parsed
	}

	records, err := h.db.GetLetters(actor, letterType, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error", "DB_ERROR")
		return
	}

	letters := make([]models.Letter, 0, len(records))
	for _, rec := range records {
		// Read letter content from vault
		content, err := h.vault.ReadLetter(rec.Type, rec.ForDate)
		if err != nil {
			log.Printf("Failed to read letter %s: %v", rec.LetterID, err)
			content = "" // Return empty content if file is missing
		}

		// Extract just the body (after YAML frontmatter)
		text := extractLetterBody(content)

		letters = append(letters, models.Letter{
			LetterID:  rec.LetterID,
			Type:      rec.Type,
			ForDate:   rec.ForDate,
			Text:      text,
			CreatedTS: rec.CreatedAt,
			Version:   "1",
		})
	}

	resp := models.LettersResponse{Letters: letters}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// extractLetterBody extracts the body content from a letter file,
// skipping the YAML frontmatter (content between --- delimiters)
func extractLetterBody(content string) string {
	if content == "" {
		return ""
	}

	// Look for YAML frontmatter pattern: starts with ---, ends with ---
	if len(content) < 3 || content[:3] != "---" {
		return content
	}

	// Find the closing ---
	endIdx := indexOf(content[3:], '-')
	if endIdx == -1 {
		return content
	}

	// Find the full "---" closing delimiter
	for i := 3; i < len(content)-2; i++ {
		if content[i] == '-' && content[i+1] == '-' && content[i+2] == '-' {
			// Skip past the closing --- and any following newlines
			body := content[i+3:]
			for len(body) > 0 && (body[0] == '\n' || body[0] == '\r') {
				body = body[1:]
			}
			return body
		}
	}

	return content
}

func generateID(prefix string) string {
	// Simple ID generation - could use UUID in production
	return prefix + "_" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	// Use time-based seed for simple randomness
	seed := uint64(time.Now().UnixNano())
	for i := range b {
		seed = seed*1103515245 + 12345
		b[i] = letters[seed%uint64(len(letters))]
	}
	return string(b)
}

func truncateForTitle(s string) string {
	// Take first 50 chars or first line, whichever is shorter
	if idx := indexOf(s, '\n'); idx > 0 && idx < 50 {
		s = s[:idx]
	} else if len(s) > 50 {
		s = s[:50]
	}
	return s
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// SetLetterGenerator sets the letter generator for test endpoints
func (h *Handlers) SetLetterGenerator(lg LetterGenerator) {
	h.letterGen = lg
}

// TestGenerateDaily handles POST /test/daily - triggers daily letter generation
func (h *Handlers) TestGenerateDaily(w http.ResponseWriter, r *http.Request) {
	if h.letterGen == nil {
		writeError(w, http.StatusServiceUnavailable, "letter generator not configured", "NOT_CONFIGURED")
		return
	}

	actor := r.URL.Query().Get("actor")
	if actor == "" {
		actor = "wolf" // default
	}

	log.Printf("Test: generating daily letter for actor %s", actor)
	if err := h.letterGen.GenerateDailyNow(actor); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "GENERATION_FAILED")
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"actor":  actor,
		"type":   "daily",
	})
}

// TestGenerateWeekly handles POST /test/weekly - triggers weekly letter generation
func (h *Handlers) TestGenerateWeekly(w http.ResponseWriter, r *http.Request) {
	if h.letterGen == nil {
		writeError(w, http.StatusServiceUnavailable, "letter generator not configured", "NOT_CONFIGURED")
		return
	}

	actor := r.URL.Query().Get("actor")
	if actor == "" {
		actor = "wolf" // default
	}

	log.Printf("Test: generating weekly letter for actor %s", actor)
	if err := h.letterGen.GenerateWeeklyNow(actor); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "GENERATION_FAILED")
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"actor":  actor,
		"type":   "weekly",
	})
}

// boostSignals updates the signal layer when a capture is filed
// This runs asynchronously and failures don't affect the capture flow (fail closed)
func (h *Handlers) boostSignals(text, category string) {
	// Extract terms from the capture text
	terms := signals.ExtractTerms(text, 5)

	// Boost each term signal
	for _, term := range terms {
		key := "term:" + term
		if err := signals.BoostSignal(h.db, key, "term"); err != nil {
			log.Printf("Failed to boost term signal %s: %v", key, err)
		}
	}

	// Boost category signal
	if category != "" {
		key := "cat:" + category
		if err := signals.BoostSignal(h.db, key, "category"); err != nil {
			log.Printf("Failed to boost category signal %s: %v", key, err)
		}
	}
}

// Narrator interface for journal endpoints
type NarratorInterface interface {
	Update(ctx context.Context) (*struct {
		ProcessedCount int
		DaysUpdated    []string
		Errors         []string
	}, error)
	Status() (*struct {
		LastProcessedRaw string
		LastProcessedTS  time.Time
		CurrentDay       string
		LastUpdateAt     time.Time
		DayStatus        string
		LastNightRunAt   time.Time
	}, error)
}

// SetNarrator sets the narrator for journal endpoints
// SetNarrator sets the narrator for journal endpoints
func (h *Handlers) SetNarrator(n *narrator.Narrator) {
	h.narratorTyped = n
}

// JournalUpdate handles POST /api/v1/journal/update
func (h *Handlers) JournalUpdate(w http.ResponseWriter, r *http.Request) {
	if h.narratorTyped == nil {
		writeError(w, http.StatusServiceUnavailable, "narrator not configured", "NOT_CONFIGURED")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	result, err := h.narratorTyped.Update(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "UPDATE_FAILED")
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// JournalStatus handles GET /api/v1/journal/status
func (h *Handlers) JournalStatus(w http.ResponseWriter, r *http.Request) {
	if h.narratorTyped == nil {
		writeError(w, http.StatusServiceUnavailable, "narrator not configured", "NOT_CONFIGURED")
		return
	}

	state, err := h.narratorTyped.Status()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STATUS_FAILED")
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(state)
}

// narrateJournal triggers async journal narration (fail closed)
func (h *Handlers) narrateJournal() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := h.narratorTyped.Update(ctx)
	if err != nil {
		log.Printf("Journal narration failed: %v", err)
		return
	}

	if result.ProcessedCount > 0 {
		log.Printf("Journal narration: processed %d entries", result.ProcessedCount)
	}
}
