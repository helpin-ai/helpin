package main

import (
	"context"
	"database/sql"
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

	firebase "firebase.google.com/go/v4"
	clickhouse "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"go.temporal.io/api/serviceerror"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	tclient "go.temporal.io/sdk/client"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/option"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/ee/billingstripe"
	eehandler "github.com/helpin-ai/helpin/server/ee/handler"
	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	eeservice "github.com/helpin-ai/helpin/server/ee/service"
	"github.com/helpin-ai/helpin/server/internal/aimodel"
	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/ee/pricing"
	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/cache"
	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/coordination"
	"github.com/helpin-ai/helpin/server/internal/crawler"
	"github.com/helpin-ai/helpin/server/internal/crmemail"
	"github.com/helpin-ai/helpin/server/internal/crmsignal"
	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/geoip"
	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/handler"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/meetingcapture"
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

	var clickHouseDB *sql.DB
	if cfg.ClickHouseDSN != "" {
		clickHouseOptions, parseErr := eventClickHouseOptions(cfg.ClickHouseDSN)
		if parseErr != nil {
			fatalWithSentry("failed to parse ClickHouse DSN", parseErr)
		}
		clickHouseCtx, cancelClickHouse := context.WithTimeout(context.Background(), 5*time.Second)
		clickHouseDB = clickhouse.OpenDB(clickHouseOptions)
		if err := clickHouseDB.PingContext(clickHouseCtx); err != nil {
			slog.Warn("ClickHouse unavailable; CRM behavioral signals disabled", "error", err)
			closeClickHouse(clickHouseDB)
			clickHouseDB = nil
		}
		if clickHouseDB != nil {
			retention, err := repository.InspectEventRetentionPolicy(clickHouseCtx, clickHouseDB)
			if err != nil {
				slog.Warn("ClickHouse event schema unavailable; CRM behavioral signals disabled", "error", err)
				closeClickHouse(clickHouseDB)
				clickHouseDB = nil
			} else {
				defer closeClickHouse(clickHouseDB)
				slog.Info("connected to ClickHouse for CRM behavioral signals",
					"ttl_configured", retention.TTLConfigured, "ttl_clause", retention.TTLClause)
			}
		}
		cancelClickHouse()
	}

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
			&model.OAuthMobileHandoff{},
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
			&model.PMTaskUpdateRead{},
			&model.PMTaskStandingBrief{},
			&model.PMTaskBriefSuggestionDismissal{},
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
			&model.DockActionProposal{},
			// Public MCP tables are intentionally excluded. Their constraints,
			// partial indexes, and retention fields are owned exclusively by
			// versioned migration 202607100003_public_mcp.sql. Letting GORM
			// reconcile those tables can attempt incompatible constraint changes.
			&model.CommandBarPlanRecord{},
			&model.CommandBarPlanDismissal{},
			&model.DockChat{},
			&model.DockChatHandoff{},
			&model.PublicShare{},
			&model.SupportRunEvidence{},
			&model.HelpcenterAnswer{},
			&model.CodingSessionStateSnapshot{},
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
			// WorkspaceEventProjectAlias is owned by versioned migration
			// 202608240001, including its overlapping single-column and
			// composite uniqueness constraints. GORM cannot safely reconcile
			// those PostgreSQL constraints as indexes during AutoMigrate.
			&model.CRMIdentityLink{},
			&model.SupportCredentialRotationAudit{},
			&model.SupportWidgetSession{},
			&model.SupportAttachment{},
			&model.CRMEmailAttachment{},
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
			&model.AIActionExecution{},
			&model.CoverageBatch{},
			&model.CoverageAnalysisAttempt{},
			&model.CoverageFinding{},
			&model.CoverageTopicV2{},
			&model.CoverageAssignmentAttempt{},
			&model.CoverageTopicMembership{},
			&model.CoverageUnreviewedSignal{},
			&model.CoverageRebuildAudit{},
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
			&model.DocsAPIReference{},
			&model.DocsAPIReferenceRevision{},
			// Notifications module
			&model.Notification{},
			&model.NotificationEvent{},
			&model.NotificationDelivery{},
			&model.NotificationPreference{},
			&model.UserNotificationSettings{},
			&model.EntityFollower{},
			&model.PushDevice{},
			// CRM module
			&model.CRMContact{},
			&model.CRMCompany{},
			&model.CRMPipeline{},
			&model.CRMPipelineStage{},
			&model.CRMDeal{},
			&model.CRMAssociation{},
			&model.CRMActivity{},
			&model.CRMImportJob{},
			&model.CRMMeeting{},
			&model.CRMMeetingCapture{},
			&model.CRMMeetingTranscript{},
			&model.CRMMeetingIntelligence{},
			&model.CRMMeetingActionItem{},
			&model.CRMMeetingSettings{},
			&model.CRMMeetingProviderEvent{},
			// CRM Phase 3: Email & Calendar
			&model.CRMEmailAccount{},
			&model.CRMEmailThread{},
			&model.CRMEmailMessage{},
			&model.CRMEmailMessageContact{},
			&model.CRMCalendarEvent{},
			&model.CRMCalendarSeriesPreference{},
			// CRM Phase 4: Intelligence
			&model.CRMEnrichmentResult{},
			&model.CRMSignal{},
			&model.CRMSignalObservation{},
			&model.CRMSignalInterpretationConfig{},
			&model.CRMSignalMotionState{},
			&model.CRMSignalRuleConfig{},
			&model.CRMSignalEvaluationRun{},
			&model.CRMSignalEvaluatorWatermark{},
			&model.CRMSignalScoringConfig{},
			&model.CRMSignalFeedback{},
			&model.CRMSignalRoutingPolicy{},
			&model.CRMSignalRoutingSettings{},
			&model.CRMSignalRolloutSettings{},
			&model.CRMCompanyCommercialState{},
			&model.CRMCompanyCommercialStateHistory{},
			&model.CRMCompanyCommercialStateHealth{},
			&model.CRMSignalConditionState{},
			&model.CRMUsageWeekdayBaseline{},
			&model.CRMSignalBatchSuppression{},
			&model.CRMSignalDelivery{},
			&model.CRMSignalExternalEvidence{},
			&model.CRMEntitySummary{},
			&model.CRMDealHealthScore{},
			&model.CRMSuggestion{},
			// Signals, Playbooks, history and scheduled events are versioned-SQL-owned;
			// AutoMigrate must not pre-create these models without their constraints.
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

	geoIPResolver := geoip.Open(geoip.Options{
		Path:        cfg.MaxMindDBPath,
		DownloadURL: cfg.MaxMindDownloadURL,
		AccountID:   cfg.MaxMindAccountID,
		LicenseKey:  cfg.MaxMindLicenseKey,
	})
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
	// Agent-runtime transcript projections are published through the durable
	// workspace-event bridge. Every API pod consumes that bridge independently,
	// so all WebSocket owners observe one sequenced publication path.
	agentRuntimeProjectionPublisher := ws.NewOrderedJetStreamPublisher(jetstream)

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
	pmAISuggestionRepo := repository.NewPMAISuggestionRepository(db)
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
	dockActionProposalRepo := repository.NewDockActionProposalRepository(db)
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
	billingRepo := eerepository.NewBillingRepository(db)
	customerIOOutboxRepo := repository.NewCustomerIOLifecycleOutboxRepository(db)
	productAnalyticsOutboxRepo := repository.NewProductAnalyticsOutboxRepository(db)
	supportTagRepo := repository.NewSupportTagRepository(db)
	productAnalytics := service.NewProductAnalyticsService(productAnalyticsOutboxRepo)
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
		eeservice.NewCustomerIOBillingReader(billingRepo, workspaceRepo),
	)
	modelCatalog, modelCatalogErr := aimodel.LoadCatalog()
	if modelCatalogErr != nil {
		log.Fatalf("load model catalog: %v", modelCatalogErr)
	}
	pricingCatalog, pricingCatalogErr := pricing.LoadCatalog()
	if pricingCatalogErr != nil {
		log.Fatalf("load AI pricing catalog: %v", pricingCatalogErr)
	}
	aiUsageRepo := eerepository.NewAIUsageRepository(db)
	aiUsageService := eeservice.NewAIUsageService(pricingCatalog, aiUsageRepo, nil)
	stripeGateway := billingstripe.New(cfg.StripeSecretKey)
	settlementCtx, settlementCancel := context.WithCancel(context.Background())
	settlementDone := make(chan struct{})
	if stripeGateway != nil {
		settlementWorker := eeservice.NewAIUsageSettlementWorker(aiUsageRepo, stripeGateway)
		go func() {
			defer close(settlementDone)
			settlementWorker.Run(settlementCtx, time.Minute)
		}()
	} else {
		close(settlementDone)
	}
	billingService := eeservice.NewBillingService(billingRepo, stripeGateway, time.Now)
	periodCtx, periodCancel := context.WithCancel(context.Background())
	periodDone := make(chan struct{})
	periodWorker := eeservice.NewAIUsagePeriodWorker(aiUsageRepo, billingService.NextAIUsagePeriodSchedule)
	go func() {
		defer close(periodDone)
		periodWorker.Run(periodCtx, time.Minute)
	}()
	reservationCtx, reservationCancel := context.WithCancel(context.Background())
	reservationDone := make(chan struct{})
	reservationSweeper := eeservice.NewAIUsageReservationSweeper(aiUsageRepo)
	go func() {
		defer close(reservationDone)
		reservationSweeper.Run(reservationCtx, time.Minute, 15*time.Minute)
	}()
	billingTestScenarioService := eeservice.NewBillingTestScenarioService(db, billingService, time.Now)
	billingService.SetPriceConfig(eeservice.BillingPriceConfig{
		StarterMonthly: cfg.StripeStarterMonthlyPriceID,
		StarterAnnual:  cfg.StripeStarterAnnualPriceID,
		GrowthMonthly:  cfg.StripeGrowthMonthlyPriceID,
		GrowthAnnual:   cfg.StripeGrowthAnnualPriceID,
	})
	billingService.SetWorkspaceRepository(workspaceRepo)
	billingService.SetCustomerIOIdentityService(customerIOIdentityService)
	billingService.SetCustomerIOLifecycleOutboxRepository(customerIOOutboxRepo)
	customerIOOutboxWorker := service.NewCustomerIOLifecycleOutboxWorker(customerIOOutboxRepo, workspaceRepo, customerIOIdentityService)
	billingService.SetProductAnalyticsService(productAnalytics)
	customerIOOutboxCtx, customerIOOutboxCancel := context.WithCancel(context.Background())
	customerIOOutboxDone := make(chan struct{})
	go func() {
		defer close(customerIOOutboxDone)
		customerIOOutboxWorker.Run(customerIOOutboxCtx, 15*time.Second)
	}()
	productAnalyticsWorker := service.NewProductAnalyticsOutboxWorker(
		productAnalyticsOutboxRepo,
		userRepo,
		workspaceRepo,
		orgRepo,
		billingRepo,
		service.NewUsermavenClient(service.UsermavenConfig{
			APIKey:      cfg.UsermavenAPIKey,
			ServerToken: cfg.UsermavenServerToken,
			Endpoint:    cfg.UsermavenEndpoint,
		}),
	)
	productAnalyticsCtx, productAnalyticsCancel := context.WithCancel(context.Background())
	productAnalyticsDone := make(chan struct{})
	go func() {
		defer close(productAnalyticsDone)
		productAnalyticsWorker.Run(productAnalyticsCtx, 15*time.Second)
	}()
	aiUsageMeter := service.NewTokenPricedAIUsageMeter(aiUsageService)
	aiActionExecutionRepo := repository.NewAIActionExecutionRepository(db)
	aiActionRegistry := aipolicy.DefaultRegistry()
	if err := aiActionRegistry.Validate(); err != nil {
		fatalWithSentry("validate AI action registry", err)
	}
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
	docsAPIReferenceRepo := repository.NewDocsAPIReferenceRepository(db)
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
	pushDeviceRepo := repository.NewPushDeviceRepository(db)
	crmContactRepo := repository.NewCRMContactRepository(db)
	crmCompanyRepo := repository.NewCRMCompanyRepository(db)
	crmCompanyTimelineRepo := repository.NewCRMCompanyTimelineRepository(db)
	crmDealRepo := repository.NewCRMDealRepository(db)
	crmAssociationRepo := repository.NewCRMAssociationRepository(db)
	crmActivityRepo := repository.NewCRMActivityRepository(db)
	crmImportRepo := repository.NewCRMImportRepository(db)
	crmEmailRepo := repository.NewCRMEmailRepository(db)
	crmEmailAttachmentRepo := repository.NewCRMEmailAttachmentRepository(db)
	crmCalendarRepo := repository.NewCRMCalendarRepository(db)
	crmEnrichmentRepo := repository.NewCRMEnrichmentRepository(db)
	crmSignalRepo := repository.NewCRMSignalRepository(db)
	eventProjectRepo := repository.NewEventProjectRepository(db)
	crmSummaryRepo := repository.NewCRMSummaryRepository(db)
	crmMeetingRepo := repository.NewCRMMeetingRepository(db)
	pmTaskInsightsRepo := repository.NewPMTaskInsightsRepository(db)
	crmSuggestionRepo := repository.NewCRMSuggestionRepository(db)
	crmSituationRepo := repository.NewCRMSituationRepository(db)
	crmSituationRepo.SetInboxSignalComposer(service.ComposeCRMInboxSignalGroups)
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
	oauthMobileHandoffRepo := repository.NewOAuthMobileHandoffRepository(db)
	passkeySessionCache := newPasskeySessionCache(redisClient, podID)
	passkeyWebAuthnClient, err := appwebauthn.NewClient(cfg.WebAuthnRPID, cfg.WebAuthnRPOrigins, passkeySessionCache)
	if err != nil {
		fatalWithSentry("failed to initialize webauthn", err)
	}
	authService := service.NewAuthService(userRepo, passwordResetRepo, orgRepo, workspaceRepo, emailVerificationRepo, jwtManager, s3Client, appEmailClient, cfg.AppBaseURL, resolveTOTPEncryptionKey(cfg))
	authService.SetOAuthMobileHandoffRepository(oauthMobileHandoffRepo)
	authService.SetCustomerIOIdentityService(customerIOIdentityService)
	authService.SetProductAnalyticsService(productAnalytics)
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
	pushDeviceService := service.NewPushDeviceService(pushDeviceRepo)
	var fcmClient service.FCMClient
	if cfg.FCMServiceAccountJSON != "" {
		fbApp, err := firebase.NewApp(context.Background(), nil, option.WithCredentialsJSON([]byte(cfg.FCMServiceAccountJSON)))
		if err != nil {
			slog.Error("failed to initialize firebase app, mobile push disabled", "error", err)
		} else if msgClient, err := fbApp.Messaging(context.Background()); err != nil {
			slog.Error("failed to initialize firebase messaging client, mobile push disabled", "error", err)
		} else {
			fcmClient = service.NewFirebaseFCMClient(msgClient)
			slog.Info("firebase cloud messaging initialized")
		}
	}
	pushSenderService := service.NewPushSenderService(pushDeviceRepo, fcmClient)
	followerService := service.NewFollowerService(followerRepo)
	pmAISuggestionService := service.NewPMAISuggestionService(pmAISuggestionRepo)
	pmTaskService := service.NewPMTaskService(pmTaskRepo, workspaceRepo, pmWorkflowRepo, pmEpicRepo, pmSprintRepo, pmLabelRepo, pmChecklistItemRepo, pmExternalLinkRepo, pmAttachmentRepo, pmActivityService, wsPublisher, pmAutomationService, notificationService, followerService)
	pmTaskService.SetProductAnalyticsService(productAnalytics)
	pmTaskService.SetTaskTemplateRepository(pmTaskTemplateRepo)
	pmRoadmapRepo := repository.NewPMRoadmapRepository(db)
	pmEpicService := service.NewPMEpicService(pmEpicRepo, pmTaskRepo, pmLabelRepo, gitRepositoryRepo, pmAttachmentRepo, workspaceRepo, pmActivityService, wsPublisher, notificationService)
	pmEpicService.SetWorkflowRepository(pmWorkflowRepo)
	pmEpicService.SetProductAnalyticsService(productAnalytics)
	pmRoadmapService := service.NewPMRoadmapService(pmEpicService, pmRoadmapRepo)
	pmSprintService := service.NewPMSprintService(pmSprintRepo, pmTaskRepo, pmLabelRepo, pmAttachmentRepo, workspaceRepo, settingsRepo, pmActivityService, wsPublisher, notificationService, pmSprintCloseoutRepo)
	pmCommentService := service.NewPMCommentService(pmCommentRepo, pmTaskRepo, pmAttachmentRepo, pmActivityService, wsPublisher, notificationService, workspaceRepo, s3Client)
	pmCommentService.SetProductAnalyticsService(productAnalytics)
	pmAttachmentService := service.NewPMAttachmentService(pmAttachmentRepo, s3Client, wsPublisher)
	docsImageEditService := service.NewDocsImageEditService(cfg.FalAPIKey, pmAttachmentService)
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
	supportInboxService.SetProductAnalyticsService(productAnalytics)
	supportInboxService.SetDocsSearchRepository(docsSearchRepo)
	supportInboxService.SetCRMCompanyRepository(crmCompanyRepo)
	supportInboxService.SetSupportTagRepo(supportTagRepo)
	supportLinkPreviewService := service.NewSupportLinkPreviewService(cfg.CrawlerProxyURLs)
	supportLinkPreviewService.SetLinkScanner(service.NewGoogleWebRiskClient(cfg.GoogleWebRiskAPIKey))
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
	supportEmbeddingProvider = service.NewGovernedEmbeddingProvider(
		supportEmbeddingProvider, aiUsageMeter, aiActionRegistry, aiActionExecutionRepo,
	)
	coverageEmbeddingProvider := supportEmbeddingProvider
	completionRoutes := service.NewAICompletionRouteRegistry(service.CRMCompletionRouteConfig{
		Primary: service.AICompletionRoute{
			Provider: cfg.CRMLLMProvider, Model: cfg.CRMLLMModel,
			OpenRouterProvider: cfg.CRMLLMOpenRouterProvider,
		},
		Fallback: service.AICompletionRoute{
			Provider: cfg.CRMLLMFallbackProvider, Model: cfg.CRMLLMFallbackModel,
			OpenRouterProvider: cfg.CRMLLMFallbackOpenRouterProvider,
		},
		MeetingFallback: service.AICompletionRoute{
			Provider: cfg.CRMMeetingFallbackProvider, Model: cfg.CRMMeetingFallbackModel,
			OpenRouterProvider: cfg.CRMMeetingFallbackOpenRouterProvider,
		},
	})
	if issues := completionRoutes.Validate(modelCatalog); len(issues) != 0 {
		fatalWithSentry("validate AI completion model routes", errors.Join(issues...))
	}
	if issues := completionRoutes.ValidateProviders(supportLLMRouter.HasChatProvider); len(issues) != 0 {
		fatalWithSentry("validate AI completion providers", errors.Join(issues...))
	}
	agentTierResolver := service.NewAgentModelTierResolver(modelCatalog, supportLLMRouter.HasChatProvider)
	if issues := agentTierResolver.ValidateSelectable(); len(issues) != 0 {
		fatalWithSentry("validate agent model sizes", errors.Join(issues...))
	}
	supportLLMProvider := service.NewAICompletionService(supportLLMRouter, aiUsageService, completionRoutes).
		SetGovernance(aiActionRegistry, aiActionExecutionRepo)
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
	pmEpicService.SetGitService(gitService)
	var agentRuntimeClient *service.AgentRuntimeClient
	if strings.TrimSpace(cfg.AgentRuntimeBaseURL) != "" {
		agentRuntimeClient, err = service.NewAgentRuntimeClient(cfg.AgentRuntimeBaseURL, cfg.AgentRuntimeAppID, cfg.AgentRuntimeServiceToken, nil, cfg.AgentRuntimeEventProtocol)
		if err != nil {
			fatalWithSentry("failed to initialize agent runtime client", err)
		}
	}
	aiConnectionService, err := service.NewAIConnectionService(repository.NewAIConnectionRepository(db), modelCatalog, agentRuntimeClient, service.AIConnectionConfig{
		EncryptionKey: cfg.AIConnectionEncryptionKey, ChatGPTEnabled: cfg.ChatGPTConnectionsEnabled, ChatGPTClientID: cfg.ChatGPTClientID, AppID: cfg.AgentRuntimeAppID,
	})
	if err != nil {
		fatalWithSentry("failed to initialize AI connections", err)
	}
	aiProfileService := service.NewAIProfileService(repository.NewAIProfileRepository(db), aiConnectionService)
	externalMCPService, err := service.NewExternalMCPService(
		externalMCPRepo,
		notificationService,
		service.ExternalMCPServiceConfig{
			Enabled:                cfg.ExternalMCPEnabled,
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
		cfg.OpenRouterAPIKey).SetTriggerExecutionRepository(agentTriggerExecutionRepo).SetCommandBarPlanRepository(commandBarPlanRepo).SetWorkspaceRepository(workspaceRepo).SetUserRepository(userRepo).SetWorkspaceSkillStore(workspaceSkillRepo, s3Client).SetNotificationService(notificationService).SetAgentTemplateRepository(agentTemplateRepo).SetCRMRepositories(crmContactRepo, crmCompanyRepo, crmDealRepo).SetAgentDraftLLM(supportLLMProvider).SetModelTierResolver(agentTierResolver).SetAIUsageMeter(aiUsageMeter).SetAgentRuntimeLaunchEnabled(cfg.AgentRuntimeLaunchEnabled)
	agentService.SetProductAnalyticsService(productAnalytics)
	if agentRuntimeClient != nil {
		agentService.SetAgentRuntimeClient(agentRuntimeClient)
	}
	agentService.SetExternalMCPService(externalMCPService)
	agentService.SetAIConnectionService(aiConnectionService).SetAIProfileService(aiProfileService)
	commandBarService := service.NewCommandBarService(agentService, commandBarPlanRepo, commandBarPlanDismissalRepo).
		SetWebsocketPublisher(wsPublisher)
	supportInboxService.SetConversationAgentRunner(agentService.RunConversationAgentAuto)
	supportInboxService.SetNotificationService(notificationService, workspaceRepo)
	supportInboxService.SetPushSenderService(pushSenderService)
	emailFallbackService.SetNotificationService(notificationService)
	emailFallbackService.SetPushSenderService(pushSenderService)

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
	agentService.SetPMObjectiveService(pmObjectiveService)
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

	// Every product-owned direct completion shares the validated route registry.
	var llmProvider llm.Provider = supportLLMProvider

	docsSpaceService := service.NewDocsSpaceService(docsSpaceRepo, wsPublisher)
	docsAPIReferenceService := service.NewDocsAPIReferenceService(docsAPIReferenceRepo, docsSpaceRepo)
	docsCollectionService := service.NewDocsCollectionService(docsCollectionRepo, docsSpaceRepo, wsPublisher, cfg.DocsOrderingUseSortKey)
	docsDocumentService := service.NewDocsDocumentService(docsDocumentRepo, docsSpaceRepo, wsPublisher, cfg.DocsOrderingUseSortKey)
	docsDocumentService.SetProductAnalyticsService(productAnalytics)
	docsDocumentService.SetRuleEngine(ruleEngine)
	docsContentService := service.NewDocsContentService(docsContentRepo, docsDocumentRepo, wsPublisher)
	docsContentService.SetMentionNotificationDependencies(notificationService, workspaceRepo)
	docsContentService.SetAgentMentionDependencies(agentService)
	docsBlockService := service.NewDocsBlockService(docsBlockRepo, docsContentService, docsDocumentRepo)
	docsBlockService.SetActivityService(pmActivityService)
	docsSearchService := service.NewDocsSearchService(docsSearchRepo)
	docsAISectionService := service.NewDocsAISectionService(
		docsAISectionCandidateRepo,
		docsBlockRepo,
		docsBlockService,
		docsDocumentRepo,
		agentService,
	)
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
	docsHelpcenterService.SetPublicationArtifactDependencies(agentRunArtifactRepo, s3Client)
	tlsAskService := service.NewTLSAskService(docsHelpcenterRepo, cfg.TLSAskExtraAllowedDomains)

	// Tiered cache for hot public help-center reads. L1 is an in-process LRU;
	// L2 is Redis when available so cache entries survive pod restarts and
	// invalidations fan out across pods via pub/sub.
	hcL1 := cache.NewLRU(2048)
	var hcCache cache.Cache = hcL1
	if redisClient != nil {
		// Public help-center reads must fail open when the shared remote cache is
		// degraded. Use a dedicated client with bounded retries/timeouts so an
		// L2 miss or write can never hold a page response for many seconds.
		hcRedisOpts := *redisClient.Options()
		hcRedisOpts.DialTimeout = 500 * time.Millisecond
		hcRedisOpts.ReadTimeout = 250 * time.Millisecond
		hcRedisOpts.WriteTimeout = 250 * time.Millisecond
		hcRedisOpts.PoolTimeout = 500 * time.Millisecond
		hcRedisOpts.MaxRetries = -1
		hcRedisClient := redis.NewClient(&hcRedisOpts)
		hcL2 := cache.NewRedis(hcRedisClient, "hc")
		tiered := cache.NewTiered(cache.TieredConfig{
			L1:      hcL1,
			L2:      hcL2,
			Redis:   hcRedisClient,
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
	docsHelpcenterTranslationService.SetPublicationArtifactDependencies(agentRunArtifactRepo, s3Client)
	docsImportService := service.NewDocsImportService(
		docsImportRepo,
		docsSpaceService,
		docsCollectionService,
		docsDocumentService,
		docsContentService,
		docsHelpcenterService,
		docsRedirectRepo,
		s3Client,
		supportLLMProvider,
		service.DocsImportAIConversionConfig{
			Enabled:      cfg.DocsImportAIConversionEnabled,
			Provider:     cfg.DocsImportAIConversionProvider,
			Model:        cfg.DocsImportAIConversionModel,
			ArticleLimit: cfg.DocsImportAIConversionArticleLimit,
		},
	).SetTemporalClient(temporalClient, resolvePMImportEncryptionKey(cfg))
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
	docsSpaceService.SetAPIReferenceRepository(docsAPIReferenceRepo)
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

	crmContactService := service.NewCRMContactService(crmContactRepo).
		SetIdentitySync(crmActivityRepo, wsPublisher).
		SetTimelineRepository(crmCompanyTimelineRepo)
	crmCompanyService := service.NewCRMCompanyService(crmCompanyRepo).
		SetTimelineRepository(crmCompanyTimelineRepo)
	crmContactService.SetProductAnalyticsService(productAnalytics)
	crmCompanyService.SetProductAnalyticsService(productAnalytics)
	crmDealService := service.NewCRMDealService(crmDealRepo, crmAssociationRepo).
		SetActivityService(pmActivityService).
		SetTimelineRepository(crmCompanyTimelineRepo)
	crmDealService.SetProductAnalyticsService(productAnalytics)
	crmAssociationService := service.NewCRMAssociationService(crmAssociationRepo)
	associationsService := service.NewAssociationsService(crmAssociationRepo, crmContactRepo, workspaceRepo, pmTaskLinkRepo, pmTaskRepo, supportConversationRepo, docsLinkRepo, docsDocumentRepo)
	crmActivityService := service.NewCRMActivityService(crmActivityRepo)
	crmImportService := service.NewCRMImportService(crmImportRepo, crmContactRepo, crmCompanyRepo, crmDealRepo).SetDealService(crmDealService)

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

	crmSummaryService := service.NewCRMSummaryService(crmSummaryRepo, crmContactRepo, crmCompanyRepo, crmDealRepo, crmAssociationRepo, crmSignalRepo, crmEmailRepo, llmProvider, temporalClient).
		SetCompanyEvidenceRepositories(crmCompanyTimelineRepo, pmTaskRepo, supportConversationRepo)
	crmContactService.SetCompanySummaryRefresh(crmSummaryService)
	crmCompanyService.SetCompanySummaryRefresh(crmSummaryService)
	crmDealService.SetCompanySummaryRefresh(crmSummaryService)
	crmAssociationService.SetCompanySummaryRefresh(crmSummaryService)
	crmActivityService.SetCompanySummaryRefresh(crmSummaryService)
	pmActivityService.SetCompanySummaryRefresh(crmSummaryService)
	pmTaskInsightsService := service.NewPMTaskInsightsService(pmTaskInsightsRepo, pmTaskRepo, pmCommentRepo, pmActivityRepo, agentRunRepo, agentRepo, taskGitLinkRepo, pmChecklistItemRepo, llmProvider)
	crmEmailService := service.NewCRMEmailService(crmEmailRepo, crmContactRepo, workspaceRepo, crmEmailSyncSettingsRepo, gmailOAuth, encryptionKey, gmailSyncClient, temporalClient, crmSummaryService)
	crmEmailService.SetAttachmentStorage(crmEmailAttachmentRepo, s3Client)
	go func() {
		cleanup := func() {
			if err := crmEmailService.CleanupStaleAttachments(context.Background()); err != nil {
				slog.Error("failed to clean stale CRM email attachments", "error", err)
			}
		}
		cleanup()
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			cleanup()
		}
	}()
	crmCalendarService := service.NewCRMCalendarService(crmCalendarRepo).
		SetCompanySummaryRefresh(crmSummaryService).
		SetSignalDetection(crmsignal.NewTemporalStarter(temporalClient, temporalapp.QueueAutomation), crmSignalRepo)
	crmEnrichmentService := service.NewCRMEnrichmentService(crmEnrichmentRepo, crmContactRepo, crmCompanyRepo, crmAssociationRepo).
		SetActivityRepository(crmActivityRepo).
		SetWebsocketPublisher(wsPublisher).
		SetCompanySummaryRefresh(crmSummaryService)
	crmSignalService := service.NewCRMSignalService(crmSignalRepo, crmSummaryService).
		SetHealthScoreDependencies(crmDealRepo).
		SetActivationDependencies(notificationService)
	crmContactService.SetMotionSignalRefresher(crmSignalService)
	crmCompanyService.SetMotionSignalRefresher(crmSignalService)
	crmDealService.SetMotionSignalReconciler(crmSignalService)
	crmSuggestionService := service.NewCRMSuggestionService(crmSuggestionRepo, crmDealRepo, crmAssociationRepo).SetDealService(crmDealService)
	crmWritingProfileService := service.NewCRMWritingProfileService(crmWritingProfileRepo)
	meetingProviderHTTPClient := &http.Client{Timeout: 45 * time.Second}
	recallMeetingProvider := meetingcapture.NewRecallProvider(meetingcapture.RecallConfig{
		BaseURL:       cfg.RecallBaseURL,
		APIKey:        cfg.RecallAPIKey,
		WebhookSecret: cfg.RecallWebhookSecret,
		HTTPClient:    meetingProviderHTTPClient,
	})
	vexaMeetingProvider := meetingcapture.NewVexaProvider(meetingcapture.VexaConfig{
		BaseURL:       cfg.VexaBaseURL,
		APIKey:        cfg.VexaAPIKey,
		WebhookSecret: cfg.VexaWebhookSecret,
		HTTPClient:    meetingProviderHTTPClient,
	})
	crmMeetingService := service.NewCRMMeetingService(
		crmMeetingRepo,
		crmAssociationRepo,
		pmTaskService,
		recallMeetingProvider,
		vexaMeetingProvider,
	).SetCaptureProvider(cfg.CRMMeetingCaptureProvider).
		SetProcessingRunner(service.NewTemporalMeetingProcessingRunner(temporalClient)).
		SetAIUsageMeter(aiUsageMeter).
		SetCaptureScheduler(service.NewTemporalMeetingCaptureScheduler(temporalClient)).
		SetCalendarIntegration(crmCalendarRepo, crmEmailRepo)
	if s3Client != nil {
		crmMeetingService.SetRecordingStore(s3Client)
	}
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
	commandService.SetSupportAttachmentRepository(supportAttachmentRepo)
	commandService.SetSupportOperationalServices(supportInboxService, supportTagService)
	commandService.SetCRMReadServices(crmContactService, crmSignalService)
	commandService.SetCRMOperationalServices(crmCompanyService, crmAssociationService)
	commandService.SetWorkspaceSearchServices(searchService, crmSearchService, supportInboxService)
	commandService.SetDocsSearchRepository(docsSearchRepo)
	commandService.SetDocsChangeProposalService(docsChangeProposalService)
	commandService.SetAgentRunDependencies(agentRunRepo, agentRunArtifactRepo)
	commandService.SetDockActionProposalRepository(dockActionProposalRepo)
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
		SetPMObjectiveService(pmObjectiveService).
		SetAgentRepository(agentRepo).
		SetBrowserAssetStore(agentRunArtifactRepo, s3Client).
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
		runFinalizers.SetProductAnalyticsService(productAnalytics)
		agentRuntimeProjectionService = service.NewAgentRuntimeProjectionService(agentRunRepo, cfg.AgentRuntimeAppID).
			SetEventProtocol(cfg.AgentRuntimeEventProtocol).
			SetOverageDependencies(agentRepo, aiUsageMeter, agentRuntimeClient).
			SetTranscriptRepositories(agentRunMessageRepo, agentRunArtifactRepo, agentRunInteractionRepo).
			SetCodingSessionSnapshotRepository(codingSessionStateSnapshotRepo).
			SetWebSocketPublisher(agentRuntimeProjectionPublisher).
			SetRunFinalizers(runFinalizers)
		agentService.SetAgentRuntimeProjectionService(agentRuntimeProjectionService)
	}
	agentRuntimeProjectionCancel := context.CancelFunc(func() {})
	var projectionCtx context.Context
	if agentRuntimeProjectionService != nil {
		projectionCtx, agentRuntimeProjectionCancel = context.WithCancel(context.Background())
		projectionLeader := coordination.NewPostgresLeader(sqlDB, coordination.PostgresLeaderOptions{
			Name: fmt.Sprintf(
				"agent-runtime-projection:%s:%s",
				cfg.AgentRuntimeAppID,
				cfg.AgentRuntimeEventProtocol,
			),
			InstanceID: realtimeInstanceID,
		})
		go func() {
			err := projectionLeader.Run(projectionCtx, func(leaderCtx context.Context) error {
				group, groupCtx := errgroup.WithContext(leaderCtx)
				group.Go(func() error {
					err := agentRuntimeProjectionService.StartNATSConsumer(groupCtx, jetstream)
					if err == nil && groupCtx.Err() == nil {
						return errors.New("agent runtime projection consumer exited unexpectedly")
					}
					return err
				})
				group.Go(func() error {
					return agentRuntimeProjectionService.StartReconciliationSweep(
						groupCtx,
						time.Minute,
						2*time.Minute,
						50,
					)
				})
				return group.Wait()
			})
			if err != nil && !errors.Is(err, context.Canceled) {
				slog.Error("agent runtime projection leadership stopped", "error", err)
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

	signalDetectionService := service.NewSignalDetectionService(llmProvider, crmSignalRepo, crmSummaryService).
		SetHealthScoreRefresh(crmSignalService)
	crmSummaryService.SetIntelligenceDependencies(signalDetectionService, crmActivityRepo, supportMessageRepo)
	crmActivityService.SetSignalDetection(crmsignal.NewTemporalStarter(temporalClient, temporalapp.QueueAutomation), crmSignalRepo)
	dealAutomationService := service.NewDealAutomationService(llmProvider, crmDealRepo, crmSignalRepo, crmSuggestionRepo, crmContactRepo, crmAssociationRepo, crmAutonomyRepo).SetDealService(crmDealService)
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
		supportAIService.SetKnowledgeReranker(service.NewGovernedSupportKnowledgeReranker(reranker, aiActionRegistry, aiActionExecutionRepo))
	}
	supportInboxService.SetSupportAIService(supportAIService)

	// Coverage telemetry: repos → services → async recorder → inject into hot-path services.
	supportEventRepo := repository.NewSupportEventRepository(db)
	supportCoverageRepo := repository.NewSupportCoverageRepository(db)
	supportCoverageAnalysisRepo := repository.NewSupportCoverageAnalysisRepository(db)
	supportCoverageV2Repo := repository.NewCoverageV2Repository(db)
	coverageV2Mode := strings.TrimSpace(os.Getenv("SUPPORT_COVERAGE_V2_MODE"))
	if coverageV2Mode == "" {
		coverageV2Mode = string(service.CoverageRolloutDisabled)
	}
	supportCoverageRolloutPolicy, err := service.NewCoverageRolloutPolicy(coverageV2Mode, os.Getenv("SUPPORT_COVERAGE_V2_WORKSPACE_MODES"))
	if err != nil {
		fatalWithSentry("invalid coverage v2 rollout policy", err)
	}
	supportCoverageService := service.NewSupportCoverageService(supportCoverageRepo)
	commandService.SetSupportCoverageService(supportCoverageService)
	agentRuntimeHostService.SetSupportCoverageService(supportCoverageService)
	if runFinalizers != nil {
		runFinalizers.SetSupportCoverageService(supportCoverageService)
	}
	agentService.SetSupportCoverageService(supportCoverageService)
	supportCoverageService.SetDocsBlockService(docsBlockService)
	supportCoverageService.SetTemporalClient(temporalClient)
	supportCoverageKnowledgeMatcher := service.NewCoverageKnowledgeMatcher(docsChunkRepo, supportContentChunkRepo, coverageEmbeddingProvider, cfg.OpenAIEmbeddingModel)
	supportCoverageClusterRebuildService := service.NewSupportCoverageClusterRebuildService(supportCoverageRepo, coverageEmbeddingProvider, cfg.OpenAIEmbeddingModel)
	supportCoverageDailyAnalyzer := service.NewSupportCoverageDailyAnalyzer(llmProvider, cfg.CRMLLMProvider, cfg.CRMLLMModel).
		SetCoverageRepositories(supportCoverageRepo, supportCoverageAnalysisRepo).
		SetCoverageV2Repository(supportCoverageV2Repo).
		SetCoverageRolloutPolicy(supportCoverageRolloutPolicy).
		SetEmbeddingProvider(coverageEmbeddingProvider, cfg.OpenAIEmbeddingModel).
		SetConversationRepositories(supportConversationRepo, supportMessageRepo).
		SetKnowledgeMatcher(supportCoverageKnowledgeMatcher, docsSpaceRepo, supportContentSourceRepo).
		SetTemporalClient(temporalClient)
	supportCoverageTraceService := service.NewSupportCoverageRetrievalTraceService(supportCoverageAnalysisRepo)
	supportEventService := service.NewSupportEventService(supportEventRepo, supportCoverageService).
		SetCoverageV2Repository(supportCoverageV2Repo).
		SetCoverageRolloutPolicy(supportCoverageRolloutPolicy).
		SetCompanySummaryRefresh(crmSummaryService).
		SetSignalDetection(supportMessageRepo, supportConversationRepo, crmsignal.NewTemporalStarter(temporalClient, temporalapp.QueueAutomation))
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
	supportFollowUpRepo := repository.NewSupportFollowUpRepository(db)
	supportFollowUpService := service.NewSupportFollowUpService(supportFollowUpRepo, supportChatService)
	commandService.SetSupportFollowUpService(supportFollowUpService)
	supportInboxService.SetFollowUpRepository(supportFollowUpRepo)
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

	orgService := service.NewOrganizationService(orgRepo)
	orgService.SetCustomerIOIdentityService(customerIOIdentityService)
	compositeDefaults := service.NewCompositeDefaultsInitializer(pmWorkflowService, pmAutomationService, crmDealService, supportInboxService, agentService)
	workspaceService := service.NewWorkspaceService(workspaceRepo, pmAttachmentRepo, s3Client, compositeDefaults)
	workspaceService.SetProductAnalyticsService(productAnalytics)
	setupService := service.NewSetupService(setupRepo)
	setupSuccessEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("SETUP_SUCCESS_ENABLED")), "true")
	if setupSuccessEnabled {
		workspaceService.SetSetupInitializer(setupService)
	}
	workspaceService.SetContextGeneratorDependencies(supportLLMProvider, nil)
	billingService.SetOrgRoleResolver(orgService)
	entitlementService := eeservice.NewEntitlementService(billingService)
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
	inviteService.SetCustomerIOIdentityService(customerIOIdentityService)
	inviteService.SetProductAnalyticsService(productAnalytics)
	// Initialize authorization service.
	authzMemberRepo := authorization.NewGORMMemberRepository(db)
	authzService := authorization.NewAuthzService(db, authzMemberRepo, moduleGrantRepo)
	authzService.SetWorkspaceMFARepository(workspaceRepo)
	aiConnectionService.SetAuthorizationService(authzService)
	crmSituationService := service.NewCRMSituationService(crmSituationRepo, authzService)
	crmPlaybookService := service.NewCRMPlaybookService(repository.NewCRMPlaybookRepository(db), authzService, crmSituationService).SetAIProfileService(aiProfileService)
	crmPlaybookExecutionRepo := repository.NewCRMPlaybookExecutionRepository(db)
	crmPlaybookLauncher := service.NewCRMPlaybookAgentLauncher(agentService, crmPlaybookExecutionRepo, aiUsageMeter)
	crmPlaybookExecution := service.NewCRMPlaybookExecutionService(crmPlaybookExecutionRepo, crmPlaybookService, crmPlaybookLauncher, authzService, workspaceRepo).SetEntitlements(entitlementService)
	crmPlaybookLauncher.SetExecutionService(crmPlaybookExecution)
	crmPlaybookExecutor := service.NewCRMPlaybookModuleExecutor(crmPlaybookExecution, crmPlaybookExecutionRepo, crmEmailService, pmTaskService, crmDealService, workspaceRepo, agentRepo, crmDealRepo)
	crmPlaybookActions := service.NewCRMPlaybookActionService(crmPlaybookExecutionRepo, crmSuggestionRepo, crmPlaybookExecution, crmPlaybookExecutor)
	crmPlaybookSetup := service.NewCRMPlaybookSetupService(crmPlaybookExecution, crmPlaybookExecutionRepo, agentService)
	crmSuggestionService.SetPlaybookActions(crmPlaybookActions)
	crmSituationService.SetAutomationLifecycle(crmPlaybookExecution)
	commandService.SetCRMPlaybookActions(crmPlaybookActions)
	agentRuntimeHostService.SetCRMPlaybookExecution(crmPlaybookExecution)
	if agentRuntimeProjectionService != nil {
		agentRuntimeProjectionService.SetCRMPlaybookExecution(crmPlaybookExecution)
	}
	crmSituationSources := service.NewCRMSituationSourceService(crmSituationRepo, crmSignalService)
	crmSuggestionService.SetSituationSources(crmSituationSources)
	crmSuggestionService.SetSituationGuard(crmSituationRepo)
	crmSituationService.SetActions(crmSuggestionService)
	commandService.SetAuthorizationService(authzService)
	commandService.SetAgentOrchestrationDependencies(commandBarService, agentRunInteractionRepo)
	agentRuntimeHostService.SetAuthorizationService(authzService)
	dockChatRepo := repository.NewDockChatRepository(db)
	commandService.SetDockChatRepository(dockChatRepo)
	dockChatService := service.NewDockChatService(dockChatRepo, agentRunRepo, agentRunMessageRepo, commandBarPlanRepo, agentService, commandService, authzService).
		SetTitleLLM(supportLLMProvider).
		SetPMAttachmentRepository(pmAttachmentRepo).
		SetMediaSourceService(supportInboxService).
		SetMediaAnalyzer(pmAttachmentService, supportLLMProvider)
	publicShareRepo := repository.NewPublicShareRepository(db)
	publicShareSource := service.NewPublicShareSource(dockChatService, agentService, dockChatRepo, agentRunMessageRepo, workspaceRepo)
	publicShareService := service.NewPublicShareService(publicShareRepo, publicShareSource, cfg.AppBaseURL)
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
	// Public help center AI search: semantic retrieval over published chunks
	// plus cached, validated one-shot answers (free tier — cost is bounded by
	// per-IP rate limits and the per-workspace daily generation budget).
	helpcenterAnswerRepo := repository.NewHelpcenterAnswerRepository(db)
	helpcenterAnswerProvider, helpcenterAnswerModel := service.ResolveHelpcenterAnswerRouting(
		cfg.HelpcenterAnswerProvider, cfg.HelpcenterAnswerModel,
		cfg.OpenRouterAPIKey != "", cfg.OpenAIAPIKey != "", cfg.AnthropicAPIKey != "",
	)
	helpcenterAISearchService := service.NewHelpcenterAISearchService(
		docsChunkRepo,
		docsSearchRepo,
		helpcenterAnswerRepo,
		supportEmbeddingProvider,
		cfg.OpenAIEmbeddingModel,
		supportLLMProvider,
		helpcenterAnswerProvider,
		helpcenterAnswerModel,
		redisClient,
	)
	// Lazy chunk backfill for workspaces whose help center predates
	// agent-independent auto-indexing.
	helpcenterAISearchService.SetAutoIndexer(docsEmbeddingService)

	handlers := router.Handlers{
		WidgetRateLimit:           middleware.WidgetRateLimit(redisClient),
		HelpcenterAnswerRateLimit: middleware.HelpcenterAnswerRateLimit(redisClient),
		Health:                    handler.NewHealthHandler(s3Client, geoIPResolver),
		Auth: handler.NewAuthHandler(authService, handler.GoogleOAuthConfig{
			ClientID:         cfg.GoogleAuthClientID,
			ClientSecret:     cfg.GoogleAuthClientSecret,
			RedirectURL:      cfg.GoogleAuthRedirectURL,
			AppBaseURL:       cfg.AppBaseURL,
			MobileAppBaseURL: cfg.MobileAppBaseURL,
		}),
		Passkey:      handler.NewPasskeyHandler(passkeyService),
		Organization: handler.NewOrganizationHandler(orgService),
		Workspace:    handler.NewWorkspaceHandler(workspaceService, authzService),
		Setup:        setupHandler,
		Edition: eehandler.NewRoutes(
			eehandler.NewBillingHandler(billingService, cfg.StripeWebhookSecret, cfg.AppBaseURL, billingTestScenarioService, strings.EqualFold(os.Getenv("BILLING_TEST_SCENARIOS_ENABLED"), "true")),
			eehandler.NewAIUsageHandler(pricingCatalog), authzService),
		Settings:            handler.NewSettingsHandler(settingsService, automationInventoryService),
		Automation:          handler.NewAutomationHandler(automationInventoryService, ruleEngine, agentService, flowTemplateRegistry, flowTemplateInstaller, flowTemplateUninstaller),
		Invite:              handler.NewInviteHandler(inviteService),
		PMWorkflow:          handler.NewPMWorkflowHandler(pmWorkflowService),
		PMImport:            handler.NewPMImportHandler(pmImportService),
		PMLabel:             handler.NewPMLabelHandler(pmLabelService),
		PMEpic:              handler.NewPMEpicHandler(pmEpicService),
		PMRoadmap:           handler.NewPMRoadmapHandler(pmRoadmapService),
		PMSprint:            handler.NewPMSprintHandler(pmSprintService),
		PMAISuggestion:      handler.NewPMAISuggestionHandler(pmAISuggestionService),
		PMTask:              handler.NewPMTaskHandler(pmTaskService),
		PMTaskInsights:      handler.NewPMTaskInsightsHandler(pmTaskInsightsService),
		PMComment:           handler.NewPMCommentHandler(pmCommentService),
		PMAttachment:        handler.NewPMAttachmentHandler(pmAttachmentService),
		PMObjective:         handler.NewPMObjectiveHandler(pmObjectiveService),
		PMChecklistItem:     handler.NewPMChecklistItemHandler(pmChecklistItemService),
		PMExternalLink:      handler.NewPMExternalLinkHandler(pmExternalLinkService),
		PMView:              handler.NewPMViewHandler(pmViewService),
		Search:              handler.NewSearchHandler(searchService),
		CommandBar:          handler.NewCommandBarHandler(commandBarService, authzService),
		DockChat:            handler.NewDockChatHandler(dockChatService, agentService),
		PublicShare:         handler.NewPublicShareHandler(publicShareService),
		PMAutomation:        handler.NewPMAutomationHandler(pmAutomationService),
		AutomationRule:      handler.NewAutomationRuleHandler(ruleEngine),
		PMTaskTemplate:      handler.NewPMTaskTemplateHandler(pmTaskTemplateService),
		PMRecurringTemplate: handler.NewPMRecurringTemplateHandler(pmRecurringTemplateService),
		Agent:               handler.NewAgentHandler(agentService),
		AIConnection:        handler.NewAIConnectionHandler(aiConnectionService),
		AIProfile:           handler.NewAIProfileHandler(aiProfileService),
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
		PushDevice:          handler.NewPushDeviceHandler(pushDeviceService),
		CRMContact:          handler.NewCRMContactHandler(crmContactService),
		CRMCompany:          handler.NewCRMCompanyHandler(crmCompanyService),
		CRMDeal:             handler.NewCRMDealHandler(crmDealService),
		CRMAssociation:      handler.NewCRMAssociationHandler(crmAssociationService),
		Associations:        handler.NewAssociationsHandler(associationsService, authzService),
		CRMActivity:         handler.NewCRMActivityHandler(crmActivityService),
		CRMMeeting:          handler.NewCRMMeetingHandler(crmMeetingService),
		CRMImport:           handler.NewCRMImportHandler(crmImportService),
		CRMEmail:            handler.NewCRMEmailHandler(crmEmailService, cfg.AppBaseURL),
		CRMCalendar:         handler.NewCRMCalendarHandler(crmCalendarService),
		CRMEnrichment:       handler.NewCRMEnrichmentHandler(crmEnrichmentService),
		CRMSignal:           handler.NewCRMSignalHandler(crmSignalService),
		CRMSummary:          handler.NewCRMSummaryHandler(crmSummaryService),
		CRMSuggestion:       handler.NewCRMSuggestionHandler(crmSuggestionService),
		CRMSituation:        handler.NewCRMSituationHandler(crmSituationService),
		CRMPlaybook:         handler.NewCRMPlaybookHandler(crmPlaybookService).SetExecution(crmPlaybookExecution, crmPlaybookSetup, crmPlaybookActions),
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
		).SetCoverageV2Service(service.NewSupportCoverageV2Service(supportCoverageV2Repo).SetRolloutPolicy(supportCoverageRolloutPolicy)),
		TLSAsk: handler.NewTLSAskHandler(tlsAskService),
	}

	// Set support event recorder on DocsHandler after handler creation.
	handlers.Docs.SetSupportEventRecorder(supportEventRecorder)
	handlers.Docs.SetImageEditService(docsImageEditService)
	handlers.Docs.SetSupportWidgetConfigProvider(supportInboxService)
	handlers.Docs.SetHelpcenterAISearchService(helpcenterAISearchService)
	handlers.Docs.SetAPIReferenceService(docsAPIReferenceService)

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
	if runEngine != nil {
		scheduleCtx, scheduleCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := runEngine.EnsureScheduledEvents(scheduleCtx); err != nil {
			slog.Error("failed to ensure shared scheduled event delivery", "error", err)
		}
		scheduleCancel()
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

	// Evaluate cross-module rules daily and behavioral rules every ten minutes.
	signalRuleCtx, signalRuleCancel := context.WithCancel(context.Background())
	signalRuleDone := make(chan struct{})
	signalRuleEvaluator := service.NewCRMSignalRuleEvaluator(crmSignalRepo, eventProjectRepo, clickHouseDB, podID)
	go func() {
		defer close(signalRuleDone)
		signalRuleEvaluator.Run(signalRuleCtx)
	}()

	// Materialize server-authenticated company state only after a workspace has
	// passed the Phase 1 shadow gate and explicitly cut over.
	commercialStateCtx, commercialStateCancel := context.WithCancel(context.Background())
	commercialStateDone := make(chan struct{})
	commercialStateSync := service.NewCRMCommercialStateSynchronizer(crmSignalService, crmSignalRepo, eventProjectRepo, clickHouseDB)
	go func() {
		defer close(commercialStateDone)
		commercialStateSync.Run(commercialStateCtx)
	}()

	// Rebuild workspace-local weekday baselines daily under a replica lease.
	usageBaselineCtx, usageBaselineCancel := context.WithCancel(context.Background())
	usageBaselineDone := make(chan struct{})
	usageBaselineSync := service.NewCRMUsageBaselineSynchronizer(crmSignalRepo, eventProjectRepo, clickHouseDB)
	go func() {
		defer close(usageBaselineDone)
		usageBaselineSync.Run(usageBaselineCtx)
	}()

	// Route only policy-eligible, versioned signals. Delivery rows make every
	// channel idempotent across replicas and restarts.
	signalRouteDone := make(chan struct{})
	go func() {
		runSignalRouteSweep := func() {
			workspaceIDs, err := workspaceRepo.ListIDs(context.Background())
			if err != nil {
				slog.Error("list workspaces for signal routing", "error", err)
				return
			}
			for _, workspaceID := range workspaceIDs {
				sweepCtx, sweepCancel := context.WithTimeout(context.Background(), 2*time.Minute)
				if err := crmSituationSources.ReconcileWorkspace(sweepCtx, workspaceID); err != nil {
					slog.WarnContext(sweepCtx, "CRM customer work reconciliation failed", "error", err, "workspace_id", workspaceID)
				}
				sweepCancel()
				if _, err := crmSignalService.RouteWorkspaceSignals(context.Background(), workspaceID); err != nil {
					slog.Warn("CRM signal routing failed", "error", err, "workspace_id", workspaceID)
				}
			}
		}
		runSignalRouteSweep()
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runSignalRouteSweep()
			case <-signalRouteDone:
				return
			}
		}
	}()

	// Produce explainable deal-health snapshots on startup and every six hours.
	healthScoreDone := make(chan struct{})
	go func() {
		runHealthScoreSweep := func() {
			workspaceIDs, err := workspaceRepo.ListIDs(context.Background())
			if err != nil {
				slog.Error("list workspaces for deal health sweep", "error", err)
				return
			}
			for _, workspaceID := range workspaceIDs {
				if err := crmSignalService.RefreshWorkspaceHealthScores(context.Background(), workspaceID); err != nil {
					slog.Warn("deal health sweep failed", "error", err, "workspace_id", workspaceID)
				}
			}
		}

		runHealthScoreSweep()
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runHealthScoreSweep()
			case <-healthScoreDone:
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

	// Start background ticker for delayed support reply fallback emails.
	delayedTeamReplyCtx, cancelDelayedTeamReply := context.WithCancel(context.Background())
	defer cancelDelayedTeamReply()
	go supportInboxService.StartDelayedTeamReplyWorker(delayedTeamReplyCtx)

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
	customerIOOutboxCancel()
	productAnalyticsCancel()
	signalRuleCancel()
	commercialStateCancel()
	usageBaselineCancel()
	settlementCancel()
	periodCancel()
	reservationCancel()
	select {
	case <-customerIOOutboxDone:
	case <-time.After(6 * time.Second):
		slog.Warn("customer.io outbox worker did not stop before shutdown timeout")
	}
	select {
	case <-productAnalyticsDone:
	case <-time.After(6 * time.Second):
		slog.Warn("product analytics outbox worker did not stop before shutdown timeout")
	}
	select {
	case <-settlementDone:
	case <-time.After(6 * time.Second):
		slog.Warn("AI usage settlement worker did not stop before shutdown timeout")
	}
	select {
	case <-periodDone:
	case <-time.After(6 * time.Second):
		slog.Warn("AI usage period worker did not stop before shutdown timeout")
	}
	select {
	case <-reservationDone:
	case <-time.After(6 * time.Second):
		slog.Warn("AI usage reservation sweeper did not stop before shutdown timeout")
	}
	select {
	case <-signalRuleDone:
	case <-time.After(6 * time.Second):
		slog.Warn("CRM signal rule evaluator did not stop before shutdown timeout")
	}
	select {
	case <-commercialStateDone:
	case <-time.After(6 * time.Second):
		slog.Warn("CRM commercial-state synchronizer did not stop before shutdown timeout")
	}
	select {
	case <-usageBaselineDone:
	case <-time.After(6 * time.Second):
		slog.Warn("CRM usage-baseline synchronizer did not stop before shutdown timeout")
	}
	if emailFallbackCancel != nil {
		emailFallbackCancel()
	}
	close(digestDone)
	close(healthScoreDone)
	close(signalRouteDone)
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

func eventClickHouseOptions(dsn string) (*clickhouse.Options, error) {
	options, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	options.Settings["do_not_merge_across_partitions_select_final"] = 1
	options.Settings["use_skip_indexes_if_final_exact_mode"] = 1
	return options, nil
}

func closeClickHouse(db *sql.DB) {
	if db == nil {
		return
	}
	if err := db.Close(); err != nil {
		slog.Warn("failed to close ClickHouse connection", "error", err)
	}
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
