package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	tclient "go.temporal.io/sdk/client"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/crmemail"
	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/handler"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/oauth"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/router"
	"github.com/helpin-ai/helpin/server/internal/service"
	"github.com/helpin-ai/helpin/server/internal/storage"
	syncpkg "github.com/helpin-ai/helpin/server/internal/sync"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	ws "github.com/helpin-ai/helpin/server/internal/websocket"
)

func main() {
	// Load .env file if present (ignored in production).
	_ = godotenv.Load()

	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize structured logger.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// Connect to PostgreSQL via GORM.
	// PreferSimpleProtocol avoids pgx prepared-statement cache errors when
	// AutoMigrate changes table schemas between restarts.
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  cfg.DatabaseURL,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger: gormlogger.New(
			slog.NewLogLogger(slog.Default().Handler(), slog.LevelWarn),
			gormlogger.Config{
				SlowThreshold:             200 * time.Millisecond,
				IgnoreRecordNotFoundError: true,
				LogLevel:                  gormlogger.Warn,
			},
		),
	})
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("failed to get underlying sql.DB", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		slog.Error("failed to ping database", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to database")

	// Ensure pgcrypto extension is available for gen_random_uuid().
	slog.Info("startup: enabling pgcrypto extension")
	db.Exec(`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`)

	slog.Info("startup: running MigrateLegacyRewardSchema")
	if err := repository.MigrateLegacyRewardSchema(db); err != nil {
		slog.Error("failed to migrate legacy reward schema", "error", err)
		os.Exit(1)
	}

	slog.Info("startup: running MigrateAgentRunTargets")
	if err := repository.MigrateAgentRunTargets(db); err != nil {
		slog.Error("failed to migrate agent run targets", "error", err)
		os.Exit(1)
	}

	slog.Info("startup: running MigratePMImportSchema")
	if err := repository.MigratePMImportSchema(db); err != nil {
		slog.Error("failed to migrate pm import schema", "error", err)
		os.Exit(1)
	}

	slog.Info("startup: running MigrateDropDocType")
	if err := repository.MigrateDropDocType(db); err != nil {
		slog.Error("failed to migrate drop doc_type", "error", err)
		os.Exit(1)
	}

	slog.Info("startup: running MigrateDropRestrictToOwners")
	if err := repository.MigrateDropRestrictToOwners(db); err != nil {
		slog.Error("failed to migrate drop restrict_to_owners", "error", err)
		os.Exit(1)
	}

	// Fix: idx_ws_member_ws_user was incorrectly created as a single-column unique
	// index on user_id only. Drop it so AutoMigrate recreates it as composite (workspace_id, user_id).
	if err := db.Exec("DROP INDEX IF EXISTS idx_ws_member_ws_user").Error; err != nil {
		slog.Error("failed to drop incorrect ws member index", "error", err)
		os.Exit(1)
	}

	// Auto-migrate all models.
	// The SQL migration files in server/migrations/ are kept as reference documentation.
	slog.Info("startup: running AutoMigrate")
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
		&model.PMCommentReaction{},
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
		&model.SupportConversation{},
		&model.SupportMessage{},
		&model.SupportCannedResponse{},
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
		&model.UserNotificationSettings{},
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
		&model.CRMEmailMessageContact{},
		&model.CRMCalendarEvent{},
		// CRM Phase 4: Intelligence
		&model.CRMEnrichmentResult{},
		&model.CRMBuyerSignal{},
		&model.CRMEntitySummary{},
		&model.CRMDealHealthScore{},
		&model.CRMSuggestion{},
		// CRM Phase 5: Sequences & Writing
		&model.CRMSequence{},
		&model.CRMSequenceEnrollment{},
		&model.CRMWritingProfile{},
		// CRM Autonomy
		&model.CRMAutonomySettings{},
		// CRM Email Sync Settings
		&model.CRMEmailSyncSettings{},
	); err != nil {
		slog.Error("failed to auto-migrate", "error", err)
		os.Exit(1)
	}
	slog.Info("startup: AutoMigrate complete")

	// Post-AutoMigrate schema migrations that reference tables created above.
	slog.Info("startup: running MigrateWorkspaceMemberSchema")
	if err := repository.MigrateWorkspaceMemberSchema(db); err != nil {
		slog.Error("failed to migrate workspace member schema", "error", err)
		os.Exit(1)
	}
	slog.Info("startup: running DropLegacyWorkspaceIdentitySchema")
	if err := repository.DropLegacyWorkspaceIdentitySchema(db); err != nil {
		slog.Error("failed to drop legacy workspace identity schema", "error", err)
		os.Exit(1)
	}
	slog.Info("startup: running MigrateCRMEmailAssociations")
	if err := repository.MigrateCRMEmailAssociations(db); err != nil {
		slog.Error("failed to migrate crm email associations", "error", err)
		os.Exit(1)
	}
	slog.Info("startup: running MigrateCRMSignalSchema")
	if err := repository.MigrateCRMSignalSchema(db); err != nil {
		slog.Error("failed to migrate crm signal schema", "error", err)
		os.Exit(1)
	}
	slog.Info("startup: running MigrateCRMSummarySchema")
	if err := repository.MigrateCRMSummarySchema(db); err != nil {
		slog.Error("failed to migrate crm summary schema", "error", err)
		os.Exit(1)
	}

	// Migrate existing workspaces to organizations (one-time, idempotent).
	slog.Info("startup: running MigrateWorkspacesToOrganizations")
	if err := repository.MigrateWorkspacesToOrganizations(db); err != nil {
		slog.Error("failed to migrate workspaces to organizations", "error", err)
		os.Exit(1)
	}
	slog.Info("startup: all migrations complete")

	// Initialize email client (nil if not configured).
	emailClient := email.NewClient(cfg.PostmarkServerToken, cfg.PostmarkFromEmail)
	if emailClient != nil {
		slog.Info("Postmark email configured")
	} else {
		slog.Info("Postmark email not configured — invitation emails will be logged only")
	}

	// Initialize S3 storage client (nil if not configured).
	s3Client := storage.NewS3Client(cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, cfg.AWSBucket, cfg.AWSRegion, cfg.AWSEndpointURL)
	if s3Client != nil {
		slog.Info("S3 storage configured")
	} else {
		slog.Info("S3 storage not configured — attachments disabled")
	}

	// Initialize JWT manager.
	jwtManager := auth.NewJWTManager(cfg.JWTSecret)

	// Initialize WebSocket hub and publisher.
	wsHub := ws.NewHub()
	wsPublisher := ws.NewPublisher(wsHub)
	wsHandler := ws.NewHandler(wsHub, jwtManager)

	// Start PG LISTEN → WS bridge for cross-process events (e.g. Temporal worker).
	pgListenerCtx, pgListenerCancel := context.WithCancel(context.Background())
	pgListener := ws.NewPGListener(cfg.DatabaseURL, wsHub)
	go pgListener.Start(pgListenerCtx)

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
	crmAutonomyRepo := repository.NewCRMAutonomyRepository(db)
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
	supportConversationRepo := repository.NewSupportConversationRepository(db)
	supportMessageRepo := repository.NewSupportMessageRepository(db)
	supportInstallRepo := repository.NewSupportInboxInstallationRepository(db)
	supportSessionRepo := repository.NewSupportInboxSessionRepository(db)
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
	userNotifSettingsRepo := repository.NewUserNotificationSettingsRepository(db)
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
	crmSummaryRepo := repository.NewCRMSummaryRepository(db)
	crmSuggestionRepo := repository.NewCRMSuggestionRepository(db)
	crmSequenceRepo := repository.NewCRMSequenceRepository(db)
	crmWritingProfileRepo := repository.NewCRMWritingProfileRepository(db)
	crmEmailSyncSettingsRepo := repository.NewCRMEmailSyncSettingsRepository(db)

	crmEmailResolver := crmemail.NewResolver(crmContactRepo)
	crmEmailBackfillRunner := crmemail.NewBackfillRunner(crmEmailRepo, crmEmailSyncSettingsRepo, crmEmailResolver)
	go func() {
		slog.Info("startup: running CRM email association backfill")
		if err := crmEmailBackfillRunner.Run(context.Background()); err != nil {
			slog.Error("crm email association backfill failed", "error", err)
			return
		}
		slog.Info("startup: CRM email association backfill complete")
	}()

	// Initialize services.
	authService := service.NewAuthService(userRepo, jwtManager)
	pmActivityService := service.NewPMActivityService(pmActivityRepo)
	pmLabelService := service.NewPMLabelService(pmLabelRepo)
	pmStoryTemplateService := service.NewPMStoryTemplateService(pmStoryTemplateRepo)
	pmWorkflowService := service.NewPMWorkflowService(pmWorkflowRepo, pmStoryRepo, pmLabelRepo)
	pmAutomationService := service.NewPMAutomationService(pmAutomationRepo, pmEpicRepo, pmStoryRepo, pmSprintRepo, pmWorkflowRepo, pmActivityService, wsPublisher)
	notificationService := service.NewNotificationService(notificationRepo, notificationPrefRepo, userNotifSettingsRepo, followerRepo, userRepo, workspaceRepo, wsPublisher, emailClient, cfg.AppBaseURL)
	userNotifSettingsService := service.NewUserNotificationSettingsService(userNotifSettingsRepo)
	followerService := service.NewFollowerService(followerRepo)
	pmStoryService := service.NewPMStoryService(pmStoryRepo, workspaceRepo, pmWorkflowRepo, pmLabelRepo, pmActivityService, wsPublisher, pmAutomationService, notificationService, followerService)
	pmEpicService := service.NewPMEpicService(pmEpicRepo, pmStoryRepo, pmLabelRepo, gitRepositoryRepo, workspaceRepo, pmActivityService, wsPublisher, notificationService)
	pmSprintService := service.NewPMSprintService(pmSprintRepo, pmLabelRepo, pmActivityService, wsPublisher, notificationService)
	pmCommentService := service.NewPMCommentService(pmCommentRepo, pmStoryRepo, pmAttachmentRepo, pmActivityService, wsPublisher, notificationService, workspaceRepo)
	pmAttachmentService := service.NewPMAttachmentService(pmAttachmentRepo, s3Client, wsPublisher)
	pmObjectiveService := service.NewPMObjectiveService(pmObjectiveRepo, pmKeyResultRepo, pmLabelRepo, workspaceRepo, pmActivityService, wsPublisher, notificationService)
	pmChecklistItemService := service.NewPMChecklistItemService(pmChecklistItemRepo, pmStoryRepo, wsPublisher, notificationService, workspaceRepo)
	pmExternalLinkService := service.NewPMExternalLinkService(pmExternalLinkRepo, wsPublisher)
	pmViewService := service.NewPMViewService(pmViewRepo)
	pmImportService := service.NewPMImportService(db, workspaceRepo, pmWorkflowRepo, pmAttachmentService)
	searchService := service.NewSearchService(searchRepo)
	cannedResponseRepo := repository.NewSupportCannedResponseRepository(db)
	supportInboxService := service.NewSupportInboxService(supportConversationRepo, supportMessageRepo, agentRepo, crmAssociationRepo, supportInstallRepo, supportSessionRepo, cannedResponseRepo, pmActivityService, wsPublisher, crmContactRepo)

	slog.Info("startup: initializing GitHub App client")
	githubAppClient, err := githubapp.NewClient(cfg.GitHubAppID, cfg.GitHubAppPrivateKey)
	if err != nil {
		slog.Error("failed to initialize github app client", "error", err)
		os.Exit(1)
	}

	var temporalClient tclient.Client
	temporalClient, err = tclient.Dial(temporalapp.BuildClientOptions(cfg))
	if err != nil {
		slog.Warn("Temporal unavailable", "address", cfg.TemporalAddress, "namespace", cfg.TemporalNamespace, "error", err)
	} else {
		defer temporalClient.Close()
		slog.Info("Temporal configured", "address", cfg.TemporalAddress, "namespace", cfg.TemporalNamespace)
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
		supportConversationRepo,
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
	).SetModelProviderConfig(cfg.AnthropicAPIKey, cfg.OpenAIAPIKey, cfg.OpenRouterAPIKey)

	// Log orchestration availability.
	if cfg.AnthropicAPIKey != "" {
		slog.Info("Anthropic API configured — orchestration enabled")
	} else {
		slog.Info("Anthropic API not configured — orchestration disabled")
	}

	docsSpaceService := service.NewDocsSpaceService(docsSpaceRepo)
	docsCollectionService := service.NewDocsCollectionService(docsCollectionRepo, docsSpaceRepo)
	docsDocumentService := service.NewDocsDocumentService(docsDocumentRepo, docsSpaceRepo)
	docsContentService := service.NewDocsContentService(docsContentRepo)
	docsVersionService := service.NewDocsVersionService(docsVersionRepo, docsContentRepo)
	docsLinkService := service.NewDocsLinkService(docsLinkRepo, pmStoryRepo, docsDocumentRepo)
	docsHelpcenterService := service.NewDocsHelpcenterService(docsHelpcenterRepo, docsDocumentRepo, docsSpaceRepo, docsCollectionRepo, s3Client)
	docsSearchService := service.NewDocsSearchService(docsSearchRepo)

	crmContactService := service.NewCRMContactService(crmContactRepo)
	crmCompanyService := service.NewCRMCompanyService(crmCompanyRepo)
	crmDealService := service.NewCRMDealService(crmDealRepo, crmAssociationRepo)
	crmAssociationService := service.NewCRMAssociationService(crmAssociationRepo)
	associationsService := service.NewAssociationsService(crmAssociationRepo, pmStoryLinkRepo, pmStoryRepo, supportConversationRepo, docsLinkRepo, docsDocumentRepo)
	crmActivityService := service.NewCRMActivityService(crmActivityRepo)
	crmPropertyService := service.NewCRMPropertyService(crmPropertyRepo)
	crmListService := service.NewCRMListService(crmListRepo)
	crmImportService := service.NewCRMImportService(crmImportRepo, crmContactRepo, crmCompanyRepo, crmDealRepo)

	// Gmail OAuth + encryption setup.
	gmailOAuth := oauth.NewGmailOAuthClient(cfg.GmailClientID, cfg.GmailClientSecret, cfg.GmailOAuthRedirectURL)
	var encryptionKey []byte
	if cfg.CRMEncryptionKey != "" {
		var err error
		encryptionKey, err = hex.DecodeString(cfg.CRMEncryptionKey)
		if err != nil {
			slog.Warn("invalid CRM_ENCRYPTION_KEY (must be hex-encoded)", "error", err)
		}
	}
	gmailSyncClient := syncpkg.NewGmailSyncClient(gmailOAuth, crmEmailRepo, encryptionKey)
	if gmailOAuth != nil {
		slog.Info("Gmail OAuth configured")
	} else {
		slog.Info("Gmail OAuth not configured — email sync disabled")
	}

	// Initialize LLM provider for signal detection and deal automation.
	var llmProvider llm.Provider
	switch cfg.CRMLLMProvider {
	case "openai":
		llmProvider = llm.NewOpenAIProvider(cfg.CRMLLMAPIKey, cfg.CRMLLMBaseURL, cfg.CRMLLMModel)
	default:
		llmProvider = llm.NewClaudeProvider(cfg.AnthropicAPIKey)
	}
	if llmProvider != nil {
		slog.Info("LLM provider configured for signal detection")
	}

	crmSummaryService := service.NewCRMSummaryService(crmSummaryRepo, crmContactRepo, crmCompanyRepo, crmDealRepo, crmAssociationRepo, crmSignalRepo, crmEmailRepo, llmProvider, temporalClient)
	crmEmailService := service.NewCRMEmailService(crmEmailRepo, crmContactRepo, workspaceRepo, crmEmailSyncSettingsRepo, gmailOAuth, encryptionKey, gmailSyncClient, temporalClient, crmSummaryService)
	crmCalendarService := service.NewCRMCalendarService(crmCalendarRepo)
	crmEnrichmentService := service.NewCRMEnrichmentService(crmEnrichmentRepo)
	crmSignalService := service.NewCRMSignalService(crmSignalRepo, crmSummaryService)
	crmSuggestionService := service.NewCRMSuggestionService(crmSuggestionRepo, crmDealRepo, crmAssociationRepo)
	crmSequenceService := service.NewCRMSequenceService(crmSequenceRepo)
	crmWritingProfileService := service.NewCRMWritingProfileService(crmWritingProfileRepo)
	crmSearchService := service.NewCRMSearchService(crmContactRepo, crmCompanyRepo, crmDealRepo)

	signalDetectionService := service.NewSignalDetectionService(llmProvider, crmSignalRepo, crmSummaryService)
	dealAutomationService := service.NewDealAutomationService(llmProvider, crmDealRepo, crmSignalRepo, crmSuggestionRepo, crmContactRepo, crmAssociationRepo, crmAutonomyRepo)
	_ = signalDetectionService // Used by Temporal workers

	orgService := service.NewOrganizationService(orgRepo)
	compositeDefaults := service.NewCompositeDefaultsInitializer(pmWorkflowService, crmDealService)
	workspaceService := service.NewWorkspaceService(workspaceRepo, pmAttachmentRepo, s3Client, compositeDefaults)
	quarterService := service.NewRewardQuarterService(quarterRepo, sprintRepo)
	sprintService := service.NewRewardSprintService(sprintRepo, scoringRepo)
	goalService := service.NewRewardGoalService(goalRepo)
	bonusService := service.NewRewardBonusService(bonusRepo, scoringRepo)
	settingsService := service.NewSettingsService(settingsRepo, pmWorkflowService, cfg.BraveSearchAPIKey)
	auditService := service.NewRewardAuditService(bonusRepo)
	draftService := service.NewRewardDraftService(draftRepo)
	inviteService := service.NewInviteService(invitationRepo, workspaceRepo, orgRepo, userRepo, settingsRepo, emailClient, cfg.AppBaseURL, jwtManager)
	orchestrationService := service.NewOrchestrationService(pmEpicRepo, agentRepo, pmActivityService, wsPublisher)

	// Initialize authorization service.
	authzMemberRepo := authorization.NewGORMMemberRepository(db)
	authzService := authorization.NewAuthzService(db, authzMemberRepo)

	// Inject authorization into WebSocket handler for workspace access checks.
	wsHandler.SetAuthzService(authzService)

	// Initialize handlers.
	handlers := router.Handlers{
		Health:             handler.NewHealthHandler(),
		Auth:               handler.NewAuthHandler(authService),
		Organization:       handler.NewOrganizationHandler(orgService),
		Workspace:          handler.NewWorkspaceHandler(workspaceService),
		RewardQuarter:      handler.NewRewardQuarterHandler(quarterService),
		RewardSprint:       handler.NewRewardSprintHandler(sprintService),
		RewardGoal:         handler.NewRewardGoalHandler(goalService),
		RewardBonus:        handler.NewRewardBonusHandler(bonusService),
		RewardFinance:      handler.NewRewardFinanceHandler(bonusService),
		Settings:           handler.NewSettingsHandler(settingsService),
		RewardAudit:        handler.NewRewardAuditHandler(auditService),
		RewardDraft:        handler.NewRewardDraftHandler(draftService),
		Invite:             handler.NewInviteHandler(inviteService),
		PMWorkflow:         handler.NewPMWorkflowHandler(pmWorkflowService),
		PMImport:           handler.NewPMImportHandler(pmImportService),
		PMLabel:            handler.NewPMLabelHandler(pmLabelService),
		PMEpic:             handler.NewPMEpicHandler(pmEpicService),
		PMSprint:           handler.NewPMSprintHandler(pmSprintService),
		PMStory:            handler.NewPMStoryHandler(pmStoryService),
		PMComment:          handler.NewPMCommentHandler(pmCommentService),
		PMAttachment:       handler.NewPMAttachmentHandler(pmAttachmentService),
		PMObjective:        handler.NewPMObjectiveHandler(pmObjectiveService),
		PMChecklistItem:    handler.NewPMChecklistItemHandler(pmChecklistItemService),
		PMExternalLink:     handler.NewPMExternalLinkHandler(pmExternalLinkService),
		PMView:             handler.NewPMViewHandler(pmViewService),
		Search:             handler.NewSearchHandler(searchService),
		PMAutomation:       handler.NewPMAutomationHandler(pmAutomationService),
		PMStoryTemplate:    handler.NewPMStoryTemplateHandler(pmStoryTemplateService),
		Agent:              handler.NewAgentHandler(agentService),
		SupportInbox:       handler.NewSupportInboxHandler(supportInboxService, agentService),
		SupportInboxWidget: handler.NewSupportInboxWidgetHandler(supportInboxService),
		Git:                handler.NewGitHandler(gitService),
		Orchestration:      handler.NewOrchestrationHandler(orchestrationService),
		Notification:       handler.NewNotificationHandler(notificationService, followerService),
		UserNotifSettings:  handler.NewUserNotificationSettingsHandler(userNotifSettingsService),
		CRMContact:         handler.NewCRMContactHandler(crmContactService),
		CRMCompany:         handler.NewCRMCompanyHandler(crmCompanyService),
		CRMDeal:            handler.NewCRMDealHandler(crmDealService),
		CRMAssociation:     handler.NewCRMAssociationHandler(crmAssociationService),
		Associations:       handler.NewAssociationsHandler(associationsService),
		CRMActivity:        handler.NewCRMActivityHandler(crmActivityService),
		CRMProperty:        handler.NewCRMPropertyHandler(crmPropertyService),
		CRMList:            handler.NewCRMListHandler(crmListService),
		CRMImport:          handler.NewCRMImportHandler(crmImportService),
		CRMEmail:           handler.NewCRMEmailHandler(crmEmailService, cfg.AppBaseURL),
		CRMCalendar:        handler.NewCRMCalendarHandler(crmCalendarService),
		CRMEnrichment:      handler.NewCRMEnrichmentHandler(crmEnrichmentService),
		CRMSignal:          handler.NewCRMSignalHandler(crmSignalService),
		CRMSummary:         handler.NewCRMSummaryHandler(crmSummaryService),
		CRMSuggestion:      handler.NewCRMSuggestionHandler(crmSuggestionService),
		CRMSequence:        handler.NewCRMSequenceHandler(crmSequenceService),
		CRMWritingProfile:  handler.NewCRMWritingProfileHandler(crmWritingProfileService),
		CRMSearch:          handler.NewCRMSearchHandler(crmSearchService),
		CRMDealAutomation:  handler.NewCRMDealAutomationHandler(dealAutomationService),
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

	if err := crmSummaryService.EnsureDailyReconciliation(context.Background()); err != nil {
		slog.Error("failed to ensure crm summary daily reconciliation workflow", "error", err)
	}

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

	// Start background ticker for digest email delivery.
	digestDone := make(chan struct{})
	go func() {
		runDigestSweep := func() {
			if err := notificationService.ProcessPendingDigests(context.Background(), time.Now()); err != nil {
				slog.Error("notification digest sweep failed", "error", err)
			}
		}

		runDigestSweep()

		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runDigestSweep()
			case <-digestDone:
				return
			}
		}
	}()

	// Start background ticker for archived notification cleanup (daily).
	cleanupDone := make(chan struct{})
	go func() {
		// Run once on startup, then every 24 hours.
		if count, err := notificationService.CleanupArchivedNotifications(context.Background(), 90); err != nil {
			slog.Error("notification cleanup failed", "error", err)
		} else if count > 0 {
			slog.Info("notification cleanup complete", "deleted_count", count)
		}

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if count, err := notificationService.CleanupArchivedNotifications(context.Background(), 90); err != nil {
					slog.Error("notification cleanup failed", "error", err)
				} else if count > 0 {
					slog.Info("notification cleanup complete", "deleted_count", count)
				}
			case <-cleanupDone:
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
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	slog.Info("server shutting down")
	pgListenerCancel()
	close(automationDone)
	close(digestDone)
	close(cleanupDone)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}
