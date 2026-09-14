package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/aimodel"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type AIProfileBootstrapOptions struct {
	WorkspaceID       string
	Funding           string
	DryRun            bool
	RotateCredentials bool
	// Credentials contains only values explicitly supplied by the operator.
	Credentials map[string]string
}

type AIProfileBootstrapResult struct {
	AgentsMigrated           int      `json:"agents_migrated"`
	AgentsDefaultedToSmall   int      `json:"agents_defaulted_to_small"`
	ProfilesCreated          int      `json:"profiles_created"`
	UnconfiguredProviders    []string `json:"unconfigured_providers"`
	RunsUsingRuntimeDefaults []string `json:"runs_using_runtime_defaults"`
}

type AIProfileBootstrapService struct {
	store *repository.AIProfileBootstrapRepository
	key   []byte
}

func NewAIProfileBootstrapService(store *repository.AIProfileBootstrapRepository, encryptionKey string) (*AIProfileBootstrapService, error) {
	key, err := parseExternalMCPEncryptionKey(encryptionKey)
	if err != nil || len(key) != 32 {
		return nil, errors.New("a valid AI_CONNECTION_ENCRYPTION_KEY is required")
	}
	return &AIProfileBootstrapService{store: store, key: key}, nil
}

// Apply imports explicit credentials and migrates current agent defaults in one
// transaction. It never touches historical versions, runs, or reviewed setups.
// This is an operator API and is deliberately not exposed through HTTP.
func (s *AIProfileBootstrapService) Apply(ctx context.Context, opts AIProfileBootstrapOptions) (*AIProfileBootstrapResult, error) {
	if opts.WorkspaceID == "" || (opts.Funding != "customer" && opts.Funding != "managed") {
		return nil, errors.New("workspace and customer or managed funding are required")
	}
	for provider, key := range opts.Credentials {
		if !bootstrapAPIProvider(provider) || strings.TrimSpace(key) == "" || len(key) > 65536 || strings.ContainsAny(key, "\r\n") {
			return nil, errors.New("bootstrap requires supported providers and valid explicit API keys")
		}
	}
	result := &AIProfileBootstrapResult{}
	err := s.store.Transaction(ctx, opts.WorkspaceID, func(store *repository.AIProfileBootstrapRepository) error {
		settings, err := store.Settings(ctx, opts.WorkspaceID)
		if err != nil {
			return err
		}
		connections := map[string]*model.AIConnection{}
		ensureConnection := func(provider string) (*model.AIConnection, error) {
			if c := connections[provider]; c != nil {
				return c, nil
			}
			c, err := s.bootstrapConnection(ctx, store, opts, provider)
			if err != nil {
				return nil, err
			}
			connections[provider] = c
			return c, nil
		}
		for provider := range opts.Credentials {
			if _, err := ensureConnection(provider); err != nil {
				return err
			}
		}
		// Keep the established tier routes. Missing keys remain visibly unconfigured.
		var defaultID string
		tierIDs := []string{}
		for _, tier := range []aimodel.Tier{aimodel.TierSmall, aimodel.TierMedium, aimodel.TierLarge, aimodel.TierFlagship} {
			route := selectableAgentTierRoutes[tier]
			c, err := ensureConnection(route.Provider)
			if err != nil {
				return err
			}
			controls := &sdk.ModelControls{}
			if quantizations := providerQuantizationsForAgentRoute(route); len(quantizations) != 0 {
				controls.OpenRouter = &sdk.OpenRouterModelControls{Provider: &sdk.OpenRouterProviderPreferences{Quantizations: quantizations}}
			}
			p := bootstrapProfile(opts.WorkspaceID, "tier:"+string(tier), strings.ToUpper(string(tier[:1]))+string(tier[1:]),
				model.AIProfileRoute{ConnectionID: c.ID, Model: sdk.RunModel{Provider: route.Provider, Model: route.Model, Controls: controls}})
			created, err := store.EnsureProfile(ctx, p)
			if err != nil {
				return err
			}
			if created {
				result.ProfilesCreated++
			}
			tierIDs = append(tierIDs, p.ID)
			if tier == aimodel.TierSmall {
				defaultID = p.ID
			}
		}
		if settings.ProfilesBootstrappedAt == nil {
			if err := s.assignBootstrapProfiles(ctx, store, opts.WorkspaceID, tierIDs, ensureConnection, result); err != nil {
				return err
			}
		}
		if err := store.EnsureDefault(ctx, &model.AIWorkspaceSettings{WorkspaceID: opts.WorkspaceID, DefaultProfileID: &defaultID, UpdatedAt: time.Now().UTC()}); err != nil {
			return err
		}
		if err := store.Complete(ctx, opts.WorkspaceID); err != nil {
			return err
		}
		for provider, c := range connections {
			if c.Status != "connected" {
				result.UnconfiguredProviders = append(result.UnconfiguredProviders, provider)
			}
		}
		runs, err := store.NonterminalRuns(ctx, opts.WorkspaceID)
		if err != nil {
			return err
		}
		for _, run := range runs {
			var input model.AgentRunInputPayload
			if err := decodeAIConnectionRunInput(run.Input, &input); err != nil {
				return err
			}
			if input.ModelConnectionID == "" {
				result.RunsUsingRuntimeDefaults = append(result.RunsUsingRuntimeDefaults, run.ID)
			}
		}
		if opts.DryRun {
			return errAIProfileBootstrapDryRun
		}
		return nil
	})
	if err != nil && !errors.Is(err, errAIProfileBootstrapDryRun) {
		return nil, err
	}
	sort.Strings(result.UnconfiguredProviders)
	return result, nil
}

func bootstrapProfile(workspace, identity, name string, route model.AIProfileRoute) *model.AIProfile {
	now := time.Now().UTC()
	return &model.AIProfile{ID: uuid.NewSHA1(uuid.NameSpaceOID, []byte("helpin-ai-profile-v1|"+workspace+"|"+identity)).String(),
		WorkspaceID: workspace, Scope: "workspace", Name: name, Revision: 1, Primary: route, CreatedAt: now, UpdatedAt: now}
}

func bootstrapAPIProvider(provider string) bool {
	return provider == "openai" || provider == "anthropic" || provider == "openrouter" || provider == "openrouter_responses"
}

func bootstrapAgentModel(agent model.Agent) (sdk.RunModel, error) {
	provider, name := strings.TrimSpace(derefString(agent.Provider)), strings.TrimSpace(derefString(agent.Model))
	controls := &sdk.ModelControls{}
	if name == "" {
		// A provider alone is not a model choice. Preserve a named size, or use
		// Small when the agent has neither a model nor a size configured.
		tier := aimodel.Tier(strings.TrimSpace(agent.ModelTier))
		if tier == "" {
			tier = aimodel.TierSmall
		}
		route, ok := selectableAgentTierRoutes[tier]
		if !ok {
			return sdk.RunModel{}, fmt.Errorf("unknown model size %q", tier)
		}
		provider, name = route.Provider, route.Model
		if quantizations := providerQuantizationsForAgentRoute(route); len(quantizations) != 0 {
			controls.OpenRouter = &sdk.OpenRouterModelControls{Provider: &sdk.OpenRouterProviderPreferences{Quantizations: quantizations}}
		}
	}
	if provider == "openrouter-responses" {
		provider = "openrouter_responses"
	}
	if provider == "" || name == "" {
		return sdk.RunModel{}, errors.New("an explicit model requires a provider before profile migration")
	}
	if len(agent.ExecutionConfig) > 0 {
		if err := json.Unmarshal(agent.ExecutionConfig, controls); err != nil {
			return sdk.RunModel{}, err
		}
	}
	route := sdk.RunModel{Provider: provider, Model: name, Controls: controls}
	return route, sdk.ValidateRunModel(&route)
}

func (s *AIProfileBootstrapService) bootstrapConnection(ctx context.Context, store *repository.AIProfileBootstrapRepository, opts AIProfileBootstrapOptions, provider string) (*model.AIConnection, error) {
	if !bootstrapAPIProvider(provider) {
		return nil, errors.New("bootstrap cannot import personal OAuth credentials")
	}
	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte("helpin-ai-connection-v1|"+opts.WorkspaceID+"|"+opts.Funding+"|"+provider)).String()
	c, err := store.Connection(ctx, id)
	if err != nil {
		return nil, err
	}
	if c != nil && (c.WorkspaceID != opts.WorkspaceID || c.Scope != "workspace" || c.UserID != nil || c.Provider != provider || c.Funding != opts.Funding) {
		return nil, errors.New("bootstrap connection identity conflicts with existing data")
	}
	apiKey := strings.TrimSpace(opts.Credentials[provider])
	if provider == "openrouter_responses" && apiKey == "" {
		apiKey = strings.TrimSpace(opts.Credentials["openrouter"])
	}
	if c != nil && c.Status != "unconfigured" {
		secret, err := (&AIConnectionService{key: s.key}).open(c)
		if err != nil {
			return nil, errors.New("cannot decrypt an existing bootstrap connection")
		}
		if apiKey == "" || (secret.APIKey == apiKey && !opts.RotateCredentials) {
			return c, nil
		}
		if !opts.RotateCredentials {
			return nil, errors.New("bootstrap preserves existing credentials; explicitly request credential rotation to replace them")
		}
	}
	if c == nil {
		c = &model.AIConnection{ID: id, WorkspaceID: opts.WorkspaceID, Scope: "workspace", Funding: opts.Funding,
			Provider: provider, Name: provider + " (" + opts.Funding + ")", Status: "unconfigured", CreatedAt: time.Now().UTC()}
	}
	if apiKey != "" {
		if err := (&AIConnectionService{key: s.key}).seal(c, aiConnectionSecret{APIKey: apiKey}); err != nil {
			return nil, err
		}
		c.Status = "connected"
	}
	c.UpdatedAt = time.Now().UTC()
	if err := store.SaveConnection(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

var errAIProfileBootstrapDryRun = errors.New("AI profile bootstrap dry run")
