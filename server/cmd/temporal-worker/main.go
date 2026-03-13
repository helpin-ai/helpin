package main

import (
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

	temporalClient, err := tclient.Dial(temporalapp.BuildClientOptions(cfg))
	if err != nil {
		log.Fatalf("failed to connect to Temporal: %v", err)
	}
	defer temporalClient.Close()

	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	storyRepo := repository.NewPMStoryRepository(db)
	storyLinkRepo := repository.NewPMStoryLinkRepository(db)
	epicRepo := repository.NewPMEpicRepository(db)
	conversationRepo := repository.NewSupportConversationRepository(db)
	commentRepo := repository.NewPMCommentRepository(db)
	checklistRepo := repository.NewPMChecklistItemRepository(db)
	supportMessageRepo := repository.NewSupportMessageRepository(db)
	gitIntRepo := repository.NewGitIntegrationRepository(db)
	gitRepo := repository.NewGitRepositoryRepository(db)
	gitLinkRepo := repository.NewStoryGitLinkRepository(db)
	deliveryRepo := repository.NewStoryDeliveryTargetRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	docsSpaceRepo := repository.NewDocsSpaceRepository(db)
	docsDocumentRepo := repository.NewDocsDocumentRepository(db)
	docsContentRepo := repository.NewDocsContentRepository(db)
	docsVersionRepo := repository.NewDocsVersionRepository(db)
	docsLinkRepo := repository.NewDocsLinkRepository(db)
	crmEmailRepo := repository.NewCRMEmailRepository(db)
	crmContactRepo := repository.NewCRMContactRepository(db)
	crmCalendarRepo := repository.NewCRMCalendarRepository(db)
	crmCompanyRepo := repository.NewCRMCompanyRepository(db)
	crmDealRepo := repository.NewCRMDealRepository(db)
	crmAssociationRepo := repository.NewCRMAssociationRepository(db)
	crmSignalRepo := repository.NewCRMSignalRepository(db)
	crmSummaryRepo := repository.NewCRMSummaryRepository(db)
	automationHealthRepo := repository.NewAutomationHealthRepository(db)

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
	activities := temporalapp.NewAgentRunActivities(
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
		runtimes,
		githubAppClient,
	)

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
	crmSummaryService := service.NewCRMSummaryService(crmSummaryRepo, crmContactRepo, crmCompanyRepo, crmDealRepo, crmAssociationRepo, crmSignalRepo, crmEmailRepo, llmProvider, temporalClient)
	emailSyncActivities := temporalapp.NewEmailSyncActivities(gmailSyncClient, crmEmailRepo, crmContactRepo, crmCalendarRepo, crmEmailSyncSettingsRepo, temporalClient, crmSummaryService)
	signalDetectionService := service.NewSignalDetectionService(llmProvider, crmSignalRepo, crmSummaryService)
	wsHub := ws.NewHub()
	wsPublisher := ws.NewPublisher(wsHub)
	automationHealthService := service.NewAutomationHealthService(automationHealthRepo)
	signalActivities := temporalapp.NewSignalDetectionActivities(signalDetectionService, wsPublisher).SetHealthObserver(automationHealthService)
	summaryActivities := temporalapp.NewCRMSummaryActivities(crmSummaryService).SetHealthObserver(automationHealthService)

	// Deal management activities.
	crmSuggestionRepo := repository.NewCRMSuggestionRepository(db)
	crmAutonomyRepo := repository.NewCRMAutonomyRepository(db)
	dealAutomationService := service.NewDealAutomationService(llmProvider, crmDealRepo, crmSignalRepo, crmSuggestionRepo, crmContactRepo, crmAssociationRepo, crmAutonomyRepo)
	dealMgmtActivities := temporalapp.NewDealManagementActivities(dealAutomationService)

	queueConfigs := selectedQueues()
	workers := make([]tworker.Worker, 0, len(queueConfigs))
	for _, queue := range queueConfigs {
		workers = append(workers, newTemporalWorker(temporalClient, queue.Name, queue.Concurrency, activities, emailSyncActivities, signalActivities, summaryActivities, dealMgmtActivities))
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
	for _, sharedWorker := range workers {
		sharedWorker.Stop()
	}

	if err := sqlDB.Close(); err != nil {
		log.Printf("close database: %v", err)
	}
}

func newTemporalWorker(client tclient.Client, taskQueue string, concurrency int, activities *temporalapp.AgentRunActivities, emailActivities *temporalapp.EmailSyncActivities, signalActivities *temporalapp.SignalDetectionActivities, summaryActivities *temporalapp.CRMSummaryActivities, dealMgmtActivities *temporalapp.DealManagementActivities) tworker.Worker {
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

func queueNames(queues []temporalapp.QueueConfig) []string {
	names := make([]string, 0, len(queues))
	for _, queue := range queues {
		names = append(names, queue.Name)
	}
	return names
}
