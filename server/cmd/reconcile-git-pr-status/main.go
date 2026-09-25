package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/githubapp"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func main() {
	_ = godotenv.Load()

	limit := flag.Int("limit", 200, "maximum open PR links to reconcile")
	apply := flag.Bool("apply", false, "persist reconciled PR status changes")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
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
		log.Fatalf("connect database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("get sql db: %v", err)
	}
	defer sqlDB.Close()

	if _, err := githubapp.NewClient(cfg.GitHubAppID, cfg.GitHubAppPrivateKey); err != nil {
		log.Fatalf("initialize github app client: %v", err)
	}
	githubAppConfig := service.NewGitHubAppConfigService(
		repository.NewGitHubAppCredentialRepository(db), nil, nil,
		service.GitHubAppConfigOptions{
			Env: githubapp.Credentials{
				AppID:         cfg.GitHubAppID,
				Slug:          cfg.GitHubAppSlug,
				PrivateKey:    cfg.GitHubAppPrivateKey,
				WebhookSecret: cfg.GitHubAppWebhookSecret,
			},
			EncryptionKey: gitEncryptionKey(cfg),
		},
	)
	githubAppClient := githubapp.NewClientWithSource(githubAppConfig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if !githubAppClient.Configured(ctx) {
		log.Fatal("github app client is not configured")
	}

	gitService := service.NewGitService(
		repository.NewGitIntegrationRepository(db),
		repository.NewGitRepositoryRepository(db),
		repository.NewTaskGitLinkRepository(db),
		repository.NewTaskDeliveryTargetRepository(db),
		repository.NewSettingsRepository(db),
		repository.NewWorkspaceRepository(db),
		repository.NewOrganizationRepository(db),
		repository.NewPMTaskRepository(db),
		nil,
		nil,
		githubAppClient,
		cfg.AppBaseURL,
		cfg.GitHubAppSlug,
		cfg.JWTSecret,
	).SetGitHubAppSource(githubAppConfig)

	result, err := gitService.ReconcileOpenPullRequestStatuses(ctx, *limit, !*apply)
	if err != nil {
		log.Fatalf("reconcile git pr status: %v", err)
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("encode result: %v", err)
	}
	fmt.Println(string(encoded))
	if result.Failed > 0 {
		os.Exit(2)
	}
}

// gitEncryptionKey returns the 32-byte key that decrypts a stored GitHub App,
// matching the API: GIT_OAUTH_ENCRYPTION_KEY, then CRM_ENCRYPTION_KEY.
func gitEncryptionKey(cfg *config.Config) []byte {
	for _, value := range []string{cfg.GitOAuthEncryptionKey, cfg.CRMEncryptionKey} {
		key, err := hex.DecodeString(strings.TrimSpace(value))
		if err == nil && len(key) == 32 {
			return key
		}
	}
	return nil
}
