package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
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

	githubAppClient, err := githubapp.NewClient(cfg.GitHubAppID, cfg.GitHubAppPrivateKey)
	if err != nil {
		log.Fatalf("initialize github app client: %v", err)
	}
	if githubAppClient == nil {
		log.Fatal("github app client is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

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
	)

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
