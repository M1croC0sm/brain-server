package vault

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"
)

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

// WriteTransaction appends a transaction to the actor's ledger file
// Uses mutex to prevent race conditions on simultaneous writes
func (v *Vault) WriteTransaction(txn Transaction) (string, error) {
	v.ledgerLock.Lock()
	defer v.ledgerLock.Unlock()

	// Path: Vault/Financial/Ledger/transactions_{actor}.jsonl
	filename := fmt.Sprintf("transactions_%s.jsonl", txn.Actor)
	relPath := filepath.Join("Financial", "Ledger", filename)
	fullPath := filepath.Join(v.basePath, relPath)

	// Marshal to JSON
	line, err := json.Marshal(txn)
	if err != nil {
		return "", fmt.Errorf("marshaling transaction: %w", err)
	}

	if err := AppendLine(fullPath, line); err != nil {
		return "", fmt.Errorf("appending transaction: %w", err)
	}

	return relPath, nil
}

// NewTransaction creates a transaction with common fields populated
func NewTransaction(id, actor, deviceID, raw string, amount float64, currency, merchant, label, notes string, confidence float64) Transaction {
	return Transaction{
		ID:         id,
		TS:         time.Now().UTC().Format(time.RFC3339),
		Actor:      actor,
		Amount:     amount,
		Currency:   currency,
		Merchant:   merchant,
		Label:      label,
		Notes:      notes,
		Confidence: confidence,
		Raw:        raw,
		DeviceID:   deviceID,
	}
}
