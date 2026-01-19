package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mrwolf/brain-server/internal/api"
	"github.com/mrwolf/brain-server/internal/config"
	"github.com/mrwolf/brain-server/internal/db"
	"github.com/mrwolf/brain-server/internal/llm"
	"github.com/mrwolf/brain-server/internal/scheduler"
	"github.com/mrwolf/brain-server/internal/vault"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting brain-server...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Open database
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Create vault
	v := vault.NewVault(cfg.VaultPath)

	// Create LLM client
	llmClient := llm.NewClient(cfg.OllamaURL, cfg.OllamaModel, cfg.OllamaModelHeavy)

	// Validate Ollama connection at startup
	log.Println("Validating Ollama connection...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := llmClient.HealthCheck(ctx); err != nil {
		log.Printf("WARNING: Ollama health check failed: %v", err)
		log.Println("Server will start but LLM features may not work")
	} else {
		log.Printf("Ollama connected: %s (models: %s, %s)", cfg.OllamaURL, cfg.OllamaModel, cfg.OllamaModelHeavy)
	}
	cancel()

	// Create router
	router := api.NewRouter(cfg, database, v, llmClient)

	// Create and start scheduler
	actors := []string{}
	if cfg.TokenWolf != "" {
		actors = append(actors, "wolf")
	}
	if cfg.TokenWife != "" {
		actors = append(actors, "wife")
	}

	sched, err := scheduler.New(database, v, llmClient, scheduler.Config{
		Timezone: cfg.Timezone,
		Actors:   actors,
	})
	if err != nil {
		log.Fatalf("Failed to create scheduler: %v", err)
	}
	if err := sched.Start(); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	// Start server
	addr := ":" + cfg.Port
	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-done
	log.Println("Shutting down gracefully...")

	// Give ongoing requests 10 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Stopping scheduler...")
	if err := sched.Stop(); err != nil {
		log.Printf("Scheduler shutdown error: %v", err)
	}

	log.Println("Closing database...")
	if err := database.Close(); err != nil {
		log.Printf("Database close error: %v", err)
	}

	log.Println("Shutdown complete")
}
