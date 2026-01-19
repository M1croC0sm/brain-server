package models

import "time"

// Capture represents an incoming capture from the client
type Capture struct {
	Text     string `json:"text"`
	TSLocal  string `json:"ts_local"`
	DeviceID string `json:"device_id"`
	Mode     string `json:"mode"` // "note" or "purchase"
	Version  string `json:"version"`
}

// CaptureResponse is returned after receiving a capture
type CaptureResponse struct {
	CaptureID string   `json:"capture_id"`
	Status    string   `json:"status"` // "received", "needs_review"
	UIMessage string   `json:"ui_message,omitempty"`
	Prompt    string   `json:"prompt,omitempty"`
	Choices   []string `json:"choices,omitempty"`
	AttemptsRemaining int `json:"attempts_remaining,omitempty"`
}

// ClarifyRequest is sent to resolve a pending clarification
type ClarifyRequest struct {
	CaptureID   string `json:"capture_id"`
	Destination string `json:"destination"`
}

// ClarifyResponse is returned after clarification
type ClarifyResponse struct {
	CaptureID string `json:"capture_id"`
	Status    string `json:"status"` // "filed", "expired", "not_found"
	UIMessage string `json:"ui_message"`
}

// PendingItem represents a capture awaiting clarification
type PendingItem struct {
	CaptureID string    `json:"capture_id"`
	Prompt    string    `json:"prompt"`
	Choices   []string  `json:"choices"`
	Preview   string    `json:"preview"`
	ExpiresAt time.Time `json:"expires_at"`
}

// PendingResponse is returned by the pending endpoint
type PendingResponse struct {
	Pending []PendingItem `json:"pending"`
}

// Letter represents a daily or weekly letter
type Letter struct {
	LetterID  string `json:"letter_id"`
	Type      string `json:"type"` // "daily", "weekly"
	ForDate   string `json:"for_date"`
	Text      string `json:"text"`
	CreatedTS string `json:"created_ts"`
	Version   string `json:"version"`
}

// LettersResponse is returned by the letters endpoint
type LettersResponse struct {
	Letters []Letter `json:"letters"`
}

// HealthResponse is returned by the health endpoint
type HealthResponse struct {
	Status  string `json:"status"`
	Ollama  string `json:"ollama"`
	Vault   string `json:"vault"`
	Version string `json:"version"`
}

// CaptureLog represents a logged capture
type CaptureLog struct {
	ID         string  `json:"id"`
	TS         string  `json:"ts"`
	Actor      string  `json:"actor"`
	Mode       string  `json:"mode"`
	Raw        string  `json:"raw"`
	RoutedTo   string  `json:"routed_to,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	Status     string  `json:"status"`
	DeviceID   string  `json:"device"`
}

// Transaction represents a financial transaction
type Transaction struct {
	ID         string  `json:"id"`
	TS         string  `json:"ts"`
	Actor      string  `json:"actor"`
	Amount     float64 `json:"amount"`
	Currency   string  `json:"currency"`
	Merchant   string  `json:"merchant"`
	Label      string  `json:"label"`
	Notes      string  `json:"notes,omitempty"`
	Confidence float64 `json:"confidence"`
	Raw        string  `json:"raw"`
	DeviceID   string  `json:"device"`
}

// ClassifierResult is the parsed response from the LLM classifier
type ClassifierResult struct {
	Category    string   `json:"category"`
	Confidence  float64  `json:"confidence"`
	Title       string   `json:"title"`
	CleanedText string   `json:"cleaned_text"`
	Tags        []string `json:"tags"`
}

// TransactionResult is the parsed response from the transaction parser
type TransactionResult struct {
	Amount     float64 `json:"amount"`
	Currency   string  `json:"currency"`
	Merchant   string  `json:"merchant"`
	Label      string  `json:"label"`
	Notes      string  `json:"notes"`
	Confidence float64 `json:"confidence"`
}

// Category constants
const (
	CategoryIdeas     = "Ideas"
	CategoryProjects  = "Projects"
	CategoryFinancial = "Financial"
	CategoryHealth    = "Health"
	CategoryLife      = "Life"
)

// Status constants
const (
	StatusReceived             = "received"
	StatusNeedsReview          = "needs_review"
	StatusFiled                = "filed"
	StatusExpired              = "expired"
	StatusNotFound             = "not_found"
	StatusPendingClassification = "pending_classification"
	StatusParseError           = "parse_error"
)
