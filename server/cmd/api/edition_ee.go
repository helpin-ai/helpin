//go:build ee

package main

import (
	"github.com/helpin-ai/helpin/server/ee/bootstrap"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/edition"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
	"gorm.io/gorm"
)

func newEditionServices(db *gorm.DB, cfg *config.Config, workspace *repository.WorkspaceRepository, org *service.OrganizationService, authz *authorization.AuthzService, analytics *service.ProductAnalyticsService, outbox *repository.CustomerIOLifecycleOutboxRepository) (*edition.Services, error) {
	return bootstrap.New(bootstrap.Options{DB: db, Config: cfg, Workspace: workspace, Organization: org, Authorization: authz, Analytics: analytics, Outbox: outbox})
}

func autoMigrateEdition(db *gorm.DB) error { return bootstrap.AutoMigrate(db) }
