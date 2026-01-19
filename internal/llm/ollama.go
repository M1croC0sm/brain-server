package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps the Ollama API
type Client struct {
	baseURL    string
	model      string
	modelHeavy string
	httpClient *http.Client
}

// NewClient creates a new Ollama client
func NewClient(baseURL, model, modelHeavy string) *Client {
	return &Client{
		baseURL:    baseURL,
		model:      model,
		modelHeavy: modelHeavy,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// GenerateRequest is the request body for /api/generate
type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Format string `json:"format,omitempty"` // "json" for JSON output
}

// GenerateResponse is the response from /api/generate
type GenerateResponse struct {
	Model     string `json:"model"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
	CreatedAt string `json:"created_at"`
}

// Generate sends a prompt to Ollama and returns the response
// Includes retry logic with exponential backoff (up to 3 attempts)
func (c *Client) Generate(ctx context.Context, prompt string, useHeavy bool) (string, error) {
	model := c.model
	if useHeavy {
		model = c.modelHeavy
	}

	req := GenerateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
		Format: "json",
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}

		response, err := c.doGenerate(ctx, body)
		if err == nil {
			return response, nil
		}
		lastErr = err
	}

	return "", fmt.Errorf("after 3 attempts: %w", lastErr)
}

func (c *Client) doGenerate(ctx context.Context, body []byte) (string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var genResp GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	return genResp.Response, nil
}

// GenerateText sends a prompt without JSON format requirement
// Includes retry logic with exponential backoff (up to 3 attempts)
func (c *Client) GenerateText(ctx context.Context, prompt string, useHeavy bool) (string, error) {
	model := c.model
	if useHeavy {
		model = c.modelHeavy
	}

	req := GenerateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}

		response, err := c.doGenerate(ctx, body)
		if err == nil {
			return response, nil
		}
		lastErr = err
	}

	return "", fmt.Errorf("after 3 attempts: %w", lastErr)
}

// HealthCheck checks if Ollama is reachable
func (c *Client) HealthCheck(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("connecting to ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	return nil
}
