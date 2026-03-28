package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"go.temporal.io/api/serviceerror"
	tclient "go.temporal.io/sdk/client"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/crawler"
	"github.com/helpin-ai/helpin/server/internal/crmemail"
	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/handler"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/oauth"
	"github.com/helpin-ai/helpin/server/internal/observability"
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

	if err := observability.InitSentry("api"); err != nil {
		log.Fatalf("sentry.Init: %v", err)
	}
	defer observability.Flush(2 * time.Second)
	defer func() {
		if recovered := recover(); recovered != nil {
			observability.CaptureRecovered(recovered)
			observability.Flush(2 * time.Second)
			panic(recovered)
		}
	}()

	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		fatalWithSentry("failed to load config", err)
	}

	// Initialize structured logger.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLogLevel(cfg.LogLevel)}))
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
		fatalWithSentry("failed to connect to database", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		fatalWithSentry("failed to get underlying sql.DB", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		fatalWithSentry("failed to ping database", err)
	}
	slog.Info("connected to database")

	// Ensure pgcrypto extension is available for gen_random_uuid().
	slog.Info("startup: enabling pgcrypto extension")
	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`).Error; err != nil {
		fatalWithSentry("failed to enable pgcrypto extension", err)
	}
	slog.Info("startup: enabling vector extension")
	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS vector`).Error; err != nil {
		fatalWithSentry("failed to enable vector extension", err)
	}

	slog.Info("startup: running MigrateAgentRunTargets")
	if err := repository.MigrateAgentRunTargets(db); err != nil {
		fatalWithSentry("failed to migrate agent run targets", err)
	}

	slog.Info("startup: running MigratePMImportSchema")
	if err := repository.MigratePMImportSchema(db); err != nil {
		fatalWithSentry("failed to migrate pm import schema", err)
	}

	slog.Info("startup: running MigrateDropDocType")
	if err := repository.MigrateDropDocType(db); err != nil {
		fatalWithSentry("failed to migrate drop doc_type", err)
	}

	slog.Info("startup: running MigrateDropRestrictToOwners")
	if err := repository.MigrateDropRestrictToOwners(db); err != nil {
		fatalWithSentry("failed to migrate drop restrict_to_owners", err)
	}

	// Fix: idx_ws_member_ws_user was incorrectly created as a single-column unique
	// index on user_id only. Drop it so AutoMigrate recreates it as composite (workspace_id, user_id).
	if err := db.Exec("DROP INDEX IF EXISTS idx_ws_member_ws_user").Error; err != nil {
		fatalWithSentry("failed to drop incorrect ws member index", err)
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
		&model.JobRoleCriteria{},
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
		&model.AutomationRule{},
		&model.WorkspaceInvitation{},
		&model.InvitationTeamPreassignment{},
		&model.PMTeamEstimateSettings{},
		&model.PMTeamFieldVisibility{},
		&model.Agent{},
		&model.WorkspaceAgentPresetVersion{},
		&model.AgentRun{},
		&model.AgentRunMessage{},
		&model.AgentRunArtifact{},
		&model.PMStoryLink{},
		&model.SupportConversation{},
		&model.SupportMailbox{},
		&model.SupportMailboxMembership{},
		&model.SupportMessage{},
		&model.SupportEmailLog{},
		&model.SupportEmailWebhookEvent{},
		&model.SupportTeammateStatusOverride{},
		&model.SupportCannedResponse{},
		&model.SupportWidgetInstallation{},
		&model.SupportWidgetSession{},
		&model.SupportAttachment{},
		&model.GitIntegration{},
		&model.GitRepository{},
		&model.PMTeamRepoDefault{},
		&model.StoryDeliveryTarget{},
		&model.StoryGitLink{},
		&model.AgentHandoff{},
		&model.PMStoryTemplate{},
		&model.PMRecurringTemplate{},
		&model.PMRecurringRun{},
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
		&model.DocsHelpcenterArticlePublication{},
		&model.DocsHelpcenterSpaceTranslation{},
		&model.DocsHelpcenterCollectionTranslation{},
		&model.DocsHelpcenterArticleTranslation{},
		&model.DocsSlugAlias{},
		&model.DocsRedirect{},
		&model.DocsReviewQueue{},
		&model.DocsArticleFeedback{},
		&model.DocsComment{},
		&model.DocsImportJob{},
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
		&model.AutomationHealthSnapshot{},
		// AI Support Agent
		&model.AgentKnowledgeSource{},
		&model.AgentContentSource{},
		&model.AIMessageProcessing{},
		&model.DocsChunk{},
		&model.SupportContentSource{},
		&model.SupportContentPage{},
		&model.SupportContentChunk{},
	); err != nil {
		fatalWithSentry("failed to auto-migrate", err)
	}
	slog.Info("startup: AutoMigrate complete")

	slog.Info("startup: running MigrateAgentSchema")
	if err := repository.MigrateAgentSchema(db); err != nil {
		fatalWithSentry("failed to migrate agent schema", err)
	}

	for _, stmt := range []string{
		`CREATE INDEX IF NOT EXISTS idx_docs_chunks_embedding_ivfflat ON docs_chunks USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100)`,
		`CREATE INDEX IF NOT EXISTS idx_docs_chunks_fts ON docs_chunks USING GIN ((setweight(to_tsvector('english', COALESCE(title, '')), 'A') || setweight(to_tsvector('english', COALESCE(content, '')), 'B')))`,
		`CREATE INDEX IF NOT EXISTS idx_support_content_chunks_embedding_ivfflat ON support_content_chunks USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100)`,
		`CREATE INDEX IF NOT EXISTS idx_support_content_chunks_fts ON support_content_chunks USING GIN ((setweight(to_tsvector('english', COALESCE(title, '')), 'A') || setweight(to_tsvector('english', COALESCE(content, '')), 'B')))`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			slog.Warn("failed to create docs chunk index", "error", err, "stmt", stmt)
		}
	}

	slog.Info("startup: running MigrateEmailFallbackSchema")
	if err := repository.MigrateEmailFallbackSchema(db); err != nil {
		fatalWithSentry("failed to migrate email fallback schema", err)
	}

	// Drop legacy ticket_id columns (renamed to conversation_id in migration 039).
	for _, stmt := range []string{
		"ALTER TABLE support_messages DROP COLUMN IF EXISTS ticket_id",
		"ALTER TABLE support_widget_sessions DROP COLUMN IF EXISTS ticket_id",
	} {
		if err := db.Exec(stmt).Error; err != nil {
			slog.Error("failed to drop legacy ticket_id column", "error", err, "stmt", stmt)
		}
	}

	// Post-AutoMigrate schema migrations that reference tables created above.
	slog.Info("startup: running MigrateWorkspaceMemberSchema")
	if err := repository.MigrateWorkspaceMemberSchema(db); err != nil {
		fatalWithSentry("failed to migrate workspace member schema", err)
	}
	slog.Info("startup: running DropLegacyWorkspaceIdentitySchema")
	if err := repository.DropLegacyWorkspaceIdentitySchema(db); err != nil {
		fatalWithSentry("failed to drop legacy workspace identity schema", err)
	}
	slog.Info("startup: running MigrateCRMEmailAssociations")
	if err := repository.MigrateCRMEmailAssociations(db); err != nil {
		fatalWithSentry("failed to migrate crm email associations", err)
	}
	slog.Info("startup: running MigrateCRMSignalSchema")
	if err := repository.MigrateCRMSignalSchema(db); err != nil {
		fatalWithSentry("failed to migrate crm signal schema", err)
	}
	slog.Info("startup: running MigrateCRMSummarySchema")
	if err := repository.MigrateCRMSummarySchema(db); err != nil {
		fatalWithSentry("failed to migrate crm summary schema", err)
	}
	slog.Info("startup: running MigrateAutomationHealthSchema")
	if err := repository.MigrateAutomationHealthSchema(db); err != nil {
		fatalWithSentry("failed to migrate automation health schema", err)
	}
	slog.Info("startup: running MigrateDocsRedirectPaths")
	if err := repository.MigrateDocsRedirectPaths(db); err != nil {
		slog.Error("failed to migrate docs redirect paths", "error", err)
		os.Exit(1)
	}

	// Migrate existing workspaces to organizations (one-time, idempotent).
	slog.Info("startup: running MigrateWorkspacesToOrganizations")
	if err := repository.MigrateWorkspacesToOrganizations(db); err != nil {
		fatalWithSentry("failed to migrate workspaces to organizations", err)
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
	s3Client := storage.NewS3Client(
		cfg.AWSAccessKeyID,
		cfg.AWSSecretAccessKey,
		cfg.AWSBucket,
		cfg.AWSRegion,
		cfg.AWSEndpointURL,
		cfg.AWSPublicBaseURL,
	)
	if s3Client != nil {
		slog.Info("S3 storage configured")
	} else {
		slog.Info("S3 storage not configured — attachments disabled")
	}

	// Initialize JWT manager.
	jwtManager := auth.NewJWTManager(cfg.JWTSecret)

	// Initialize WebSocket hub and publisher.
	wsHub := ws.NewHub()
	podID := os.Getenv("HOSTNAME") // K8s sets this to pod name
	if podID == "" {
		podID = fmt.Sprintf("pod-%d", time.Now().UnixNano()%10000)
	}

	// Initialize Redis relay for cross-pod event broadcasting (optional).
	var redisRelay *ws.RedisRelay
	var redisClient *redis.Client
	if cfg.RedisURL != "" {
		redisOpts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			fatalWithSentry("invalid REDIS_URL", err)
		}
		redisClient = redis.NewClient(redisOpts)
		if err := redisClient.Ping(context.Background()).Err(); err != nil {
			fatalWithSentry("Redis unreachable at startup — cannot run multi-pod", err)
		}
		redisRelay = ws.NewRedisRelay(redisClient, wsHub, podID)
		redisRelayCtx, redisRelayCancel := context.WithCancel(context.Background())
		go redisRelay.Start(redisRelayCtx)
		_ = redisRelayCancel // stored for shutdown
		slog.Info("Redis connected for WebSocket scaling", "url", cfg.RedisURL, "pod", podID)
	} else {
		slog.Info("REDIS_URL not set — running in local-only mode (single pod)")
	}

	wsHub.SetRelay(redisRelay) // nil in local-only mode

	// When Redis is available, use RedisPresence for shared state across pods.
	// Otherwise, the default in-memory PresenceState set in NewHub() is used.
	if cfg.RedisURL != "" {
		redisOpts2, _ := redis.ParseURL(cfg.RedisURL)
		presenceRedis := redis.NewClient(redisOpts2)
		wsHub.SetPresenceProvider(ws.NewRedisPresence(presenceRedis, podID))
		slog.Info("Redis presence provider enabled")
	}

	wsPublisher := ws.NewPublisher(wsHub, redisRelay)
	wsHandler := ws.NewHandler(wsHub, jwtManager)

	// Start JetStream -> WS bridge for cross-process events (e.g. Temporal worker).
	realtimeCtx, realtimeCancel := context.WithCancel(context.Background())
	realtimeInstanceID := ws.ResolveRealtimeInstanceID()
	natsConn, jetstream, err := ws.ConnectJetStream(cfg.NatsURL, "helpin-api-"+realtimeInstanceID)
	if err != nil {
		fatalWithSentry("failed to connect to NATS", err, "url", cfg.NatsURL)
	}
	defer natsConn.Close()
	if err := ws.EnsureJetStreamInfrastructure(jetstream); err != nil {
		fatalWithSentry("failed to ensure JetStream infrastructure", err)
	}
	jetstreamBridge := ws.NewJetStreamBridge(jetstream, wsHub, realtimeInstanceID)
	go func() {
		if err := jetstreamBridge.Start(realtimeCtx); err != nil {
			fatalWithSentry("jetstream bridge stopped", err)
		}
	}()

	// Initialize repositories.
	userRepo := repository.NewUserRepository(db)
	orgRepo := repository.NewOrganizationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	crmAutonomyRepo := repository.NewCRMAutonomyRepository(db)
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
	automationRuleRepo := repository.NewAutomationRuleRepository(db)
	pmStoryTemplateRepo := repository.NewPMStoryTemplateRepository(db)
	pmRecurringTemplateRepo := repository.NewPMRecurringTemplateRepository(db)
	searchRepo := repository.NewSearchRepository(db)
	invitationRepo := repository.NewInvitationRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	workspacePresetVersionRepo := repository.NewWorkspaceAgentPresetVersionRepository(db)
	agentRunRepo := repository.NewAgentRunRepository(db)
	agentRunMessageRepo := repository.NewAgentRunMessageRepository(db)
	agentRunRepo.SetNotifier(ws.NewRunNotifier(wsPublisher)) // publishes run events via Redis/local Hub
	agentRunArtifactRepo := repository.NewAgentRunArtifactRepository(db)
	pmStoryLinkRepo := repository.NewPMStoryLinkRepository(db)
	supportConversationRepo := repository.NewSupportConversationRepository(db)
	supportMailboxRepo := repository.NewSupportMailboxRepository(db)
	supportMessageRepo := repository.NewSupportMessageRepository(db)
	supportEmailLogRepo := repository.NewSupportEmailLogRepository(db)
	supportEmailWebhookEventRepo := repository.NewSupportEmailWebhookEventRepository(db)
	supportInstallRepo := repository.NewSupportInboxInstallationRepository(db)
	supportSessionRepo := repository.NewSupportInboxSessionRepository(db)
	supportAttachmentRepo := repository.NewSupportAttachmentRepository(db)
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
	docsHelpcenterTranslationRepo := repository.NewDocsHelpcenterTranslationRepository(db)
	docsHelpcenterPublicationRepo := repository.NewDocsHelpcenterPublicationRepository(db)
	docsSearchRepo := repository.NewDocsSearchRepository(db)
	docsImportRepo := repository.NewDocsImportRepository(db)
	docsRedirectRepo := repository.NewDocsRedirectRepository(db)
	docsChunkRepo := repository.NewDocsChunkRepository(db)
	supportContentSourceRepo := repository.NewSupportContentSourceRepository(db)
	agentContentSourceRepo := repository.NewAgentContentSourceRepository(db)
	supportContentPageRepo := repository.NewSupportContentPageRepository(db)
	supportContentChunkRepo := repository.NewSupportContentChunkRepository(db)
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
	automationHealthRepo := repository.NewAutomationHealthRepository(db)

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
	authService := service.NewAuthService(userRepo, orgRepo, jwtManager, s3Client)
	pmActivityService := service.NewPMActivityService(pmActivityRepo)
	pmLabelService := service.NewPMLabelService(pmLabelRepo, wsPublisher)
	pmStoryTemplateService := service.NewPMStoryTemplateService(pmStoryTemplateRepo, wsPublisher)
	pmRecurringTemplateService := service.NewPMRecurringTemplateService(pmRecurringTemplateRepo, pmStoryRepo, pmWorkflowRepo, pmSprintRepo, workspaceRepo, pmChecklistItemRepo, pmExternalLinkRepo, pmActivityService, wsPublisher)
	pmWorkflowService := service.NewPMWorkflowService(pmWorkflowRepo, pmStoryRepo, pmLabelRepo, wsPublisher)
	pmAutomationService := service.NewPMAutomationService(pmAutomationRepo, pmEpicRepo, pmStoryRepo, pmSprintRepo, pmWorkflowRepo, pmActivityService, wsPublisher)
	automationHealthService := service.NewAutomationHealthService(automationHealthRepo)
	pmAutomationService.SetHealthObserver(automationHealthService)
	notificationService := service.NewNotificationService(notificationRepo, notificationPrefRepo, userNotifSettingsRepo, followerRepo, userRepo, workspaceRepo, wsPublisher, emailClient, cfg.AppBaseURL)
	userNotifSettingsService := service.NewUserNotificationSettingsService(userNotifSettingsRepo)
	followerService := service.NewFollowerService(followerRepo)
	pmStoryService := service.NewPMStoryService(pmStoryRepo, workspaceRepo, pmWorkflowRepo, pmLabelRepo, pmChecklistItemRepo, pmExternalLinkRepo, pmAttachmentRepo, pmActivityService, wsPublisher, pmAutomationService, notificationService, followerService)
	pmRoadmapRepo := repository.NewPMRoadmapRepository(db)
	pmEpicService := service.NewPMEpicService(pmEpicRepo, pmStoryRepo, pmLabelRepo, gitRepositoryRepo, pmAttachmentRepo, workspaceRepo, pmActivityService, wsPublisher, notificationService)
	pmRoadmapService := service.NewPMRoadmapService(pmEpicService, pmRoadmapRepo)
	pmSprintService := service.NewPMSprintService(pmSprintRepo, pmLabelRepo, pmAttachmentRepo, workspaceRepo, pmActivityService, wsPublisher, notificationService)
	pmCommentService := service.NewPMCommentService(pmCommentRepo, pmStoryRepo, pmAttachmentRepo, pmActivityService, wsPublisher, notificationService, workspaceRepo)
	pmAttachmentService := service.NewPMAttachmentService(pmAttachmentRepo, s3Client, wsPublisher)
	pmObjectiveService := service.NewPMObjectiveService(pmObjectiveRepo, pmKeyResultRepo, pmLabelRepo, pmAttachmentRepo, workspaceRepo, pmActivityService, wsPublisher, notificationService)
	pmChecklistItemService := service.NewPMChecklistItemService(pmChecklistItemRepo, pmStoryRepo, wsPublisher, notificationService, workspaceRepo)
	pmExternalLinkService := service.NewPMExternalLinkService(pmExternalLinkRepo, wsPublisher)
	pmViewService := service.NewPMViewService(pmViewRepo, wsPublisher)
	pmImportService := service.NewPMImportService(db, workspaceRepo, pmWorkflowRepo, pmAttachmentService)
	searchService := service.NewSearchService(searchRepo)
	cannedResponseRepo := repository.NewSupportCannedResponseRepository(db)
	supportTeammateStatusOverrideRepo := repository.NewSupportTeammateStatusOverrideRepository(db)
	supportInboxService := service.NewSupportInboxService(supportConversationRepo, supportMailboxRepo, supportMessageRepo, agentRepo, crmAssociationRepo, supportInstallRepo, supportSessionRepo, cannedResponseRepo, pmActivityService, wsPublisher, crmContactRepo, userRepo, docsSpaceRepo, docsCollectionRepo, docsHelpcenterRepo)
	supportLinkPreviewService := service.NewSupportLinkPreviewService(cfg.CrawlerProxyURLs)
	emailFallbackService := service.NewEmailFallbackService(
		redisClient,
		wsHub,
		wsPublisher,
		emailClient,
		supportMessageRepo,
		supportConversationRepo,
		supportEmailLogRepo,
		supportEmailWebhookEventRepo,
		supportInstallRepo,
		supportSessionRepo,
		workspaceRepo,
		cfg.SupportEmailReplyDomain,
		cfg.AppBaseURL,
		podID,
	)
	supportAttachmentService := service.NewSupportAttachmentService(supportAttachmentRepo, s3Client)
	supportInboxService.SetAttachmentService(supportAttachmentService)
	supportInboxService.SetLinkPreviewService(supportLinkPreviewService)
	supportInboxService.SetEmailFallbackService(emailFallbackService)
	supportInboxService.SetWorkspaceRepo(workspaceRepo)
	supportInboxService.SetPresenceProvider(wsHub.Presence)
	supportInboxService.SetStatusOverrideRepo(supportTeammateStatusOverrideRepo)
	emailFallbackService.SetLinkPreviewService(supportLinkPreviewService)
	notificationService.SetSupportRoutingDependencies(supportInstallRepo, supportMailboxRepo, wsHub.Presence, supportTeammateStatusOverrideRepo)

	// AI Support Agent — new repositories and service
	agentKnowledgeSourceRepo := repository.NewAgentKnowledgeSourceRepository(db)
	aiMessageProcessingRepo := repository.NewAIMessageProcessingRepository(db)
	supportLLMRouter, supportEmbeddingProvider := llm.NewSupportRouter(
		cfg.AnthropicAPIKey,
		cfg.OpenAIAPIKey,
		cfg.OpenAIBaseURL,
		cfg.OpenRouterAPIKey,
		cfg.OpenRouterBaseURL,
	)

	slog.Info("startup: initializing GitHub App client")
	githubAppClient, err := githubapp.NewClient(cfg.GitHubAppID, cfg.GitHubAppPrivateKey)
	if err != nil {
		fatalWithSentry("failed to initialize github app client", err)
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
		workspacePresetVersionRepo,
		agentRunRepo,
		agentRunMessageRepo,
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
	supportInboxService.SetConversationAgentRunner(agentService.RunConversationAgentAuto)
	supportInboxService.SetNotificationService(notificationService, workspaceRepo)
	emailFallbackService.SetNotificationService(notificationService)

	// Automation Rule Engine — wired after agent + story services to break circular deps.
	ruleEngine := service.NewAutomationRuleEngine(
		automationRuleRepo,
		pmStoryRepo,
		pmWorkflowRepo,
		storyDeliveryTargetRepo,
		gitService,
		notificationService,
		pmActivityService,
		wsPublisher,
	)
	ruleEngine.SetAgentService(agentService)
	ruleEngine.SetStoryService(pmStoryService)
	ruleEngine.SetHealthObserver(automationHealthService)
	pmStoryService.SetRuleEngine(ruleEngine)
	pmStoryService.SetAgentService(agentService)
	pmStoryService.SetRecurringService(pmRecurringTemplateService)
	pmRecurringTemplateService.SetStoryService(pmStoryService)
	agentService.SetRuleEngine(ruleEngine)
	agentService.SetWorkflowService(pmWorkflowService)
	pmRecurringTemplateService.SetTemporalClient(temporalClient)

	go func() {
		slog.Info("startup: backfilling built-in agents for existing workspaces")
		workspaceIDs, err := workspaceRepo.ListIDs(context.Background())
		if err != nil {
			slog.Error("failed to list workspaces for built-in agent backfill", "error", err)
			return
		}
		seeded := 0
		for _, workspaceID := range workspaceIDs {
			if err := agentService.SeedWorkspaceDefaults(context.Background(), workspaceID, ""); err != nil {
				slog.Error("failed to backfill built-in agents", "workspace_id", workspaceID, "error", err)
				continue
			}
			seeded++
		}
		slog.Info("startup: built-in agent backfill complete", "workspaces_processed", seeded)
	}()

	// Log orchestration availability.
	if cfg.AnthropicAPIKey != "" {
		slog.Info("Anthropic API configured — orchestration enabled")
	} else {
		slog.Info("Anthropic API not configured — orchestration disabled")
	}

	// Initialize LLM provider for docs translation generation, signal detection, and deal automation.
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

	docsSpaceService := service.NewDocsSpaceService(docsSpaceRepo, wsPublisher)
	docsCollectionService := service.NewDocsCollectionService(docsCollectionRepo, docsSpaceRepo, wsPublisher)
	docsDocumentService := service.NewDocsDocumentService(docsDocumentRepo, docsSpaceRepo, wsPublisher)
	docsContentService := service.NewDocsContentService(docsContentRepo, docsDocumentRepo, wsPublisher)
	docsVersionService := service.NewDocsVersionService(docsVersionRepo, docsContentRepo, docsDocumentRepo, wsPublisher)
	docsLinkService := service.NewDocsLinkService(docsLinkRepo, pmStoryRepo, docsDocumentRepo, wsPublisher)
	docsHelpcenterService := service.NewDocsHelpcenterService(docsHelpcenterRepo, docsHelpcenterPublicationRepo, docsDocumentRepo, docsContentRepo, docsSpaceRepo, docsCollectionRepo, docsRedirectRepo, s3Client, wsPublisher)
	docsHelpcenterTranslationService := service.NewDocsHelpcenterTranslationService(docsHelpcenterTranslationRepo, docsHelpcenterRepo, docsHelpcenterPublicationRepo, docsRedirectRepo, docsDocumentRepo, docsContentRepo, docsSpaceRepo, docsCollectionRepo, llmProvider)
	docsSearchService := service.NewDocsSearchService(docsSearchRepo)
	docsImportService := service.NewDocsImportService(docsImportRepo, docsSpaceService, docsCollectionService, docsDocumentService, docsContentService, docsHelpcenterService, docsRedirectRepo, s3Client)
	docsSpaceService.SetTranslationService(docsHelpcenterTranslationService)
	docsCollectionService.SetTranslationService(docsHelpcenterTranslationService)
	docsDocumentService.SetTranslationService(docsHelpcenterTranslationService)
	docsContentService.SetTranslationService(docsHelpcenterTranslationService)
	docsHelpcenterService.SetTranslationService(docsHelpcenterTranslationService)
	contentCrawler := crawler.NewSmartCrawler(
		cfg.CrawlerMode,
		cfg.CloudflareAccountID,
		cfg.CloudflareAPIToken,
		cfg.CloudflareAPIBaseURL,
		cfg.CrawlerProxyURLs,
		slog.Default(),
	)
	docsEmbeddingService := service.NewDocsEmbeddingService(
		docsChunkRepo,
		agentKnowledgeSourceRepo,
		docsContentRepo,
		docsSpaceRepo,
		docsHelpcenterRepo,
		docsDocumentRepo,
		supportEmbeddingProvider,
		cfg.OpenAIEmbeddingModel,
		runEngine,
	)
	agentKnowledgeSourceService := service.NewAgentKnowledgeSourceService(agentKnowledgeSourceRepo, docsSpaceRepo, docsEmbeddingService)
	supportContentSyncService := service.NewSupportContentSyncService(
		supportContentSourceRepo,
		supportContentPageRepo,
		supportContentChunkRepo,
		supportEmbeddingProvider,
		cfg.OpenAIEmbeddingModel,
		contentCrawler,
		runEngine,
	)
	supportContentSourceService := service.NewSupportContentSourceService(
		supportContentSourceRepo,
		agentRepo,
		agentContentSourceRepo,
		supportContentPageRepo,
		supportContentChunkRepo,
		supportContentSyncService,
	)
	agentContentSourceService := service.NewAgentContentSourceService(
		agentContentSourceRepo,
		agentRepo,
		supportContentSourceRepo,
		supportContentSyncService,
	)

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

	crmSummaryService := service.NewCRMSummaryService(crmSummaryRepo, crmContactRepo, crmCompanyRepo, crmDealRepo, crmAssociationRepo, crmSignalRepo, crmEmailRepo, llmProvider, temporalClient)
	crmEmailService := service.NewCRMEmailService(crmEmailRepo, crmContactRepo, workspaceRepo, crmEmailSyncSettingsRepo, gmailOAuth, encryptionKey, gmailSyncClient, temporalClient, crmSummaryService)
	crmCalendarService := service.NewCRMCalendarService(crmCalendarRepo)
	crmEnrichmentService := service.NewCRMEnrichmentService(crmEnrichmentRepo)
	crmSignalService := service.NewCRMSignalService(crmSignalRepo, crmSummaryService)
	crmSuggestionService := service.NewCRMSuggestionService(crmSuggestionRepo, crmDealRepo, crmAssociationRepo)
	crmSequenceService := service.NewCRMSequenceService(crmSequenceRepo)
	crmWritingProfileService := service.NewCRMWritingProfileService(crmWritingProfileRepo)
	crmSearchService := service.NewCRMSearchService(crmContactRepo, crmCompanyRepo, crmDealRepo)
	commandService := service.NewInternalCommandService(
		agentService,
		pmStoryService,
		crmDealService,
		crmActivityService,
		docsContentService,
		docsLinkService,
		pmStoryRepo,
		pmStoryLinkRepo,
	)
	commandService.SetPMAutomationService(pmAutomationService)
	commandService.SetGitService(gitService)
	ruleEngine.SetCommandService(commandService)

	signalDetectionService := service.NewSignalDetectionService(llmProvider, crmSignalRepo, crmSummaryService)
	dealAutomationService := service.NewDealAutomationService(llmProvider, crmDealRepo, crmSignalRepo, crmSuggestionRepo, crmContactRepo, crmAssociationRepo, crmAutonomyRepo)
	_ = signalDetectionService // Used by Temporal workers

	// AI Support Agent — wire SupportAIService with LLM provider and JetStream.
	supportAIService := service.NewSupportAIService(
		supportLLMRouter, supportEmbeddingProvider, cfg.OpenAIEmbeddingModel, docsChunkRepo,
		agentKnowledgeSourceRepo, supportContentChunkRepo, agentContentSourceRepo, aiMessageProcessingRepo,
		supportConversationRepo, supportMessageRepo, supportAttachmentRepo,
		agentRepo, agentHandoffRepo, supportInstallRepo,
		wsPublisher, jetstream, redisClient, db,
		cfg.QueryExpansionModel, cfg.QueryExpansionProvider,
	)
	supportAIService.SetSupportRoutingDependencies(workspaceRepo, wsHub.Presence, supportTeammateStatusOverrideRepo)
	supportAIService.SetMailboxRepository(supportMailboxRepo)
	supportAIService.SetLinkPreviewService(supportLinkPreviewService)
	supportInboxService.SetSupportAIService(supportAIService)

	orgService := service.NewOrganizationService(orgRepo)
	compositeDefaults := service.NewCompositeDefaultsInitializer(pmWorkflowService, pmAutomationService, crmDealService, supportInboxService, agentService)
	workspaceService := service.NewWorkspaceService(workspaceRepo, pmAttachmentRepo, s3Client, compositeDefaults)
	settingsService := service.NewSettingsService(settingsRepo, pmWorkflowService, wsPublisher)
	automationInventoryService := service.NewAutomationInventoryService(settingsRepo, pmAutomationRepo, crmEmailRepo, automationHealthRepo, automationRuleRepo)
	if err := pmRecurringTemplateService.EnsureScheduler(context.Background()); err != nil {
		slog.Error("failed to ensure PM recurring scheduler", "error", err)
	}
	inviteService := service.NewInviteService(invitationRepo, workspaceRepo, orgRepo, userRepo, settingsRepo, emailClient, cfg.AppBaseURL, jwtManager)
	// Initialize authorization service.
	authzMemberRepo := authorization.NewGORMMemberRepository(db)
	authzService := authorization.NewAuthzService(db, authzMemberRepo)

	// Inject authorization into WebSocket handler for workspace access checks.
	wsHandler.SetAuthzService(authzService)

	// Inject user lookup for agent identity in typing events.
	wsHandler.SetUserLookup(func(ctx context.Context, userID string) (string, *string) {
		user, err := userRepo.GetByID(ctx, userID)
		if err != nil || user == nil {
			return "", nil
		}
		return user.FullName, user.AvatarURL
	})

	// Inject mark-read for support:conversation:read WS messages.
	wsHandler.SetMarkRead(func(ctx context.Context, workspaceID, conversationID, userID string) error {
		return supportInboxService.MarkConversationRead(ctx, workspaceID, conversationID, userID)
	})

	// Widget WebSocket handler — authenticates via session_token, not JWT.
	widgetWsHandler := ws.NewWidgetHandler(wsHub, supportInboxService)

	// Initialize handlers.
	handlers := router.Handlers{
		Health:              handler.NewHealthHandler(s3Client),
		Auth:                handler.NewAuthHandler(authService),
		Organization:        handler.NewOrganizationHandler(orgService),
		Workspace:           handler.NewWorkspaceHandler(workspaceService),
		Settings:            handler.NewSettingsHandler(settingsService, automationInventoryService),
		Invite:              handler.NewInviteHandler(inviteService),
		PMWorkflow:          handler.NewPMWorkflowHandler(pmWorkflowService),
		PMImport:            handler.NewPMImportHandler(pmImportService),
		PMLabel:             handler.NewPMLabelHandler(pmLabelService),
		PMEpic:              handler.NewPMEpicHandler(pmEpicService),
		PMRoadmap:           handler.NewPMRoadmapHandler(pmRoadmapService),
		PMSprint:            handler.NewPMSprintHandler(pmSprintService),
		PMStory:             handler.NewPMStoryHandler(pmStoryService),
		PMComment:           handler.NewPMCommentHandler(pmCommentService),
		PMAttachment:        handler.NewPMAttachmentHandler(pmAttachmentService),
		PMObjective:         handler.NewPMObjectiveHandler(pmObjectiveService),
		PMChecklistItem:     handler.NewPMChecklistItemHandler(pmChecklistItemService),
		PMExternalLink:      handler.NewPMExternalLinkHandler(pmExternalLinkService),
		PMView:              handler.NewPMViewHandler(pmViewService),
		Search:              handler.NewSearchHandler(searchService),
		PMAutomation:        handler.NewPMAutomationHandler(pmAutomationService),
		AutomationRule:      handler.NewAutomationRuleHandler(ruleEngine),
		PMStoryTemplate:     handler.NewPMStoryTemplateHandler(pmStoryTemplateService),
		PMRecurringTemplate: handler.NewPMRecurringTemplateHandler(pmRecurringTemplateService),
		Agent:               handler.NewAgentHandler(agentService),
		SupportInbox:        handler.NewSupportInboxHandler(supportInboxService, agentService),
		SupportInboxWidget:  handler.NewSupportInboxWidgetHandler(supportInboxService),
		SupportAI:           handler.NewSupportAIHandler(supportAIService, supportInboxService, agentKnowledgeSourceService, supportContentSourceService, agentContentSourceService),
		SupportAttachment:   handler.NewSupportAttachmentHandler(supportAttachmentService, supportInboxService),
		PostmarkInbound:     handler.NewPostmarkInboundHandler(emailFallbackService, cfg.PostmarkInboundWebhookSecret),
		AdminWebhookEvent:   handler.NewAdminWebhookEventHandler(supportEmailWebhookEventRepo),
		AdminEmailQueue:     handler.NewAdminEmailQueueHandler(emailFallbackService),
		Git:                 handler.NewGitHandler(gitService),
		Notification:        handler.NewNotificationHandler(notificationService, followerService),
		UserNotifSettings:   handler.NewUserNotificationSettingsHandler(userNotifSettingsService),
		CRMContact:          handler.NewCRMContactHandler(crmContactService),
		CRMCompany:          handler.NewCRMCompanyHandler(crmCompanyService),
		CRMDeal:             handler.NewCRMDealHandler(crmDealService),
		CRMAssociation:      handler.NewCRMAssociationHandler(crmAssociationService),
		Associations:        handler.NewAssociationsHandler(associationsService),
		CRMActivity:         handler.NewCRMActivityHandler(crmActivityService),
		CRMProperty:         handler.NewCRMPropertyHandler(crmPropertyService),
		CRMList:             handler.NewCRMListHandler(crmListService),
		CRMImport:           handler.NewCRMImportHandler(crmImportService),
		CRMEmail:            handler.NewCRMEmailHandler(crmEmailService, cfg.AppBaseURL),
		CRMCalendar:         handler.NewCRMCalendarHandler(crmCalendarService),
		CRMEnrichment:       handler.NewCRMEnrichmentHandler(crmEnrichmentService),
		CRMSignal:           handler.NewCRMSignalHandler(crmSignalService),
		CRMSummary:          handler.NewCRMSummaryHandler(crmSummaryService),
		CRMSuggestion:       handler.NewCRMSuggestionHandler(crmSuggestionService),
		CRMSequence:         handler.NewCRMSequenceHandler(crmSequenceService),
		CRMWritingProfile:   handler.NewCRMWritingProfileHandler(crmWritingProfileService),
		CRMSearch:           handler.NewCRMSearchHandler(crmSearchService),
		CRMDealAutomation:   handler.NewCRMDealAutomationHandler(dealAutomationService),
		SDKAssets: func() *handler.SDKAssetsHandler {
			sdkDist := os.Getenv("SDK_DIST_DIR")
			if sdkDist == "" {
				sdkDist = "../packages/sdk-js/dist"
			}
			return handler.NewSDKAssetsHandler(sdkDist)
		}(),
		Docs: handler.NewDocsHandler(
			docsSpaceService,
			docsCollectionService,
			docsDocumentService,
			docsContentService,
			docsVersionService,
			docsLinkService,
			docsHelpcenterService,
			docsHelpcenterTranslationService,
			docsSearchService,
			docsImportService,
			docsEmbeddingService,
			agentService,
			jwtManager,
		),
	}

	// Slug resolver adapts workspace repo for RBAC middleware.
	slugResolver := authorization.SlugResolver(func(ctx context.Context, slug string) (string, error) {
		ws, err := workspaceRepo.GetBySlug(ctx, slug)
		if err != nil {
			return "", err
		}
		if ws == nil {
			return "", errors.New("workspace not found")
		}
		return ws.ID, nil
	})

	// Set up router.
	r := router.New(handlers, jwtManager, authzService, slugResolver, cfg.CORSOrigins)

	if err := crmSummaryService.EnsureDailyReconciliation(context.Background()); err != nil {
		slog.Error("failed to ensure crm summary daily reconciliation workflow", "error", err)
	}

	// Start sprint automation cron workflow via Temporal (replaces local ticker).
	if temporalClient != nil {
		if err := ensureSprintCronWorkflow(temporalClient); err != nil {
			slog.Error("failed to ensure sprint cron workflow", "error", err)
		}
	}

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

	// Start background ticker for delayed support reply fallback emails.
	supportReplyEmailDone := make(chan struct{})
	go func() {
		runSupportReplySweep := func() {
			if err := notificationService.ProcessPendingSupportReplyEmails(context.Background(), time.Now()); err != nil {
				slog.Error("support reply email sweep failed", "error", err)
			}
		}

		runSupportReplySweep()

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runSupportReplySweep()
			case <-supportReplyEmailDone:
				return
			}
		}
	}()

	// Start the email fallback poller only when both Redis and Postmark are available.
	var emailFallbackCancel context.CancelFunc
	if redisClient != nil && emailClient != nil {
		var emailFallbackCtx context.Context
		emailFallbackCtx, emailFallbackCancel = context.WithCancel(context.Background())
		go emailFallbackService.StartPoller(emailFallbackCtx)
	}

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
	if wsHandler != nil || widgetWsHandler != nil {
		var sentryWSHandler http.Handler
		if wsHandler != nil {
			sentryWSHandler = middleware.SentryHTTP(middleware.SentryRequestContext(wsHandler))
		}
		var sentryWidgetWSHandler http.Handler
		if widgetWsHandler != nil {
			sentryWidgetWSHandler = middleware.SentryHTTP(middleware.SentryRequestContext(widgetWsHandler))
		}
		topHandler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.URL.Path == "/api/ws" && sentryWSHandler != nil {
				sentryWSHandler.ServeHTTP(w, req)
				return
			}
			if req.URL.Path == "/widget/ws" && sentryWidgetWSHandler != nil {
				sentryWidgetWSHandler.ServeHTTP(w, req)
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
			fatalWithSentry("server error", err)
		}
	}()

	<-done
	slog.Info("server shutting down")
	realtimeCancel()
	if emailFallbackCancel != nil {
		emailFallbackCancel()
	}
	close(digestDone)
	close(supportReplyEmailDone)
	close(cleanupDone)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		fatalWithSentry("server forced to shutdown", err)
	}

	slog.Info("server stopped")
}

func parseLogLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ensureSprintCronWorkflow starts the sprint automation cron workflow if not already running.
func ensureSprintCronWorkflow(client tclient.Client) error {
	if client == nil {
		return nil
	}
	_, err := client.ExecuteWorkflow(context.Background(), tclient.StartWorkflowOptions{
		ID:           "sprint-automation-cron",
		TaskQueue:    temporalapp.QueueAutomation,
		CronSchedule: "0 * * * *",
	}, temporalapp.SprintAutomationCronWorkflow, temporalapp.SprintAutomationInput{})
	if err != nil {
		// Already running is not an error.
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &alreadyStarted) {
			return nil
		}
		return fmt.Errorf("start sprint cron workflow: %w", err)
	}
	slog.Info("sprint automation cron workflow started")
	return nil
}

func fatalWithSentry(message string, err error, attrs ...any) {
	if err != nil {
		observability.CaptureException(err)
	}
	logAttrs := make([]any, 0, len(attrs)+2)
	if err != nil {
		logAttrs = append(logAttrs, "error", err)
	}
	logAttrs = append(logAttrs, attrs...)
	slog.Error(message, logAttrs...)
	observability.Flush(2 * time.Second)
	os.Exit(1)
}
