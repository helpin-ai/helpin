//go:build ee

// Package bootstrap assembles commercial policies for the explicit EE build.
package bootstrap

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/helpin-ai/helpin/server/ee/aiconnections"
	"github.com/helpin-ai/helpin/server/ee/billingstripe"
	eehandler "github.com/helpin-ai/helpin/server/ee/handler"
	"github.com/helpin-ai/helpin/server/ee/pricing"
	eerepository "github.com/helpin-ai/helpin/server/ee/repository"
	eeservice "github.com/helpin-ai/helpin/server/ee/service"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/config"
	"github.com/helpin-ai/helpin/server/internal/edition"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
	"gorm.io/gorm"
)

type Options struct {
	DB            *gorm.DB
	Config        *config.Config
	Workspace     *repository.WorkspaceRepository
	Organization  *service.OrganizationService
	Authorization *authorization.AuthzService
	Analytics     *service.ProductAnalyticsService
	Outbox        *repository.CustomerIOLifecycleOutboxRepository
}

func New(opts Options) (*edition.Services, error) {
	catalog, err := pricing.LoadCatalog()
	if err != nil {
		return nil, fmt.Errorf("load AI pricing: %w", err)
	}
	billingRepo := eerepository.NewBillingRepository(opts.DB)
	usageRepo := eerepository.NewAIUsageRepository(opts.DB)
	gateway := billingstripe.New(opts.Config.StripeSecretKey)
	billing := eeservice.NewBillingService(billingRepo, gateway, time.Now)
	billing.SetWorkspaceRepository(opts.Workspace)
	if opts.Organization != nil {
		billing.SetOrgRoleResolver(opts.Organization)
	}
	billing.SetPriceConfig(eeservice.BillingPriceConfig{
		StarterMonthly: opts.Config.StripeStarterMonthlyPriceID, StarterAnnual: opts.Config.StripeStarterAnnualPriceID,
		GrowthMonthly: opts.Config.StripeGrowthMonthlyPriceID, GrowthAnnual: opts.Config.StripeGrowthAnnualPriceID,
	})
	billing.SetCustomerIOLifecycleOutboxRepository(opts.Outbox)
	billing.SetProductAnalyticsService(opts.Analytics)
	credentials := map[string]string{}
	for provider, key := range map[string]string{"openai": opts.Config.OpenAIAPIKey, "anthropic": opts.Config.AnthropicAPIKey, "openrouter": opts.Config.OpenRouterAPIKey} {
		if strings.TrimSpace(key) != "" {
			credentials[provider] = key
		}
	}
	profiles, err := service.NewAIStandardProfiles(repository.NewAIStandardProfileRepository(opts.DB), opts.Config.AIConnectionEncryptionKey, "managed", credentials)
	if err != nil {
		return nil, fmt.Errorf("configure managed AI profiles: %w", err)
	}
	result := &edition.Services{
		InitializeAIProfiles:       profiles.EnsureAll,
		Usage:                      eeservice.NewAIUsageService(catalog, usageRepo, nil),
		Entitlements:               eeservice.NewEntitlementService(billing),
		WorkspaceLifecycle:         &workspaceLifecycle{billing: billing, profiles: profiles},
		Seats:                      billing,
		BillingInsights:            eeservice.NewCustomerIOBillingReader(billingRepo, opts.Workspace),
		BillingMetadata:            billingRepo,
		ConnectionPolicy:           aiconnections.NewPolicy(opts.DB),
		RequireConfiguredProviders: true,
		ValidateCompletionRoute: func(route service.AICompletionRoute) error {
			_, err := catalog.Resolve(route.Provider, route.Model, route.Model, route.ServiceTier)
			return err
		},
		AttachIdentity: func(identity *service.CustomerIOIdentityService) { billing.SetCustomerIOIdentityService(identity) },
		RunWorkers:     workers(usageRepo, billing, gateway),
	}
	if opts.Authorization != nil {
		scenarios := eeservice.NewBillingTestScenarioService(opts.DB, billing, time.Now)
		result.Routes = eehandler.NewRoutes(
			eehandler.NewBillingHandler(billing, opts.Config.StripeWebhookSecret, opts.Config.AppBaseURL, scenarios, strings.EqualFold(os.Getenv("BILLING_TEST_SCENARIOS_ENABLED"), "true")),
			eehandler.NewAIUsageHandler(catalog), opts.Authorization)
	}
	return result, nil
}

// Historical SQL remains in the shared immutable ledger. Only EE startup asks
// GORM to evolve commercial models.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&model.WorkspaceBilling{}, &model.StripeWebhookEvent{}, &model.OrganizationBilling{}, &model.BillingPaymentMethod{})
}
