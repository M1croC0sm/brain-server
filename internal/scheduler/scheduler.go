package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/mrwolf/brain-server/internal/db"
	"github.com/mrwolf/brain-server/internal/llm"
	"github.com/mrwolf/brain-server/internal/models"
	"github.com/mrwolf/brain-server/internal/signals"
	"github.com/mrwolf/brain-server/internal/vault"
)

// Scheduler manages scheduled jobs
type Scheduler struct {
	scheduler gocron.Scheduler
	db        *db.DB
	vault     *vault.Vault
	llm       *llm.Client
	letterGen *LetterGenerator
	timezone  *time.Location
	actors    []string
}

// Config holds scheduler configuration
type Config struct {
	Timezone string
	Actors   []string
}

// New creates a new scheduler
func New(database *db.DB, v *vault.Vault, llmClient *llm.Client, cfg Config) (*Scheduler, error) {
	tz, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		tz = time.UTC
	}

	s, err := gocron.NewScheduler(gocron.WithLocation(tz))
	if err != nil {
		return nil, err
	}

	return &Scheduler{
		scheduler: s,
		db:        database,
		vault:     v,
		llm:       llmClient,
		letterGen: NewLetterGenerator(llmClient, database),
		timezone:  tz,
		actors:    cfg.Actors,
	}, nil
}

// Start starts the scheduler and registers all jobs
func (s *Scheduler) Start() error {
	// Signal decay at 03:45 (before daily letters, ready for 4am breakfast)
	_, err := s.scheduler.NewJob(
		gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(3, 45, 0))),
		gocron.NewTask(s.decaySignals),
		gocron.WithName("signal-decay"),
	)
	if err != nil {
		return err
	}

	// Daily letter at 03:50 (ready for 4am breakfast)
	_, err = s.scheduler.NewJob(
		gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(3, 50, 0))),
		gocron.NewTask(s.generateDailyLetters),
		gocron.WithName("daily-letters"),
	)
	if err != nil {
		return err
	}

	// Weekly letter on Sunday at 03:50 (ready for 4am breakfast)
	_, err = s.scheduler.NewJob(
		gocron.WeeklyJob(1, gocron.NewWeekdays(time.Sunday), gocron.NewAtTimes(gocron.NewAtTime(3, 50, 0))),
		gocron.NewTask(s.generateWeeklyLetters),
		gocron.WithName("weekly-letters"),
	)
	if err != nil {
		return err
	}

	// Expire pending clarifications every hour
	_, err = s.scheduler.NewJob(
		gocron.DurationJob(1*time.Hour),
		gocron.NewTask(s.expirePending),
		gocron.WithName("expire-pending"),
	)
	if err != nil {
		return err
	}

	// Health check Ollama every 5 minutes
	_, err = s.scheduler.NewJob(
		gocron.DurationJob(5*time.Minute),
		gocron.NewTask(s.healthCheck),
		gocron.WithName("health-check"),
	)
	if err != nil {
		return err
	}

	s.scheduler.Start()
	log.Println("Scheduler started")
	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() error {
	return s.scheduler.Shutdown()
}

func (s *Scheduler) generateDailyLetters() {
	log.Println("Running daily letter generation...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	for _, actor := range s.actors {
		s.generateDailyLetterForActor(ctx, actor)
	}
}

func (s *Scheduler) generateDailyLetterForActor(ctx context.Context, actor string) {
	now := time.Now().In(s.timezone)

	// Use signal-based letter generation
	content, err := s.letterGen.GenerateDailyLetter(ctx, actor, now)
	if err != nil {
		log.Printf("Error generating daily letter for %s: %v", actor, err)
		return
	}

	// Validate letter content
	validation := signals.ValidateLetter(content, true)
	if !validation.Valid {
		log.Printf("Letter validation failed for %s: %v", actor, validation.Errors)
		// Sanitize and use anyway for now
		content, _ = signals.SanitizeLetter(content)
	}

	// Write letter to vault
	today := now.Format("2006-01-02")
	letterID := "let_" + today + "_" + actor + "_daily"

	letter := vault.Letter{
		ID:      letterID,
		Type:    "daily",
		ForDate: today,
		Actor:   actor,
		Content: content,
	}

	path, err := s.vault.WriteLetter(letter)
	if err != nil {
		log.Printf("Error writing daily letter for %s: %v", actor, err)
		return
	}

	// Record in database
	s.db.SaveLetter(letterID, "daily", today, path)
	log.Printf("Generated daily letter for %s: %s", actor, path)
}

func (s *Scheduler) generateWeeklyLetters() {
	log.Println("Running weekly letter generation...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	for _, actor := range s.actors {
		s.generateWeeklyLetterForActor(ctx, actor)
	}
}

func (s *Scheduler) generateWeeklyLetterForActor(ctx context.Context, actor string) {
	now := time.Now().In(s.timezone)

	// Use signal-based letter generation
	content, err := s.letterGen.GenerateWeeklyLetter(ctx, actor, now)
	if err != nil {
		log.Printf("Error generating weekly letter for %s: %v", actor, err)
		return
	}

	// Validate letter content
	validation := signals.ValidateLetter(content, false)
	if !validation.Valid {
		log.Printf("Weekly letter validation failed for %s: %v", actor, validation.Errors)
		content, _ = signals.SanitizeLetter(content)
	}

	// Write letter to vault
	year, week := now.ISOWeek()
	weekStr := fmt.Sprintf("%d-W%02d", year, week)
	letterID := "let_" + weekStr + "_" + actor + "_weekly"

	letter := vault.Letter{
		ID:      letterID,
		Type:    "weekly",
		ForDate: weekStr,
		Actor:   actor,
		Content: content,
	}

	path, err := s.vault.WriteLetter(letter)
	if err != nil {
		log.Printf("Error writing weekly letter for %s: %v", actor, err)
		return
	}

	// Record in database
	s.db.SaveLetter(letterID, "weekly", weekStr, path)
	log.Printf("Generated weekly letter for %s: %s", actor, path)
}

func (s *Scheduler) expirePending() {
	expired, err := s.db.ExpirePending()
	if err != nil {
		log.Printf("Error expiring pending: %v", err)
		return
	}
	if len(expired) > 0 {
		log.Printf("Expired %d pending clarifications", len(expired))
		// Log each expired capture to the vault
		for _, e := range expired {
			logEntry := vault.NewCaptureLog(e.CaptureID, e.Actor, "note", e.RawText, "", models.StatusExpired, "", 0)
			if err := s.vault.LogCapture(logEntry); err != nil {
				log.Printf("Error logging expired capture %s: %v", e.CaptureID, err)
			}
		}
	}
}

func (s *Scheduler) healthCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.llm.HealthCheck(ctx); err != nil {
		log.Printf("Health check failed - Ollama unreachable: %v", err)
	}
}

func (s *Scheduler) decaySignals() {
	log.Println("Running signal decay...")
	if err := signals.DecayAllSignals(s.db); err != nil {
		log.Printf("Signal decay failed: %v", err)
	} else {
		log.Println("Signal decay completed")
	}
}

func (s *Scheduler) getRecentCaptures(actor string, duration time.Duration) ([]CaptureEntry, error) {
	since := time.Now().Add(-duration)
	records, err := s.db.GetRecentCaptures(actor, since)
	if err != nil {
		return nil, err
	}

	entries := make([]CaptureEntry, len(records))
	for i, r := range records {
		entries[i] = CaptureEntry{
			Text:      r.RawText,
			Category:  r.RoutedTo,
			Timestamp: r.CreatedAt,
		}
	}
	return entries, nil
}

// GenerateDailyNow triggers daily letter generation immediately (for testing)
func (s *Scheduler) GenerateDailyNow(actor string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s.generateDailyLetterForActor(ctx, actor)
	return nil
}

// GenerateWeeklyNow triggers weekly letter generation immediately (for testing)
func (s *Scheduler) GenerateWeeklyNow(actor string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s.generateWeeklyLetterForActor(ctx, actor)
	return nil
}
