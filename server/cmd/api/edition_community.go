//go:build !ee

package main

import (
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/edition"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
	"gorm.io/gorm"
)

func newEditionServices(db *gorm.DB, cfg *config.Config, workspace *repository.WorkspaceRepository, org *service.OrganizationService, authz *authorization.AuthzService, analytics *service.ProductAnalyticsService, outbox *repository.CustomerIOLifecycleOutboxRepository) (*edition.Services, error) {
	return edition.Community(db), nil
}

func autoMigrateEdition(db *gorm.DB) error { return nil }
