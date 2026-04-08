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

	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/crawler"
	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/oauth"
	"github.com/helpin-ai/helpin/server/internal/observability"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
	syncpkg "github.com/helpin-ai/helpin/server/internal/sync"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	ws "github.com/helpin-ai/helpin/server/internal/websocket"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

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
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)
	sessionSnapshotRepo := repository.NewCodingSessionStateSnapshotRepository(db)
	codexWorkspaceAuthRepo := repository.NewCodexWorkspaceAuthRepository(db)
	storyRepo := repository.NewPMTaskRepository(db)
	taskLinkRepo := repository.NewPMTaskLinkRepository(db)
	epicRepo := repository.NewPMEpicRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	workflowRepo := repository.NewPMWorkflowRepository(db)
	labelRepo := repository.NewPMLabelRepository(db)
	conversationRepo := repository.NewSupportConversationRepository(db)
	commentRepo := repository.NewPMCommentRepository(db)
	checklistRepo := repository.NewPMChecklistItemRepository(db)
	externalLinkRepo := repository.NewPMExternalLinkRepository(db)
	recurringRepo := repository.NewPMRecurringTemplateRepository(db)
	sprintRepo := repository.NewPMSprintRepository(db)
	pmActivityRepo := repository.NewPMActivityRepository(db)
	supportMessageRepo := repository.NewSupportMessageRepository(db)
	supportMailboxRepo := repository.NewSupportMailboxRepository(db)
	supportConversationTriageRepo := repository.NewSupportConversationTriageRepository(db)
	supportConversationTriageEventRepo := repository.NewSupportConversationTriageEventRepository(db)
	supportTriageRuleRepo := repository.NewSupportTriageRuleRepository(db)
	supportTeammateStatusOverrideRepo := repository.NewSupportTeammateStatusOverrideRepository(db)
	userRepo := repository.NewUserRepository(db)
	gitIntRepo := repository.NewGitIntegrationRepository(db)
	gitRepo := repository.NewGitRepositoryRepository(db)
	gitLinkRepo := repository.NewTaskGitLinkRepository(db)
	deliveryRepo := repository.NewTaskDeliveryTargetRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	handoffRepo := repository.NewAgentHandoffRepository(db)
	docsSpaceRepo := repository.NewDocsSpaceRepository(db)
	docsDocumentRepo := repository.NewDocsDocumentRepository(db)
	docsContentRepo := repository.NewDocsContentRepository(db)
	docsVersionRepo := repository.NewDocsVersionRepository(db)
	docsLinkRepo := repository.NewDocsLinkRepository(db)
	docsHelpcenterRepo := repository.NewDocsHelpcenterRepository(db)
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
	crmSummaryRepo := repository.NewCRMSummaryRepository(db)
	automationHealthRepo := repository.NewAutomationHealthRepository(db)
	pmAttachmentRepo := repository.NewPMAttachmentRepository(db)
	pmAutomationRepo := repository.NewPMAutomationRepository(db)
	supportAttachmentRepo := repository.NewSupportAttachmentRepository(db)

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
			OpenAIAPIKey:              cfg.OpenAIAPIKey,
			OpenAIBaseURL:             cfg.OpenAIBaseURL,
			OpenAIAuthMode:            cfg.CodexOpenAIAuthMode,
			EnableManagedChatGPTOAuth: cfg.CodexEnableChatGPTOAuth,
			ChatGPTAccessToken:        cfg.CodexChatGPTAccessToken,
			ChatGPTAccountID:          cfg.CodexChatGPTAccountID,
			ChatGPTPlanType:           cfg.CodexChatGPTPlanType,
			OpenRouterAPIKey:          cfg.OpenRouterAPIKey,
			OpenRouterBaseURL:         cfg.OpenRouterBaseURL,
		},
		cfg.AnthropicAPIKey,
		cfg.AnthropicBaseURL,
		cfg.OpenAIAPIKey,
		cfg.OpenAIBaseURL,
		cfg.OpenRouterAPIKey,
		cfg.OpenRouterBaseURL,
		cfg.BraveSearchAPIKey,
		runRepo,
		artifactRepo,
		codexWorkspaceAuthStore,
	)
	githubAppClient, err := githubapp.NewClient(cfg.GitHubAppID, cfg.GitHubAppPrivateKey)
	if err != nil {
		fatalWithSentry("failed to initialize github app client", err)
	}
	wsPublisher := ws.NewJetStreamPublisher(jetstream)
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

	crmSummaryService := service.NewCRMSummaryService(crmSummaryRepo, crmContactRepo, crmCompanyRepo, crmDealRepo, crmAssociationRepo, crmSignalRepo, crmEmailRepo, llmProvider, temporalClient)
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
	)
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
		storyRepo,
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
		nil,
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
	).SetModelProviderConfig(
		cfg.AnthropicAPIKey,
		cfg.OpenAIAPIKey,
		cfg.OpenRouterAPIKey,
		cfg.CodexOpenAIAuthMode,
		cfg.CodexEnableChatGPTOAuth,
		cfg.CodexChatGPTAccessToken,
		cfg.CodexChatGPTAccountID,
	).SetTriggerExecutionRepository(triggerExecutionRepo)
	agentService.SetWorkflowService(pmWorkflowService)
	docsContentService := service.NewDocsContentService(docsContentRepo, docsDocumentRepo, nil)
	docsLinkService := service.NewDocsLinkService(docsLinkRepo, storyRepo, docsDocumentRepo, nil)
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
		nil,
	)
	supportContentSyncService := service.NewSupportContentSyncService(
		supportContentSourceRepo,
		supportContentPageRepo,
		supportContentChunkRepo,
		supportEmbeddingProvider,
		cfg.OpenAIEmbeddingModel,
		contentCrawler,
		nil,
	)
	crmDealService := service.NewCRMDealService(crmDealRepo, crmAssociationRepo)
	crmActivityService := service.NewCRMActivityService(crmActivityRepo)
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
	activities = temporalapp.NewAgentRunActivities(
		runRepo,
		runMessageRepo,
		agentRepo,
		artifactRepo,
		interactionRepo,
		sessionSnapshotRepo,
		storyRepo,
		taskLinkRepo,
		epicRepo,
		conversationRepo,
		commentRepo,
		checklistRepo,
		supportMessageRepo,
		gitIntRepo,
		gitRepo,
		gitLinkRepo,
		deliveryRepo,
		settingsRepo,
		workspaceRepo,
		docsSpaceRepo,
		docsDocumentRepo,
		docsContentRepo,
		docsVersionRepo,
		docsLinkRepo,
		docsSearchRepo,
		crmDealRepo,
		crmContactRepo,
		crmSignalRepo,
		crmActivityRepo,
		commandService,
		wsPublisher,
		runtimes,
		githubAppClient,
		runEngine,
	)
	automationHealthService := service.NewAutomationHealthService(automationHealthRepo)
	signalActivities := temporalapp.NewSignalDetectionActivities(signalDetectionService, wsPublisher).SetHealthObserver(automationHealthService)
	summaryActivities := temporalapp.NewCRMSummaryActivities(crmSummaryService).SetHealthObserver(automationHealthService)

	// Deal management activities.
	crmSuggestionRepo := repository.NewCRMSuggestionRepository(db)
	crmAutonomyRepo := repository.NewCRMAutonomyRepository(db)
	dealAutomationService := service.NewDealAutomationService(llmProvider, crmDealRepo, crmSignalRepo, crmSuggestionRepo, crmContactRepo, crmAssociationRepo, crmAutonomyRepo)
	dealMgmtActivities := temporalapp.NewDealManagementActivities(dealAutomationService)

	_ = crmCompanyRepo // available for future enrichment activities

	scheduleActivities := temporalapp.NewScheduledAgentActivities(agentRepo, runRepo).SetTriggerExecutionRepository(triggerExecutionRepo)
	recurringActivities := service.NewPMRecurringTemplateActivities(pmRecurringTemplateService)

	// Sprint automation activities.
	sprintAutomationActivities := temporalapp.NewSprintAutomationActivities(pmAutomationService)
	docsEmbeddingActivities := temporalapp.NewDocsEmbeddingActivities(docsEmbeddingService)
	contentSourceSyncActivities := temporalapp.NewContentSourceSyncActivities(supportContentSyncService)

	queueConfigs := selectedQueues()
	workers := make([]tworker.Worker, 0, len(queueConfigs))
	for _, queue := range queueConfigs {
		workers = append(workers, newTemporalWorker(temporalClient, queue.Name, queue.Concurrency, activities, emailSyncActivities, signalActivities, summaryActivities, dealMgmtActivities, scheduleActivities, recurringActivities, sprintAutomationActivities, docsEmbeddingActivities, contentSourceSyncActivities))
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

func newTemporalWorker(client tclient.Client, taskQueue string, concurrency int, activities *temporalapp.AgentRunActivities, emailActivities *temporalapp.EmailSyncActivities, signalActivities *temporalapp.SignalDetectionActivities, summaryActivities *temporalapp.CRMSummaryActivities, dealMgmtActivities *temporalapp.DealManagementActivities, scheduleActivities *temporalapp.ScheduledAgentActivities, recurringActivities *service.PMRecurringTemplateActivities, sprintActivities *temporalapp.SprintAutomationActivities, docsEmbeddingActivities *temporalapp.DocsEmbeddingActivities, contentSourceSyncActivities *temporalapp.ContentSourceSyncActivities) tworker.Worker {
	options := tworker.Options{
		MaxConcurrentActivityExecutionSize: concurrency,
	}
	w := tworker.New(client, taskQueue, options)
	w.RegisterWorkflow(temporalapp.AgentRunWorkflow)
	w.RegisterActivityWithOptions(activities.PrepareRunActivity, activity.RegisterOptions{
		Name: "AgentRunActivities.PrepareRunActivity",
	})
	w.RegisterActivityWithOptions(activities.ExecuteRunActivity, activity.RegisterOptions{
		Name: "AgentRunActivities.ExecuteRunActivity",
	})
	w.RegisterActivityWithOptions(activities.MarkRunFailedActivity, activity.RegisterOptions{
		Name: "AgentRunActivities.MarkRunFailedActivity",
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

	// Register deal management cron workflow and activities.
	w.RegisterWorkflow(temporalapp.DealManagementCronWorkflow)
	if dealMgmtActivities != nil {
		w.RegisterActivityWithOptions(dealMgmtActivities.EvaluateProgressionActivity, activity.RegisterOptions{
			Name: "DealManagementActivities.EvaluateProgressionActivity",
		})
	}

	// Register scheduled agent workflow and activities.
	w.RegisterWorkflow(temporalapp.ScheduledAgentWorkflow)
	if scheduleActivities != nil {
		w.RegisterActivityWithOptions(scheduleActivities.CreateScheduledRun, activity.RegisterOptions{
			Name: "ScheduledAgentActivities.CreateScheduledRun",
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

	// Register content source sync workflow and activities.
	w.RegisterWorkflow(temporalapp.ContentSourceSyncWorkflow)
	if contentSourceSyncActivities != nil {
		w.RegisterActivityWithOptions(contentSourceSyncActivities.SyncContentSourceActivity, activity.RegisterOptions{
			Name: "ContentSourceSyncActivities.SyncContentSourceActivity",
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
