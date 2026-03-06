package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicKey == "" {
		log.Fatal("ANTHROPIC_API_KEY is required")
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dbURL,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to get underlying sql.DB: %v", err)
	}
	defer sqlDB.Close()

	// Ensure pgcrypto.
	db.Exec(`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`)

	// Migrate job table.
	if err := db.AutoMigrate(&model.AgentJob{}); err != nil {
		log.Fatalf("failed to migrate agent_jobs: %v", err)
	}

	// Generate worker ID.
	workerID := generateWorkerID()
	log.Printf("worker ID: %s", workerID)

	// Initialize repos.
	jobRepo := repository.NewAgentJobRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	storyRepo := repository.NewPMStoryRepository(db)
	ticketRepo := repository.NewSupportTicketRepository(db)
	commentRepo := repository.NewPMCommentRepository(db)
	checklistRepo := repository.NewPMChecklistItemRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	gitIntRepo := repository.NewGitIntegrationRepository(db)
	gitLinkRepo := repository.NewStoryGitLinkRepository(db)

	// Initialize Claude client and runtime adapters.
	claudeClient := worker.NewClaudeClient(anthropicKey)
	nativeAdapter := worker.NewExecutor("native_claude", claudeClient, runRepo, artifactRepo)
	claudeCodeAdapter := worker.NewExecutor("claude_code", claudeClient, runRepo, artifactRepo)
	runtimes := worker.NewRuntimeRegistry(nativeAdapter, claudeCodeAdapter)

	// WebSocket publisher (nil in worker — events propagated via pg_notify or direct DB reads).
	var wsPublisher *websocket.Publisher

	// Create worker.
	w := worker.NewWorker(
		workerID,
		jobRepo,
		runRepo,
		agentRepo,
		artifactRepo,
		storyRepo,
		ticketRepo,
		commentRepo,
		checklistRepo,
		messageRepo,
		gitIntRepo,
		gitLinkRepo,
		runtimes,
		wsPublisher,
	)

	// Run with graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-done
		log.Println("shutdown signal received")
		cancel()
	}()

	log.Println("worker starting...")
	w.Run(ctx)
	log.Println("worker stopped")
}

func generateWorkerID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	hostname, _ := os.Hostname()
	return fmt.Sprintf("%s-%s", hostname, hex.EncodeToString(b))
}
