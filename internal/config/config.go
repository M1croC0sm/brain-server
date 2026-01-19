package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port            string
	VaultPath       string
	DBPath          string
	OllamaURL       string
	OllamaModel     string
	OllamaModelHeavy string
	TokenWolf       string
	TokenWife       string
	Timezone        string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:            getEnv("BRAIN_PORT", "8080"),
		VaultPath:       getEnv("BRAIN_VAULT_PATH", ""),
		DBPath:          getEnv("BRAIN_DB_PATH", ""),
		OllamaURL:       getEnv("BRAIN_OLLAMA_URL", "http://localhost:11434"),
		OllamaModel:     getEnv("BRAIN_OLLAMA_MODEL", "qwen2.5:7b"),
		OllamaModelHeavy: getEnv("BRAIN_OLLAMA_MODEL_HEAVY", "qwen2.5:14b"),
		TokenWolf:       getEnv("BRAIN_TOKEN_WOLF", ""),
		TokenWife:       getEnv("BRAIN_TOKEN_WIFE", ""),
		Timezone:        getEnv("BRAIN_TIMEZONE", "Europe/London"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.VaultPath == "" {
		return fmt.Errorf("BRAIN_VAULT_PATH is required")
	}
	if c.DBPath == "" {
		return fmt.Errorf("BRAIN_DB_PATH is required")
	}
	if c.TokenWolf == "" && c.TokenWife == "" {
		return fmt.Errorf("at least one of BRAIN_TOKEN_WOLF or BRAIN_TOKEN_WIFE is required")
	}
	return nil
}

func (c *Config) ActorFromToken(token string) (string, bool) {
	switch token {
	case c.TokenWolf:
		if c.TokenWolf != "" {
			return "wolf", true
		}
	case c.TokenWife:
		if c.TokenWife != "" {
			return "wife", true
		}
	}
	return "", false
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
