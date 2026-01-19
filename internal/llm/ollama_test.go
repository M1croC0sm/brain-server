package llm

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:11434", "llama2", "llama2:70b")

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	if client.baseURL != "http://localhost:11434" {
		t.Errorf("baseURL = %q, want %q", client.baseURL, "http://localhost:11434")
	}

	if client.model != "llama2" {
		t.Errorf("model = %q, want %q", client.model, "llama2")
	}

	if client.modelHeavy != "llama2:70b" {
		t.Errorf("modelHeavy = %q, want %q", client.modelHeavy, "llama2:70b")
	}

	if client.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestGenerateRequest(t *testing.T) {
	req := GenerateRequest{
		Model:  "llama2",
		Prompt: "test prompt",
		Stream: false,
		Format: "json",
	}

	if req.Model != "llama2" {
		t.Errorf("Model = %q, want %q", req.Model, "llama2")
	}

	if req.Prompt != "test prompt" {
		t.Errorf("Prompt = %q, want %q", req.Prompt, "test prompt")
	}

	if req.Stream != false {
		t.Error("Stream should be false")
	}

	if req.Format != "json" {
		t.Errorf("Format = %q, want %q", req.Format, "json")
	}
}

func TestGenerateResponse(t *testing.T) {
	resp := GenerateResponse{
		Model:     "llama2",
		Response:  "test response",
		Done:      true,
		CreatedAt: "2024-01-01T00:00:00Z",
	}

	if resp.Model != "llama2" {
		t.Errorf("Model = %q, want %q", resp.Model, "llama2")
	}

	if resp.Response != "test response" {
		t.Errorf("Response = %q, want %q", resp.Response, "test response")
	}

	if !resp.Done {
		t.Error("Done should be true")
	}
}
