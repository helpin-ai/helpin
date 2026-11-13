// Package edition defines the optional policies consumed by product startup.
// Community construction has no dependency on commercial packages or prices.
package edition

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/router"
	"github.com/helpin-ai/helpin/server/internal/service"
	"gorm.io/gorm"
)

type BillingMetadata interface {
	GetByWorkspaceID(context.Context, string) (*model.WorkspaceBilling, error)
}

// Extension hands an edition the product services that are built after the
// edition itself, so it can add features on top of them.
type Extension struct {
	DB                     *gorm.DB
	HelpcenterHostedDomain string
	MCP                    *service.MCPService
	Users                  *repository.UserRepository
	Organizations          *service.OrganizationService
	Workspaces             *service.WorkspaceService
	Helpcenter             *service.DocsHelpcenterService
	DocsImport             *service.DocsImportService
	DocsSpaces             *service.DocsSpaceService
	DocsCollections        *service.DocsCollectionService
	SupportInbox           *service.SupportInboxService
	CRMCompanies           *repository.CRMCompanyRepository
	CRMActivities          *service.CRMActivityService
}

type Services struct {
	InitializeAIProfiles       func(context.Context) error
	Usage                      service.AIUsageLifecycle
	Entitlements               service.EntitlementPolicy
	WorkspaceLifecycle         service.WorkspaceLifecyclePolicy
	Seats                      service.WorkspaceSeatPolicy
	BillingInsights            service.CustomerIOBillingInsights
	BillingMetadata            BillingMetadata
	ConnectionPolicy           service.AIConnectionAdmissionPolicy
	Routes                     router.EditionRoutes
	RequireConfiguredProviders bool
	ValidateCompletionRoute    func(service.AICompletionRoute) error
	AttachIdentity             func(*service.CustomerIOIdentityService)
	RunWorkers                 func(context.Context)
	// Extend, when set, is called once after the product services exist and
	// before workers start. Community leaves it nil.
	Extend func(Extension) error
}

// ExtendWith passes the product services to the edition. It does nothing in
// an edition that adds no features on top of them.
func (s *Services) ExtendWith(ext Extension) error {
	if s == nil || s.Extend == nil {
		return nil
	}
	return s.Extend(ext)
}

func Community(db *gorm.DB) *Services {
	return &Services{Usage: service.NewCommunityAIUsage(repository.NewAIExecutionUsageRepository(db))}
}

// StartWorkers returns a completion channel in both editions so shutdown follows
// one path. The worker implementation owns and joins all of its goroutines.
func (s *Services) StartWorkers(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	if s.RunWorkers == nil {
		close(done)
		return done
	}
	go func() { defer close(done); s.RunWorkers(ctx) }()
	return done
}
