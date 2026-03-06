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
	tclient "go.temporal.io/sdk/client"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/handler"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/router"
	"github.com/helpin-ai/helpin/server/internal/service"
	"github.com/helpin-ai/helpin/server/internal/storage"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	ws "github.com/helpin-ai/helpin/server/internal/websocket"
	"github.com/helpin-ai/helpin/server/internal/worker"
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
	// PreferSimpleProtocol avoids pgx prepared-statement cache errors when
	// AutoMigrate changes table schemas between restarts.
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  cfg.DatabaseURL,
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

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	log.Println("connected to database")

	// Ensure pgcrypto extension is available for gen_random_uuid().
	db.Exec(`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`)

	if err := repository.MigrateLegacyRewardSchema(db); err != nil {
		log.Fatalf("failed to migrate legacy reward schema: %v", err)
	}

	if err := repository.MigrateAgentRunTargets(db); err != nil {
		log.Fatalf("failed to migrate agent run targets: %v", err)
	}

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
		&model.TeamUserMembership{},
		&model.WorkspaceManager{},
		&model.JobRoleCriteria{},
		&model.BonusTier{},
		&model.RewardQuarter{},
		&model.RewardSprint{},
		&model.RewardCompanyGoal{},
		&model.RewardGoalTeamContribution{},
		&model.RewardSprintGoal{},
		&model.RewardGoalDraft{},
		&model.RewardIndividualCheck{},
		&model.RewardBonusCalculation{},
		&model.RewardFinanceSettings{},
		&model.RewardAuditLog{},
		&model.PMWorkflow{},
		&model.PMWorkflowState{},
		&model.PMEpicWorkflowState{},
		&model.PMLabel{},
		&model.PMEpic{},
		&model.PMEpicObjective{},
		&model.PMEpicLabel{},
		&model.PMSprint{},
		&model.PMSprintLabel{},
		&model.PMStory{},
		&model.PMStoryOwner{},
		&model.PMStoryFollower{},
		&model.PMStoryLabel{},
		&model.PMComment{},
		&model.PMActivityLog{},
		&model.PMAttachment{},
		&model.PMObjective{},
		&model.PMKeyResult{},
		&model.PMObjectiveTeam{},
		&model.PMObjectiveOwner{},
		&model.PMObjectiveLabel{},
		&model.PMChecklistItem{},
		&model.PMExternalLink{},
		&model.PMView{},
		&model.PMAutomation{},
		&model.WorkspaceInvitation{},
		&model.InvitationTeamPreassignment{},
		&model.PMTeamEstimateSettings{},
		&model.PMTeamFieldVisibility{},
		&model.Agent{},
		&model.AgentRun{},
		&model.AgentRunArtifact{},
		&model.SupportTicket{},
		&model.SupportMessage{},
		&model.SupportWidgetInstallation{},
		&model.SupportWidgetSession{},
		&model.GitIntegration{},
		&model.GitRepository{},
		&model.PMTeamRepoDefault{},
		&model.StoryDeliveryTarget{},
		&model.StoryGitLink{},
		&model.AgentHandoff{},
	); err != nil {
		log.Fatalf("failed to auto-migrate: %v", err)
	}
	log.Println("database migration complete")

	// Initialize email client (nil if not configured).
	emailClient := email.NewClient(cfg.PostmarkServerToken, cfg.PostmarkFromEmail)
	if emailClient != nil {
		log.Println("Postmark email configured")
	} else {
		log.Println("Postmark email not configured — invitation emails will be logged only")
	}

	// Initialize S3 storage client (nil if not configured).
	s3Client := storage.NewS3Client(cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, cfg.AWSBucket, cfg.AWSRegion, cfg.AWSEndpointURL)
	if s3Client != nil {
		log.Println("S3 storage configured")
	} else {
		log.Println("S3 storage not configured — attachments disabled")
	}

	// Initialize JWT manager.
	jwtManager := auth.NewJWTManager(cfg.JWTSecret)

	// Initialize WebSocket hub and publisher.
	wsHub := ws.NewHub()
	wsPublisher := ws.NewPublisher(wsHub)
	wsHandler := ws.NewHandler(wsHub, jwtManager)

	// Initialize repositories.
	userRepo := repository.NewUserRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	quarterRepo := repository.NewRewardQuarterRepository(db)
	sprintRepo := repository.NewRewardSprintRepository(db)
	goalRepo := repository.NewRewardGoalRepository(db)
	scoringRepo := repository.NewRewardScoringRepository(db)
	bonusRepo := repository.NewRewardBonusRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	draftRepo := repository.NewRewardDraftRepository(db)
	pmWorkflowRepo := repository.NewPMWorkflowRepository(db)
	pmLabelRepo := repository.NewPMLabelRepository(db)
	pmEpicRepo := repository.NewPMEpicRepository(db)
	pmSprintRepo := repository.NewPMSprintRepository(db)
	pmStoryRepo := repository.NewPMStoryRepository(db)
	pmCommentRepo := repository.NewPMCommentRepository(db)
	pmActivityRepo := repository.NewPMActivityRepository(db)
	pmAttachmentRepo := repository.NewPMAttachmentRepository(db)
	pmObjectiveRepo := repository.NewPMObjectiveRepository(db)
	pmKeyResultRepo := repository.NewPMKeyResultRepository(db)
	pmChecklistItemRepo := repository.NewPMChecklistItemRepository(db)
	pmExternalLinkRepo := repository.NewPMExternalLinkRepository(db)
	pmViewRepo := repository.NewPMViewRepository(db)
	pmAutomationRepo := repository.NewPMAutomationRepository(db)
	searchRepo := repository.NewSearchRepository(db)
	invitationRepo := repository.NewInvitationRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	agentRunRepo := repository.NewAgentRunRepository(db)
	agentRunArtifactRepo := repository.NewAgentRunArtifactRepository(db)
	supportTicketRepo := repository.NewSupportTicketRepository(db)
	supportMessageRepo := repository.NewSupportMessageRepository(db)
	widgetInstallRepo := repository.NewWidgetInstallationRepository(db)
	widgetSessionRepo := repository.NewWidgetSessionRepository(db)
	gitIntegrationRepo := repository.NewGitIntegrationRepository(db)
	gitRepositoryRepo := repository.NewGitRepositoryRepository(db)
	storyDeliveryTargetRepo := repository.NewStoryDeliveryTargetRepository(db)
	storyGitLinkRepo := repository.NewStoryGitLinkRepository(db)
	agentHandoffRepo := repository.NewAgentHandoffRepository(db)

	// Initialize services.
	authService := service.NewAuthService(userRepo, jwtManager)
	pmActivityService := service.NewPMActivityService(pmActivityRepo)
	pmLabelService := service.NewPMLabelService(pmLabelRepo)
	pmWorkflowService := service.NewPMWorkflowService(pmWorkflowRepo, pmStoryRepo, pmLabelRepo)
	pmAutomationService := service.NewPMAutomationService(pmAutomationRepo, pmEpicRepo, pmStoryRepo, pmSprintRepo, pmWorkflowRepo, pmActivityService, wsPublisher)
	pmStoryService := service.NewPMStoryService(pmStoryRepo, pmWorkflowRepo, pmLabelRepo, pmActivityService, wsPublisher, pmAutomationService)
	pmEpicService := service.NewPMEpicService(pmEpicRepo, pmStoryRepo, pmLabelRepo, pmActivityService, wsPublisher)
	pmSprintService := service.NewPMSprintService(pmSprintRepo, pmLabelRepo, pmActivityService, wsPublisher)
	pmCommentService := service.NewPMCommentService(pmCommentRepo, pmStoryRepo, pmActivityService, wsPublisher)
	pmAttachmentService := service.NewPMAttachmentService(pmAttachmentRepo, s3Client, wsPublisher)
	pmObjectiveService := service.NewPMObjectiveService(pmObjectiveRepo, pmKeyResultRepo, pmLabelRepo, pmActivityService, wsPublisher)
	pmChecklistItemService := service.NewPMChecklistItemService(pmChecklistItemRepo, wsPublisher)
	pmExternalLinkService := service.NewPMExternalLinkService(pmExternalLinkRepo, wsPublisher)
	pmViewService := service.NewPMViewService(pmViewRepo)
	searchService := service.NewSearchService(searchRepo)
	supportService := service.NewSupportService(supportTicketRepo, supportMessageRepo, widgetInstallRepo, widgetSessionRepo, pmActivityService, wsPublisher)

	githubAppClient, err := githubapp.NewClient(cfg.GitHubAppID, cfg.GitHubAppPrivateKey)
	if err != nil {
		log.Fatalf("failed to initialize github app client: %v", err)
	}

	var temporalClient tclient.Client
	temporalClient, err = tclient.Dial(tclient.Options{
		HostPort:  cfg.TemporalAddress,
		Namespace: cfg.TemporalNamespace,
	})
	if err != nil {
		log.Printf("Temporal unavailable at %s (namespace=%s): %v", cfg.TemporalAddress, cfg.TemporalNamespace, err)
	} else {
		defer temporalClient.Close()
		log.Printf("Temporal configured at %s (namespace=%s)", cfg.TemporalAddress, cfg.TemporalNamespace)
	}
	runEngine := temporalapp.NewRunEngine(temporalClient, cfg.TemporalNamespace)

	gitService := service.NewGitService(
		gitIntegrationRepo,
		gitRepositoryRepo,
		storyGitLinkRepo,
		storyDeliveryTargetRepo,
		settingsRepo,
		workspaceRepo,
		pmStoryRepo,
		pmActivityService,
		wsPublisher,
		githubAppClient,
		cfg.AppBaseURL,
		cfg.GitHubAppSlug,
		cfg.JWTSecret,
	)
	agentService := service.NewAgentService(
		agentRepo,
		agentRunRepo,
		agentRunArtifactRepo,
		pmStoryRepo,
		supportTicketRepo,
		supportMessageRepo,
		agentHandoffRepo,
		runEngine,
		gitService,
		pmActivityService,
		wsPublisher,
	)

	// Initialize Claude client for orchestration (optional).
	var claudeClient *worker.ClaudeClient
	if cfg.AnthropicAPIKey != "" {
		claudeClient = worker.NewClaudeClient(cfg.AnthropicAPIKey)
		log.Println("Anthropic API configured — orchestration enabled")
	} else {
		log.Println("Anthropic API not configured — orchestration disabled")
	}

	workspaceService := service.NewWorkspaceService(workspaceRepo, pmWorkflowService)
	quarterService := service.NewRewardQuarterService(quarterRepo, sprintRepo)
	sprintService := service.NewRewardSprintService(sprintRepo, scoringRepo)
	goalService := service.NewRewardGoalService(goalRepo)
	bonusService := service.NewRewardBonusService(bonusRepo, scoringRepo)
	settingsService := service.NewSettingsService(settingsRepo)
	auditService := service.NewRewardAuditService(bonusRepo)
	draftService := service.NewRewardDraftService(draftRepo)
	inviteService := service.NewInviteService(invitationRepo, workspaceRepo, userRepo, settingsRepo, emailClient, cfg.AppBaseURL)
	orchestrationService := service.NewOrchestrationService(pmEpicRepo, pmStoryService, agentRepo, agentHandoffRepo, pmActivityService, wsPublisher, claudeClient)

	// Initialize handlers.
	handlers := router.Handlers{
		Health:          handler.NewHealthHandler(),
		Auth:            handler.NewAuthHandler(authService),
		Workspace:       handler.NewWorkspaceHandler(workspaceService),
		RewardQuarter:   handler.NewRewardQuarterHandler(quarterService),
		RewardSprint:    handler.NewRewardSprintHandler(sprintService),
		RewardGoal:      handler.NewRewardGoalHandler(goalService),
		RewardBonus:     handler.NewRewardBonusHandler(bonusService),
		RewardFinance:   handler.NewRewardFinanceHandler(bonusService),
		Settings:        handler.NewSettingsHandler(settingsService),
		RewardAudit:     handler.NewRewardAuditHandler(auditService),
		RewardDraft:     handler.NewRewardDraftHandler(draftService),
		Invite:          handler.NewInviteHandler(inviteService),
		PMWorkflow:      handler.NewPMWorkflowHandler(pmWorkflowService),
		PMLabel:         handler.NewPMLabelHandler(pmLabelService),
		PMEpic:          handler.NewPMEpicHandler(pmEpicService),
		PMSprint:        handler.NewPMSprintHandler(pmSprintService),
		PMStory:         handler.NewPMStoryHandler(pmStoryService),
		PMComment:       handler.NewPMCommentHandler(pmCommentService),
		PMAttachment:    handler.NewPMAttachmentHandler(pmAttachmentService),
		PMObjective:     handler.NewPMObjectiveHandler(pmObjectiveService),
		PMChecklistItem: handler.NewPMChecklistItemHandler(pmChecklistItemService),
		PMExternalLink:  handler.NewPMExternalLinkHandler(pmExternalLinkService),
		PMView:          handler.NewPMViewHandler(pmViewService),
		Search:          handler.NewSearchHandler(searchService),
		PMAutomation:    handler.NewPMAutomationHandler(pmAutomationService),
		Agent:           handler.NewAgentHandler(agentService),
		Support:         handler.NewSupportHandler(supportService, agentService),
		Widget:          handler.NewWidgetHandler(supportService),
		Git:             handler.NewGitHandler(gitService),
		Orchestration:   handler.NewOrchestrationHandler(orchestrationService),
	}

	// Set up router.
	r := router.New(handlers, jwtManager, cfg.CORSOrigin)

	// Start background ticker for iteration automations.
	automationDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				pmAutomationService.RunSprintAutomations(context.Background())
			case <-automationDone:
				return
			}
		}
	}()

	// Wrap router so /api/ws bypasses Chi middleware (Recoverer strips
	// http.Hijacker which WebSocket upgrade requires).
	var topHandler http.Handler = r
	if wsHandler != nil {
		topHandler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.URL.Path == "/api/ws" {
				wsHandler.ServeHTTP(w, req)
				return
			}
			r.ServeHTTP(w, req)
		})
	}

	// Start HTTP server with graceful shutdown.
	// WriteTimeout must be 0 for long-lived WebSocket connections.
	// ReadTimeout is safe to keep because the topHandler wrapper
	// routes /api/ws before Chi middleware processes the request.
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      topHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0,
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
	close(automationDone)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("server stopped")
}
