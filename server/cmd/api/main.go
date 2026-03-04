package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/d4interactive/teampulse/server/internal/auth"
	"github.com/d4interactive/teampulse/server/internal/config"
	"github.com/d4interactive/teampulse/server/internal/handler"
	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/repository"
	"github.com/d4interactive/teampulse/server/internal/router"
	"github.com/d4interactive/teampulse/server/internal/service"
	"github.com/d4interactive/teampulse/server/internal/storage"
)

func main() {
	// Load .env file if present (ignored in production).
	_ = godotenv.Load()

	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Connect to PostgreSQL via GORM.
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to get underlying sql.DB: %v", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	log.Println("connected to database")

	// Ensure pgcrypto extension is available for gen_random_uuid().
	db.Exec(`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`)

	// Auto-migrate all models.
	// The SQL migration files in server/migrations/ are kept as reference documentation.
	if err := db.AutoMigrate(
		&model.User{},
		&model.Workspace{},
		&model.WorkspaceMember{},
		&model.WorkspaceSettings{},
		&model.WorkspaceTeam{},
		&model.WorkspacePerson{},
		&model.TeamMembership{},
		&model.WorkspaceManager{},
		&model.JobRoleCriteria{},
		&model.BonusTier{},
		&model.Quarter{},
		&model.Sprint{},
		&model.CompanyGoal{},
		&model.GoalTeamContribution{},
		&model.SprintGoal{},
		&model.GoalDraft{},
		&model.IndividualCheck{},
		&model.BonusCalculation{},
		&model.FinanceSettings{},
		&model.BonusAuditLog{},
		&model.PMWorkflow{},
		&model.PMWorkflowState{},
		&model.PMEpicWorkflowState{},
		&model.PMLabel{},
		&model.PMEpic{},
		&model.PMEpicObjective{},
		&model.PMEpicLabel{},
		&model.PMIteration{},
		&model.PMIterationLabel{},
		&model.PMStory{},
		&model.PMStoryOwner{},
		&model.PMStoryFollower{},
		&model.PMStoryLabel{},
		&model.PMComment{},
		&model.PMActivityLog{},
		&model.PMAttachment{},
	); err != nil {
		log.Fatalf("failed to auto-migrate: %v", err)
	}
	log.Println("database migration complete")

	// Initialize S3 storage client (nil if not configured).
	s3Client := storage.NewS3Client(cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, cfg.AWSBucket, cfg.AWSRegion, cfg.AWSEndpointURL)
	if s3Client != nil {
		log.Println("S3 storage configured")
	} else {
		log.Println("S3 storage not configured — attachments disabled")
	}

	// Initialize JWT manager.
	jwtManager := auth.NewJWTManager(cfg.JWTSecret)

	// Initialize repositories.
	userRepo := repository.NewUserRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	quarterRepo := repository.NewQuarterRepository(db)
	sprintRepo := repository.NewSprintRepository(db)
	goalRepo := repository.NewGoalRepository(db)
	scoringRepo := repository.NewScoringRepository(db)
	bonusRepo := repository.NewBonusRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	draftRepo := repository.NewDraftRepository(db)
	pmWorkflowRepo := repository.NewPMWorkflowRepository(db)
	pmLabelRepo := repository.NewPMLabelRepository(db)
	pmEpicRepo := repository.NewPMEpicRepository(db)
	pmIterationRepo := repository.NewPMIterationRepository(db)
	pmStoryRepo := repository.NewPMStoryRepository(db)
	pmCommentRepo := repository.NewPMCommentRepository(db)
	pmActivityRepo := repository.NewPMActivityRepository(db)
	pmAttachmentRepo := repository.NewPMAttachmentRepository(db)

	// Initialize services.
	authService := service.NewAuthService(userRepo, jwtManager)
	pmActivityService := service.NewPMActivityService(pmActivityRepo)
	pmLabelService := service.NewPMLabelService(pmLabelRepo)
	pmWorkflowService := service.NewPMWorkflowService(pmWorkflowRepo, pmStoryRepo, pmLabelRepo)
	pmStoryService := service.NewPMStoryService(pmStoryRepo, pmWorkflowRepo, pmActivityService)
	pmEpicService := service.NewPMEpicService(pmEpicRepo, pmStoryRepo, pmActivityService)
	pmIterationService := service.NewPMIterationService(pmIterationRepo, pmActivityService)
	pmCommentService := service.NewPMCommentService(pmCommentRepo, pmStoryRepo, pmActivityService)
	pmAttachmentService := service.NewPMAttachmentService(pmAttachmentRepo, s3Client)

	workspaceService := service.NewWorkspaceService(workspaceRepo, pmWorkflowService)
	quarterService := service.NewQuarterService(quarterRepo, sprintRepo)
	sprintService := service.NewSprintService(sprintRepo, scoringRepo)
	goalService := service.NewGoalService(goalRepo)
	bonusService := service.NewBonusService(bonusRepo, scoringRepo)
	settingsService := service.NewSettingsService(settingsRepo)
	auditService := service.NewAuditService(bonusRepo)
	draftService := service.NewDraftService(draftRepo)

	// Initialize handlers.
	handlers := router.Handlers{
		Health:      handler.NewHealthHandler(),
		Auth:        handler.NewAuthHandler(authService),
		Workspace:   handler.NewWorkspaceHandler(workspaceService),
		Quarter:     handler.NewQuarterHandler(quarterService),
		Sprint:      handler.NewSprintHandler(sprintService),
		Goal:        handler.NewGoalHandler(goalService),
		Bonus:       handler.NewBonusHandler(bonusService),
		Finance:     handler.NewFinanceHandler(bonusService),
		Settings:    handler.NewSettingsHandler(settingsService),
		Audit:       handler.NewAuditHandler(auditService),
		Draft:       handler.NewDraftHandler(draftService),
		Invite:      handler.NewInviteHandler(),
		PMWorkflow:  handler.NewPMWorkflowHandler(pmWorkflowService),
		PMLabel:     handler.NewPMLabelHandler(pmLabelService),
		PMEpic:      handler.NewPMEpicHandler(pmEpicService),
		PMIteration: handler.NewPMIterationHandler(pmIterationService),
		PMStory:     handler.NewPMStoryHandler(pmStoryService),
		PMComment:    handler.NewPMCommentHandler(pmCommentService),
		PMAttachment: handler.NewPMAttachmentHandler(pmAttachmentService),
	}

	// Set up router.
	r := router.New(handlers, jwtManager, cfg.CORSOrigin)

	// Start HTTP server with graceful shutdown.
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Listen for shutdown signals.
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("server starting on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-done
	log.Println("server shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("server stopped")
}
