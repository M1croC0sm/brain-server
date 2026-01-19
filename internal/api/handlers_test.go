package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/mrwolf/brain-server/internal/config"
	"github.com/mrwolf/brain-server/internal/db"
	"github.com/mrwolf/brain-server/internal/llm"
	"github.com/mrwolf/brain-server/internal/vault"
)

func setupTestServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()

	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "brain-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}

	vaultPath := tmpDir + "/vault"
	dbPath := tmpDir + "/test.db"

	os.MkdirAll(vaultPath, 0755)

	cfg := &config.Config{
		Port:            "0",
		VaultPath:       vaultPath,
		DBPath:          dbPath,
		OllamaURL:       "http://localhost:11434",
		OllamaModel:     "qwen2.5:7b",
		OllamaModelHeavy: "qwen2.5:14b",
		TokenWolf:       "test_wolf_token",
		TokenWife:       "test_wife_token",
		Timezone:        "UTC",
	}

	database, err := db.Open(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("opening database: %v", err)
	}

	v := vault.NewVault(vaultPath)
	llmClient := llm.NewClient(cfg.OllamaURL, cfg.OllamaModel, cfg.OllamaModelHeavy)

	router := NewRouter(cfg, database, v, llmClient)
	server := httptest.NewServer(router)

	cleanup := func() {
		server.Close()
		database.Close()
		os.RemoveAll(tmpDir)
	}

	return server, cleanup
}

func TestHealthEndpoint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %v", body["status"])
	}
	if body["version"] != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %v", body["version"])
	}
}

func TestCaptureRequiresAuth(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	payload := `{"text":"test capture","mode":"note","device_id":"test","ts_local":"2024-01-15T09:00:00Z","version":"1"}`
	resp, err := http.Post(server.URL+"/api/v1/capture", "application/json", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatalf("POST /capture: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401 without auth, got %d", resp.StatusCode)
	}
}

func TestCaptureWithAuth(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	payload := `{"text":"test capture","mode":"note","device_id":"test","ts_local":"2024-01-15T09:00:00Z","version":"1"}`
	req, _ := http.NewRequest("POST", server.URL+"/api/v1/capture", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test_wolf_token")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /capture: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 with auth, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	if body["capture_id"] == nil {
		t.Error("expected capture_id in response")
	}
}

func TestPendingEndpoint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req, _ := http.NewRequest("GET", server.URL+"/api/v1/pending", nil)
	req.Header.Set("Authorization", "Bearer test_wolf_token")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /pending: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	if body["pending"] == nil {
		t.Error("expected pending array in response")
	}
}

func TestLettersEndpoint(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req, _ := http.NewRequest("GET", server.URL+"/api/v1/letters", nil)
	req.Header.Set("Authorization", "Bearer test_wolf_token")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /letters: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	if body["letters"] == nil {
		t.Error("expected letters array in response")
	}
}

func TestInvalidToken(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	req, _ := http.NewRequest("GET", server.URL+"/api/v1/pending", nil)
	req.Header.Set("Authorization", "Bearer invalid_token")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /pending: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401 with invalid token, got %d", resp.StatusCode)
	}
}

func TestActorResolution(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		token       string
		expectActor string
	}{
		{"test_wolf_token", "wolf"},
		{"test_wife_token", "wife"},
	}

	for _, tc := range tests {
		req, _ := http.NewRequest("GET", server.URL+"/api/v1/pending", nil)
		req.Header.Set("Authorization", "Bearer "+tc.token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET /pending: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 for token %s, got %d", tc.token, resp.StatusCode)
		}
	}
}

func TestExtractLetterBody(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty content",
			input:    "",
			expected: "",
		},
		{
			name:     "no frontmatter",
			input:    "Just some content",
			expected: "Just some content",
		},
		{
			name: "with frontmatter",
			input: `---
id: let_2024-01-15_wolf_daily
type: daily
for_date: 2024-01-15
actor: wolf
created: 2024-01-15T06:00:00Z
---

This is the letter body.
It has multiple lines.`,
			expected: `This is the letter body.
It has multiple lines.`,
		},
		{
			name: "frontmatter only",
			input: `---
id: test
---`,
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractLetterBody(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestLettersEndpointWithSince(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	// Test with RFC3339 format
	req, _ := http.NewRequest("GET", server.URL+"/api/v1/letters?since=2024-01-01T00:00:00Z", nil)
	req.Header.Set("Authorization", "Bearer test_wolf_token")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /letters: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 with RFC3339 since, got %d", resp.StatusCode)
	}

	// Test with date-only format
	req2, _ := http.NewRequest("GET", server.URL+"/api/v1/letters?since=2024-01-01", nil)
	req2.Header.Set("Authorization", "Bearer test_wolf_token")

	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("GET /letters: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 with date-only since, got %d", resp2.StatusCode)
	}

	// Test with invalid format
	req3, _ := http.NewRequest("GET", server.URL+"/api/v1/letters?since=invalid", nil)
	req3.Header.Set("Authorization", "Bearer test_wolf_token")

	resp3, err := client.Do(req3)
	if err != nil {
		t.Fatalf("GET /letters: %v", err)
	}
	defer resp3.Body.Close()

	if resp3.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 with invalid since, got %d", resp3.StatusCode)
	}
}
