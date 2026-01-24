package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Set required env vars
	os.Setenv("BRAIN_VAULT_PATH", "/tmp/test-vault")
	os.Setenv("BRAIN_DB_PATH", "/tmp/test.db")
	os.Setenv("BRAIN_TOKEN_WOLF", "test_token")
	defer func() {
		os.Unsetenv("BRAIN_VAULT_PATH")
		os.Unsetenv("BRAIN_DB_PATH")
		os.Unsetenv("BRAIN_TOKEN_WOLF")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.VaultPath != "/tmp/test-vault" {
		t.Errorf("expected vault path /tmp/test-vault, got %s", cfg.VaultPath)
	}

	if cfg.Port != "8080" {
		t.Errorf("expected default port 8080, got %s", cfg.Port)
	}

	if cfg.OllamaURL != "http://localhost:11434" {
		t.Errorf("expected default ollama URL, got %s", cfg.OllamaURL)
	}
}

func TestLoadConfigMissingRequired(t *testing.T) {
	// Clear env vars
	os.Unsetenv("BRAIN_VAULT_PATH")
	os.Unsetenv("BRAIN_DB_PATH")
	os.Unsetenv("BRAIN_TOKEN_WOLF")
	os.Unsetenv("BRAIN_TOKEN_WIFE")

	_, err := Load()
	if err == nil {
		t.Error("expected error when missing required config")
	}
}

func TestActorFromToken(t *testing.T) {
	cfg := &Config{
		TokenWolf: "wolf_secret",
		TokenWife: "wife_secret",
	}

	tests := []struct {
		token      string
		wantActor  string
		wantValid  bool
	}{
		{"wolf_secret", "wolf", true},
		{"wife_secret", "wife", true},
		{"invalid", "", false},
		{"", "", false},
	}

	for _, tc := range tests {
		actor, valid := cfg.ActorFromToken(tc.token)
		if actor != tc.wantActor || valid != tc.wantValid {
			t.Errorf("ActorFromToken(%q) = (%q, %v), want (%q, %v)",
				tc.token, actor, valid, tc.wantActor, tc.wantValid)
		}
	}
}

func TestConfigDefaults(t *testing.T) {
	os.Setenv("BRAIN_VAULT_PATH", "/tmp/v")
	os.Setenv("BRAIN_DB_PATH", "/tmp/d")
	os.Setenv("BRAIN_TOKEN_WOLF", "t")
	defer func() {
		os.Unsetenv("BRAIN_VAULT_PATH")
		os.Unsetenv("BRAIN_DB_PATH")
		os.Unsetenv("BRAIN_TOKEN_WOLF")
	}()

	cfg, _ := Load()

	// Check defaults
	if cfg.Port != "8080" {
		t.Errorf("default port should be 8080")
	}
	if cfg.OllamaModel != "qwen2.5:14b-instruct" {
		t.Errorf("default model should be qwen2.5:14b-instruct")
	}
	if cfg.Timezone != "Europe/London" {
		t.Errorf("default timezone should be Europe/London")
	}
}
