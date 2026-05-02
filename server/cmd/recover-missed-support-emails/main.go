package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func main() {
	_ = godotenv.Load()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	fromRaw := flag.String("from", "", "RFC3339 start of the recovery window")
	toRaw := flag.String("to", "", "RFC3339 end of the recovery window")
	limit := flag.Int("limit", 250, "maximum missed messages to inspect in this run")
	execute := flag.Bool("execute", false, "send emails; omit for dry-run")
	flag.Parse()

	from := mustParseTime("from", *fromRaw)
	to := mustParseTime("to", *toRaw)

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		fatal("DATABASE_URL is required")
	}
	replyToken := firstNonEmpty(os.Getenv("POSTMARK_REPLY_SERVER_TOKEN"), os.Getenv("POSTMARK_SERVER_TOKEN"))
	replyFrom := firstNonEmpty(os.Getenv("POSTMARK_REPLY_FROM_EMAIL"), os.Getenv("POSTMARK_FROM_EMAIL"))
	if strings.TrimSpace(replyToken) == "" {
		fatal("POSTMARK_REPLY_SERVER_TOKEN is required")
	}
	if strings.TrimSpace(replyFrom) == "" {
		fatal("POSTMARK_REPLY_FROM_EMAIL is required")
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  databaseURL,
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
		fatalf("connect database: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	conversationRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	emailLogRepo := repository.NewSupportEmailLogRepository(db)
	webhookRepo := repository.NewSupportEmailWebhookEventRepository(db)
	installRepo := repository.NewSupportInboxInstallationRepository(db)
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	mailboxRepo := repository.NewSupportMailboxRepository(db)

	emailClient := email.NewClient(replyToken, replyFrom)
	fallbackService := service.NewEmailFallbackService(
		nil,
		nil,
		nil,
		emailClient,
		messageRepo,
		conversationRepo,
		emailLogRepo,
		webhookRepo,
		installRepo,
		sessionRepo,
		workspaceRepo,
		firstNonEmpty(os.Getenv("SUPPORT_EMAIL_REPLY_DOMAIN"), "replies.helpin.email"),
		firstNonEmpty(os.Getenv("APP_BASE_URL"), "https://app.helpin.ai"),
		"recover-missed-support-emails",
	)
	fallbackService.SetCRMContactRepository(contactRepo)
	supportInboxService := service.NewSupportInboxService(conversationRepo, mailboxRepo, messageRepo, nil, nil, installRepo, sessionRepo, nil, nil, nil, contactRepo, nil, nil, nil, nil)
	supportInboxService.SetWorkspaceRepo(workspaceRepo)
	supportInboxService.SetRouteDomain(firstNonEmpty(os.Getenv("SUPPORT_EMAIL_ROUTE_DOMAIN"), os.Getenv("SUPPORT_EMAIL_REPLY_DOMAIN"), "on.helpin.email"))
	fallbackService.SetSupportInboxService(supportInboxService)

	result, err := fallbackService.BackfillMissedOutboundEmails(ctx, service.EmailFallbackBackfillOptions{
		From:   from,
		To:     to,
		Limit:  *limit,
		DryRun: !*execute,
	})
	if err != nil {
		fatalf("recover missed support emails: %v", err)
	}

	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fatalf("encode result: %v", err)
	}
	fmt.Println(string(encoded))
	if !*execute {
		fmt.Fprintln(os.Stderr, "dry-run only; rerun with --execute to send emails")
	}
}

func mustParseTime(name, value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		fatalf("--%s is required", name)
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		fatalf("parse --%s: %v", name, err)
	}
	return parsed.UTC()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
