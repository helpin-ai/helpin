package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"log/slog"
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

	"github.com/helpin-ai/helpin/server/internal/billingstripe"
	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/crawler"
	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/oauth"
	"github.com/helpin-ai/helpin/server/internal/observability"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
	"github.com/helpin-ai/helpin/server/internal/storage"
	syncpkg "github.com/helpin-ai/helpin/server/internal/sync"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	ws "github.com/helpin-ai/helpin/server/internal/websocket"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
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
	}), &gorm.Config{})
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
	runEngine := temporalapp.NewRunEngine(temporalClient, cfg.TemporalNamespace)

	runRepo := repository.NewAgentRunRepository(db)
	triggerExecutionRepo := repository.NewAgentTriggerExecutionRepository(db)
	runMessageRepo := repository.NewAgentRunMessageRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	workspacePresetVersionRepo := repository.NewWorkspaceAgentPresetVersionRepository(db)
	workspaceSkillRepo := repository.NewWorkspaceSkillRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)
	commandBarPlanRepo := repository.NewCommandBarPlanRepository(db)
	sessionSnapshotRepo := repository.NewCodingSessionStateSnapshotRepository(db)
	codexWorkspaceAuthRepo := repository.NewCodexWorkspaceAuthRepository(db)
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
	supportMailboxRepo := repository.NewSupportMailboxRepository(db)
	supportConversationTriageRepo := repository.NewSupportConversationTriageRepository(db)
	supportConversationTriageEventRepo := repository.NewSupportConversationTriageEventRepository(db)
	supportTriageRuleRepo := repository.NewSupportTriageRuleRepository(db)
	supportTeammateStatusOverrideRepo := repository.NewSupportTeammateStatusOverrideRepository(db)
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
	docsDocumentKeyRepo := repository.NewDocsDocumentKeyRepository(db)
	docsContentRepo := repository.NewDocsContentRepository(db)
	docsBlockRepo := repository.NewDocsBlockRepository(db)
	docsAISectionCandidateRepo := repository.NewDocsAISectionCandidateRepository(db)
	docsChangeProposalRepo := repository.NewDocsChangeProposalRepository(db)
	docsContentRepo.SetBlockRepository(docsBlockRepo)
	docsVersionRepo := repository.NewDocsVersionRepository(db)
	docsLinkRepo := repository.NewDocsLinkRepository(db)
	docsHelpcenterRepo := repository.NewDocsHelpcenterRepository(db, false)
	docsAssetReferenceRepo := repository.NewDocsAssetReferenceRepository(db)
	docsSearchRepo := repository.NewDocsSearchRepository(db)
	docsChunkRepo := repository.NewDocsChunkRepository(db)
	agentKnowledgeSourceRepo := repository.NewAgentKnowledgeSourceRepository(db)
	supportContentSourceRepo := repository.NewSupportContentSourceRepository(db)
	agentContentSourceRepo := repository.NewAgentContentSourceRepository(db)
	supportContentPageRepo := repository.NewSupportContentPageRepository(db)
	supportContentChunkRepo := repository.NewSupportContentChunkRepository(db)
	aiMessageProcessingRepo := repository.NewAIMessageProcessingRepository(db)
	supportInstallRepo := repository.NewSupportInboxInstallationRepository(db)
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
	automationHealthRepo := repository.NewAutomationHealthRepository(db)
	pmAttachmentRepo := repository.NewPMAttachmentRepository(db)
	pmAutomationRepo := repository.NewPMAutomationRepository(db)
	supportAttachmentRepo := repository.NewSupportAttachmentRepository(db)
	billingRepo := repository.NewBillingRepository(db)
	billingGateway := billingstripe.New(cfg.StripeSecretKey, cfg.StripeCreditBlockPriceID)
	billingService := service.NewBillingService(billingRepo, billingGateway, time.Now)
	billingService.SetWorkspaceRepository(workspaceRepo)
	aiUsageMeter := service.NewAIUsageMeter(billingService)

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
	codexWorkspaceAuthStore := workerpkg.NewCodexWorkspaceAuthStore(codexWorkspaceAuthRepo, resolveCodexAuthEncryptionKey(cfg))
	if strings.TrimSpace(cfg.CodexOpenAIAuthMode) == "chatgpt_device_code" && codexWorkspaceAuthStore == nil {
		slog.Warn("Codex workspace auth persistence disabled; set CODEX_AUTH_ENCRYPTION_KEY or a valid CRM_ENCRYPTION_KEY for durable device-code auth")
	}

	runtimes := workerpkg.NewDefaultRuntimeRegistry(
		cfg.OpenCodePath,
		workerpkg.CodexRuntimeConfig{
			Path:                      cfg.CodexPath,
			DefaultModel:              cfg.CodexModel,
			SandboxMode:               cfg.CodexSandboxMode,
			OpenAIAPIKey:              cfg.OpenAIAPIKey,
			OpenAIBaseURL:             cfg.OpenAIBaseURL,
			OpenAIAuthMode:            cfg.CodexOpenAIAuthMode,
			EnableManagedChatGPTOAuth: cfg.CodexEnableChatGPTOAuth,
			ChatGPTAccessToken:        cfg.CodexChatGPTAccessToken,
			ChatGPTAccountID:          cfg.CodexChatGPTAccountID,
			ChatGPTPlanType:           cfg.CodexChatGPTPlanType,
			OpenRouterAPIKey:          cfg.OpenRouterAPIKey,
			OpenRouterBaseURL:         cfg.OpenRouterBaseURL,
			HelpinAPIBaseURL:          cfg.CodexHelpinAPIBaseURL,
			HelpinRunToolTokenSecret:  cfg.JWTSecret,
			HelpinMCPBridgePath:       cfg.CodexHelpinMCPBridgePath,
		},
		cfg.AnthropicAPIKey,
		cfg.AnthropicBaseURL,
		cfg.OpenAIAPIKey,
		cfg.OpenAIBaseURL,
		cfg.OpenRouterAPIKey,
		cfg.OpenRouterBaseURL,
		cfg.BraveSearchAPIKey,
		cfg.ExaSearchAPIKey,
		cfg.CrawlerProxyURLs,
		runRepo,
		artifactRepo,
		codexWorkspaceAuthStore,
		func(ctx context.Context, run *model.AgentRun, agent *model.Agent) error {
			return service.PreflightAgentRunAIUsage(ctx, aiUsageMeter, run, agent)
		},
		func(ctx context.Context, run *model.AgentRun, agent *model.Agent) error {
			return service.RecordAgentRunAIUsage(ctx, aiUsageMeter, run, agent)
		},
	)
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
	var activities *temporalapp.AgentRunActivities

	// Email sync activities (may be nil if Gmail not configured).
	crmEmailSyncSettingsRepo := repository.NewCRMEmailSyncSettingsRepository(db)

	// Signal detection activities.
	var llmProvider llm.Provider
	switch cfg.CRMLLMProvider {
	case "openai":
		if provider := llm.NewOpenAIProvider(cfg.CRMLLMAPIKey, cfg.CRMLLMBaseURL, cfg.CRMLLMModel); provider != nil {
			llmProvider = provider
		} else {
			slog.Warn("CRM OpenAI provider not configured; CRM_LLM_API_KEY is empty")
		}
	default:
		llmProvider = llm.NewClaudeProvider(cfg.AnthropicAPIKey)
	}
	supportLLMRouter, supportEmbeddingProvider := llm.NewSupportRouter(
		cfg.AnthropicAPIKey,
		cfg.OpenAIAPIKey,
		cfg.OpenAIBaseURL,
		cfg.OpenRouterAPIKey,
		cfg.OpenRouterBaseURL,
	)
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
	// AI Support Agent consumer — runs alongside Temporal workers.
	supportInboxService := service.NewSupportInboxService(
		conversationRepo,
		supportMailboxRepo,
		supportMessageRepo,
		nil,
		nil,
		supportInstallRepo,
		nil,
		nil,
		nil,
		nil,
		crmContactRepo,
		userRepo,
		docsSpaceRepo,
		nil,
		docsHelpcenterRepo,
	)
	supportInboxTriageService := service.NewSupportInboxTriageService(
		supportInboxService,
		supportConversationTriageRepo,
		supportConversationTriageEventRepo,
		supportTriageRuleRepo,
		supportInstallRepo,
		supportMailboxRepo,
		conversationRepo,
		supportMessageRepo,
		supportLLMRouter,
	)
	supportInboxService.SetTriageService(supportInboxTriageService)

	supportAIService := service.NewSupportAIService(
		supportLLMRouter, supportEmbeddingProvider, cfg.OpenAIEmbeddingModel, docsChunkRepo,
		agentKnowledgeSourceRepo, supportContentChunkRepo, agentContentSourceRepo, aiMessageProcessingRepo,
		conversationRepo, supportMessageRepo, supportAttachmentRepo,
		agentRepo, handoffRepo, supportInstallRepo,
		wsPublisher, jetstream, redisClient, db,
		cfg.QueryExpansionModel, cfg.QueryExpansionProvider,
	)
	supportAIService.SetSupportRoutingDependencies(workspaceRepo, nil, supportTeammateStatusOverrideRepo)
	supportAIService.SetMailboxRepository(supportMailboxRepo)
	supportAIService.SetTriageService(supportInboxTriageService)
	supportAIService.SetLinkPreviewService(service.NewSupportLinkPreviewService(cfg.CrawlerProxyURLs))
	aiConsumerCtx, aiConsumerCancel := context.WithCancel(context.Background())
	go func() {
		if err := supportAIService.StartNATSConsumer(aiConsumerCtx); err != nil {
			log.Printf("support AI consumer stopped: %v", err)
		}
	}()
	_ = aiConsumerCancel // used at shutdown

	gitGraceCleanupCtx, gitGraceCleanupCancel := context.WithCancel(context.Background())
	go workerpkg.NewGitGraceCleanup(gitIntRepo, gitRepo).Start(gitGraceCleanupCtx)
	_ = gitGraceCleanupCancel // used at shutdown

	crmSummaryService := service.NewCRMSummaryService(crmSummaryRepo, crmContactRepo, crmCompanyRepo, crmDealRepo, crmAssociationRepo, crmSignalRepo, crmEmailRepo, llmProvider, temporalClient)
	supportCoverageRepo := repository.NewSupportCoverageRepository(db)
	supportCoverageAnalysisRepo := repository.NewSupportCoverageAnalysisRepository(db)
	supportCoverageService := service.NewSupportCoverageService(supportCoverageRepo)
	supportCoverageEnrichmentService := service.NewSupportCoverageEnrichmentService(db, llmProvider)
	supportCoverageKnowledgeMatcher := service.NewCoverageKnowledgeMatcher(docsChunkRepo, supportContentChunkRepo, supportEmbeddingProvider, cfg.OpenAIEmbeddingModel)
	supportCoverageDailyAnalyzer := service.NewSupportCoverageDailyAnalyzer(llmProvider, cfg.CRMLLMProvider, cfg.CRMLLMModel).
		SetCoverageRepositories(supportCoverageRepo, supportCoverageAnalysisRepo).
		SetEmbeddingProvider(supportEmbeddingProvider, cfg.OpenAIEmbeddingModel).
		SetConversationRepositories(conversationRepo, supportMessageRepo).
		SetKnowledgeMatcher(supportCoverageKnowledgeMatcher, docsSpaceRepo, supportContentSourceRepo).
		SetTemporalClient(temporalClient)
	supportCoverageTraceService := service.NewSupportCoverageRetrievalTraceService(supportCoverageAnalysisRepo)
	supportAIService.SetSupportAIRetrievalTraceRecorder(supportCoverageTraceService)
	emailSyncActivities := temporalapp.NewEmailSyncActivities(gmailSyncClient, crmEmailRepo, crmContactRepo, crmCalendarRepo, crmEmailSyncSettingsRepo, temporalClient, crmSummaryService)
	signalDetectionService := service.NewSignalDetectionService(llmProvider, crmSignalRepo, crmSummaryService)
	runRepo.SetNotifier(ws.NewRunNotifier(wsPublisher))
	runRepo.SetTriggerExecutionRepository(triggerExecutionRepo)
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
		runEngine,
		gitService,
		pmStoryService,
		pmActivityService,
		wsPublisher,
	).SetWorkspaceSkillStore(workspaceSkillRepo, nil).SetModelProviderConfig(
		cfg.AnthropicAPIKey,
		cfg.OpenAIAPIKey,
		cfg.OpenRouterAPIKey,
		cfg.CodexOpenAIAuthMode,
		cfg.CodexEnableChatGPTOAuth,
		cfg.CodexChatGPTAccessToken,
		cfg.CodexChatGPTAccountID,
	).SetTriggerExecutionRepository(triggerExecutionRepo).SetCommandBarPlanRepository(commandBarPlanRepo).SetNotificationService(notificationService).SetCRMRepositories(crmContactRepo, crmDealRepo)
	agentService.SetWorkflowService(pmWorkflowService)
	docsDocumentService := service.NewDocsDocumentService(docsDocumentRepo, docsSpaceRepo, wsPublisher, cfg.DocsOrderingUseSortKey)
	docsContentService := service.NewDocsContentService(docsContentRepo, docsDocumentRepo, nil)
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
	docsEmbeddingService := service.NewDocsEmbeddingService(
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
	crmDealService := service.NewCRMDealService(crmDealRepo, crmAssociationRepo)
	crmActivityService := service.NewCRMActivityService(crmActivityRepo)
	crmEnrichmentService := service.NewCRMEnrichmentService(crmEnrichmentRepo, crmContactRepo, crmCompanyRepo, crmAssociationRepo)
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
	commandService.SetDocsCreateDependencies(docsDocumentService, docsContentRepo)
	commandService.SetDocsBlockService(docsBlockService)
	activities = temporalapp.NewAgentRunActivities(
		runRepo,
		runMessageRepo,
		agentRepo,
		workspaceSkillRepo,
		s3Client,
		artifactRepo,
		interactionRepo,
		sessionSnapshotRepo,
		storyRepo,
		taskLinkRepo,
		epicRepo,
		conversationRepo,
		commentRepo,
		checklistRepo,
		workflowRepo,
		supportMessageRepo,
		gitIntRepo,
		gitRepo,
		gitLinkRepo,
		deliveryRepo,
		settingsRepo,
		workspaceRepo,
		docsSpaceRepo,
		docsCollectionRepo,
		docsDocumentRepo,
		docsDocumentKeyRepo,
		docsContentRepo,
		docsBlockRepo,
		docsAISectionCandidateRepo,
		docsChangeProposalRepo,
		docsVersionRepo,
		docsLinkRepo,
		docsSearchRepo,
		crmDealRepo,
		crmContactRepo,
		crmSignalRepo,
		crmActivityRepo,
		commandService,
		notificationService,
		releaseFactsService,
		wsPublisher,
		runtimes,
		githubAppClient,
		gitCredentialRepo,
		resolveGitOAuthEncryptionKey(cfg),
		runEngine,
		agentService,
	)
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
	activities.SetRuleEngine(ruleEngine)
	signalActivities := temporalapp.NewSignalDetectionActivities(signalDetectionService, wsPublisher).SetHealthObserver(automationHealthService)
	summaryActivities := temporalapp.NewCRMSummaryActivities(crmSummaryService).SetHealthObserver(automationHealthService)
	coverageActivities := temporalapp.NewCoverageGapActivities(supportCoverageEnrichmentService, supportCoverageService)
	coverageAnalysisActivities := temporalapp.NewCoverageAnalysisActivities(supportCoverageDailyAnalyzer)

	// Deal management activities.
	crmSuggestionRepo := repository.NewCRMSuggestionRepository(db)
	crmAutonomyRepo := repository.NewCRMAutonomyRepository(db)
	dealAutomationService := service.NewDealAutomationService(llmProvider, crmDealRepo, crmSignalRepo, crmSuggestionRepo, crmContactRepo, crmAssociationRepo, crmAutonomyRepo)
	dealMgmtActivities := temporalapp.NewDealManagementActivities(dealAutomationService)

	_ = crmCompanyRepo // available for future enrichment activities

	scheduledRuleActivities := temporalapp.NewScheduledRuleActivities(ruleEngine)
	recurringActivities := service.NewPMRecurringTemplateActivities(pmRecurringTemplateService)

	// Sprint automation activities.
	sprintAutomationActivities := temporalapp.NewSprintAutomationActivities(pmAutomationService)
	docsEmbeddingActivities := temporalapp.NewDocsEmbeddingActivities(docsEmbeddingService)
	docsAssetCleanupActivities := temporalapp.NewDocsAssetCleanupActivities(docsAssetReferenceRepo, s3Client)
	contentSourceSyncActivities := temporalapp.NewContentSourceSyncActivities(supportContentSyncService)
	pmImportActivities := service.NewPMImportActivities(pmImportService)

	queueConfigs := selectedQueues()
	workers := make([]tworker.Worker, 0, len(queueConfigs))
	for _, queue := range queueConfigs {
		workers = append(workers, newTemporalWorker(temporalClient, queue.Name, queue.Concurrency, activities, emailSyncActivities, signalActivities, summaryActivities, coverageActivities, coverageAnalysisActivities, dealMgmtActivities, scheduledRuleActivities, recurringActivities, sprintAutomationActivities, docsEmbeddingActivities, docsAssetCleanupActivities, contentSourceSyncActivities, pmImportActivities))
	}

	for _, sharedWorker := range workers {
		if err := sharedWorker.Start(); err != nil {
			fatalWithSentry("failed to start temporal worker", err)
		}
	}
	log.Printf("temporal workers started for namespace=%s queues=%v", cfg.TemporalNamespace, queueNames(queueConfigs))

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-stopCh

	log.Println("shutting down temporal workers")
	aiConsumerCancel() // stop AI support consumer
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

func newTemporalWorker(client tclient.Client, taskQueue string, concurrency int, activities *temporalapp.AgentRunActivities, emailActivities *temporalapp.EmailSyncActivities, signalActivities *temporalapp.SignalDetectionActivities, summaryActivities *temporalapp.CRMSummaryActivities, coverageActivities *temporalapp.CoverageGapActivities, coverageAnalysisActivities *temporalapp.CoverageAnalysisActivities, dealMgmtActivities *temporalapp.DealManagementActivities, scheduledRuleActivities *temporalapp.ScheduledRuleActivities, recurringActivities *service.PMRecurringTemplateActivities, sprintActivities *temporalapp.SprintAutomationActivities, docsEmbeddingActivities *temporalapp.DocsEmbeddingActivities, docsAssetCleanupActivities *temporalapp.DocsAssetCleanupActivities, contentSourceSyncActivities *temporalapp.ContentSourceSyncActivities, pmImportActivities *service.PMImportActivities) tworker.Worker {
	options := tworker.Options{
		MaxConcurrentActivityExecutionSize: concurrency,
		WorkerStopTimeout:                  temporalWorkerStopTimeout,
	}
	w := tworker.New(client, taskQueue, options)
	w.RegisterWorkflow(temporalapp.AgentRunWorkflow)
	w.RegisterWorkflow(temporalapp.CommandBarPlanWorkflow)
	w.RegisterActivityWithOptions(activities.PrepareRunActivity, activity.RegisterOptions{
		Name: "AgentRunActivities.PrepareRunActivity",
	})
	w.RegisterActivityWithOptions(activities.ExecuteRunActivity, activity.RegisterOptions{
		Name: "AgentRunActivities.ExecuteRunActivity",
	})
	w.RegisterActivityWithOptions(activities.MarkRunFailedActivity, activity.RegisterOptions{
		Name: "AgentRunActivities.MarkRunFailedActivity",
	})
	w.RegisterActivityWithOptions(activities.AdvanceCommandBarPlanActivity, activity.RegisterOptions{
		Name: "AgentRunActivities.AdvanceCommandBarPlanActivity",
	})
	w.RegisterActivityWithOptions(activities.StartReadyCommandBarPlanStepsActivity, activity.RegisterOptions{
		Name: "AgentRunActivities.StartReadyCommandBarPlanStepsActivity",
	})

	// Register email sync workflow and activities.
	w.RegisterWorkflow(temporalapp.EmailSyncWorkflow)
	if emailActivities != nil {
		w.RegisterActivityWithOptions(emailActivities.BackfillEmailsActivity, activity.RegisterOptions{
			Name: "EmailSyncActivities.BackfillEmailsActivity",
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

func resolveCodexAuthEncryptionKey(cfg *config.Config) []byte {
	if cfg == nil {
		return nil
	}
	if key, err := decodeOptionalAES256HexKey(strings.TrimSpace(cfg.CodexAuthEncryptionKey)); err != nil {
		slog.Warn("invalid CODEX_AUTH_ENCRYPTION_KEY (must be a 32-byte hex-encoded AES key)", "error", err)
	} else if len(key) == 32 {
		return key
	}
	if key, err := decodeOptionalAES256HexKey(strings.TrimSpace(cfg.CRMEncryptionKey)); err != nil {
		slog.Warn("invalid CRM_ENCRYPTION_KEY for Codex workspace auth fallback (must be a 32-byte hex-encoded AES key)", "error", err)
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
