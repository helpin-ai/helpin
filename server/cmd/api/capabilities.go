package main

import (
	"context"
	"errors"
	"strconv"
	"strings"

	enumspb "go.temporal.io/api/enums/v1"
	tclient "go.temporal.io/sdk/client"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/deployment"
	"github.com/helpin-ai/helpin/server/internal/email"
	"github.com/helpin-ai/helpin/server/internal/handler"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
	"github.com/helpin-ai/helpin/server/internal/storage"
	"github.com/helpin-ai/helpin/server/internal/temporalapp"
)

// capabilityWiring collects the already-constructed dependencies behind the
// capability endpoints so main only passes them through.
type capabilityWiring struct {
	appEmail             email.AppSender
	emailDiagnostics     model.EmailDiagnosticsConfig
	aiConnectionsEnabled bool
	chatProviders        []string
	embeddingModel       string
	storage              *storage.S3Client
	temporal             tclient.Client
	gitHubAppConfigured  func(context.Context) bool
}

func newCapabilityHandler(db *gorm.DB, cfg *config.Config, deps capabilityWiring) *handler.CapabilityHandler {
	d := deps.emailDiagnostics
	supportEmailConfigured := (strings.TrimSpace(d.SupportEmailReplyDomain) != "" && d.ReplyEmailConfigured && d.ReplyInboundSecretSet) ||
		(strings.TrimSpace(d.SupportEmailRouteDomain) != "" && d.RouteEmailConfigured && d.RouteInboundSecretSet)
	capabilityConfig := service.CapabilityConfig{
		Edition:                 deployment.EditionName,
		Modules:                 cfg.EnabledModules,
		AppEmailConfigured:      deps.appEmail != nil,
		AppEmailFingerprint:     appEmailFingerprint(cfg, deps.appEmail),
		AIConnectionsEnabled:    deps.aiConnectionsEnabled,
		ServerChatProviders:     deps.chatProviders,
		EmbeddingModel:          deps.embeddingModel,
		SupportEmailConfigured:  supportEmailConfigured,
		ObjectStorageConfigured: deps.storage != nil,
		GitHubAppConfigured:     deps.gitHubAppConfigured,
	}
	if deps.storage != nil {
		capabilityConfig.StorageProbe = deps.storage.CheckBucket
	}
	temporal := deps.temporal
	capabilityConfig.WorkerProbe = func(ctx context.Context) (bool, error) {
		if temporal == nil {
			return false, errors.New("temporal client unavailable")
		}
		resp, err := temporal.DescribeTaskQueue(ctx, temporalapp.QueueAutomation, enumspb.TASK_QUEUE_TYPE_WORKFLOW)
		if err != nil {
			return false, err
		}
		return len(resp.GetPollers()) > 0, nil
	}
	capabilities := service.NewCapabilityService(capabilityConfig, repository.NewCapabilityRepository(db),
		repository.NewSetupRepository(db), repository.NewAIConnectionRepository(db))
	var sender service.TestEmailSender
	if deps.appEmail != nil {
		sender = deps.appEmail
	}
	capabilities.SetTestEmail(sender, repository.NewUserRepository(db))
	return handler.NewCapabilityHandler(capabilities)
}

// appEmailFingerprint covers the non-secret application mail settings. A
// stored test result stops applying when any of them changes.
func appEmailFingerprint(cfg *config.Config, sender email.AppSender) string {
	if sender == nil {
		return ""
	}
	if cfg.SMTPHost != "" {
		return service.AppEmailFingerprint("smtp", cfg.SMTPHost, strconv.Itoa(cfg.SMTPPort), cfg.SMTPUsername, cfg.SMTPFrom, cfg.SMTPTLSMode)
	}
	return service.AppEmailFingerprint("provider", sender.FromEmail())
}
