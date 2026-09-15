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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"go.temporal.io/sdk/activity"
	tclient "go.temporal.io/sdk/client"
	tworker "go.temporal.io/sdk/worker"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/aimodel"
	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/crawler"
	"github.com/helpin-ai/helpin/server/internal/crmsignal"
	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/meetingcapture"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/oauth"
	"github.com/helpin-ai/helpin/server/internal/observability"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
	"github.com/helpin-ai/helpin/server/internal/storage"
	syncpkg "github.com/helpin-ai/helpin/server/internal/sync"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	ws "github.com/helpin-ai/helpin/server/internal/websocket"
)

const temporalWorkerStopTimeout = 10 * time.Minute

func main() {
	_ = godotenv.Load()

	if err := observability.InitSentry("temporal-worker"); err != nil {
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

	cfg, err := config.Load()
	if err != nil {
		fatalWithSentry("failed to load config", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLogLevel(cfg.LogLevel)}))
	slog.SetDefault(logger)

	s3Client := storage.NewS3Client(
		cfg.AWSAccessKeyID,
		cfg.AWSSecretAccessKey,
		cfg.AWSBucket,
		cfg.AWSRegion,
		cfg.AWSEndpointURL,
		cfg.AWSPublicBaseURL,
	)

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  cfg.DatabaseURL,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger: gormlogger.New(
			slog.NewLogLogger(slog.Default().Handler(), slog.LevelWarn),
			gormlogger.Config{
				SlowThreshold:             time.Second,
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
		fatalWithSentry("failed to get sql.DB", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		fatalWithSentry("failed to ping database", err)
	}
	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS vector`).Error; err != nil {
		fatalWithSentry("failed to enable vector extension", err)
	}
	if err := repository.MigrateAgentKnowledgeSourceSchema(db); err != nil {
		fatalWithSentry("failed to migrate agent knowledge source schema", err)
	}

	realtimeInstanceID := ws.ResolveRealtimeInstanceID()
	natsConn, jetstream, err := ws.ConnectJetStream(cfg.NatsURL, "helpin-temporal-worker-"+realtimeInstanceID)
	if err != nil {
		fatalWithSentry("failed to connect to NATS", err)
	}
	defer natsConn.Close()
	if err := ws.EnsureJetStreamInfrastructure(jetstream); err != nil {
		fatalWithSentry("failed to ensure JetStream infrastructure", err)
	}

	temporalClient, err := tclient.Dial(temporalapp.BuildClientOptions(cfg))
	if err != nil {
		fatalWithSentry("failed to connect to Temporal", err)
	}
	defer temporalClient.Close()
	runRepo := repository.NewAgentRunRepository(db)
	triggerExecutionRepo := repository.NewAgentTriggerExecutionRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	workspacePresetVersionRepo := repository.NewWorkspaceAgentPresetVersionRepository(db)
	workspaceSkillRepo := repository.NewWorkspaceSkillRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)
	dockActionProposalRepo := repository.NewDockActionProposalRepository(db)
	commandBarPlanRepo := repository.NewCommandBarPlanRepository(db)
	sessionSnapshotRepo := repository.NewCodingSessionStateSnapshotRepository(db)
	storyRepo := repository.NewPMTaskRepository(db)
	taskLinkRepo := repository.NewPMTaskLinkRepository(db)
	epicRepo := repository.NewPMEpicRepository(db)
	epicDeliveryTargetRepo := repository.NewEpicDeliveryTargetRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	workflowRepo := repository.NewPMWorkflowRepository(db)
	labelRepo := repository.NewPMLabelRepository(db)
	conversationRepo := repository.NewSupportConversationRepository(db)
	commentRepo := repository.NewPMCommentRepository(db)
	checklistRepo := repository.NewPMChecklistItemRepository(db)
	externalLinkRepo := repository.NewPMExternalLinkRepository(db)
	recurringRepo := repository.NewPMRecurringTemplateRepository(db)
	sprintRepo := repository.NewPMSprintRepository(db)
	sprintCloseoutRepo := repository.NewPMSprintCloseoutRepository(db)
	pmActivityRepo := repository.NewPMActivityRepository(db)
	supportMessageRepo := repository.NewSupportMessageRepository(db)
	userRepo := repository.NewUserRepository(db)
	notificationRepo := repository.NewNotificationRepository(db)
	notificationPrefRepo := repository.NewNotificationPreferenceRepository(db)
	userNotifSettingsRepo := repository.NewUserNotificationSettingsRepository(db)
	followerRepo := repository.NewFollowerRepository(db)
	gitIntRepo := repository.NewGitIntegrationRepository(db)
	gitCredentialRepo := repository.NewGitCredentialRepository(db)
	gitRepo := repository.NewGitRepositoryRepository(db)
	gitLinkRepo := repository.NewTaskGitLinkRepository(db)
	deliveryRepo := repository.NewTaskDeliveryTargetRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	automationRuleRepo := repository.NewAutomationRuleRepository(db)
	handoffRepo := repository.NewAgentHandoffRepository(db)
	docsSpaceRepo := repository.NewDocsSpaceRepository(db)
	docsCollectionRepo := repository.NewDocsCollectionRepository(db, cfg.DocsOrderingUseSortKey)
	docsDocumentRepo := repository.NewDocsDocumentRepository(db, cfg.DocsOrderingUseSortKey)
	docsContentRepo := repository.NewDocsContentRepository(db)
	docsBlockRepo := repository.NewDocsBlockRepository(db)
	docsChangeProposalRepo := repository.NewDocsChangeProposalRepository(db)
	docsContentRepo.SetBlockRepository(docsBlockRepo)
	docsVersionRepo := repository.NewDocsVersionRepository(db)
	docsLinkRepo := repository.NewDocsLinkRepository(db)
	docsHelpcenterRepo := repository.NewDocsHelpcenterRepository(db, false)
	docsHelpcenterPublicationRepo := repository.NewDocsHelpcenterPublicationRepository(db)
	docsHelpcenterSearchRepo := repository.NewDocsHelpcenterSearchRepository(db)
	docsRedirectRepo := repository.NewDocsRedirectRepository(db)
	docsImportRepo := repository.NewDocsImportRepository(db)
	modelCatalog, modelCatalogErr := aimodel.LoadCatalog()
	if modelCatalogErr != nil {
		fatalWithSentry("load model catalog", modelCatalogErr)
	}
	editionServices, err := newEditionServices(db, cfg, workspaceRepo)
	if err != nil {
		fatalWithSentry("configure edition", err)
	}
	if editionServices.InitializeAIProfiles != nil {
		if err := editionServices.InitializeAIProfiles(context.Background()); err != nil {
			fatalWithSentry("failed to initialize standard AI profiles", err)
		}
	}
	aiUsageService := editionServices.Usage
	aiUsageMeter := service.NewTokenPricedAIUsageMeter(aiUsageService)
	aiActionExecutionRepo := repository.NewAIActionExecutionRepository(db)
	aiActionRegistry := aipolicy.DefaultRegistry()
	if err := aiActionRegistry.Validate(); err != nil {
		fatalWithSentry("validate AI action registry", err)
	}
	docsAssetReferenceRepo := repository.NewDocsAssetReferenceRepository(db)
	docsSearchRepo := repository.NewDocsSearchRepository(db)
	docsChunkRepo := repository.NewDocsChunkRepository(db)
	agentKnowledgeSourceRepo := repository.NewAgentKnowledgeSourceRepository(db)
	supportContentSourceRepo := repository.NewSupportContentSourceRepository(db)
	supportContentPageRepo := repository.NewSupportContentPageRepository(db)
	supportContentChunkRepo := repository.NewSupportContentChunkRepository(db)
	crmEmailRepo := repository.NewCRMEmailRepository(db)
	crmContactRepo := repository.NewCRMContactRepository(db)
	crmCalendarRepo := repository.NewCRMCalendarRepository(db)
	crmCompanyRepo := repository.NewCRMCompanyRepository(db)
	crmDealRepo := repository.NewCRMDealRepository(db)
	crmAssociationRepo := repository.NewCRMAssociationRepository(db)
	crmSignalRepo := repository.NewCRMSignalRepository(db)
	crmActivityRepo := repository.NewCRMActivityRepository(db)
	crmEnrichmentRepo := repository.NewCRMEnrichmentRepository(db)
	crmSummaryRepo := repository.NewCRMSummaryRepository(db)
	crmMeetingRepo := repository.NewCRMMeetingRepository(db)
	automationHealthRepo := repository.NewAutomationHealthRepository(db)
	pmAttachmentRepo := repository.NewPMAttachmentRepository(db)
	pmAutomationRepo := repository.NewPMAutomationRepository(db)

	// Gmail OAuth + encryption for email sync.
	gmailOAuth := oauth.NewGmailOAuthClient(cfg.GmailClientID, cfg.GmailClientSecret, cfg.GmailOAuthRedirectURL)
	var encryptionKey []byte
	if cfg.CRMEncryptionKey != "" {
		var err error
		encryptionKey, err = hex.DecodeString(cfg.CRMEncryptionKey)
		if err != nil {
			log.Printf("invalid CRM_ENCRYPTION_KEY (must be hex-encoded): %v", err)
		}
	}
	gmailSyncClient := syncpkg.NewGmailSyncClient(gmailOAuth, crmEmailRepo, encryptionKey)
	githubAppClient, err := githubapp.NewClient(cfg.GitHubAppID, cfg.GitHubAppPrivateKey)
	if err != nil {
		fatalWithSentry("failed to initialize github app client", err)
	}
	wsPublisher := ws.NewJetStreamPublisher(jetstream)
	notificationService := service.NewNotificationService(
		notificationRepo,
		notificationPrefRepo,
		userNotifSettingsRepo,
		followerRepo,
		userRepo,
		workspaceRepo,
		wsPublisher,
		nil,
		cfg.AppBaseURL,
	)
	// Email sync activities (may be nil if Gmail not configured).
	crmEmailSyncSettingsRepo := repository.NewCRMEmailSyncSettingsRepository(db)

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
	if issues := completionRoutes.ValidateAvailability(editionServices.ValidateCompletionRoute); len(issues) != 0 {
		fatalWithSentry("validate edition completion routes", errors.Join(issues...))
	}
	if issues := completionRoutes.ValidateProviders(supportLLMRouter.HasChatProvider); editionServices.RequireConfiguredProviders && len(issues) != 0 {
		fatalWithSentry("validate AI completion providers", errors.Join(issues...))
	}
	agentTierResolver := service.NewAgentModelTierResolver(modelCatalog, supportLLMRouter.HasChatProvider)
	if issues := agentTierResolver.ValidateSelectable(); editionServices.RequireConfiguredProviders && len(issues) != 0 {
		fatalWithSentry("validate agent model sizes", errors.Join(issues...))
	}
	supportLLMProvider := service.NewAICompletionService(supportLLMRouter, aiUsageService, completionRoutes).
		SetGovernance(aiActionRegistry, aiActionExecutionRepo)
	var llmProvider llm.Provider = supportLLMProvider
	var redisClient *redis.Client
	if cfg.RedisURL != "" {
		redisOpts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			fatalWithSentry("invalid REDIS_URL", err)
		}
		redisClient = redis.NewClient(redisOpts)
		if err := redisClient.Ping(context.Background()).Err(); err != nil {
			fatalWithSentry("redis unreachable", err)
		}
		defer redisClient.Close()
	}
	// The support AI visitor-message consumer now runs in the API process
	// (chat-mode runs need the agent-runtime projection); the worker keeps
	// only knowledge indexing and coverage analytics.

	gitGraceCleanupCtx, gitGraceCleanupCancel := context.WithCancel(context.Background())
	go service.NewGitGraceCleanup(gitIntRepo, gitRepo).Start(gitGraceCleanupCtx)
	_ = gitGraceCleanupCancel // used at shutdown

	crmSummaryService := service.NewCRMSummaryService(crmSummaryRepo, crmContactRepo, crmCompanyRepo, crmDealRepo, crmAssociationRepo, crmSignalRepo, crmEmailRepo, llmProvider, temporalClient).
		SetCompanyEvidenceRepositories(repository.NewCRMCompanyTimelineRepository(db), storyRepo, conversationRepo)
	supportCoverageRepo := repository.NewSupportCoverageRepository(db)
	supportCoverageAnalysisRepo := repository.NewSupportCoverageAnalysisRepository(db)
	supportCoverageV2Repo := repository.NewCoverageV2Repository(db)
	coverageV2Mode := strings.TrimSpace(os.Getenv("SUPPORT_COVERAGE_V2_MODE"))
	if coverageV2Mode == "" {
		coverageV2Mode = string(service.CoverageRolloutDisabled)
	}
	supportCoverageRolloutPolicy, err := service.NewCoverageRolloutPolicy(coverageV2Mode, os.Getenv("SUPPORT_COVERAGE_V2_WORKSPACE_MODES"))
	if err != nil {
		log.Fatalf("invalid coverage v2 rollout policy: %v", err)
	}
	supportCoverageService := service.NewSupportCoverageService(supportCoverageRepo)
	supportCoverageEnrichmentService := service.NewSupportCoverageEnrichmentService(db, llmProvider)
	supportCoverageKnowledgeMatcher := service.NewCoverageKnowledgeMatcher(docsChunkRepo, supportContentChunkRepo, coverageEmbeddingProvider, cfg.OpenAIEmbeddingModel)
	supportCoverageDailyAnalyzer := service.NewSupportCoverageDailyAnalyzer(llmProvider, cfg.CRMLLMProvider, cfg.CRMLLMModel).
		SetCoverageRepositories(supportCoverageRepo, supportCoverageAnalysisRepo).
		SetCoverageV2Repository(supportCoverageV2Repo).
		SetCoverageRolloutPolicy(supportCoverageRolloutPolicy).
		SetEmbeddingProvider(coverageEmbeddingProvider, cfg.OpenAIEmbeddingModel).
		SetConversationRepositories(conversationRepo, supportMessageRepo).
		SetKnowledgeMatcher(supportCoverageKnowledgeMatcher, docsSpaceRepo, supportContentSourceRepo).
		SetTemporalClient(temporalClient)
	meetingCaptureScheduler := service.NewTemporalMeetingCaptureScheduler(temporalClient)
	calendarMeetingPolicyService := service.NewCRMMeetingService(crmMeetingRepo, crmAssociationRepo, nil).
		SetCaptureProvider(cfg.CRMMeetingCaptureProvider).
		SetCaptureScheduler(meetingCaptureScheduler).
		SetCalendarIntegration(crmCalendarRepo, crmEmailRepo)
	emailSyncActivities := temporalapp.NewEmailSyncActivities(gmailSyncClient, crmEmailRepo, crmContactRepo, crmCalendarRepo, crmEmailSyncSettingsRepo, temporalClient, crmSummaryService).
		SetMeetingRepository(crmMeetingRepo).
		SetMeetingCaptureScheduler(meetingCaptureScheduler).
		SetMeetingPolicyReconciler(calendarMeetingPolicyService).
		SetCalendarSignalRepository(crmSignalRepo)
	crmSignalService := service.NewCRMSignalService(crmSignalRepo, crmSummaryService).
		SetHealthScoreDependencies(crmDealRepo)
	signalDetectionService := service.NewSignalDetectionService(llmProvider, crmSignalRepo, crmSummaryService).
		SetHealthScoreRefresh(crmSignalService)
	crmSummaryService.SetIntelligenceDependencies(signalDetectionService, crmActivityRepo, supportMessageRepo)
	runRepo.SetNotifier(ws.NewRunNotifier(wsPublisher))
	runRepo.SetTriggerExecutionRepository(triggerExecutionRepo)
	var agentRuntimeClient *service.AgentRuntimeClient
	if strings.TrimSpace(cfg.AgentRuntimeBaseURL) != "" {
		agentRuntimeClient, err = service.NewAgentRuntimeClient(cfg.AgentRuntimeBaseURL, cfg.AgentRuntimeAppID, cfg.AgentRuntimeServiceToken, nil)
		if err != nil {
			fatalWithSentry("failed to initialize agent runtime client", err)
		}
	}
	pmActivityService := service.NewPMActivityService(pmActivityRepo)
	pmRecurringTemplateService := service.NewPMRecurringTemplateService(
		recurringRepo,
		storyRepo,
		workflowRepo,
		sprintRepo,
		workspaceRepo,
		checklistRepo,
		externalLinkRepo,
		pmActivityService,
		nil,
	)
	pmAutomationService := service.NewPMAutomationService(
		pmAutomationRepo,
		epicRepo,
		storyRepo,
		sprintRepo,
		workflowRepo,
		pmActivityService,
		wsPublisher,
		sprintCloseoutRepo,
	)
	pmAttachmentService := service.NewPMAttachmentService(pmAttachmentRepo, s3Client, wsPublisher)
	pmImportService := service.NewPMImportService(db, workspaceRepo, workflowRepo, pmAttachmentService, resolvePMImportEncryptionKey(cfg))
	pmImportService.SetPublisher(wsPublisher)
	pmWorkflowService := service.NewPMWorkflowService(workflowRepo, storyRepo, labelRepo, nil)
	pmStoryService := service.NewPMTaskService(
		storyRepo,
		workspaceRepo,
		workflowRepo,
		epicRepo,
		sprintRepo,
		labelRepo,
		checklistRepo,
		externalLinkRepo,
		pmAttachmentRepo,
		pmActivityService,
		wsPublisher,
		nil,
		nil,
		nil,
	)
	pmStoryService.SetRecurringService(pmRecurringTemplateService)
	pmRecurringTemplateService.SetTaskService(pmStoryService)
	gitService := service.NewGitService(
		gitIntRepo,
		gitRepo,
		gitLinkRepo,
		deliveryRepo,
		settingsRepo,
		workspaceRepo,
		repository.NewOrganizationRepository(db),
		storyRepo,
		pmActivityService,
		wsPublisher,
		githubAppClient,
		cfg.AppBaseURL,
		cfg.GitHubAppSlug,
		cfg.JWTSecret,
	).
		SetEpicDeliveryDependencies(epicDeliveryTargetRepo, epicRepo).
		SetGitLabDependencies(gitCredentialRepo, resolveGitOAuthEncryptionKey(cfg))
	pmStoryService.SetGitService(gitService)
	agentService := service.NewAgentService(
		agentRepo,
		workspacePresetVersionRepo,
		runRepo,
		runMessageRepo,
		artifactRepo,
		interactionRepo,
		sessionSnapshotRepo,
		storyRepo,
		taskLinkRepo,
		epicRepo,
		conversationRepo,
		supportMessageRepo,
		handoffRepo,
		automationRuleRepo,
		nil,
		settingsRepo,
		docsSpaceRepo,
		docsDocumentRepo,
		docsContentRepo,
		docsVersionRepo,
		docsLinkRepo,
		gitService,
		pmStoryService,
		pmActivityService,
		wsPublisher,
	).SetWorkspaceSkillStore(workspaceSkillRepo, nil).SetModelProviderConfig(
		cfg.AnthropicAPIKey,
		cfg.OpenAIAPIKey,
		cfg.OpenRouterAPIKey).SetTriggerExecutionRepository(triggerExecutionRepo).SetCommandBarPlanRepository(commandBarPlanRepo).SetWorkspaceRepository(workspaceRepo).SetNotificationService(notificationService).SetCRMRepositories(crmContactRepo, crmCompanyRepo, crmDealRepo).SetModelTierResolver(agentTierResolver).SetAgentRuntimeLaunchEnabled(cfg.AgentRuntimeLaunchEnabled)

	aiConnectionService, err := service.NewAIConnectionService(repository.NewAIConnectionRepository(db), modelCatalog, agentRuntimeClient, service.AIConnectionConfig{
		EncryptionKey: cfg.AIConnectionEncryptionKey, ChatGPTEnabled: cfg.ChatGPTConnectionsEnabled, ChatGPTClientID: cfg.ChatGPTClientID, AppID: cfg.AgentRuntimeAppID,
	})
	if err != nil {
		fatalWithSentry("initialize AI connections", err)
	}
	agentService.SetAIConnectionService(aiConnectionService).SetAIProfileService(service.NewAIProfileService(repository.NewAIProfileRepository(db), aiConnectionService).SetAdmissionPolicy(editionServices.ConnectionPolicy).CheckRuntimeReadiness())
	if agentRuntimeClient != nil {
		agentService.SetAgentRuntimeClient(agentRuntimeClient)
	}
	agentService.SetWorkflowService(pmWorkflowService)
	docsDocumentService := service.NewDocsDocumentService(docsDocumentRepo, docsSpaceRepo, wsPublisher, cfg.DocsOrderingUseSortKey)
	docsSpaceService := service.NewDocsSpaceService(docsSpaceRepo, wsPublisher)
	docsCollectionService := service.NewDocsCollectionService(docsCollectionRepo, docsSpaceRepo, wsPublisher, cfg.DocsOrderingUseSortKey)
	docsContentService := service.NewDocsContentService(docsContentRepo, docsDocumentRepo, nil)
	docsHelpcenterService := service.NewDocsHelpcenterService(
		docsHelpcenterRepo,
		docsHelpcenterPublicationRepo,
		docsDocumentRepo,
		docsContentRepo,
		docsSpaceRepo,
		docsCollectionRepo,
		docsRedirectRepo,
		s3Client,
		wsPublisher,
	)
	docsHelpcenterService.SetSearchRepository(docsHelpcenterSearchRepo)
	docsHelpcenterService.SetPublicationArtifactDependencies(artifactRepo, s3Client)
	docsDocumentService.SetHelpcenterService(docsHelpcenterService)
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
	).SetTemporalClient(nil, resolvePMImportEncryptionKey(cfg))
	docsBlockService := service.NewDocsBlockService(docsBlockRepo, docsContentService, docsDocumentRepo)
	docsBlockService.SetActivityService(pmActivityService)
	supportCoverageService.SetDocsBlockService(docsBlockService)
	pmImportService.SetDocsImportDependencies(docsDocumentService, docsContentService)
	docsLinkService := service.NewDocsLinkService(docsLinkRepo, storyRepo, docsDocumentRepo, nil)
	releaseFactsService := service.NewReleaseFactsService(
		gitIntRepo,
		gitRepo,
		storyRepo,
		gitLinkRepo,
		commentRepo,
		docsLinkRepo,
		docsDocumentRepo,
		docsContentRepo,
		workspaceRepo,
		githubAppClient,
	)
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
		nil,
	)
	supportContentSyncService := service.NewSupportContentSyncService(
		supportContentSourceRepo,
		supportContentPageRepo,
		supportContentChunkRepo,
		supportEmbeddingProvider,
		cfg.OpenAIEmbeddingModel,
		contentCrawler,
		s3Client,
		nil,
	)
	crmDealService := service.NewCRMDealService(crmDealRepo, crmAssociationRepo).
		SetActivityService(pmActivityService)
	crmCompanyService := service.NewCRMCompanyService(crmCompanyRepo)
	crmAssociationService := service.NewCRMAssociationService(crmAssociationRepo)
	crmActivityService := service.NewCRMActivityService(crmActivityRepo)
	crmActivityService.SetSignalDetection(crmsignal.NewTemporalStarter(temporalClient, temporalapp.QueueAutomation), crmSignalRepo)
	crmDealService.SetCompanySummaryRefresh(crmSummaryService)
	crmCompanyService.SetCompanySummaryRefresh(crmSummaryService)
	crmAssociationService.SetCompanySummaryRefresh(crmSummaryService)
	crmActivityService.SetCompanySummaryRefresh(crmSummaryService)
	pmActivityService.SetCompanySummaryRefresh(crmSummaryService)
	crmEnrichmentService := service.NewCRMEnrichmentService(crmEnrichmentRepo, crmContactRepo, crmCompanyRepo, crmAssociationRepo).
		SetActivityRepository(crmActivityRepo).
		SetWebsocketPublisher(wsPublisher).
		SetCompanySummaryRefresh(crmSummaryService)
	pmLabelService := service.NewPMLabelService(labelRepo, wsPublisher)
	pmCommentService := service.NewPMCommentService(commentRepo, storyRepo, pmAttachmentRepo, pmActivityService, wsPublisher, notificationService, workspaceRepo, s3Client)
	commandService := service.NewInternalCommandService(
		agentService,
		pmStoryService,
		crmDealService,
		crmActivityService,
		docsContentService,
		docsLinkService,
		storyRepo,
		taskLinkRepo,
	)
	commandService.SetPMLabelService(pmLabelService)
	commandService.SetPMCommentService(pmCommentService)
	commandService.SetCRMEnrichmentService(crmEnrichmentService)
	commandService.SetPMOperationalServices(workspaceRepo, nil, nil, nil, nil, nil)
	commandService.SetGitService(gitService)
	commandService.SetDocsCreateDependencies(docsDocumentService, docsContentRepo)
	commandService.SetDocsOrganizationServices(docsSpaceService, nil)
	commandService.SetDocsBlockService(docsBlockService)
	commandService.SetSupportDependencies(supportMessageRepo, conversationRepo, wsPublisher)
	commandService.SetSupportAttachmentRepository(repository.NewSupportAttachmentRepository(db))
	commandService.SetCRMReadServices(
		service.NewCRMContactService(crmContactRepo),
		crmSignalService,
	)
	commandService.SetCRMOperationalServices(crmCompanyService, crmAssociationService)
	commandService.SetWorkspaceSearchServices(
		service.NewSearchService(repository.NewSearchRepository(db), workspaceRepo),
		service.NewCRMSearchService(crmContactRepo, crmCompanyRepo, crmDealRepo),
		nil,
	)
	commandService.SetDocsSearchRepository(docsSearchRepo)
	commandService.SetDocsChangeProposalService(service.NewDocsChangeProposalService(
		docsChangeProposalRepo,
		docsDocumentRepo,
		docsContentService,
		docsBlockService,
		nil,
		nil,
	))
	commandService.SetAgentRunDependencies(runRepo, artifactRepo)
	commandService.SetSupportCoverageService(supportCoverageService)
	commandService.SetDockActionProposalRepository(dockActionProposalRepo)
	commandService.SetReleaseFactsProvider(releaseFactsService)
	automationHealthService := service.NewAutomationHealthService(automationHealthRepo)
	ruleEngine := service.NewAutomationRuleEngine(
		automationRuleRepo,
		storyRepo,
		workflowRepo,
		deliveryRepo,
		gitService,
		notificationService,
		pmActivityService,
		wsPublisher,
	)
	ruleEngine.SetAgentService(agentService)
	ruleEngine.SetTaskService(pmStoryService)
	ruleEngine.SetHealthObserver(automationHealthService)
	ruleEngine.SetTriggerExecutionRepository(triggerExecutionRepo)
	ruleEngine.SetRunEngine(temporalapp.NewRunEngine(temporalClient, cfg.TemporalNamespace))

	signalActivities := temporalapp.NewSignalDetectionActivities(signalDetectionService, wsPublisher).SetHealthObserver(automationHealthService)
	summaryActivities := temporalapp.NewCRMSummaryActivities(crmSummaryService).SetHealthObserver(automationHealthService)
	coverageActivities := temporalapp.NewCoverageGapActivities(supportCoverageEnrichmentService, supportCoverageService)
	coverageAnalysisActivities := temporalapp.NewCoverageAnalysisActivities(supportCoverageDailyAnalyzer)

	// Deal management activities.
	crmSuggestionRepo := repository.NewCRMSuggestionRepository(db)
	crmAutonomyRepo := repository.NewCRMAutonomyRepository(db)
	dealAutomationService := service.NewDealAutomationService(llmProvider, crmDealRepo, crmSignalRepo, crmSuggestionRepo, crmContactRepo, crmAssociationRepo, crmAutonomyRepo).SetDealService(crmDealService)
	dealMgmtActivities := temporalapp.NewDealManagementActivities(dealAutomationService)
	crmSuggestionService := service.NewCRMSuggestionService(crmSuggestionRepo, crmDealRepo, crmAssociationRepo).SetDealService(crmDealService)
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
	var meetingProcessor *service.CRMMeetingProcessingService
	if s3Client != nil {
		meetingProcessor = service.NewCRMMeetingProcessingService(
			crmMeetingRepo, crmAssociationRepo, supportLLMProvider, s3Client,
			&http.Client{Timeout: 30 * time.Minute}, recallMeetingProvider, vexaMeetingProvider,
		)
	} else {
		meetingProcessor = service.NewCRMMeetingProcessingService(
			crmMeetingRepo, crmAssociationRepo, supportLLMProvider, nil,
			&http.Client{Timeout: 30 * time.Minute}, recallMeetingProvider, vexaMeetingProvider,
		)
	}
	meetingProcessor.SetCRMOutputs(signalDetectionService, crmActivityService, crmSuggestionService).SetFollowUpRoutingStore(crmSuggestionRepo)
	meetingCaptureService := service.NewCRMMeetingService(
		crmMeetingRepo, nil, nil, recallMeetingProvider, vexaMeetingProvider,
	).SetCaptureProvider(cfg.CRMMeetingCaptureProvider).
		SetAIUsageMeter(service.NewTokenPricedAIUsageMeter(aiUsageService))
	meetingActivities := temporalapp.NewCRMMeetingActivities(meetingProcessor).SetCaptureLauncher(meetingCaptureService).SetFollowUpRouter(meetingProcessor)

	scheduledRuleActivities := temporalapp.NewScheduledRuleActivities(ruleEngine)
	crmPlaybookAuthz := authorization.NewAuthzService(db, authorization.NewGORMMemberRepository(db), repository.NewWorkspaceModuleGrantRepository(db))
	crmPlaybookAuthz.SetDeploymentModules(cfg.EnabledModules)
	crmSituationService := service.NewCRMSituationService(repository.NewCRMSituationRepository(db), crmPlaybookAuthz)
	crmPlaybookService := service.NewCRMPlaybookService(repository.NewCRMPlaybookRepository(db), crmPlaybookAuthz, crmSituationService)
	crmPlaybookExecutionRepo := repository.NewCRMPlaybookExecutionRepository(db)
	crmPlaybookLauncher := service.NewCRMPlaybookAgentLauncher(agentService, crmPlaybookExecutionRepo, aiUsageMeter)
	crmPlaybookExecution := service.NewCRMPlaybookExecutionService(crmPlaybookExecutionRepo, crmPlaybookService, crmPlaybookLauncher, crmPlaybookAuthz, workspaceRepo).SetEntitlements(editionServices.Entitlements)
	crmPlaybookLauncher.SetExecutionService(crmPlaybookExecution)
	scheduledEventsService := service.NewAutomationScheduledEventService(repository.NewAutomationScheduledEventRepository(db),
		map[string]service.ScheduledEventHandler{
			model.CRMCheckpointEvent:  service.NewCRMSituationCheckpointService(repository.NewCRMSituationRepository(db)),
			model.CRMPlaybookWorkDue:  crmPlaybookExecution,
			model.CRMPlaybookEntryDue: crmPlaybookExecution,
		}).SetMaintenance(crmPlaybookExecution.MaintainScheduledWork)
	crmSequenceEmail := service.NewCRMEmailService(crmEmailRepo, crmContactRepo, workspaceRepo, crmEmailSyncSettingsRepo, gmailOAuth, encryptionKey, gmailSyncClient, temporalClient, crmSummaryService)
	crmOutreachService := service.NewCRMOutreachService(repository.NewCRMOutreachRepository(db), crmSequenceEmail, crmEmailRepo, crmPlaybookAuthz, pmStoryService, cfg.AppBaseURL)
	scheduledEventsActivities := temporalapp.NewScheduledEventsActivities(scheduledEventsService).SetAdditionalDispatcher(crmOutreachService)
	recurringActivities := service.NewPMRecurringTemplateActivities(pmRecurringTemplateService)

	// Sprint automation activities.
	sprintAutomationActivities := temporalapp.NewSprintAutomationActivities(pmAutomationService)
	docsEmbeddingActivities := temporalapp.NewDocsEmbeddingActivities(docsEmbeddingService)
	docsAssetCleanupActivities := temporalapp.NewDocsAssetCleanupActivities(docsAssetReferenceRepo, s3Client)
	contentSourceSyncActivities := temporalapp.NewContentSourceSyncActivities(supportContentSyncService)
	pmImportActivities := service.NewPMImportActivities(pmImportService)
	docsImportActivities := service.NewDocsImportActivities(docsImportService)

	queueConfigs := selectedQueues()
	workers := make([]tworker.Worker, 0, len(queueConfigs))
	for _, queue := range queueConfigs {
		workers = append(workers, newTemporalWorker(temporalClient, queue.Name, queue.Concurrency, emailSyncActivities, signalActivities, summaryActivities, meetingActivities, coverageActivities, coverageAnalysisActivities, dealMgmtActivities, scheduledRuleActivities, scheduledEventsActivities, recurringActivities, sprintAutomationActivities, docsEmbeddingActivities, docsAssetCleanupActivities, contentSourceSyncActivities, pmImportActivities, docsImportActivities))
	}

	for _, sharedWorker := range workers {
		if err := sharedWorker.Start(); err != nil {
			fatalWithSentry("failed to start temporal worker", err)
		}
	}
	log.Printf("temporal workers started for namespace=%s queues=%v", cfg.TemporalNamespace, queueNames(queueConfigs))
	scheduleCtx, scheduleCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := temporalapp.NewRunEngine(temporalClient, cfg.TemporalNamespace).EnsureScheduledEvents(scheduleCtx); err != nil {
		slog.Error("failed to ensure shared scheduled event delivery", "error", err)
	}
	scheduleCancel()
	routingScheduleCtx, routingScheduleCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := temporalapp.NewRunEngine(temporalClient, cfg.TemporalNamespace).EnsureMeetingFollowUpRouting(routingScheduleCtx); err != nil {
		slog.Warn("meeting follow-up routing schedule unavailable", "error", err)
	}
	routingScheduleCancel()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-stopCh

	log.Println("shutting down temporal workers")
	gitGraceCleanupCancel()
	for _, sharedWorker := range workers {
		sharedWorker.Stop()
	}

	if err := sqlDB.Close(); err != nil {
		log.Printf("close database: %v", err)
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

func newTemporalWorker(client tclient.Client, taskQueue string, concurrency int, emailActivities *temporalapp.EmailSyncActivities, signalActivities *temporalapp.SignalDetectionActivities, summaryActivities *temporalapp.CRMSummaryActivities, meetingActivities *temporalapp.CRMMeetingActivities, coverageActivities *temporalapp.CoverageGapActivities, coverageAnalysisActivities *temporalapp.CoverageAnalysisActivities, dealMgmtActivities *temporalapp.DealManagementActivities, scheduledRuleActivities *temporalapp.ScheduledRuleActivities, scheduledEventsActivities *temporalapp.ScheduledEventsActivities, recurringActivities *service.PMRecurringTemplateActivities, sprintActivities *temporalapp.SprintAutomationActivities, docsEmbeddingActivities *temporalapp.DocsEmbeddingActivities, docsAssetCleanupActivities *temporalapp.DocsAssetCleanupActivities, contentSourceSyncActivities *temporalapp.ContentSourceSyncActivities, pmImportActivities *service.PMImportActivities, docsImportActivities *service.DocsImportActivities) tworker.Worker {
	options := tworker.Options{
		MaxConcurrentActivityExecutionSize: concurrency,
		WorkerStopTimeout:                  temporalWorkerStopTimeout,
	}
	w := tworker.New(client, taskQueue, options)
	// Agent Runtime owns agent execution. This worker hosts product background
	// jobs only.

	// Register email sync workflow and activities.
	w.RegisterWorkflow(temporalapp.EmailSyncWorkflow)
	if emailActivities != nil {
		w.RegisterActivityWithOptions(emailActivities.BackfillEmailsActivity, activity.RegisterOptions{
			Name: "EmailSyncActivities.BackfillEmailsActivity",
		})
		w.RegisterActivityWithOptions(emailActivities.HistoricalBackfillEmailsActivity, activity.RegisterOptions{
			Name: "EmailSyncActivities.HistoricalBackfillEmailsActivity",
		})
		w.RegisterActivityWithOptions(emailActivities.IncrementalSyncActivity, activity.RegisterOptions{
			Name: "EmailSyncActivities.IncrementalSyncActivity",
		})
	}

	// Register signal detection workflow and activities.
	w.RegisterWorkflow(temporalapp.SignalDetectionWorkflow)
	if signalActivities != nil {
		w.RegisterActivityWithOptions(signalActivities.ExtractSignalsActivity, activity.RegisterOptions{
			Name: "SignalDetectionActivities.ExtractSignalsActivity",
		})
		w.RegisterActivityWithOptions(signalActivities.NotifySignalsActivity, activity.RegisterOptions{
			Name: "SignalDetectionActivities.NotifySignalsActivity",
		})
	}

	// Register CRM summary workflows and activities.
	w.RegisterWorkflow(temporalapp.CRMEntitySummaryWorkflow)
	w.RegisterWorkflow(temporalapp.CRMSummaryDailyReconciliationWorkflow)
	if summaryActivities != nil {
		w.RegisterActivityWithOptions(summaryActivities.RefreshSummaryActivity, activity.RegisterOptions{
			Name: "CRMSummaryActivities.RefreshSummaryActivity",
		})
		w.RegisterActivityWithOptions(summaryActivities.DailyReconciliationActivity, activity.RegisterOptions{
			Name: "CRMSummaryActivities.DailyReconciliationActivity",
		})
	}

	// Register provider-neutral CRM meeting capture and processing.
	w.RegisterWorkflow(temporalapp.CRMMeetingCaptureScheduleWorkflow)
	w.RegisterWorkflow(temporalapp.CRMMeetingProcessingWorkflow)
	w.RegisterWorkflow(temporalapp.CRMMeetingFollowUpRoutingWorkflow)
	if meetingActivities != nil {
		w.RegisterActivityWithOptions(meetingActivities.RouteMeetingFollowUps, activity.RegisterOptions{Name: "CRMMeetingActivities.RouteMeetingFollowUps"})
		w.RegisterActivityWithOptions(meetingActivities.ProcessMeetingActivity, activity.RegisterOptions{
			Name: "CRMMeetingActivities.ProcessMeetingActivity",
		})
		w.RegisterActivityWithOptions(meetingActivities.StartScheduledCaptureActivity, activity.RegisterOptions{
			Name: temporalapp.CRMMeetingStartScheduledActivityName,
		})
	}

	// Register support coverage gap enrichment workflows and activities.
	w.RegisterWorkflow(temporalapp.CoverageGapEnrichmentWorkflow)
	w.RegisterWorkflow(temporalapp.CoverageGapDailyBatchWorkflow)
	w.RegisterWorkflow(temporalapp.CoverageGapPerWorkspaceWorkflow)
	if coverageActivities != nil {
		w.RegisterActivityWithOptions(coverageActivities.EnrichTopicActivity, activity.RegisterOptions{
			Name: temporalapp.CoverageGapEnrichmentActivityName,
		})
		w.RegisterActivityWithOptions(coverageActivities.ListWorkspacesActivity, activity.RegisterOptions{
			Name: temporalapp.CoverageGapListWorkspacesActivityName,
		})
		w.RegisterActivityWithOptions(coverageActivities.ListTopicsForBatchActivity, activity.RegisterOptions{
			Name: temporalapp.CoverageGapListBatchActivityName,
		})
	}
	w.RegisterWorkflow(temporalapp.CoverageDailyAnalysisWorkflow)
	w.RegisterWorkflow(temporalapp.CoverageWorkspaceAnalysisWorkflow)
	if coverageAnalysisActivities != nil {
		w.RegisterActivityWithOptions(coverageAnalysisActivities.ListWorkspacesActivity, activity.RegisterOptions{
			Name: temporalapp.CoverageListAnalysisWorkspacesActivity,
		})
		w.RegisterActivityWithOptions(coverageAnalysisActivities.RunWorkspaceAnalysisActivity, activity.RegisterOptions{
			Name: temporalapp.CoverageRunWorkspaceAnalysisActivityName,
		})
	}

	// Register deal management cron workflow and activities.
	w.RegisterWorkflow(temporalapp.DealManagementCronWorkflow)
	if dealMgmtActivities != nil {
		w.RegisterActivityWithOptions(dealMgmtActivities.EvaluateProgressionActivity, activity.RegisterOptions{
			Name: "DealManagementActivities.EvaluateProgressionActivity",
		})
	}

	w.RegisterWorkflow(temporalapp.ScheduledRuleWorkflow)
	w.RegisterWorkflow(temporalapp.ScheduledEventsWorkflow)
	if scheduledEventsActivities != nil {
		w.RegisterActivityWithOptions(scheduledEventsActivities.DispatchDue, activity.RegisterOptions{
			Name: "ScheduledEventsActivities.DispatchDue",
		})
	}
	if scheduledRuleActivities != nil {
		w.RegisterActivityWithOptions(scheduledRuleActivities.ExecuteScheduledRule, activity.RegisterOptions{
			Name: "ScheduledRuleActivities.ExecuteScheduledRule",
		})
	}

	// Register recurring template scheduler workflow and activities.
	w.RegisterWorkflow(temporalapp.PMRecurringTemplateSchedulerWorkflow)
	if recurringActivities != nil {
		w.RegisterActivityWithOptions(recurringActivities.ProcessDueTemplatesActivity, activity.RegisterOptions{
			Name: "PMRecurringTemplateActivities.ProcessDueTemplatesActivity",
		})
	}

	// Register sprint automation cron workflow and activities.
	w.RegisterWorkflow(temporalapp.SprintAutomationCronWorkflow)
	if sprintActivities != nil {
		w.RegisterActivityWithOptions(sprintActivities.RunSprintAutomationsActivity, activity.RegisterOptions{
			Name: "SprintAutomationActivities.RunSprintAutomationsActivity",
		})
	}

	// Register docs embedding workflow and activities.
	w.RegisterWorkflow(temporalapp.DocsEmbeddingSyncWorkflow)
	if docsEmbeddingActivities != nil {
		w.RegisterActivityWithOptions(docsEmbeddingActivities.SyncSpaceActivity, activity.RegisterOptions{
			Name: "DocsEmbeddingActivities.SyncSpaceActivity",
		})
	}

	w.RegisterWorkflow(temporalapp.DocsAssetCleanupWorkflow)
	if docsAssetCleanupActivities != nil {
		w.RegisterActivityWithOptions(docsAssetCleanupActivities.CleanupAssetActivity, activity.RegisterOptions{
			Name: "DocsAssetCleanupActivities.CleanupAssetActivity",
		})
	}

	// Register content source sync workflow and activities.
	w.RegisterWorkflow(temporalapp.ContentSourceSyncWorkflow)
	if contentSourceSyncActivities != nil {
		w.RegisterActivityWithOptions(contentSourceSyncActivities.SyncContentSourceActivity, activity.RegisterOptions{
			Name: "ContentSourceSyncActivities.SyncContentSourceActivity",
		})
	}

	// Register PM import workflows and activities.
	w.RegisterWorkflow(temporalapp.ShortcutImportWorkflow)
	if pmImportActivities != nil {
		w.RegisterActivityWithOptions(pmImportActivities.ExecuteShortcutAPIImportActivity, activity.RegisterOptions{
			Name: "PMImportActivities.ExecuteShortcutAPIImportActivity",
		})
	}

	// Register durable docs import workflow and activity.
	w.RegisterWorkflow(temporalapp.DocsImportWorkflow)
	if docsImportActivities != nil {
		w.RegisterActivityWithOptions(docsImportActivities.ExecuteHelpScoutImportActivity, activity.RegisterOptions{
			Name: temporalapp.DocsImportExecuteActivityName,
		})
	}

	return w
}

func selectedQueues() []temporalapp.QueueConfig {
	configured := strings.TrimSpace(os.Getenv("TEMPORAL_WORKER_QUEUES"))
	all := temporalapp.SharedQueues()
	if configured == "" {
		return all
	}

	lookup := make(map[string]temporalapp.QueueConfig, len(all))
	for _, queue := range all {
		lookup[queue.Name] = queue
	}

	selected := make([]temporalapp.QueueConfig, 0, len(all))
	for _, raw := range strings.Split(configured, ",") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		queue, ok := lookup[name]
		if !ok {
			fatalMessageWithSentry("unknown TEMPORAL_WORKER_QUEUES entry " + strconv.Quote(name))
		}
		selected = append(selected, queue)
	}
	if len(selected) == 0 {
		fatalMessageWithSentry("TEMPORAL_WORKER_QUEUES did not contain any valid queues")
	}
	return selected
}

func queueNames(queues []temporalapp.QueueConfig) []string {
	names := make([]string, 0, len(queues))
	for _, queue := range queues {
		names = append(names, queue.Name)
	}
	return names
}

func fatalWithSentry(message string, err error) {
	if err != nil {
		observability.CaptureException(err)
		log.Printf("%s: %v", message, err)
	} else {
		log.Print(message)
	}
	observability.Flush(2 * time.Second)
	os.Exit(1)
}

func fatalMessageWithSentry(message string) {
	observability.CaptureMessage(message)
	log.Print(message)
	observability.Flush(2 * time.Second)
	os.Exit(1)
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
