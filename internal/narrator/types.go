package narrator

import "time"

// JournalState tracks the processing state for journal narration
type JournalState struct {
	LastProcessedRaw string    `json:"last_processed_raw"`
	LastProcessedTS  time.Time `json:"last_processed_ts"`
	CurrentDay       string    `json:"current_day"`       // YYYY-MM-DD
	LastUpdateAt     time.Time `json:"last_update_at"`
	DayStatus        string    `json:"day_status"`        // "open" or "closed"
	LastNightRunAt   time.Time `json:"last_night_run_at"`
}

// RawEntry represents a single raw journal capture
type RawEntry struct {
	Filename string    // Original filename
	ID       string    // Capture ID from frontmatter
	Created  time.Time // Creation timestamp
	Actor    string    // Who created it (e.g., "wolf")
	Device   string    // Device used (e.g., "phone")
	Content  string    // The actual journal text
	DayDate  string    // YYYY-MM-DD extracted from filename
}

// NarrationMapping is the audit trail entry for each narration batch
type NarrationMapping struct {
	Day            string   `json:"day"`
	GeneratedAt    string   `json:"generated_at"`
	RawFiles       []string `json:"raw_files"`
	Model          string   `json:"model"`
	VerifierPassed bool     `json:"verifier_passed"`
}

// Claim represents an extracted fact from raw journal text
type Claim struct {
	Fact  string `json:"fact"`
	Quote string `json:"quote"` // Supporting quote from source
}

// ClaimSet holds extracted claims for a batch of entries
type ClaimSet struct {
	Claims []Claim `json:"claims"`
	Date   string  `json:"date"`
}

// VerificationResult holds the output of the verification step
type VerificationResult struct {
	Passed             bool     `json:"passed"`
	UnsupportedClaims  []string `json:"unsupported_claims,omitempty"`
	Feedback           string   `json:"feedback,omitempty"`
}

// NarrationConfig holds configuration for the narrator
type NarrationConfig struct {
	VaultPath    string         // Path to the vault root
	JournalPath  string         // Relative path to Journal folder within vault
	Timezone     *time.Location // Local timezone for day boundaries
	Model        string         // LLM model to use (e.g., "qwen2.5:14b")
	MaxRetries   int            // Max verification retries before giving up
	BatchSize    int            // Max raw entries to process in one batch
}

// DefaultConfig returns sensible defaults
func DefaultConfig(vaultPath string) NarrationConfig {
	loc, _ := time.LoadLocation("Local")
	return NarrationConfig{
		VaultPath:   vaultPath,
		JournalPath: "Journal",
		Timezone:    loc,
		Model:       "qwen2.5:14b",
		MaxRetries:  2,
		BatchSize:   10,
	}
}
