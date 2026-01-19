package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mrwolf/brain-server/internal/config"
	"github.com/mrwolf/brain-server/internal/db"
	"github.com/mrwolf/brain-server/internal/llm"
	"github.com/mrwolf/brain-server/internal/vault"
)

func NewRouter(cfg *config.Config, database *db.DB, v *vault.Vault, llmClient *llm.Client) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(LoggingMiddleware)

	handlers := NewHandlers(cfg, database, v, llmClient)

	// Public endpoints
	r.Get("/health", handlers.Health)

	// API v1 routes (authenticated)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(AuthMiddleware(cfg))
		r.Use(JSONContentType)

		r.Post("/capture", handlers.Capture)
		r.Post("/clarify", handlers.Clarify)
		r.Get("/pending", handlers.Pending)
		r.Get("/letters", handlers.Letters)
	})

	return r
}
