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
	workflowservice "go.temporal.io/api/workflowservice/v1"
	tclient "go.temporal.io/sdk/client"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/billingstripe"
	"github.com/helpin-ai/helpin/server/internal/cache"
	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/crawler"
	"github.com/helpin-ai/helpin/server/internal/crmemail"
	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/geoip"
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
	flowtemplates "github.com/helpin-ai/helpin/server/internal/templates"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	appwebauthn "github.com/helpin-ai/helpin/server/internal/webauthn"
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
				ParameterizedQueries:      true,
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

	if cfg.RunAutoMigrate {
		// Fix: idx_ws_member_ws_user was incorrectly created as a single-column unique
		// index on user_id only. Drop it so AutoMigrate recreates it as composite (workspace_id, user_id).
		if err := db.Exec("DROP INDEX IF EXISTS idx_ws_member_ws_user").Error; err != nil {
			fatalWithSentry("failed to drop incorrect ws member index", err)
		}

		// Auto-migrate all models.
		slog.Info("startup: running AutoMigrate")
		if err := db.AutoMigrate(
			&model.User{},
			&model.UserPasskey{},
			&model.PasswordResetToken{},
			&model.EmailVerificationToken{},
			&model.Organization{},
			&model.OrganizationMember{},
			&model.Workspace{},
			&model.WorkspaceMember{},
			&model.WorkspaceModuleGrant{},
			&model.WorkspaceSettings{},
			&model.SetupGoal{},
			&model.SetupIntent{},
			&model.SetupAchievement{},
			&model.SetupActionIntent{},
			&model.MemberSetupPreference{},
			&model.WorkspaceBilling{},
			&model.BillingCreditLedgerEntry{},
			&model.StripeWebhookEvent{},
			&model.OrganizationBilling{},
			&model.BillingPaymentMethod{},
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
			&model.PMTask{},
			&model.PMTaskOwner{},
			&model.PMTaskFollower{},
			&model.PMTaskLabel{},
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
			&model.SupportInboxView{},
			&model.PMAutomation{},
			&model.AutomationRule{},
			&model.WorkspaceInvitation{},
			&model.InvitationTeamPreassignment{},
			&model.PMTeamEstimateSettings{},
			&model.PMTeamFieldVisibility{},
			&model.Agent{},
			&model.AgentTeamAccess{},
			&model.AgentTemplate{},
			&model.WorkspaceAgentPresetVersion{},
			&model.AgentVersion{},
			&model.WorkspaceSkill{},
			&model.AgentRun{},
			&model.AgentTriggerExecution{},
			&model.AgentRunMessage{},
			&model.AgentRunArtifact{},
			&model.AgentRunInteraction{},
			// Public MCP tables are intentionally excluded. Their constraints,
			// partial indexes, and retention fields are owned exclusively by
			// versioned migration 202607100003_public_mcp.sql. Letting GORM
			// reconcile those tables can attempt incompatible constraint changes.
			&model.CommandBarPlanRecord{},
			&model.CommandBarPlanDismissal{},
			&model.DockChat{},
			&model.SupportRunEvidence{},
			&model.CodingSessionStateSnapshot{},
			&model.CodexWorkspaceAuth{},
			&model.PMTaskLink{},
			&model.SupportConversation{},
			&model.SupportConversationTriage{},
			&model.SupportConversationTriageEvent{},
			&model.SupportMailbox{},
			&model.SupportMailboxMembership{},
			&model.SupportTriageRule{},
			&model.SupportEmailRoute{},
			&model.SupportEmailSender{},
			&model.SupportEmailSenderMailbox{},
			&model.SupportEmailSenderDomain{},
			&model.SupportMessage{},
			&model.SupportEmailLog{},
			&model.SupportEmailWebhookEvent{},
			&model.SupportTag{},
			&model.SupportConversationTag{},
			&model.SupportTeammateStatusOverride{},
			&model.SupportCannedResponse{},
			&model.SupportWidgetInstallation{},
			&model.SupportWidgetSession{},
			&model.SupportAttachment{},
			&model.SupportEvent{},
			&model.SupportCoverageTopic{},
			&model.SupportCoverageGap{},
			&model.SupportGapEvidence{},
			&model.SupportGapSuggestion{},
			&model.SupportCoverageGapArticle{},
			&model.SupportCoverageSnapshot{},
			&model.SupportCoverageDigestDelivery{},
			&model.SupportCoverageAnalysisRun{},
			&model.SupportCoverageConversationAnalysis{},
			&model.SupportAIRetrievalTrace{},
			&model.SupportCoverageRecommendation{},
			&model.SupportCoverageClusterRebuildRun{},
			&model.SupportCoverageGapMergeSuggestion{},
			&model.SupportCoverageGapPairDecision{},
			&model.GitIntegration{},
			&model.GitCredential{},
			&model.GitRepository{},
			&model.PMTeamRepoDefault{},
			&model.TaskDeliveryTarget{},
			&model.EpicDeliveryTarget{},
			&model.TaskGitLink{},
			&model.GitWebhookEvent{},
			&model.AgentHandoff{},
			&model.PMTaskTemplate{},
			&model.PMRecurringTemplate{},
			&model.PMRecurringRun{},
			&model.PMImportJob{},
			&authorization.AuthorizationRelation{},
			// Docs module
			&model.DocsSpace{},
			&model.DocsSpaceTeam{},
			&model.DocsCollection{},
			&model.DocsDocument{},
			&model.DocsDocumentKey{},
			&model.DocsContent{},
			&model.DocsBlock{},
			&model.DocsAISectionCandidate{},
			&model.DocsChangeProposal{},
			&model.DocsVersion{},
			&model.DocsLink{},
			&model.DocsHelpcenterConfig{},
			&model.DocsHelpcenterArticle{},
			&model.DocsHelpcenterArticlePublication{},
			&model.DocsHelpcenterSearchEntry{},
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
			// CRM Phase 5: Writing
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
			&model.CuratedGuidance{},
		); err != nil {
			fatalWithSentry("failed to auto-migrate", err)
		}
		slog.Info("startup: AutoMigrate complete")
	} else {
		slog.Info("startup: AutoMigrate disabled by RUN_AUTO_MIGRATE")
	}

	slog.Info("startup: running MigrateAgentKnowledgeSourceSchema")
	if err := repository.MigrateAgentKnowledgeSourceSchema(db); err != nil {
		fatalWithSentry("failed to migrate agent knowledge source schema", err)
	}

	slog.Info("startup: running MigrateAgentSchema")
	if err := repository.MigrateAgentSchema(db); err != nil {
		fatalWithSentry("failed to migrate agent schema", err)
	}

	for _, stmt := range []string{
		`CREATE INDEX IF NOT EXISTS idx_docs_chunks_embedding_ivfflat ON docs_chunks USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100)`,
		`CREATE INDEX IF NOT EXISTS idx_docs_chunks_fts ON docs_chunks USING GIN ((setweight(to_tsvector('english', COALESCE(title, '')), 'A') || setweight(to_tsvector('english', COALESCE(content, '')), 'B')))`,
		`CREATE INDEX IF NOT EXISTS idx_support_content_chunks_embedding_ivfflat ON support_content_chunks USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100)`,
		`CREATE INDEX IF NOT EXISTS idx_support_content_chunks_fts ON support_content_chunks USING GIN ((setweight(to_tsvector('english', COALESCE(title, '')), 'A') || setweight(to_tsvector('english', COALESCE(content, '')), 'B')))`,
		`CREATE INDEX IF NOT EXISTS idx_docs_chunks_fts_v2 ON docs_chunks USING GIN ((setweight(to_tsvector('english', COALESCE(title, '')), 'A') || setweight(to_tsvector('english', COALESCE(NULLIF(search_content, ''), content, '')), 'B')))`,
		`CREATE INDEX IF NOT EXISTS idx_support_content_chunks_fts_v2 ON support_content_chunks USING GIN ((setweight(to_tsvector('english', COALESCE(title, '')), 'A') || setweight(to_tsvector('english', COALESCE(NULLIF(search_content, ''), content, '')), 'B')))`,
		`CREATE INDEX IF NOT EXISTS idx_curated_guidance_embedding_ivfflat ON curated_guidance USING ivfflat (embedding vector_cosine_ops) WITH (lists = 20) WHERE embedding IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_curated_guidance_fts ON curated_guidance USING GIN ((setweight(to_tsvector('english', COALESCE(title, '')), 'A') || setweight(to_tsvector('english', COALESCE(array_to_string(question_patterns, ' '), '')), 'A') || setweight(to_tsvector('english', COALESCE(answer, '')), 'B')))`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			slog.Warn("failed to create docs chunk index", "error", err, "stmt", stmt)
		}
	}

	slog.Info("startup: running MigrateEmailFallbackSchema")
	if err := repository.MigrateEmailFallbackSchema(db); err != nil {
		fatalWithSentry("failed to migrate email fallback schema", err)
	}
	slog.Info("startup: running MigrateSupportEmailRouteSchema")
	if err := repository.MigrateSupportEmailRouteSchema(db); err != nil {
		fatalWithSentry("failed to migrate support email route schema", err)
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

	// Initialize email clients (nil if not configured).
	appEmailClient := email.NewClient(cfg.PostmarkAppServerToken, cfg.PostmarkAppFromEmail)
	replyEmailClient := email.NewClient(cfg.PostmarkReplyServerToken, cfg.PostmarkReplyFromEmail)
	postmarkDomainClient := email.NewDomainClient(cfg.PostmarkAccountToken)
	if appEmailClient != nil {
		slog.Info("Postmark app email configured")
	} else {
		slog.Info("Postmark app email not configured — product emails will be logged only")
	}
	if replyEmailClient != nil {
		slog.Info("Postmark support reply email configured")
	} else {
		slog.Info("Postmark support reply email not configured — support reply emails will be logged only")
	}
	if postmarkDomainClient != nil {
		slog.Info("Postmark account domain API configured")
	} else {
		slog.Info("Postmark account domain API not configured — custom sender domain onboarding disabled")
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

	geoIPResolver, err := geoip.Open(geoip.Options{
		Path:        cfg.MaxMindDBPath,
		DownloadURL: cfg.MaxMindDownloadURL,
		AccountID:   cfg.MaxMindAccountID,
		LicenseKey:  cfg.MaxMindLicenseKey,
	})
	if err != nil {
		fatalWithSentry("failed to initialize MaxMind DB", err)
	}
	if geoIPResolver != nil {
		defer func() {
			if closeErr := geoIPResolver.Close(); closeErr != nil {
				slog.Warn("maxmind db close failed", "error", closeErr)
			}
		}()
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
		slog.Info("Redis connected for WebSocket scaling", "addr", redisOpts.Addr, "db", redisOpts.DB, "pod", podID)
	} else {
		slog.Info("REDIS_URL not set — running in local-only mode (single pod)")
	}

	wsHub.SetRelay(redisRelay) // nil in local-only mode

	// When Redis is available, use RedisPresence for shared state across pods.
	// Otherwise, the default in-memory PresenceState set in NewHub() is used.
	if redisClient != nil {
		wsHub.SetPresenceProvider(ws.NewRedisPresence(redisClient, podID))
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
			slog.Error("jetstream bridge stopped (non-fatal in dev)", "error", err)
		}
	}()

	// Initialize repositories.
	userRepo := repository.NewUserRepository(db)
	if len(cfg.PlatformAdminEmails) > 0 {
		updated, err := userRepo.GrantPlatformAdminByEmails(context.Background(), cfg.PlatformAdminEmails)
		if err != nil {
			fatalWithSentry("failed to bootstrap platform admins", err)
		}
		slog.Info("startup: platform admin bootstrap complete", "configured_emails", len(cfg.PlatformAdminEmails), "updated_users", updated)
	}
	passkeyRepo := repository.NewPasskeyRepository(db)
	orgRepo := repository.NewOrganizationRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	mcpRepo := repository.NewMCPRepository(db)
	externalMCPRepo := repository.NewExternalMCPRepository(db)
	if err := mcpRepo.CleanupExpired(context.Background(), time.Now()); err != nil {
		slog.Warn("MCP retention cleanup skipped", "error", err)
	}
	setupRepo := repository.NewSetupRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	moduleGrantRepo := repository.NewWorkspaceModuleGrantRepository(db)
	crmAutonomyRepo := repository.NewCRMAutonomyRepository(db)
	pmWorkflowRepo := repository.NewPMWorkflowRepository(db)
	pmLabelRepo := repository.NewPMLabelRepository(db)
	pmEpicRepo := repository.NewPMEpicRepository(db)
	pmSprintRepo := repository.NewPMSprintRepository(db)
	pmTaskRepo := repository.NewPMTaskRepository(db)
	pmCommentRepo := repository.NewPMCommentRepository(db)
	pmActivityRepo := repository.NewPMActivityRepository(db)
	pmAttachmentRepo := repository.NewPMAttachmentRepository(db)
	pmObjectiveRepo := repository.NewPMObjectiveRepository(db)
	pmKeyResultRepo := repository.NewPMKeyResultRepository(db)
	pmChecklistItemRepo := repository.NewPMChecklistItemRepository(db)
	pmExternalLinkRepo := repository.NewPMExternalLinkRepository(db)
	pmViewRepo := repository.NewPMViewRepository(db)
	pmAutomationRepo := repository.NewPMAutomationRepository(db)
	pmSprintCloseoutRepo := repository.NewPMSprintCloseoutRepository(db)
	automationRuleRepo := repository.NewAutomationRuleRepository(db)
	pmTaskTemplateRepo := repository.NewPMTaskTemplateRepository(db)
	pmRecurringTemplateRepo := repository.NewPMRecurringTemplateRepository(db)
	searchRepo := repository.NewSearchRepository(db)
	invitationRepo := repository.NewInvitationRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	workspacePresetVersionRepo := repository.NewWorkspaceAgentPresetVersionRepository(db)
	agentTemplateRepo := repository.NewAgentTemplateRepository(db)
	workspaceSkillRepo := repository.NewWorkspaceSkillRepository(db)
	agentRunRepo := repository.NewAgentRunRepository(db)
	agentTriggerExecutionRepo := repository.NewAgentTriggerExecutionRepository(db)
	agentRunMessageRepo := repository.NewAgentRunMessageRepository(db)
	agentRunRepo.SetNotifier(ws.NewRunNotifier(wsPublisher)) // publishes run events via Redis/local Hub
	agentRunRepo.SetTriggerExecutionRepository(agentTriggerExecutionRepo)
	agentRunArtifactRepo := repository.NewAgentRunArtifactRepository(db)
	agentRunInteractionRepo := repository.NewAgentRunInteractionRepository(db)
	commandBarPlanRepo := repository.NewCommandBarPlanRepository(db)
	commandBarPlanDismissalRepo := repository.NewCommandBarPlanDismissalRepository(db)
	codingSessionStateSnapshotRepo := repository.NewCodingSessionStateSnapshotRepository(db)
	pmTaskLinkRepo := repository.NewPMTaskLinkRepository(db)
	supportConversationRepo := repository.NewSupportConversationRepository(db)
	supportInboxViewRepo := repository.NewSupportInboxViewRepository(db)
	supportConversationTriageRepo := repository.NewSupportConversationTriageRepository(db)
	supportConversationTriageEventRepo := repository.NewSupportConversationTriageEventRepository(db)
	supportMailboxRepo := repository.NewSupportMailboxRepository(db)
	supportTriageRuleRepo := repository.NewSupportTriageRuleRepository(db)
	supportEmailRouteRepo := repository.NewSupportEmailRouteRepository(db)
	supportEmailSenderRepo := repository.NewSupportEmailSenderRepository(db)
	supportEmailSenderDomainRepo := repository.NewSupportEmailSenderDomainRepository(db)
	supportMessageRepo := repository.NewSupportMessageRepository(db)
	supportEmailLogRepo := repository.NewSupportEmailLogRepository(db)
	supportEmailWebhookEventRepo := repository.NewSupportEmailWebhookEventRepository(db)
	billingRepo := repository.NewBillingRepository(db)
	supportTagRepo := repository.NewSupportTagRepository(db)
	supportInstallRepo := repository.NewSupportInboxInstallationRepository(db)
	supportSessionRepo := repository.NewSupportInboxSessionRepository(db)
	supportAttachmentRepo := repository.NewSupportAttachmentRepository(db)
	customerIOIdentityService := service.NewCustomerIOIdentityService(
		service.NewCustomerIOTrackClient(service.CustomerIOTrackConfig{
			SiteID:                   cfg.CustomerIOSiteID,
			APIKey:                   cfg.CustomerIOTrackAPIKey,
			Region:                   cfg.CustomerIORegion,
			WorkspaceObjectTypeID:    cfg.CustomerIOWorkspaceObjectTypeID,
			OrganizationObjectTypeID: cfg.CustomerIOOrganizationObjectTypeID,
		}),
		userRepo,
		workspaceRepo,
		orgRepo,
		billingRepo,
	)
	stripeGateway := billingstripe.New(cfg.StripeSecretKey, cfg.StripeCreditBlockPriceID)
	billingService := service.NewBillingService(billingRepo, stripeGateway, time.Now)
	billingTestScenarioService := service.NewBillingTestScenarioService(db, billingService, time.Now)
	billingService.SetPriceConfig(service.BillingPriceConfig{
		StarterMonthly: cfg.StripeStarterMonthlyPriceID,
		StarterAnnual:  cfg.StripeStarterAnnualPriceID,
		GrowthMonthly:  cfg.StripeGrowthMonthlyPriceID,
		GrowthAnnual:   cfg.StripeGrowthAnnualPriceID,
	})
	billingService.SetWorkspaceRepository(workspaceRepo)
	billingService.SetCustomerIOIdentityService(customerIOIdentityService)
	aiUsageMeter := service.NewAIUsageMeter(billingService)
	gitIntegrationRepo := repository.NewGitIntegrationRepository(db)
	gitCredentialRepo := repository.NewGitCredentialRepository(db)
	gitRepositoryRepo := repository.NewGitRepositoryRepository(db)
	taskDeliveryTargetRepo := repository.NewTaskDeliveryTargetRepository(db)
	epicDeliveryTargetRepo := repository.NewEpicDeliveryTargetRepository(db)
	taskGitLinkRepo := repository.NewTaskGitLinkRepository(db)
	gitWebhookEventRepo := repository.NewGitWebhookEventRepository(db)
	agentHandoffRepo := repository.NewAgentHandoffRepository(db)
	docsSpaceRepo := repository.NewDocsSpaceRepository(db)
	docsCollectionRepo := repository.NewDocsCollectionRepository(db, cfg.DocsOrderingUseSortKey)
	docsDocumentRepo := repository.NewDocsDocumentRepository(db, cfg.DocsOrderingUseSortKey)
	docsContentRepo := repository.NewDocsContentRepository(db)
	docsBlockRepo := repository.NewDocsBlockRepository(db)
	docsAISectionCandidateRepo := repository.NewDocsAISectionCandidateRepository(db)
	docsChangeProposalRepo := repository.NewDocsChangeProposalRepository(db)
	docsContentRepo.SetBlockRepository(docsBlockRepo)
	docsVersionRepo := repository.NewDocsVersionRepository(db)
	docsLinkRepo := repository.NewDocsLinkRepository(db)
	docsHelpcenterRepo := repository.NewDocsHelpcenterRepository(db, cfg.DocsOrderingUseSortKey)
	docsHelpcenterTranslationRepo := repository.NewDocsHelpcenterTranslationRepository(db)
	docsHelpcenterPublicationRepo := repository.NewDocsHelpcenterPublicationRepository(db)
	docsHelpcenterSearchRepo := repository.NewDocsHelpcenterSearchRepository(db)
	docsSearchRepo := repository.NewDocsSearchRepository(db)
	docsImportRepo := repository.NewDocsImportRepository(db)
	docsRedirectRepo := repository.NewDocsRedirectRepository(db)
	docsChunkRepo := repository.NewDocsChunkRepository(db)
	supportContentSourceRepo := repository.NewSupportContentSourceRepository(db)
	agentContentSourceRepo := repository.NewAgentContentSourceRepository(db)
	supportContentPageRepo := repository.NewSupportContentPageRepository(db)
	supportContentChunkRepo := repository.NewSupportContentChunkRepository(db)
	curatedGuidanceRepo := repository.NewCuratedGuidanceRepository(db)
	notificationRepo := repository.NewNotificationRepository(db)
	notificationPrefRepo := repository.NewNotificationPreferenceRepository(db)
	followerRepo := repository.NewFollowerRepository(db)
	userNotifSettingsRepo := repository.NewUserNotificationSettingsRepository(db)
	crmContactRepo := repository.NewCRMContactRepository(db)
	crmCompanyRepo := repository.NewCRMCompanyRepository(db)
	crmDealRepo := repository.NewCRMDealRepository(db)
	crmAssociationRepo := repository.NewCRMAssociationRepository(db)
	crmActivityRepo := repository.NewCRMActivityRepository(db)
	crmImportRepo := repository.NewCRMImportRepository(db)
	crmEmailRepo := repository.NewCRMEmailRepository(db)
	crmCalendarRepo := repository.NewCRMCalendarRepository(db)
	crmEnrichmentRepo := repository.NewCRMEnrichmentRepository(db)
	crmSignalRepo := repository.NewCRMSignalRepository(db)
	crmSummaryRepo := repository.NewCRMSummaryRepository(db)
	crmSuggestionRepo := repository.NewCRMSuggestionRepository(db)
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
	passwordResetRepo := repository.NewPasswordResetTokenRepository(db)
	emailVerificationRepo := repository.NewEmailVerificationTokenRepository(db)
	passkeySessionCache := newPasskeySessionCache(redisClient, podID)
	passkeyWebAuthnClient, err := appwebauthn.NewClient(cfg.WebAuthnRPID, cfg.WebAuthnRPOrigins, passkeySessionCache)
	if err != nil {
		fatalWithSentry("failed to initialize webauthn", err)
	}
	authService := service.NewAuthService(userRepo, passwordResetRepo, orgRepo, workspaceRepo, emailVerificationRepo, jwtManager, s3Client, appEmailClient, cfg.AppBaseURL, resolveTOTPEncryptionKey(cfg))
	authService.SetCustomerIOIdentityService(customerIOIdentityService)
	passkeyService := service.NewPasskeyService(userRepo, passkeyRepo, jwtManager, passkeyWebAuthnClient, resolveTOTPEncryptionKey(cfg))
	pmActivityService := service.NewPMActivityService(pmActivityRepo)
	pmLabelService := service.NewPMLabelService(pmLabelRepo, wsPublisher)
	pmTaskTemplateService := service.NewPMTaskTemplateService(pmTaskTemplateRepo, wsPublisher)
	pmTaskTemplateService.SetAttachmentRepository(pmAttachmentRepo)
	pmRecurringTemplateService := service.NewPMRecurringTemplateService(pmRecurringTemplateRepo, pmTaskRepo, pmWorkflowRepo, pmSprintRepo, workspaceRepo, pmChecklistItemRepo, pmExternalLinkRepo, pmActivityService, wsPublisher)
	pmWorkflowService := service.NewPMWorkflowService(pmWorkflowRepo, pmTaskRepo, pmLabelRepo, wsPublisher)
	pmAutomationService := service.NewPMAutomationService(pmAutomationRepo, pmEpicRepo, pmTaskRepo, pmSprintRepo, pmWorkflowRepo, pmActivityService, wsPublisher, pmSprintCloseoutRepo)
	automationHealthService := service.NewAutomationHealthService(automationHealthRepo)
	pmAutomationService.SetHealthObserver(automationHealthService)
	notificationService := service.NewNotificationService(notificationRepo, notificationPrefRepo, userNotifSettingsRepo, followerRepo, userRepo, workspaceRepo, wsPublisher, appEmailClient, cfg.AppBaseURL)
	userNotifSettingsService := service.NewUserNotificationSettingsService(userNotifSettingsRepo)
	followerService := service.NewFollowerService(followerRepo)
	pmTaskService := service.NewPMTaskService(pmTaskRepo, workspaceRepo, pmWorkflowRepo, pmEpicRepo, pmSprintRepo, pmLabelRepo, pmChecklistItemRepo, pmExternalLinkRepo, pmAttachmentRepo, pmActivityService, wsPublisher, pmAutomationService, notificationService, followerService)
	pmTaskService.SetTaskTemplateRepository(pmTaskTemplateRepo)
	pmRoadmapRepo := repository.NewPMRoadmapRepository(db)
	pmEpicService := service.NewPMEpicService(pmEpicRepo, pmTaskRepo, pmLabelRepo, gitRepositoryRepo, pmAttachmentRepo, workspaceRepo, pmActivityService, wsPublisher, notificationService)
	pmRoadmapService := service.NewPMRoadmapService(pmEpicService, pmRoadmapRepo)
	pmSprintService := service.NewPMSprintService(pmSprintRepo, pmLabelRepo, pmAttachmentRepo, workspaceRepo, settingsRepo, pmActivityService, wsPublisher, notificationService, pmSprintCloseoutRepo)
	pmCommentService := service.NewPMCommentService(pmCommentRepo, pmTaskRepo, pmAttachmentRepo, pmActivityService, wsPublisher, notificationService, workspaceRepo, s3Client)
	pmAttachmentService := service.NewPMAttachmentService(pmAttachmentRepo, s3Client, wsPublisher)
	pmObjectiveService := service.NewPMObjectiveService(pmObjectiveRepo, pmKeyResultRepo, pmLabelRepo, pmAttachmentRepo, workspaceRepo, pmActivityService, wsPublisher, notificationService)
	pmChecklistItemService := service.NewPMChecklistItemService(pmChecklistItemRepo, pmTaskRepo, wsPublisher, notificationService, workspaceRepo)
	docsEmbedResolverService := service.NewDocsEmbedResolverService(cfg.CrawlerProxyURLs)
	pmExternalLinkService := service.NewPMExternalLinkService(pmExternalLinkRepo, wsPublisher)
	pmExternalLinkService.SetMetadataResolver(docsEmbedResolverService)
	pmViewService := service.NewPMViewService(pmViewRepo, wsPublisher)
	pmImportService := service.NewPMImportService(db, workspaceRepo, pmWorkflowRepo, pmAttachmentService, resolvePMImportEncryptionKey(cfg))
	pmImportService.SetPublisher(wsPublisher)
	searchService := service.NewSearchService(searchRepo, workspaceRepo)
	cannedResponseRepo := repository.NewSupportCannedResponseRepository(db)
	supportTeammateStatusOverrideRepo := repository.NewSupportTeammateStatusOverrideRepository(db)
	supportInboxViewService := service.NewSupportInboxViewService(supportInboxViewRepo, supportConversationRepo, wsPublisher)
	supportTagService := service.NewSupportTagService(supportTagRepo, supportConversationRepo, wsPublisher).
		SetMessageRepo(supportMessageRepo).
		SetUserRepo(userRepo)
	supportInboxService := service.NewSupportInboxService(supportConversationRepo, supportMailboxRepo, supportMessageRepo, agentRepo, crmAssociationRepo, supportInstallRepo, supportSessionRepo, cannedResponseRepo, pmActivityService, wsPublisher, crmContactRepo, userRepo, docsSpaceRepo, docsCollectionRepo, docsHelpcenterRepo)
	supportInboxService.SetDocsSearchRepository(docsSearchRepo)
	supportInboxService.SetSupportTagRepo(supportTagRepo)
	supportLinkPreviewService := service.NewSupportLinkPreviewService(cfg.CrawlerProxyURLs)
	emailFallbackService := service.NewEmailFallbackService(
		redisClient,
		wsHub,
		wsPublisher,
		replyEmailClient,
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
	emailFallbackService.SetCRMContactRepository(crmContactRepo)
	supportMessageActionsService := service.NewSupportMessageActionsService(supportMessageRepo, emailFallbackService, supportEmailLogRepo, wsPublisher)
	supportAttachmentService := service.NewSupportAttachmentService(supportAttachmentRepo, s3Client)
	emailFallbackService.SetAttachmentService(supportAttachmentService)
	supportInboxService.SetAttachmentService(supportAttachmentService)
	supportInboxService.SetLinkPreviewService(supportLinkPreviewService)
	supportInboxService.SetEmailFallbackService(emailFallbackService)
	supportInboxService.SetRouteDomain(cfg.SupportEmailRouteDomain)
	supportInboxService.SetEmailRouteRepository(supportEmailRouteRepo)
	supportInboxService.SetEmailSenderRepository(supportEmailSenderRepo)
	supportInboxService.SetEmailSenderDomainRepository(supportEmailSenderDomainRepo)
	supportInboxService.SetPostmarkDomainClient(postmarkDomainClient)
	supportInboxService.SetEmailLogRepo(supportEmailLogRepo)
	supportInboxService.SetTriageEventRepo(supportConversationTriageEventRepo)
	supportInboxService.SetWorkspaceRepo(workspaceRepo)
	supportInboxService.SetTaskService(pmTaskService)
	supportInboxService.SetPresenceProvider(wsHub.Presence)
	supportInboxService.SetStatusOverrideRepo(supportTeammateStatusOverrideRepo)
	supportInboxService.SetGeoIPResolver(geoIPResolver)
	if geoIPResolver != nil {
		if updated, err := supportInboxService.BackfillWidgetSessionGeo(context.Background(), 5000); err != nil {
			slog.Warn("support widget geoip backfill failed", "error", err)
		} else if updated > 0 {
			slog.Info("support widget geoip backfill completed", "updated_sessions", updated)
		}
	}
	emailFallbackService.SetSupportInboxService(supportInboxService)
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
	supportLLMProvider := service.NewMeteredLLMProvider(supportLLMRouter, aiUsageMeter)
	supportInboxTriageService := service.NewSupportInboxTriageService(
		supportInboxService,
		supportConversationTriageRepo,
		supportConversationTriageEventRepo,
		supportTriageRuleRepo,
		supportInstallRepo,
		supportMailboxRepo,
		supportConversationRepo,
		supportMessageRepo,
		supportLLMProvider,
	)
	supportInboxService.SetTriageService(supportInboxTriageService)

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
		taskGitLinkRepo,
		taskDeliveryTargetRepo,
		settingsRepo,
		workspaceRepo,
		orgRepo,
		pmTaskRepo,
		pmActivityService,
		wsPublisher,
		githubAppClient,
		cfg.AppBaseURL,
		cfg.GitHubAppSlug,
		cfg.JWTSecret,
	).
		SetEpicDeliveryDependencies(epicDeliveryTargetRepo, pmEpicRepo).
		SetGitLabDependencies(gitCredentialRepo, resolveGitOAuthEncryptionKey(cfg))
	pmTaskService.SetGitService(gitService)
	var agentRuntimeClient *service.AgentRuntimeClient
	if strings.TrimSpace(cfg.AgentRuntimeBaseURL) != "" {
		agentRuntimeClient, err = service.NewAgentRuntimeClient(cfg.AgentRuntimeBaseURL, cfg.AgentRuntimeAppID, cfg.AgentRuntimeServiceToken, nil, cfg.AgentRuntimeEventProtocol)
		if err != nil {
			fatalWithSentry("failed to initialize agent runtime client", err)
		}
	}
	externalMCPService, err := service.NewExternalMCPService(
		externalMCPRepo,
		notificationService,
		service.ExternalMCPServiceConfig{
			Enabled:                cfg.ExternalMCPEnabled,
			CustomServersEnabled:   cfg.ExternalMCPCustomServersEnabled,
			EncryptionKey:          cfg.ExternalMCPEncryptionKey,
			AllowedHosts:           cfg.ExternalMCPAllowedHosts,
			OAuthRedirectURL:       cfg.ExternalMCPOAuthRedirectURL,
			AppBaseURL:             cfg.AppBaseURL,
			OAuthClientID:          cfg.ExternalMCPOAuthClientID,
			OAuthClientSecret:      cfg.ExternalMCPOAuthClientSecret,
			OAuthClientAuthMethod:  cfg.ExternalMCPOAuthClientAuthMethod,
			AllowInsecureLocalhost: cfg.ExternalMCPAllowInsecureLocalhost,
		},
	)
	if err != nil {
		fatalWithSentry("failed to initialize external MCP service", err)
	}
	agentService := service.NewAgentService(
		agentRepo,
		workspacePresetVersionRepo,
		agentRunRepo,
		agentRunMessageRepo,
		agentRunArtifactRepo,
		agentRunInteractionRepo,
		codingSessionStateSnapshotRepo,
		pmTaskRepo,
		pmTaskLinkRepo,
		pmEpicRepo,
		supportConversationRepo,
		supportMessageRepo,
		agentHandoffRepo,
		automationRuleRepo,
		supportInstallRepo,
		settingsRepo,
		docsSpaceRepo,
		docsDocumentRepo,
		docsContentRepo,
		docsVersionRepo,
		docsLinkRepo,
		gitService,
		pmTaskService,
		pmActivityService,
		wsPublisher,
	).SetModelProviderConfig(
		cfg.AnthropicAPIKey,
		cfg.OpenAIAPIKey,
		cfg.OpenRouterAPIKey,
		cfg.CodexOpenAIAuthMode,
		cfg.CodexEnableChatGPTOAuth,
		cfg.CodexChatGPTAccessToken,
		cfg.CodexChatGPTAccountID,
	).SetTriggerExecutionRepository(agentTriggerExecutionRepo).SetCommandBarPlanRepository(commandBarPlanRepo).SetWorkspaceRepository(workspaceRepo).SetUserRepository(userRepo).SetWorkspaceSkillStore(workspaceSkillRepo, s3Client).SetNotificationService(notificationService).SetAgentTemplateRepository(agentTemplateRepo).SetCRMRepositories(crmContactRepo, crmCompanyRepo, crmDealRepo).SetAgentDraftLLM(supportLLMProvider).SetAIUsageMeter(aiUsageMeter).SetAgentRuntimeLaunchEnabled(cfg.AgentRuntimeLaunchEnabled)
	if agentRuntimeClient != nil {
		agentService.SetAgentRuntimeClient(agentRuntimeClient)
	}
	agentService.SetExternalMCPService(externalMCPService)
	commandBarService := service.NewCommandBarService(agentService, commandBarPlanRepo, commandBarPlanDismissalRepo).
		SetWebsocketPublisher(wsPublisher)
	supportInboxService.SetConversationAgentRunner(agentService.RunConversationAgentAuto)
	supportInboxService.SetNotificationService(notificationService, workspaceRepo)
	emailFallbackService.SetNotificationService(notificationService)

	// Automation Rule Engine — wired after agent + story services to break circular deps.
	ruleEngine := service.NewAutomationRuleEngine(
		automationRuleRepo,
		pmTaskRepo,
		pmWorkflowRepo,
		taskDeliveryTargetRepo,
		gitService,
		notificationService,
		pmActivityService,
		wsPublisher,
	)
	ruleEngine.SetAgentService(agentService)
	ruleEngine.SetTaskService(pmTaskService)
	ruleEngine.SetHealthObserver(automationHealthService)
	ruleEngine.SetTriggerExecutionRepository(agentTriggerExecutionRepo)
	ruleEngine.SetRunEngine(runEngine)
	gitService.SetRuleEngine(ruleEngine)
	pmTaskService.SetRuleEngine(ruleEngine)
	pmTaskService.SetAgentService(agentService)
	pmEpicService.SetAgentService(agentService)
	pmTaskService.SetRecurringService(pmRecurringTemplateService)
	pmRecurringTemplateService.SetTaskService(pmTaskService)
	agentService.SetRuleEngine(ruleEngine)
	agentService.SetWorkflowService(pmWorkflowService)
	agentService.SetPMSprintService(pmSprintService)
	pmRecurringTemplateService.SetTemporalClient(temporalClient)
	pmImportService.SetTemporalClient(temporalClient)

	slog.Info("startup: backfilling built-in agents for existing workspaces")
	if err := agentService.EnsureSystemTemplates(context.Background()); err != nil {
		fatalWithSentry("failed to seed system agent templates", err)
	}
	workspaceIDs, err := workspaceRepo.ListIDs(context.Background())
	if err != nil {
		fatalWithSentry("failed to list workspaces for built-in agent backfill", err)
	}
	seeded := 0
	for _, workspaceID := range workspaceIDs {
		if err := agentService.SeedWorkspaceDefaults(context.Background(), workspaceID, ""); err != nil {
			fatalWithSentry("failed to backfill built-in agents", fmt.Errorf("workspace %s: %w", workspaceID, err))
		}
		seeded++
	}
	slog.Info("startup: built-in agent backfill complete", "workspaces_processed", seeded)

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
	llmProvider = service.NewMeteredLLMProvider(llmProvider, aiUsageMeter)
	if llmProvider != nil {
		slog.Info("LLM provider configured for signal detection")
	}

	docsSpaceService := service.NewDocsSpaceService(docsSpaceRepo, wsPublisher)
	docsCollectionService := service.NewDocsCollectionService(docsCollectionRepo, docsSpaceRepo, wsPublisher, cfg.DocsOrderingUseSortKey)
	docsDocumentService := service.NewDocsDocumentService(docsDocumentRepo, docsSpaceRepo, wsPublisher, cfg.DocsOrderingUseSortKey)
	docsDocumentService.SetRuleEngine(ruleEngine)
	docsContentService := service.NewDocsContentService(docsContentRepo, docsDocumentRepo, wsPublisher)
	docsContentService.SetMentionNotificationDependencies(notificationService, workspaceRepo)
	docsContentService.SetAgentMentionDependencies(agentService)
	docsBlockService := service.NewDocsBlockService(docsBlockRepo, docsContentService, docsDocumentRepo)
	docsBlockService.SetActivityService(pmActivityService)
	docsSearchService := service.NewDocsSearchService(docsSearchRepo)
	docsAISectionService := service.NewDocsAISectionService(docsAISectionCandidateRepo, docsBlockRepo, docsBlockService, docsDocumentRepo, docsSearchService, supportConversationRepo, agentService, llmProvider, cfg.CrawlerProxyURLs)
	docsAISectionService.SetRuleEngine(ruleEngine)
	docsAISectionService.SetActivityService(pmActivityService)
	docsVersionService := service.NewDocsVersionService(docsVersionRepo, docsContentRepo, docsDocumentRepo, wsPublisher)
	docsChangeProposalService := service.NewDocsChangeProposalService(docsChangeProposalRepo, docsDocumentRepo, docsContentService, docsBlockService, docsVersionService, wsPublisher)
	docsReferencesService := service.NewDocsReferencesService(docsLinkRepo, docsBlockRepo, docsDocumentRepo, pmCommentService, agentService)
	var docsEntityReferenceResolverService *service.DocsEntityReferenceResolverService
	pmImportService.SetDocsImportDependencies(docsDocumentService, docsContentService)
	docsLinkService := service.NewDocsLinkService(docsLinkRepo, pmTaskRepo, docsDocumentRepo, wsPublisher)
	docsHelpcenterService := service.NewDocsHelpcenterService(docsHelpcenterRepo, docsHelpcenterPublicationRepo, docsDocumentRepo, docsContentRepo, docsSpaceRepo, docsCollectionRepo, docsRedirectRepo, s3Client, wsPublisher)
	docsHelpcenterService.SetSearchRepository(docsHelpcenterSearchRepo)
	tlsAskService := service.NewTLSAskService(docsHelpcenterRepo, cfg.TLSAskExtraAllowedDomains)

	// Tiered cache for hot public help-center reads. L1 is an in-process LRU;
	// L2 is Redis when available so cache entries survive pod restarts and
	// invalidations fan out across pods via pub/sub.
	hcL1 := cache.NewLRU(2048)
	var hcCache cache.Cache = hcL1
	if redisClient != nil {
		hcL2 := cache.NewRedis(redisClient, "hc")
		tiered := cache.NewTiered(cache.TieredConfig{
			L1:      hcL1,
			L2:      hcL2,
			Redis:   redisClient,
			Channel: "cache:hc:invalidate",
			PodID:   podID,
			L1TTL:   60 * time.Second,
		})
		tiered.StartInvalidationSubscriber(context.Background())
		hcCache = tiered
		slog.Info("help-center cache: tiered L1+L2 (Redis) enabled")
	} else {
		slog.Info("help-center cache: L1-only (no Redis) — single-pod consistency only")
	}
	docsHelpcenterService.SetHelpcenterCache(hcCache)
	docsHelpcenterTranslationService := service.NewDocsHelpcenterTranslationService(docsHelpcenterTranslationRepo, docsHelpcenterRepo, docsHelpcenterPublicationRepo, docsRedirectRepo, docsDocumentRepo, docsContentRepo, docsSpaceRepo, docsCollectionRepo, llmProvider)
	docsHelpcenterTranslationService.SetSearchRepository(docsHelpcenterSearchRepo)
	docsImportService := service.NewDocsImportService(docsImportRepo, docsSpaceService, docsCollectionService, docsDocumentService, docsContentService, docsHelpcenterService, docsRedirectRepo, s3Client)
	docsSpaceService.SetTranslationService(docsHelpcenterTranslationService)
	docsCollectionService.SetTranslationService(docsHelpcenterTranslationService)
	docsDocumentService.SetTranslationService(docsHelpcenterTranslationService)
	docsContentService.SetTranslationService(docsHelpcenterTranslationService)
	docsHelpcenterService.SetTranslationService(docsHelpcenterTranslationService)
	docsDocumentService.SetHelpcenterService(docsHelpcenterService)
	docsDeletionDeps := service.DocsDocumentDeletionDependencies{
		ContentRepo:     docsContentRepo,
		BlockRepo:       docsBlockRepo,
		VersionRepo:     docsVersionRepo,
		LinkRepo:        docsLinkRepo,
		ChunkRepo:       docsChunkRepo,
		HelpcenterRepo:  docsHelpcenterRepo,
		PublicationRepo: docsHelpcenterPublicationRepo,
		SearchRepo:      docsHelpcenterSearchRepo,
		TranslationRepo: docsHelpcenterTranslationRepo,
	}
	if s3Client != nil {
		docsDeletionDeps.AssetStore = s3Client
	}
	if cleanupEnqueuer := service.NewTemporalDocsAssetCleanupEnqueuer(temporalClient); cleanupEnqueuer != nil {
		docsDeletionDeps.CleanupEnqueuer = cleanupEnqueuer
	}
	docsDocumentService.SetDeletionDependencies(docsDeletionDeps)
	docsCollectionService.SetPermanentDeleteDependencies(docsDocumentRepo, docsDocumentService, docsHelpcenterTranslationRepo)
	docsCollectionService.SetHelpcenterRepository(docsHelpcenterRepo)
	docsSpaceService.SetPermanentDeleteDependencies(docsCollectionRepo, docsDocumentRepo, docsDocumentService, docsHelpcenterTranslationRepo)
	docsSpaceService.SetHelpcenterRepository(docsHelpcenterRepo)
	contentCrawler := crawler.NewSmartCrawler(
		cfg.CrawlerMode,
		cfg.CloudflareAccountID,
		cfg.CloudflareAPIToken,
		cfg.CloudflareAPIBaseURL,
		cfg.CrawlerProxyURLs,
		slog.Default(),
	)
	docsEmbeddingService := service.NewDocsEmbeddingServiceWithCollections(
		docsChunkRepo,
		docsBlockRepo,
		agentKnowledgeSourceRepo,
		docsContentRepo,
		docsSpaceRepo,
		docsCollectionRepo,
		docsHelpcenterRepo,
		docsDocumentRepo,
		supportEmbeddingProvider,
		cfg.OpenAIEmbeddingModel,
		runEngine,
	)
	agentKnowledgeSourceService := service.NewAgentKnowledgeSourceServiceWithScopes(agentKnowledgeSourceRepo, docsSpaceRepo, docsCollectionRepo, docsDocumentRepo, docsEmbeddingService)
	supportContentSyncService := service.NewSupportContentSyncService(
		supportContentSourceRepo,
		supportContentPageRepo,
		supportContentChunkRepo,
		supportEmbeddingProvider,
		cfg.OpenAIEmbeddingModel,
		contentCrawler,
		s3Client,
		runEngine,
	)
	supportContentSourceService := service.NewSupportContentSourceService(
		supportContentSourceRepo,
		agentRepo,
		agentContentSourceRepo,
		supportContentPageRepo,
		supportContentChunkRepo,
		supportContentSyncService,
		s3Client,
	)
	agentContentSourceService := service.NewAgentContentSourceService(
		agentContentSourceRepo,
		agentRepo,
		supportContentSourceRepo,
		supportContentSyncService,
	)
	curatedGuidanceService := service.NewCuratedGuidanceService(
		curatedGuidanceRepo,
		agentRepo,
		supportEmbeddingProvider,
		cfg.OpenAIEmbeddingModel,
	)

	crmContactService := service.NewCRMContactService(crmContactRepo)
	crmCompanyService := service.NewCRMCompanyService(crmCompanyRepo)
	crmDealService := service.NewCRMDealService(crmDealRepo, crmAssociationRepo)
	crmAssociationService := service.NewCRMAssociationService(crmAssociationRepo)
	associationsService := service.NewAssociationsService(crmAssociationRepo, crmContactRepo, workspaceRepo, pmTaskLinkRepo, pmTaskRepo, supportConversationRepo, docsLinkRepo, docsDocumentRepo)
	crmActivityService := service.NewCRMActivityService(crmActivityRepo)
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
	crmEnrichmentService := service.NewCRMEnrichmentService(crmEnrichmentRepo, crmContactRepo, crmCompanyRepo, crmAssociationRepo)
	crmSignalService := service.NewCRMSignalService(crmSignalRepo, crmSummaryService)
	crmSuggestionService := service.NewCRMSuggestionService(crmSuggestionRepo, crmDealRepo, crmAssociationRepo)
	crmWritingProfileService := service.NewCRMWritingProfileService(crmWritingProfileRepo)
	crmSearchService := service.NewCRMSearchService(crmContactRepo, crmCompanyRepo, crmDealRepo)
	commandService := service.NewInternalCommandService(
		agentService,
		pmTaskService,
		crmDealService,
		crmActivityService,
		docsContentService,
		docsLinkService,
		pmTaskRepo,
		pmTaskLinkRepo,
	)
	commandService.SetPMAutomationService(pmAutomationService)
	commandService.SetPMLabelService(pmLabelService)
	commandService.SetPMCommentService(pmCommentService)
	commandService.SetPMOperationalServices(workspaceRepo, pmEpicService, pmSprintService, pmObjectiveService, pmWorkflowService, pmChecklistItemService)
	commandService.SetGitService(gitService)
	commandService.SetSettingsRepository(settingsRepo)
	commandService.SetCRMEnrichmentService(crmEnrichmentService)
	commandService.SetDocsCreateDependencies(docsDocumentService, docsContentRepo)
	commandService.SetDocsOrganizationServices(docsSpaceService, docsCollectionService)
	commandService.SetDocsBlockService(docsBlockService)
	commandService.SetSupportDependencies(supportMessageRepo, supportConversationRepo, wsPublisher)
	commandService.SetCRMReadServices(crmContactService, crmSignalService)
	commandService.SetDocsSearchRepository(docsSearchRepo)
	commandService.SetDocsChangeProposalService(docsChangeProposalService)
	commandService.SetAgentRunDependencies(agentRunRepo, agentRunArtifactRepo)
	commandService.SetReleaseFactsProvider(service.NewReleaseFactsService(
		gitIntegrationRepo,
		gitRepositoryRepo,
		pmTaskRepo,
		taskGitLinkRepo,
		pmCommentRepo,
		docsLinkRepo,
		docsDocumentRepo,
		docsContentRepo,
		workspaceRepo,
		githubAppClient,
	))
	commandBarService.SetInternalCommandService(commandService).
		SetReadOnlyDataServices(docsDocumentService, crmDealService, crmContactService, crmCompanyService)
	ruleEngine.SetCommandService(commandService)
	agentRuntimeHostService := service.NewAgentRuntimeHostService(
		cfg.AgentRuntimeAppID,
		agentRunRepo,
		workspaceRepo,
		pmTaskRepo,
		pmEpicRepo,
		supportConversationRepo,
		docsDocumentRepo,
		crmContactRepo,
		crmCompanyRepo,
		crmDealRepo,
		commandService,
		gitService,
	).SetPMSprintService(pmSprintService).
		SetAgentRepository(agentRepo).
		SetWorkspaceSkillStore(workspaceSkillRepo, s3Client)
	var agentRuntimeProjectionService *service.AgentRuntimeProjectionService
	var runFinalizers *service.AgentRunFinalizerService
	if strings.TrimSpace(cfg.AgentRuntimeBaseURL) != "" {
		runFinalizers = service.NewAgentRunFinalizerService(
			agentRunRepo,
			agentRepo,
			pmTaskRepo,
			pmEpicRepo,
			supportConversationRepo,
			supportMessageRepo,
			ruleEngine,
			wsPublisher,
		).SetRepositoryDeliveryService(gitService).SetCommandBarPlanAdvancer(agentService)
		agentRuntimeProjectionService = service.NewAgentRuntimeProjectionService(agentRunRepo, cfg.AgentRuntimeAppID).
			SetEventProtocol(cfg.AgentRuntimeEventProtocol).
			SetOverageDependencies(agentRepo, aiUsageMeter, agentRuntimeClient).
			SetTranscriptRepositories(agentRunMessageRepo, agentRunArtifactRepo, agentRunInteractionRepo).
			SetCodingSessionSnapshotRepository(codingSessionStateSnapshotRepo).
			SetWebSocketPublisher(wsPublisher).
			SetRunFinalizers(runFinalizers)
	}
	agentRuntimeProjectionCancel := context.CancelFunc(func() {})
	var projectionCtx context.Context
	if agentRuntimeProjectionService != nil {
		projectionCtx, agentRuntimeProjectionCancel = context.WithCancel(context.Background())
		go func() {
			if err := agentRuntimeProjectionService.StartNATSConsumer(projectionCtx, jetstream); err != nil {
				slog.Error("agent runtime projection consumer stopped", "error", err)
			}
		}()
		go func() {
			if err := agentRuntimeProjectionService.StartReconciliationSweep(projectionCtx, time.Minute, 2*time.Minute, 50); err != nil {
				slog.Error("agent runtime reconciliation sweep stopped", "error", err)
			}
		}()
		// Self-healing backstop for command-bar plan advancement: the
		// projection finalizer advances plans on terminal run events; this
		// sweep re-kicks plans that stall between events (replaces the
		// retired CommandBarPlanWorkflow watchdog timer).
		go func() {
			if err := agentService.StartCommandBarPlanSweep(projectionCtx, time.Minute, 3*time.Minute, 50); err != nil {
				slog.Error("command bar plan sweep stopped", "error", err)
			}
		}()
	}

	signalDetectionService := service.NewSignalDetectionService(llmProvider, crmSignalRepo, crmSummaryService)
	dealAutomationService := service.NewDealAutomationService(llmProvider, crmDealRepo, crmSignalRepo, crmSuggestionRepo, crmContactRepo, crmAssociationRepo, crmAutonomyRepo)
	_ = signalDetectionService // Used by Temporal workers

	// AI Support Agent — wire SupportAIService with LLM provider and JetStream.
	supportAIService := service.NewSupportAIService(
		supportLLMProvider, supportEmbeddingProvider, cfg.OpenAIEmbeddingModel, docsChunkRepo,
		agentKnowledgeSourceRepo, supportContentChunkRepo, agentContentSourceRepo, aiMessageProcessingRepo,
		supportConversationRepo, supportMessageRepo, supportAttachmentRepo,
		agentRepo, agentHandoffRepo, supportInstallRepo,
		wsPublisher, jetstream, redisClient, db,
		cfg.QueryExpansionModel, cfg.QueryExpansionProvider,
	)
	supportAIService.SetQueryExpansionTimeout(time.Duration(cfg.QueryExpansionTimeoutMS) * time.Millisecond)
	supportAIService.SetSupportRoutingDependencies(workspaceRepo, wsHub.Presence, supportTeammateStatusOverrideRepo)
	supportAIService.SetMailboxRepository(supportMailboxRepo)
	supportAIService.SetTriageService(supportInboxTriageService)
	supportAIService.SetLinkPreviewService(supportLinkPreviewService)
	supportAIService.SetCuratedGuidanceRepository(curatedGuidanceRepo)
	if reranker := service.NewHTTPSupportKnowledgeReranker(cfg.SupportRerankerURL, cfg.SupportRerankerModel, cfg.SupportRerankerAPIKey); reranker != nil {
		supportAIService.SetKnowledgeReranker(reranker)
	}
	if strings.TrimSpace(cfg.AnthropicAPIKey) != "" {
		supportAIService.SetTaskDraftLLM(service.NewEinoSupportTaskDraftLLM(cfg.AnthropicAPIKey, "claude-sonnet-4-6"))
	}
	supportInboxService.SetSupportAIService(supportAIService)

	// Coverage telemetry: repos → services → async recorder → inject into hot-path services.
	supportEventRepo := repository.NewSupportEventRepository(db)
	supportCoverageRepo := repository.NewSupportCoverageRepository(db)
	supportCoverageAnalysisRepo := repository.NewSupportCoverageAnalysisRepository(db)
	supportCoverageService := service.NewSupportCoverageService(supportCoverageRepo)
	agentService.SetSupportCoverageService(supportCoverageService)
	supportCoverageService.SetDocsBlockService(docsBlockService)
	supportCoverageService.SetTemporalClient(temporalClient)
	supportCoverageKnowledgeMatcher := service.NewCoverageKnowledgeMatcher(docsChunkRepo, supportContentChunkRepo, supportEmbeddingProvider, cfg.OpenAIEmbeddingModel)
	supportCoverageClusterRebuildService := service.NewSupportCoverageClusterRebuildService(supportCoverageRepo, supportEmbeddingProvider, cfg.OpenAIEmbeddingModel)
	supportCoverageDailyAnalyzer := service.NewSupportCoverageDailyAnalyzer(llmProvider, cfg.CRMLLMProvider, cfg.CRMLLMModel).
		SetCoverageRepositories(supportCoverageRepo, supportCoverageAnalysisRepo).
		SetEmbeddingProvider(supportEmbeddingProvider, cfg.OpenAIEmbeddingModel).
		SetConversationRepositories(supportConversationRepo, supportMessageRepo).
		SetKnowledgeMatcher(supportCoverageKnowledgeMatcher, docsSpaceRepo, supportContentSourceRepo).
		SetTemporalClient(temporalClient)
	supportCoverageTraceService := service.NewSupportCoverageRetrievalTraceService(supportCoverageAnalysisRepo)
	supportEventService := service.NewSupportEventService(supportEventRepo, supportCoverageService)
	supportEventRecorder := service.NewSupportEventAsyncRecorder(supportEventService, 250)
	supportAIService.SetSupportEventRecorder(supportEventRecorder)
	supportAIService.SetSupportAIRetrievalTraceRecorder(supportCoverageTraceService)
	supportInboxService.SetSupportEventRecorder(supportEventRecorder)
	// search_knowledge runtime tool: agent-scoped knowledge search for
	// support chat runs, executed in this process (full retrieval wiring
	// including curated guidance and the reranker).
	supportRunEvidenceRepo := repository.NewSupportRunEvidenceRepository(db)
	commandService.SetSupportKnowledgeDependencies(supportAIService, supportRunEvidenceRepo)
	commandService.SetSupportReplyDependencies(supportAIService, aiMessageProcessingRepo, aiUsageMeter)
	// Support chat lifecycle: conversation = agent-runtime chat-mode run.
	// Dark until the consumer cutover — only the pause hook and sweep are
	// live (both no-op without support_chat-trigger runs).
	supportChatService := service.NewSupportChatService(
		supportConversationRepo,
		supportMessageRepo,
		aiMessageProcessingRepo,
		agentRunRepo,
		commandBarPlanRepo,
		agentService,
		supportAIService,
	)
	supportChatService.SetResearchEvidenceDependencies(supportRunEvidenceRepo, workspaceRepo)
	if agentRuntimeProjectionService != nil {
		agentRuntimeProjectionService.SetSupportChatPauseHook(supportChatService.OnSupportChatRunPaused)
	}
	if projectionCtx != nil {
		go func() {
			if err := supportChatService.StartSupportChatSweep(projectionCtx, 30*time.Second, 30*time.Second, 50); err != nil {
				slog.Error("support chat sweep stopped", "error", err)
			}
		}()
		// Visitor-message consumer: NATS stays the serializer/retry layer;
		// each message now drives the conversation's chat-mode run.
		go func() {
			if err := supportAIService.StartNATSConsumer(projectionCtx, supportChatService.HandleVisitorMessage); err != nil {
				slog.Error("support AI consumer stopped", "error", err)
			}
		}()
	}

	supportCoverageDigestService := service.NewSupportCoverageDigestService(
		supportCoverageRepo, workspaceRepo, appEmailClient, cfg.AppBaseURL,
	)
	_ = supportCoverageDigestService // wired to ticker in follow-up

	orgService := service.NewOrganizationService(orgRepo)
	orgService.SetCustomerIOIdentityService(customerIOIdentityService)
	compositeDefaults := service.NewCompositeDefaultsInitializer(pmWorkflowService, pmAutomationService, crmDealService, supportInboxService, agentService)
	workspaceService := service.NewWorkspaceService(workspaceRepo, pmAttachmentRepo, s3Client, compositeDefaults)
	setupService := service.NewSetupService(setupRepo)
	setupSuccessEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("SETUP_SUCCESS_ENABLED")), "true")
	if setupSuccessEnabled {
		workspaceService.SetSetupInitializer(setupService)
	}
	workspaceService.SetContextGeneratorDependencies(supportLLMProvider, nil)
	billingService.SetOrgRoleResolver(orgService)
	entitlementService := service.NewEntitlementService(billingService)
	setupService.SetEntitlementService(entitlementService)
	pmImportService.SetEntitlementService(entitlementService)
	supportInboxService.SetEntitlementService(entitlementService)
	supportInboxTriageService.SetEntitlementService(entitlementService)
	agentService.SetEntitlementService(entitlementService)
	ruleEngine.SetEntitlementService(entitlementService)
	docsDocumentService.SetEntitlementService(entitlementService)
	docsHelpcenterTranslationService.SetEntitlementService(entitlementService)
	crmContactService.SetEntitlementService(entitlementService)
	crmImportService.SetEntitlementService(entitlementService)
	dealAutomationService.SetEntitlementService(entitlementService)
	workspaceService.SetBillingService(billingService)
	workspaceService.SetCustomerIOIdentityService(customerIOIdentityService)
	workspaceService.SetPresenceProvider(wsHub.Presence)
	workspaceService.SetStatusOverrideRepo(supportTeammateStatusOverrideRepo)
	settingsService := service.NewSettingsService(settingsRepo, moduleGrantRepo, pmWorkflowService, wsPublisher).
		SetGitRepositoryRepository(gitRepositoryRepo).
		SetEntitlementService(entitlementService)
	automationInventoryService := service.NewAutomationInventoryService(settingsRepo, pmAutomationRepo, crmEmailRepo, automationHealthRepo, automationRuleRepo, agentTriggerExecutionRepo, agentRunRepo, agentRepo, workspaceRepo, pmTaskRepo, supportInstallRepo).
		SetTargetResolvers(pmEpicRepo, docsDocumentRepo, supportConversationRepo, crmContactRepo, crmDealRepo, gitRepositoryRepo, supportCoverageRepo)
	flowTemplateRegistry, err := flowtemplates.LoadSystemRegistry()
	if err != nil {
		fatalWithSentry("failed to load flow templates", err)
	}
	flowTemplateInstaller := flowtemplates.NewInstaller(db, flowTemplateRegistry)
	flowTemplateInstaller.SetScheduleManager(ruleEngine).SetAgentValidator(agentService)
	flowTemplateUninstaller := flowtemplates.NewUninstaller(db)
	flowTemplateUninstaller.SetScheduleManager(ruleEngine)
	if err := pmRecurringTemplateService.EnsureScheduler(context.Background()); err != nil {
		slog.Error("failed to ensure PM recurring scheduler", "error", err)
	}
	inviteService := service.NewInviteService(invitationRepo, workspaceRepo, orgRepo, userRepo, settingsRepo, appEmailClient, cfg.AppBaseURL, jwtManager)
	inviteService.SetBillingService(billingService)
	// Initialize authorization service.
	authzMemberRepo := authorization.NewGORMMemberRepository(db)
	authzService := authorization.NewAuthzService(db, authzMemberRepo, moduleGrantRepo)
	authzService.SetWorkspaceMFARepository(workspaceRepo)
	commandService.SetAuthorizationService(authzService)
	commandService.SetAgentOrchestrationDependencies(commandBarService, agentRunInteractionRepo)
	agentRuntimeHostService.SetAuthorizationService(authzService)
	dockChatRepo := repository.NewDockChatRepository(db)
	dockChatService := service.NewDockChatService(dockChatRepo, agentRunRepo, agentRunMessageRepo, commandBarPlanRepo, agentService, commandService, authzService)
	if runFinalizers != nil {
		// Immediate delivery of settled child-plan results into dock chats;
		// the sweep below retries chats that were mid-turn at that moment.
		runFinalizers.SetDockChatResultNotifier(dockChatService)
		runFinalizers.SetSupportChatResultNotifier(supportChatService)
	}
	if projectionCtx != nil {
		go func() {
			if err := dockChatService.StartDockChatResultSweep(projectionCtx, 30*time.Second, 50); err != nil {
				slog.Error("dock chat result sweep stopped", "error", err)
			}
		}()
	}
	docsEntityReferenceResolverService = service.NewDocsEntityReferenceResolverService(pmTaskService, pmEpicService, supportInboxService, crmDealService, crmContactService, crmCompanyService, docsDocumentService, authzService)
	docsReferencesService.SetEntityReferenceResolver(docsEntityReferenceResolverService)
	agentService.SetMCPRepository(mcpRepo)
	mcpService := service.NewMCPService(
		mcpRepo,
		workspaceRepo,
		userRepo,
		authzService,
		jwtManager,
		commandService,
		agentService,
		searchService,
		pmTaskService,
		docsDocumentService,
		docsSpaceService,
		docsCollectionService,
		docsLinkService,
		docsEntityReferenceResolverService,
		pmChecklistItemService,
		crmContactService,
		crmDealService,
		supportInboxService,
		service.MCPServiceConfig{
			AppBaseURL:           cfg.AppBaseURL,
			IssuerURL:            cfg.MCPPublicBaseURL,
			ResourceURL:          cfg.MCPPublicBaseURL + "/mcp",
			ServerEnabled:        cfg.MCPServerEnabled,
			OAuthEnabled:         cfg.MCPOAuthEnabled,
			ServiceTokensEnabled: cfg.MCPServiceTokensEnabled,
			PMWriteEnabled:       cfg.MCPPMWriteEnabled,
			DocsWriteEnabled:     cfg.MCPDocsWriteEnabled,
			AgentRunEnabled:      cfg.MCPAgentRunEnabled,
			CRMEnabled:           cfg.MCPCRMEnabled,
			SupportEnabled:       cfg.MCPSupportEnabled,
		},
	)
	supportInboxService.SetAuthzService(authzService)

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
	emailDiagnosticsConfig := model.EmailDiagnosticsConfig{
		AppEmailConfigured:        appEmailClient != nil,
		ReplyEmailConfigured:      replyEmailClient != nil,
		RouteEmailConfigured:      strings.TrimSpace(cfg.PostmarkRouteServerToken) != "",
		RedisConfigured:           redisClient != nil,
		FallbackPollerEnabled:     redisClient != nil && replyEmailClient != nil,
		AppFromEmail:              cfg.PostmarkAppFromEmail,
		ReplyFromEmail:            cfg.PostmarkReplyFromEmail,
		VerifiedFallbackFromEmail: cfg.PostmarkReplyFromEmail,
		SupportEmailReplyDomain:   cfg.SupportEmailReplyDomain,
		SupportEmailRouteDomain:   cfg.SupportEmailRouteDomain,
		ReplyInboundSecretSet:     strings.TrimSpace(cfg.PostmarkReplyInboundWebhookSecret) != "",
		RouteInboundSecretSet:     strings.TrimSpace(cfg.PostmarkRouteInboundWebhookSecret) != "",
	}

	var setupHandler *handler.SetupHandler
	if setupSuccessEnabled {
		setupHandler = handler.NewSetupHandler(setupService, authzService)
	}
	handlers := router.Handlers{
		WidgetRateLimit: middleware.WidgetRateLimit(redisClient),
		Health:          handler.NewHealthHandler(s3Client, geoIPResolver),
		Auth: handler.NewAuthHandler(authService, handler.GoogleOAuthConfig{
			ClientID:     cfg.GoogleAuthClientID,
			ClientSecret: cfg.GoogleAuthClientSecret,
			RedirectURL:  cfg.GoogleAuthRedirectURL,
			AppBaseURL:   cfg.AppBaseURL,
		}),
		Passkey:             handler.NewPasskeyHandler(passkeyService),
		Organization:        handler.NewOrganizationHandler(orgService),
		Workspace:           handler.NewWorkspaceHandler(workspaceService, authzService),
		Setup:               setupHandler,
		Billing:             handler.NewBillingHandler(billingService, cfg.StripeWebhookSecret, cfg.AppBaseURL, billingTestScenarioService, strings.EqualFold(os.Getenv("BILLING_TEST_SCENARIOS_ENABLED"), "true")),
		Settings:            handler.NewSettingsHandler(settingsService, automationInventoryService),
		Automation:          handler.NewAutomationHandler(automationInventoryService, ruleEngine, agentService, flowTemplateRegistry, flowTemplateInstaller, flowTemplateUninstaller),
		Invite:              handler.NewInviteHandler(inviteService),
		PMWorkflow:          handler.NewPMWorkflowHandler(pmWorkflowService),
		PMImport:            handler.NewPMImportHandler(pmImportService),
		PMLabel:             handler.NewPMLabelHandler(pmLabelService),
		PMEpic:              handler.NewPMEpicHandler(pmEpicService),
		PMRoadmap:           handler.NewPMRoadmapHandler(pmRoadmapService),
		PMSprint:            handler.NewPMSprintHandler(pmSprintService),
		PMTask:              handler.NewPMTaskHandler(pmTaskService),
		PMComment:           handler.NewPMCommentHandler(pmCommentService),
		PMAttachment:        handler.NewPMAttachmentHandler(pmAttachmentService),
		PMObjective:         handler.NewPMObjectiveHandler(pmObjectiveService),
		PMChecklistItem:     handler.NewPMChecklistItemHandler(pmChecklistItemService),
		PMExternalLink:      handler.NewPMExternalLinkHandler(pmExternalLinkService),
		PMView:              handler.NewPMViewHandler(pmViewService),
		Search:              handler.NewSearchHandler(searchService),
		CommandBar:          handler.NewCommandBarHandler(commandBarService, authzService),
		DockChat:            handler.NewDockChatHandler(dockChatService, agentService),
		PMAutomation:        handler.NewPMAutomationHandler(pmAutomationService),
		AutomationRule:      handler.NewAutomationRuleHandler(ruleEngine),
		PMTaskTemplate:      handler.NewPMTaskTemplateHandler(pmTaskTemplateService),
		PMRecurringTemplate: handler.NewPMRecurringTemplateHandler(pmRecurringTemplateService),
		Agent:               handler.NewAgentHandler(agentService),
		AgentRuntimeHost:    handler.NewAgentRuntimeHostHandler(agentRuntimeHostService).SetProjectionService(agentRuntimeProjectionService),
		MCP:                 handler.NewMCPHandler(mcpService),
		ExternalMCP:         handler.NewExternalMCPHandler(externalMCPService, agentService, authzService, cfg.AppBaseURL),
		SupportInbox:        handler.NewSupportInboxHandler(supportInboxService, agentService, supportMessageActionsService),
		SupportInboxView:    handler.NewSupportInboxViewHandler(supportInboxViewService),
		SupportTag:          handler.NewSupportTagHandler(supportTagService),
		SupportInboxWidget:  handler.NewSupportInboxWidgetHandler(supportInboxService),
		SupportAI:           handler.NewSupportAIHandler(supportAIService, supportInboxService, agentKnowledgeSourceService, supportContentSourceService, agentContentSourceService, curatedGuidanceService),
		SupportAttachment:   handler.NewSupportAttachmentHandler(supportAttachmentService, supportInboxService),
		PostmarkInbound:     handler.NewPostmarkInboundHandler(emailFallbackService, cfg.PostmarkReplyInboundWebhookSecret, cfg.PostmarkRouteInboundWebhookSecret),
		EmailImageProxy:     handler.NewEmailImageProxyHandler(),
		AdminWebhookEvent:   handler.NewAdminWebhookEventHandler(supportEmailWebhookEventRepo),
		AdminEmailQueue:     handler.NewAdminEmailQueueHandler(emailFallbackService, supportEmailLogRepo, supportEmailWebhookEventRepo, emailDiagnosticsConfig),
		Git:                 handler.NewGitHandler(gitService, gitWebhookEventRepo),
		Notification:        handler.NewNotificationHandler(notificationService, followerService),
		UserNotifSettings:   handler.NewUserNotificationSettingsHandler(userNotifSettingsService),
		CRMContact:          handler.NewCRMContactHandler(crmContactService),
		CRMCompany:          handler.NewCRMCompanyHandler(crmCompanyService),
		CRMDeal:             handler.NewCRMDealHandler(crmDealService),
		CRMAssociation:      handler.NewCRMAssociationHandler(crmAssociationService),
		Associations:        handler.NewAssociationsHandler(associationsService),
		CRMActivity:         handler.NewCRMActivityHandler(crmActivityService),
		CRMImport:           handler.NewCRMImportHandler(crmImportService),
		CRMEmail:            handler.NewCRMEmailHandler(crmEmailService, cfg.AppBaseURL),
		CRMCalendar:         handler.NewCRMCalendarHandler(crmCalendarService),
		CRMEnrichment:       handler.NewCRMEnrichmentHandler(crmEnrichmentService),
		CRMSignal:           handler.NewCRMSignalHandler(crmSignalService),
		CRMSummary:          handler.NewCRMSummaryHandler(crmSummaryService),
		CRMSuggestion:       handler.NewCRMSuggestionHandler(crmSuggestionService),
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
			docsBlockService,
			docsAISectionService,
			docsChangeProposalService,
			docsReferencesService,
			docsVersionService,
			docsLinkService,
			docsHelpcenterService,
			docsHelpcenterTranslationService,
			docsSearchService,
			docsImportService,
			docsEmbeddingService,
			docsEmbedResolverService,
			docsEntityReferenceResolverService,
			agentService,
			pmCommentService,
			jwtManager,
		),
		SupportCoverage: handler.NewSupportCoverageHandler(
			supportCoverageService,
			supportEventService,
			service.NewSupportCoverageDraftService(supportCoverageRepo, docsDocumentService, docsContentService, docsVersionService, llmProvider),
			supportCoverageClusterRebuildService,
		),
		TLSAsk: handler.NewTLSAskHandler(tlsAskService),
	}

	// Set support event recorder on DocsHandler after handler creation.
	handlers.Docs.SetSupportEventRecorder(supportEventRecorder)
	handlers.Docs.SetSupportWidgetConfigProvider(supportInboxService)

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
	if err := ruleEngine.EnsureScheduledRules(context.Background()); err != nil {
		slog.Error("failed to ensure automation rule schedules", "error", err)
	}
	if terminated, err := removeLegacyAgentScheduleWorkflows(context.Background(), temporalClient); err != nil {
		slog.Error("failed to remove legacy agent schedule workflows", "error", err)
	} else if terminated > 0 {
		slog.Info("removed legacy agent schedule workflows", "count", terminated)
	}
	if err := supportCoverageService.EnsureDailyEnrichment(context.Background()); err != nil {
		slog.Error("failed to ensure coverage gap daily enrichment workflow", "error", err)
	}
	if err := supportCoverageDailyAnalyzer.EnsureDailyAnalysis(context.Background()); err != nil {
		slog.Error("failed to ensure coverage daily analysis workflow", "error", err)
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

	// Start background ticker for Helpin-managed trial expiry (daily).
	billingTrialExpiryDone := make(chan struct{})
	go func() {
		runBillingTrialExpirySweep := func() {
			count, err := billingService.ExpireOverdueTrials(context.Background())
			if err != nil {
				slog.Error("billing trial expiry sweep failed", "error", err)
				return
			}
			if count > 0 {
				slog.Info("billing trial expiry sweep complete", "expired_count", count)
			}
		}

		runBillingTrialExpirySweep()

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runBillingTrialExpirySweep()
			case <-billingTrialExpiryDone:
				return
			}
		}
	}()

	// Start email fallback workers only when both Redis and Postmark are available.
	var emailFallbackCancel context.CancelFunc
	if redisClient != nil && replyEmailClient != nil {
		var emailFallbackCtx context.Context
		emailFallbackCtx, emailFallbackCancel = context.WithCancel(context.Background())
		slog.Info("email fallback workers starting",
			"redis_configured", redisClient != nil,
			"postmark_reply_configured", replyEmailClient != nil,
		)
		go emailFallbackService.StartPoller(emailFallbackCtx)
		go emailFallbackService.StartReconciler(emailFallbackCtx)
	} else {
		slog.Warn("email fallback workers not started",
			"redis_configured", redisClient != nil,
			"postmark_reply_configured", replyEmailClient != nil,
		)
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
		ReadTimeout:  5 * time.Minute,
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
	agentRuntimeProjectionCancel()
	if emailFallbackCancel != nil {
		emailFallbackCancel()
	}
	close(digestDone)
	close(supportReplyEmailDone)
	close(billingTrialExpiryDone)
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

func removeLegacyAgentScheduleWorkflows(ctx context.Context, client tclient.Client) (int, error) {
	if client == nil {
		return 0, nil
	}

	query := `WorkflowId STARTS_WITH "agent-schedule-" AND CloseTime IS NULL`
	var (
		terminated    int
		nextPageToken []byte
	)

	for {
		resp, err := client.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{
			Query:         query,
			PageSize:      200,
			NextPageToken: nextPageToken,
		})
		if err != nil {
			return terminated, err
		}
		for _, execution := range resp.GetExecutions() {
			workflowExec := execution.GetExecution()
			if workflowExec == nil || strings.TrimSpace(workflowExec.GetWorkflowId()) == "" {
				continue
			}
			if err := client.TerminateWorkflow(ctx, workflowExec.GetWorkflowId(), workflowExec.GetRunId(), "legacy agent schedule workflow removed"); err != nil {
				var notFound *serviceerror.NotFound
				if errors.As(err, &notFound) {
					continue
				}
				return terminated, err
			}
			terminated++
		}
		nextPageToken = resp.GetNextPageToken()
		if len(nextPageToken) == 0 {
			break
		}
	}
	return terminated, nil
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

func resolveTOTPEncryptionKey(cfg *config.Config) []byte {
	if cfg == nil {
		return nil
	}
	if key, err := decodeOptionalAES256HexKey(strings.TrimSpace(cfg.TOTPEncryptionKey)); err != nil {
		slog.Warn("invalid TOTP_ENCRYPTION_KEY (must be a 32-byte hex-encoded AES key)", "error", err)
	} else if len(key) == 32 {
		return key
	}
	if key, err := decodeOptionalAES256HexKey(strings.TrimSpace(cfg.CRMEncryptionKey)); err != nil {
		slog.Warn("invalid CRM_ENCRYPTION_KEY for TOTP fallback (must be a 32-byte hex-encoded AES key)", "error", err)
	} else if len(key) == 32 {
		return key
	}
	return nil
}

func resolvePMImportEncryptionKey(cfg *config.Config) []byte {
	if cfg == nil {
		return nil
	}
	if key, err := decodeOptionalAES256HexKey(strings.TrimSpace(cfg.PMImportEncryptionKey)); err != nil {
		slog.Warn("invalid PM_IMPORT_ENCRYPTION_KEY (must be a 32-byte hex-encoded AES key)", "error", err)
	} else if len(key) == 32 {
		return key
	}
	if key, err := decodeOptionalAES256HexKey(strings.TrimSpace(cfg.CRMEncryptionKey)); err != nil {
		slog.Warn("invalid CRM_ENCRYPTION_KEY for PM import fallback (must be a 32-byte hex-encoded AES key)", "error", err)
	} else if len(key) == 32 {
		return key
	}
	return nil
}

func resolveGitOAuthEncryptionKey(cfg *config.Config) []byte {
	if cfg == nil {
		return nil
	}
	if key, err := decodeOptionalAES256HexKey(strings.TrimSpace(cfg.GitOAuthEncryptionKey)); err != nil {
		slog.Warn("invalid GIT_OAUTH_ENCRYPTION_KEY (must be a 32-byte hex-encoded AES key)", "error", err)
	} else if len(key) == 32 {
		return key
	}
	if key, err := decodeOptionalAES256HexKey(strings.TrimSpace(cfg.CRMEncryptionKey)); err != nil {
		slog.Warn("invalid CRM_ENCRYPTION_KEY for git oauth fallback (must be a 32-byte hex-encoded AES key)", "error", err)
	} else if len(key) == 32 {
		return key
	}
	return nil
}

func newPasskeySessionCache(redisClient *redis.Client, podID string) cache.Cache {
	l1 := cache.NewLRU(2048)
	if redisClient == nil {
		slog.Info("passkey cache: L1-only (no Redis) — single-pod consistency only")
		return l1
	}

	l2 := cache.NewRedis(redisClient, "passkey")
	tiered := cache.NewTiered(cache.TieredConfig{
		L1:      l1,
		L2:      l2,
		Redis:   redisClient,
		Channel: "cache:passkey:invalidate",
		PodID:   podID,
		L1TTL:   5 * time.Minute,
	})
	tiered.StartInvalidationSubscriber(context.Background())
	slog.Info("passkey cache: tiered L1+L2 (Redis) enabled")
	return tiered
}

func decodeOptionalAES256HexKey(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	key, err := hex.DecodeString(trimmed)
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("expected 32 bytes after hex decode, got %d", len(key))
	}
	return key, nil
}
