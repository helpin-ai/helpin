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
	"github.com/helpin-ai/helpin/server/internal/authorization"
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

	if err := repository.MigratePMImportSchema(db); err != nil {
		log.Fatalf("failed to migrate pm import schema: %v", err)
	}

	if err := repository.MigrateWorkspaceMemberSchema(db); err != nil {
		log.Fatalf("failed to migrate workspace member schema: %v", err)
	}
	if err := repository.DropLegacyWorkspaceIdentitySchema(db); err != nil {
		log.Fatalf("failed to drop legacy workspace identity schema: %v", err)
	}

	// Auto-migrate all models.
	// The SQL migration files in server/migrations/ are kept as reference documentation.
	if err := db.AutoMigrate(
		&model.User{},
		&model.Organization{},
		&model.OrganizationMember{},
		&model.Workspace{},
		&model.WorkspaceMember{},
		&model.WorkspaceSettings{},
		&model.WorkspaceTeam{},
		&model.TeamWorkspaceMembership{},
		&model.WorkspaceManager{},
		&model.RewardProfile{},
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
		&model.PMStoryLink{},
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
		&model.PMStoryTemplate{},
		&model.PMImportJob{},
		&authorization.AuthorizationRelation{},
		// Docs module
		&model.DocsSpace{},
		&model.DocsSpaceTeam{},
		&model.DocsCollection{},
		&model.DocsDocument{},
		&model.DocsContent{},
		&model.DocsVersion{},
		&model.DocsLink{},
		&model.DocsHelpcenterConfig{},
		&model.DocsHelpcenterArticle{},
		&model.DocsSlugAlias{},
		&model.DocsReviewQueue{},
		&model.DocsArticleFeedback{},
		&model.DocsComment{},
		// Notifications module
		&model.Notification{},
		&model.NotificationEvent{},
		&model.NotificationDelivery{},
		&model.NotificationPreference{},
		&model.EntityFollower{},
		// CRM module
		&model.CRMContact{},
		&model.CRMCompany{},
		&model.CRMPipeline{},
		&model.CRMPipelineStage{},
		&model.CRMDeal{},
		&model.CRMAssociation{},
		&model.CRMActivity{},
		&model.CRMPropertyDefinition{},
		&model.CRMPropertyGroup{},
		&model.CRMList{},
		&model.CRMListMember{},
		&model.CRMImportJob{},
		// CRM Phase 3: Email & Calendar
		&model.CRMEmailAccount{},
		&model.CRMEmailThread{},
		&model.CRMEmailMessage{},
		&model.CRMCalendarEvent{},
		// CRM Phase 4: Intelligence
		&model.CRMEnrichmentResult{},
		&model.CRMBuyerSignal{},
		&model.CRMDealHealthScore{},
		&model.CRMSuggestion{},
		// CRM Phase 5: Sequences & Writing
		&model.CRMSequence{},
		&model.CRMSequenceEnrollment{},
		&model.CRMWritingProfile{},
	); err != nil {
		log.Fatalf("failed to auto-migrate: %v", err)
	}
	log.Println("database migration complete")

	// Migrate legacy objective states to lifecycle states + health (idempotent).
	db.Exec("UPDATE pm_objectives SET state = 'not_started' WHERE state = 'to_do'")
	db.Exec("UPDATE pm_objectives SET state = 'active' WHERE state IN ('in_progress', 'on_track', 'behind', 'at_risk')")
	db.Exec("UPDATE pm_objectives SET state = 'closed' WHERE state = 'done'")

	// Migrate existing workspaces to organizations (one-time, idempotent).
	if err := repository.MigrateWorkspacesToOrganizations(db); err != nil {
		log.Fatalf("failed to migrate workspaces to organizations: %v", err)
	}

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
	orgRepo := repository.NewOrganizationRepository(db)
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
	pmStoryTemplateRepo := repository.NewPMStoryTemplateRepository(db)
	searchRepo := repository.NewSearchRepository(db)
	invitationRepo := repository.NewInvitationRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	agentRunRepo := repository.NewAgentRunRepository(db)
	agentRunArtifactRepo := repository.NewAgentRunArtifactRepository(db)
	pmStoryLinkRepo := repository.NewPMStoryLinkRepository(db)
	supportTicketRepo := repository.NewSupportTicketRepository(db)
	supportMessageRepo := repository.NewSupportMessageRepository(db)
	widgetInstallRepo := repository.NewWidgetInstallationRepository(db)
	widgetSessionRepo := repository.NewWidgetSessionRepository(db)
	gitIntegrationRepo := repository.NewGitIntegrationRepository(db)
	gitRepositoryRepo := repository.NewGitRepositoryRepository(db)
	storyDeliveryTargetRepo := repository.NewStoryDeliveryTargetRepository(db)
	storyGitLinkRepo := repository.NewStoryGitLinkRepository(db)
	agentHandoffRepo := repository.NewAgentHandoffRepository(db)
	docsSpaceRepo := repository.NewDocsSpaceRepository(db)
	docsCollectionRepo := repository.NewDocsCollectionRepository(db)
	docsDocumentRepo := repository.NewDocsDocumentRepository(db)
	docsContentRepo := repository.NewDocsContentRepository(db)
	docsVersionRepo := repository.NewDocsVersionRepository(db)
	docsLinkRepo := repository.NewDocsLinkRepository(db)
	docsHelpcenterRepo := repository.NewDocsHelpcenterRepository(db)
	docsSearchRepo := repository.NewDocsSearchRepository(db)
	notificationRepo := repository.NewNotificationRepository(db)
	notificationPrefRepo := repository.NewNotificationPreferenceRepository(db)
	followerRepo := repository.NewFollowerRepository(db)
	crmContactRepo := repository.NewCRMContactRepository(db)
	crmCompanyRepo := repository.NewCRMCompanyRepository(db)
	crmDealRepo := repository.NewCRMDealRepository(db)
	crmAssociationRepo := repository.NewCRMAssociationRepository(db)
	crmActivityRepo := repository.NewCRMActivityRepository(db)
	crmPropertyRepo := repository.NewCRMPropertyRepository(db)
	crmListRepo := repository.NewCRMListRepository(db)
	crmImportRepo := repository.NewCRMImportRepository(db)
	crmEmailRepo := repository.NewCRMEmailRepository(db)
	crmCalendarRepo := repository.NewCRMCalendarRepository(db)
	crmEnrichmentRepo := repository.NewCRMEnrichmentRepository(db)
	crmSignalRepo := repository.NewCRMSignalRepository(db)
	crmSuggestionRepo := repository.NewCRMSuggestionRepository(db)
	crmSequenceRepo := repository.NewCRMSequenceRepository(db)
	crmWritingProfileRepo := repository.NewCRMWritingProfileRepository(db)

	// Initialize services.
	authService := service.NewAuthService(userRepo, jwtManager)
	pmActivityService := service.NewPMActivityService(pmActivityRepo)
	pmLabelService := service.NewPMLabelService(pmLabelRepo)
	pmStoryTemplateService := service.NewPMStoryTemplateService(pmStoryTemplateRepo)
	pmWorkflowService := service.NewPMWorkflowService(pmWorkflowRepo, pmStoryRepo, pmLabelRepo)
	pmAutomationService := service.NewPMAutomationService(pmAutomationRepo, pmEpicRepo, pmStoryRepo, pmSprintRepo, pmWorkflowRepo, pmActivityService, wsPublisher)
	notificationService := service.NewNotificationService(notificationRepo, notificationPrefRepo, followerRepo, wsPublisher)
	followerService := service.NewFollowerService(followerRepo)
	pmStoryService := service.NewPMStoryService(pmStoryRepo, workspaceRepo, pmWorkflowRepo, pmLabelRepo, pmActivityService, wsPublisher, pmAutomationService, notificationService, followerService)
	pmEpicService := service.NewPMEpicService(pmEpicRepo, pmStoryRepo, pmLabelRepo, gitRepositoryRepo, workspaceRepo, pmActivityService, wsPublisher, notificationService)
	pmSprintService := service.NewPMSprintService(pmSprintRepo, pmLabelRepo, pmActivityService, wsPublisher, notificationService)
	pmCommentService := service.NewPMCommentService(pmCommentRepo, pmStoryRepo, pmActivityService, wsPublisher, notificationService, workspaceRepo)
	pmAttachmentService := service.NewPMAttachmentService(pmAttachmentRepo, s3Client, wsPublisher)
	pmObjectiveService := service.NewPMObjectiveService(pmObjectiveRepo, pmKeyResultRepo, pmLabelRepo, workspaceRepo, pmActivityService, wsPublisher, notificationService)
	pmChecklistItemService := service.NewPMChecklistItemService(pmChecklistItemRepo, pmStoryRepo, wsPublisher, notificationService, workspaceRepo)
	pmExternalLinkService := service.NewPMExternalLinkService(pmExternalLinkRepo, wsPublisher)
	pmViewService := service.NewPMViewService(pmViewRepo)
	pmImportService := service.NewPMImportService(db, workspaceRepo, pmWorkflowRepo, pmAttachmentService)
	searchService := service.NewSearchService(searchRepo)
	supportService := service.NewSupportService(supportTicketRepo, supportMessageRepo, agentRepo, widgetInstallRepo, widgetSessionRepo, pmActivityService, wsPublisher)

	githubAppClient, err := githubapp.NewClient(cfg.GitHubAppID, cfg.GitHubAppPrivateKey)
	if err != nil {
		log.Fatalf("failed to initialize github app client: %v", err)
	}

	var temporalClient tclient.Client
	temporalClient, err = tclient.Dial(temporalapp.BuildClientOptions(cfg))
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
		pmStoryLinkRepo,
		pmEpicRepo,
		supportTicketRepo,
		supportMessageRepo,
		agentHandoffRepo,
		settingsRepo,
		docsSpaceRepo,
		docsDocumentRepo,
		docsContentRepo,
		docsVersionRepo,
		docsLinkRepo,
		runEngine,
		gitService,
		pmStoryService,
		pmActivityService,
		wsPublisher,
	)

	// Log orchestration availability.
	if cfg.AnthropicAPIKey != "" {
		log.Println("Anthropic API configured — orchestration enabled")
	} else {
		log.Println("Anthropic API not configured — orchestration disabled")
	}

	docsSpaceService := service.NewDocsSpaceService(docsSpaceRepo)
	docsCollectionService := service.NewDocsCollectionService(docsCollectionRepo, docsSpaceRepo)
	docsDocumentService := service.NewDocsDocumentService(docsDocumentRepo, docsSpaceRepo)
	docsContentService := service.NewDocsContentService(docsContentRepo)
	docsVersionService := service.NewDocsVersionService(docsVersionRepo, docsContentRepo)
	docsLinkService := service.NewDocsLinkService(docsLinkRepo)
	docsHelpcenterService := service.NewDocsHelpcenterService(docsHelpcenterRepo, docsDocumentRepo, docsSpaceRepo, docsCollectionRepo)
	docsSearchService := service.NewDocsSearchService(docsSearchRepo)

	crmContactService := service.NewCRMContactService(crmContactRepo)
	crmCompanyService := service.NewCRMCompanyService(crmCompanyRepo)
	crmDealService := service.NewCRMDealService(crmDealRepo, crmAssociationRepo)
	crmAssociationService := service.NewCRMAssociationService(crmAssociationRepo)
	crmActivityService := service.NewCRMActivityService(crmActivityRepo)
	crmPropertyService := service.NewCRMPropertyService(crmPropertyRepo)
	crmListService := service.NewCRMListService(crmListRepo)
	crmImportService := service.NewCRMImportService(crmImportRepo, crmContactRepo, crmCompanyRepo, crmDealRepo)
	crmEmailService := service.NewCRMEmailService(crmEmailRepo, crmContactRepo)
	crmCalendarService := service.NewCRMCalendarService(crmCalendarRepo)
	crmEnrichmentService := service.NewCRMEnrichmentService(crmEnrichmentRepo)
	crmSignalService := service.NewCRMSignalService(crmSignalRepo)
	crmSuggestionService := service.NewCRMSuggestionService(crmSuggestionRepo)
	crmSequenceService := service.NewCRMSequenceService(crmSequenceRepo)
	crmWritingProfileService := service.NewCRMWritingProfileService(crmWritingProfileRepo)
	crmSearchService := service.NewCRMSearchService(crmContactRepo, crmCompanyRepo, crmDealRepo)

	orgService := service.NewOrganizationService(orgRepo)
	compositeDefaults := service.NewCompositeDefaultsInitializer(pmWorkflowService, crmDealService)
	workspaceService := service.NewWorkspaceService(workspaceRepo, pmAttachmentRepo, s3Client, compositeDefaults)
	quarterService := service.NewRewardQuarterService(quarterRepo, sprintRepo)
	sprintService := service.NewRewardSprintService(sprintRepo, scoringRepo)
	goalService := service.NewRewardGoalService(goalRepo)
	bonusService := service.NewRewardBonusService(bonusRepo, scoringRepo)
	settingsService := service.NewSettingsService(settingsRepo, cfg.BraveSearchAPIKey)
	auditService := service.NewRewardAuditService(bonusRepo)
	draftService := service.NewRewardDraftService(draftRepo)
	inviteService := service.NewInviteService(invitationRepo, workspaceRepo, userRepo, settingsRepo, emailClient, cfg.AppBaseURL, jwtManager)
	orchestrationService := service.NewOrchestrationService(pmEpicRepo, agentRepo, pmActivityService, wsPublisher)

	// Initialize authorization service.
	authzMemberRepo := authorization.NewGORMMemberRepository(db)
	authzService := authorization.NewAuthzService(db, authzMemberRepo)

	// Inject authorization into WebSocket handler for workspace access checks.
	wsHandler.SetAuthzService(authzService)

	// Initialize handlers.
	handlers := router.Handlers{
		Health:            handler.NewHealthHandler(),
		Auth:              handler.NewAuthHandler(authService),
		Organization:      handler.NewOrganizationHandler(orgService),
		Workspace:         handler.NewWorkspaceHandler(workspaceService),
		RewardQuarter:     handler.NewRewardQuarterHandler(quarterService),
		RewardSprint:      handler.NewRewardSprintHandler(sprintService),
		RewardGoal:        handler.NewRewardGoalHandler(goalService),
		RewardBonus:       handler.NewRewardBonusHandler(bonusService),
		RewardFinance:     handler.NewRewardFinanceHandler(bonusService),
		Settings:          handler.NewSettingsHandler(settingsService),
		RewardAudit:       handler.NewRewardAuditHandler(auditService),
		RewardDraft:       handler.NewRewardDraftHandler(draftService),
		Invite:            handler.NewInviteHandler(inviteService),
		PMWorkflow:        handler.NewPMWorkflowHandler(pmWorkflowService),
		PMImport:          handler.NewPMImportHandler(pmImportService),
		PMLabel:           handler.NewPMLabelHandler(pmLabelService),
		PMEpic:            handler.NewPMEpicHandler(pmEpicService),
		PMSprint:          handler.NewPMSprintHandler(pmSprintService),
		PMStory:           handler.NewPMStoryHandler(pmStoryService),
		PMComment:         handler.NewPMCommentHandler(pmCommentService),
		PMAttachment:      handler.NewPMAttachmentHandler(pmAttachmentService),
		PMObjective:       handler.NewPMObjectiveHandler(pmObjectiveService),
		PMChecklistItem:   handler.NewPMChecklistItemHandler(pmChecklistItemService),
		PMExternalLink:    handler.NewPMExternalLinkHandler(pmExternalLinkService),
		PMView:            handler.NewPMViewHandler(pmViewService),
		Search:            handler.NewSearchHandler(searchService),
		PMAutomation:      handler.NewPMAutomationHandler(pmAutomationService),
		PMStoryTemplate:   handler.NewPMStoryTemplateHandler(pmStoryTemplateService),
		Agent:             handler.NewAgentHandler(agentService),
		Support:           handler.NewSupportHandler(supportService, agentService),
		Widget:            handler.NewWidgetHandler(supportService),
		Git:               handler.NewGitHandler(gitService),
		Orchestration:     handler.NewOrchestrationHandler(orchestrationService),
		Notification:      handler.NewNotificationHandler(notificationService, followerService),
		CRMContact:        handler.NewCRMContactHandler(crmContactService),
		CRMCompany:        handler.NewCRMCompanyHandler(crmCompanyService),
		CRMDeal:           handler.NewCRMDealHandler(crmDealService),
		CRMAssociation:    handler.NewCRMAssociationHandler(crmAssociationService),
		CRMActivity:       handler.NewCRMActivityHandler(crmActivityService),
		CRMProperty:       handler.NewCRMPropertyHandler(crmPropertyService),
		CRMList:           handler.NewCRMListHandler(crmListService),
		CRMImport:         handler.NewCRMImportHandler(crmImportService),
		CRMEmail:          handler.NewCRMEmailHandler(crmEmailService),
		CRMCalendar:       handler.NewCRMCalendarHandler(crmCalendarService),
		CRMEnrichment:     handler.NewCRMEnrichmentHandler(crmEnrichmentService),
		CRMSignal:         handler.NewCRMSignalHandler(crmSignalService),
		CRMSuggestion:     handler.NewCRMSuggestionHandler(crmSuggestionService),
		CRMSequence:       handler.NewCRMSequenceHandler(crmSequenceService),
		CRMWritingProfile: handler.NewCRMWritingProfileHandler(crmWritingProfileService),
		CRMSearch:         handler.NewCRMSearchHandler(crmSearchService),
		Docs: handler.NewDocsHandler(
			docsSpaceService,
			docsCollectionService,
			docsDocumentService,
			docsContentService,
			docsVersionService,
			docsLinkService,
			docsHelpcenterService,
			docsSearchService,
		),
	}

	// Slug resolver adapts workspace repo for RBAC middleware.
	slugResolver := authorization.SlugResolver(func(ctx context.Context, slug string) (string, error) {
		ws, err := workspaceRepo.GetBySlug(ctx, slug)
		if err != nil {
			return "", err
		}
		return ws.ID, nil
	})

	// Set up router.
	r := router.New(handlers, jwtManager, authzService, slugResolver, cfg.CORSOrigins)

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
