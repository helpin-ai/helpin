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

type Services struct {
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
