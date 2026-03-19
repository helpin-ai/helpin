package main

import (
	"context"
	"encoding/hex"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
	"go.temporal.io/sdk/activity"
	tclient "go.temporal.io/sdk/client"
	tworker "go.temporal.io/sdk/worker"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/oauth"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
	syncpkg "github.com/helpin-ai/helpin/server/internal/sync"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
	ws "github.com/helpin-ai/helpin/server/internal/websocket"
	workerpkg "github.com/helpin-ai/helpin/server/internal/worker"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  cfg.DatabaseURL,
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to get sql.DB: %v", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	realtimeInstanceID := ws.ResolveRealtimeInstanceID()
	natsConn, jetstream, err := ws.ConnectJetStream(cfg.NatsURL, "helpin-temporal-worker-"+realtimeInstanceID)
	if err != nil {
		log.Fatalf("failed to connect to NATS: %v", err)
	}
	defer natsConn.Close()
	if err := ws.EnsureJetStreamInfrastructure(jetstream); err != nil {
		log.Fatalf("failed to ensure JetStream infrastructure: %v", err)
	}

	temporalClient, err := tclient.Dial(temporalapp.BuildClientOptions(cfg))
	if err != nil {
		log.Fatalf("failed to connect to Temporal: %v", err)
	}
	defer temporalClient.Close()
	runEngine := temporalapp.NewRunEngine(temporalClient, cfg.TemporalNamespace)

	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	storyRepo := repository.NewPMStoryRepository(db)
	storyLinkRepo := repository.NewPMStoryLinkRepository(db)
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
	gitIntRepo := repository.NewGitIntegrationRepository(db)
	gitRepo := repository.NewGitRepositoryRepository(db)
	gitLinkRepo := repository.NewStoryGitLinkRepository(db)
	deliveryRepo := repository.NewStoryDeliveryTargetRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	handoffRepo := repository.NewAgentHandoffRepository(db)
	flowRepo := repository.NewFlowRepository(db)
	docsSpaceRepo := repository.NewDocsSpaceRepository(db)
	docsDocumentRepo := repository.NewDocsDocumentRepository(db)
	docsContentRepo := repository.NewDocsContentRepository(db)
	docsVersionRepo := repository.NewDocsVersionRepository(db)
	docsLinkRepo := repository.NewDocsLinkRepository(db)
	docsSearchRepo := repository.NewDocsSearchRepository(db)
	agentKnowledgeSourceRepo := repository.NewAgentKnowledgeSourceRepository(db)
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

	runtimes := workerpkg.NewDefaultRuntimeRegistry(
		cfg.OpenCodePath,
		cfg.AnthropicAPIKey,
		cfg.OpenAIAPIKey,
		cfg.OpenAIBaseURL,
		cfg.OpenRouterAPIKey,
		cfg.OpenRouterBaseURL,
		cfg.BraveSearchAPIKey,
		runRepo,
		artifactRepo,
	)
	githubAppClient, err := githubapp.NewClient(cfg.GitHubAppID, cfg.GitHubAppPrivateKey)
	if err != nil {
		log.Fatalf("failed to initialize github app client: %v", err)
	}
	wsPublisher := ws.NewJetStreamPublisher(jetstream)
	var activities *temporalapp.AgentRunActivities

	// Email sync activities (may be nil if Gmail not configured).
	crmEmailSyncSettingsRepo := repository.NewCRMEmailSyncSettingsRepository(db)

	// Signal detection activities.
	var llmProvider llm.Provider
	switch cfg.CRMLLMProvider {
	case "openai":
		llmProvider = llm.NewOpenAIProvider(cfg.CRMLLMAPIKey, cfg.CRMLLMBaseURL, cfg.CRMLLMModel)
	default:
		llmProvider = llm.NewClaudeProvider(cfg.AnthropicAPIKey)
	}
	// AI Support Agent consumer — runs alongside Temporal workers.
	supportAIService := service.NewSupportAIService(
		llmProvider, docsSearchRepo, docsContentRepo, docsSpaceRepo,
		agentKnowledgeSourceRepo, aiMessageProcessingRepo,
		conversationRepo, supportMessageRepo,
		agentRepo, handoffRepo, supportInstallRepo,
		wsPublisher, jetstream, nil, db,
	)
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
	pmStoryService := service.NewPMStoryService(
		storyRepo,
		workspaceRepo,
		workflowRepo,
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
	pmRecurringTemplateService.SetStoryService(pmStoryService)
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
		runRepo,
		artifactRepo,
		storyRepo,
		storyLinkRepo,
		epicRepo,
		conversationRepo,
		supportMessageRepo,
		handoffRepo,
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
	docsContentService := service.NewDocsContentService(docsContentRepo)
	docsLinkService := service.NewDocsLinkService(docsLinkRepo, storyRepo, docsDocumentRepo)
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
		storyLinkRepo,
	)
	activities = temporalapp.NewAgentRunActivities(
		runRepo,
		agentRepo,
		artifactRepo,
		storyRepo,
		storyLinkRepo,
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

	scheduleActivities := temporalapp.NewScheduledAgentActivities(agentRepo, runRepo)
	recurringActivities := service.NewPMRecurringTemplateActivities(pmRecurringTemplateService)

	// Sprint automation activities.
	sprintAutomationActivities := temporalapp.NewSprintAutomationActivities(pmAutomationService)

	queueConfigs := selectedQueues()
	workers := make([]tworker.Worker, 0, len(queueConfigs))
	for _, queue := range queueConfigs {
		workers = append(workers, newTemporalWorker(temporalClient, queue.Name, queue.Concurrency, activities, emailSyncActivities, signalActivities, summaryActivities, dealMgmtActivities, scheduleActivities, recurringActivities, sprintAutomationActivities))
	}

	// Planning session worker — separate queue with session pinning.
	// Uses JetStream to relay events cross-process to the API server's WS hub.
	planningModels := &workerpkg.EinoModelFactory{
		AnthropicAPIKey: cfg.AnthropicAPIKey,
		OpenAIAPIKey:    cfg.OpenAIAPIKey,
		OpenAIBaseURL:   cfg.OpenAIBaseURL,
		OpenRouterKey:   cfg.OpenRouterAPIKey,
		OpenRouterURL:   cfg.OpenRouterBaseURL,
	}

	if cfg.AnthropicAPIKey != "" || cfg.OpenAIAPIKey != "" || cfg.OpenRouterAPIKey != "" {
		planningSessionRepo := repository.NewPlanningSessionRepository(db)

		var webSearchClient workerpkg.WebSearchClient
		toolRegistry := workerpkg.NewToolRegistry(webSearchClient)

		jsStreamer := ws.NewJetStreamSessionStreamer(jetstream)

		planningService := service.NewPlanningSessionService(
			planningSessionRepo, epicRepo, agentRepo, settingsRepo,
			docsContentRepo, docsVersionRepo, docsLinkRepo,
			docsDocumentRepo, docsSpaceRepo,
			planningModels, toolRegistry,
			jsStreamer, wsPublisher,
		)
		planningService.SetWorkflowStarter(&planningWorkflowAdapter{engine: runEngine})
		flowService := service.NewFlowService(
			flowRepo,
			epicRepo,
			storyRepo,
			crmDealRepo,
			agentRepo,
			runRepo,
			planningSessionRepo,
			agentService,
			planningService,
			runEngine,
			wsPublisher,
		)
		flowService.SetCommandService(commandService)
		flowActivities := service.NewFlowRuntimeActivities(flowService)

		planningActivities := temporalapp.NewPlanningSessionActivities(
			planningSessionRepo, epicRepo, gitIntRepo, gitRepo, githubAppClient,
			planningService.RunAgentTurnWithContext,
			planningService.RunFinalizationTurnWithContext,
		)
		planningWorker := tworker.New(temporalClient, temporalapp.QueuePlanningInteractive, tworker.Options{
			MaxConcurrentActivityExecutionSize: 4,
			EnableSessionWorker:                true,
		})
		flowWorker := tworker.New(temporalClient, temporalapp.QueueFlowOrchestrator, tworker.Options{
			MaxConcurrentActivityExecutionSize: 4,
		})
		planningWorker.RegisterWorkflow(temporalapp.PlanningSessionWorkflow)
		flowWorker.RegisterWorkflow(temporalapp.FlowRunWorkflow)
		planningWorker.RegisterActivityWithOptions(planningActivities.PrepareWorkspaceActivity, activity.RegisterOptions{
			Name: "PlanningSessionActivities.PrepareWorkspaceActivity",
		})
		planningWorker.RegisterActivityWithOptions(planningActivities.RunTurnActivity, activity.RegisterOptions{
			Name: "PlanningSessionActivities.RunTurnActivity",
		})
		planningWorker.RegisterActivityWithOptions(planningActivities.FinalizeTurnActivity, activity.RegisterOptions{
			Name: "PlanningSessionActivities.FinalizeTurnActivity",
		})
		planningWorker.RegisterActivityWithOptions(planningActivities.CleanupWorkspaceActivity, activity.RegisterOptions{
			Name: "PlanningSessionActivities.CleanupWorkspaceActivity",
		})
		flowWorker.RegisterActivityWithOptions(flowActivities.BootstrapRunActivity, activity.RegisterOptions{
			Name: "FlowRuntimeActivities.BootstrapRunActivity",
		})
		flowWorker.RegisterActivityWithOptions(flowActivities.FinalizeInteractiveNodeActivity, activity.RegisterOptions{
			Name: "FlowRuntimeActivities.FinalizeInteractiveNodeActivity",
		})
		flowWorker.RegisterActivityWithOptions(flowActivities.HandleApprovalActionActivity, activity.RegisterOptions{
			Name: "FlowRuntimeActivities.HandleApprovalActionActivity",
		})
		flowWorker.RegisterActivityWithOptions(flowActivities.RetryNodeActivity, activity.RegisterOptions{
			Name: "FlowRuntimeActivities.RetryNodeActivity",
		})
		flowWorker.RegisterActivityWithOptions(flowActivities.CancelRunActivity, activity.RegisterOptions{
			Name: "FlowRuntimeActivities.CancelRunActivity",
		})
		flowWorker.RegisterActivityWithOptions(flowActivities.ProgressRunStateActivity, activity.RegisterOptions{
			Name: "FlowRuntimeActivities.ProgressRunStateActivity",
		})
		flowWorker.RegisterActivityWithOptions(flowActivities.LoadRunStateActivity, activity.RegisterOptions{
			Name: "FlowRuntimeActivities.LoadRunStateActivity",
		})
		flowWorker.RegisterActivityWithOptions(flowActivities.HandleChildStateActivity, activity.RegisterOptions{
			Name: "FlowRuntimeActivities.HandleChildStateActivity",
		})
		workers = append(workers, planningWorker)
		workers = append(workers, flowWorker)
		log.Printf("planning session worker registered on queues %s and %s", temporalapp.QueuePlanningInteractive, temporalapp.QueueFlowOrchestrator)
	}

	for _, sharedWorker := range workers {
		if err := sharedWorker.Start(); err != nil {
			log.Fatalf("failed to start temporal worker: %v", err)
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

func newTemporalWorker(client tclient.Client, taskQueue string, concurrency int, activities *temporalapp.AgentRunActivities, emailActivities *temporalapp.EmailSyncActivities, signalActivities *temporalapp.SignalDetectionActivities, summaryActivities *temporalapp.CRMSummaryActivities, dealMgmtActivities *temporalapp.DealManagementActivities, scheduleActivities *temporalapp.ScheduledAgentActivities, recurringActivities *service.PMRecurringTemplateActivities, sprintActivities *temporalapp.SprintAutomationActivities) tworker.Worker {
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
			log.Fatalf("unknown TEMPORAL_WORKER_QUEUES entry %q", name)
		}
		selected = append(selected, queue)
	}
	if len(selected) == 0 {
		log.Fatal("TEMPORAL_WORKER_QUEUES did not contain any valid queues")
	}
	return selected
}

type planningWorkflowAdapter struct {
	engine *temporalapp.RunEngine
}

func (a *planningWorkflowAdapter) StartPlanningSession(ctx context.Context, sessionID string) error {
	return a.engine.StartPlanningSession(ctx, sessionID)
}

func (a *planningWorkflowAdapter) SignalPlanningMessage(ctx context.Context, sessionID string) error {
	return a.engine.SignalPlanningSession(ctx, sessionID, temporalapp.PlanningSessionSignal{
		Type: temporalapp.PlanningSessionSignalTypeMessage,
	})
}

func (a *planningWorkflowAdapter) SignalPlanningFinalize(ctx context.Context, sessionID, actorID string) error {
	return a.engine.SignalPlanningSession(ctx, sessionID, temporalapp.PlanningSessionSignal{
		Type:    temporalapp.PlanningSessionSignalTypeFinalize,
		ActorID: actorID,
	})
}

func (a *planningWorkflowAdapter) SignalPlanningAbandon(ctx context.Context, sessionID string) error {
	return a.engine.SignalPlanningSession(ctx, sessionID, temporalapp.PlanningSessionSignal{
		Type: temporalapp.PlanningSessionSignalTypeAbandon,
	})
}

func (a *planningWorkflowAdapter) SignalFlowChildState(ctx context.Context, flowRunID, nodeRunID, childType, childID, childStatus string) error {
	return a.engine.SignalFlowRun(ctx, flowRunID, temporalapp.FlowRunSignal{
		Type:        temporalapp.FlowSignalTypeChildState,
		NodeRunID:   nodeRunID,
		ChildType:   childType,
		ChildID:     childID,
		ChildStatus: childStatus,
	})
}

func queueNames(queues []temporalapp.QueueConfig) []string {
	names := make([]string, 0, len(queues))
	for _, queue := range queues {
		names = append(names, queue.Name)
	}
	return names
}
