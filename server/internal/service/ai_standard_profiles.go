package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/aimodel"
	"github.com/helpin-ai/helpin/server/internal/crypto"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// AIStandardProfiles provisions shared sizes without modifying agent selections.
type AIStandardProfiles struct {
	store       *repository.AIStandardProfileRepository
	key         []byte
	funding     string
	credentials map[string]string
}

func NewAIStandardProfiles(store *repository.AIStandardProfileRepository, encryptionKey, funding string, credentials map[string]string) (*AIStandardProfiles, error) {
	if funding != "managed" && funding != "customer" {
		return nil, errors.New("invalid standard connection funding")
	}
	var key []byte
	if encryptionKey != "" {
		var err error
		key, err = parseExternalMCPEncryptionKey(encryptionKey)
		if err != nil {
			return nil, errors.New("invalid AI_CONNECTION_ENCRYPTION_KEY")
		}
	}
	if funding == "managed" && len(key) != 32 {
		return nil, errors.New("AI_CONNECTION_ENCRYPTION_KEY is required for managed connections")
	}
	return &AIStandardProfiles{store: store, key: key, funding: funding, credentials: credentials}, nil
}

func (s *AIStandardProfiles) EnsureAll(ctx context.Context) error {
	ids, err := s.store.Workspaces(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.EnsureWorkspace(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// EnsureWorkspace provisions the standard connections, profiles, and default,
// then re-maps untouched standard profiles to the providers that are connected.
func (s *AIStandardProfiles) EnsureWorkspace(ctx context.Context, workspace string) error {
	return s.store.Transaction(ctx, workspace, func(store *repository.AIStandardProfileRepository) error {
		standard := map[string]*model.AIConnection{}
		for _, tier := range standardTierOrder {
			route, err := standardModelForTier(string(tier))
			if err != nil {
				return err
			}
			if standard[route.Provider] != nil {
				continue
			}
			c, err := s.ensureConnection(ctx, store, workspace, route.Provider)
			if err != nil {
				return err
			}
			standard[route.Provider] = c
		}
		usable, shared, err := sharedStandardConnections(ctx, store, workspace, standard)
		if err != nil {
			return err
		}
		for _, tier := range standardTierOrder {
			route, c, err := standardAssignment(string(tier), standard, usable)
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			p := &model.AIProfile{ID: model.StandardAIProfileID(workspace, string(tier)), WorkspaceID: workspace, Scope: "workspace", Name: strings.ToUpper(string(tier[:1])) + string(tier[1:]), Revision: 1,
				Primary: model.AIProfileRoute{ConnectionID: c.ID, Model: route}, CreatedAt: now, UpdatedAt: now}
			created, err := store.EnsureProfile(ctx, p)
			if err != nil {
				return err
			}
			if created {
				continue
			}
			if err := remapStandardProfile(ctx, store, workspace, string(tier), p.Primary, shared); err != nil {
				return err
			}
		}
		small := model.StandardAIProfileID(workspace, "small")
		if err := store.EnsureDefault(ctx, &model.AIWorkspaceSettings{WorkspaceID: workspace, DefaultProfileID: &small, UpdatedAt: time.Now().UTC()}); err != nil {
			return err
		}
		return repointStandardDefault(ctx, store, workspace)
	})
}

// AIConnectionsChanged re-maps standard profiles after a shared connection changes.
func (s *AIStandardProfiles) AIConnectionsChanged(ctx context.Context, workspace string) error {
	return s.EnsureWorkspace(ctx, workspace)
}

// StandardRoute returns the model the tier's standard profile currently runs,
// falling back to the provider-aware resolver when the profile is missing.
func (s *AIStandardProfiles) StandardRoute(ctx context.Context, workspace, tier string) (sdk.RunModel, error) {
	p, err := s.store.Profile(ctx, model.StandardAIProfileID(workspace, tier))
	if err != nil {
		return sdk.RunModel{}, err
	}
	if p != nil && p.DeletedAt == nil && p.Primary.Model.Provider != "" && p.Primary.Model.Model != "" {
		route := p.Primary.Model
		if route.Controls == nil {
			route.Controls = &sdk.ModelControls{}
		}
		return route, nil
	}
	usable, _, err := sharedStandardConnections(ctx, s.store, workspace, nil)
	if err != nil {
		return sdk.RunModel{}, err
	}
	return standardRouteForTier(tier, connectedProviderSet(usable))
}

// standardAssignment picks the route and connection for one standard tier. A
// managed (platform-funded) fixed connection is never bypassed, even when it is
// disconnected, so edition policy decides its availability.
func standardAssignment(tier string, standard, usable map[string]*model.AIConnection) (sdk.RunModel, *model.AIConnection, error) {
	fixed, err := standardModelForTier(tier)
	if err != nil {
		return sdk.RunModel{}, nil, err
	}
	if home := standard[fixed.Provider]; home != nil && home.Funding == "managed" {
		return fixed, home, nil
	}
	route, err := standardRouteForTier(tier, connectedProviderSet(usable))
	if err != nil {
		return sdk.RunModel{}, nil, err
	}
	if c := usable[route.Provider]; c != nil {
		return route, c, nil
	}
	if c := standard[route.Provider]; c != nil {
		return route, c, nil
	}
	return fixed, standard[fixed.Provider], nil
}

// sharedStandardConnections returns the usable connection per provider,
// preferring the generated standard connection, and every live shared
// connection by ID.
func sharedStandardConnections(ctx context.Context, store *repository.AIStandardProfileRepository, workspace string, standard map[string]*model.AIConnection) (map[string]*model.AIConnection, map[string]*model.AIConnection, error) {
	list, err := store.SharedConnections(ctx, workspace, standardProviderPreference)
	if err != nil {
		return nil, nil, err
	}
	usable := map[string]*model.AIConnection{}
	shared := map[string]*model.AIConnection{}
	for provider, c := range standard {
		if c != nil {
			shared[c.ID] = c
		}
		if usableStandardConnection(c) {
			usable[provider] = c
		}
	}
	for i := range list {
		c := &list[i]
		if shared[c.ID] == nil {
			shared[c.ID] = c
		}
		if c.ID == model.StandardAIConnectionID(workspace, c.Provider) && usable[c.Provider] == nil && usableStandardConnection(c) {
			usable[c.Provider] = c
		}
	}
	for i := range list {
		c := &list[i]
		if usable[c.Provider] == nil && usableStandardConnection(c) {
			usable[c.Provider] = c
		}
	}
	return usable, shared, nil
}

func connectedProviderSet(usable map[string]*model.AIConnection) map[string]bool {
	connected := make(map[string]bool, len(usable))
	for provider := range usable {
		connected[provider] = true
	}
	return connected
}

// remapStandardProfile updates a generated profile only while it is untouched:
// initial revision, no fallback, and a primary on a shared standard-provider
// connection with a catalog or fixed standard route.
func remapStandardProfile(ctx context.Context, store *repository.AIStandardProfileRepository, workspace, tier string, desired model.AIProfileRoute, shared map[string]*model.AIConnection) error {
	p, err := store.Profile(ctx, model.StandardAIProfileID(workspace, tier))
	if err != nil {
		return err
	}
	if p == nil || p.DeletedAt != nil || p.Revision != 1 || p.Fallback != nil || p.Scope != "workspace" || p.UserID != nil {
		return nil
	}
	current := p.Primary
	if current.ConnectionID == desired.ConnectionID && current.Model.Provider == desired.Model.Provider && current.Model.Model == desired.Model.Model {
		return nil
	}
	c := shared[current.ConnectionID]
	if c == nil || c.Provider != current.Model.Provider || !isStandardRoute(tier, current.Model) {
		return nil
	}
	updated, err := store.UpdateUntouchedPrimary(ctx, p.ID, desired, time.Now().UTC())
	if err != nil {
		return err
	}
	if updated {
		slog.InfoContext(ctx, "standard AI profile re-mapped", "workspace_id", p.WorkspaceID, "profile_id", p.ID,
			"tier", tier, "provider", desired.Model.Provider, "model", desired.Model.Model, "connection_id", desired.ConnectionID)
	}
	return nil
}

func isStandardRoute(tier string, route sdk.RunModel) bool {
	if fixed, err := standardModelForTier(tier); err == nil && fixed.Provider == route.Provider && fixed.Model == route.Model {
		return true
	}
	resolver := loadDefaultAgentModelTierResolver()
	if resolver == nil || resolver.catalog == nil {
		return false
	}
	for _, provider := range standardProviderPreference {
		if provider == route.Provider {
			_, err := resolver.catalog.Resolve(route.Provider, route.Model, route.Model, defaultAICompletionServiceTier)
			return err == nil
		}
	}
	return false
}

// repointStandardDefault moves a default that still names the standard Small
// profile to the first runnable standard profile when Small cannot run.
func repointStandardDefault(ctx context.Context, store *repository.AIStandardProfileRepository, workspace string) error {
	settings, err := store.Settings(ctx, workspace)
	if err != nil || settings == nil {
		return err
	}
	small := model.StandardAIProfileID(workspace, string(aimodel.TierSmall))
	if derefString(settings.DefaultProfileID) != small {
		return nil
	}
	runnable, err := standardProfileRunnable(ctx, store, small)
	if err != nil || runnable {
		return err
	}
	for _, tier := range standardTierOrder[1:] {
		id := model.StandardAIProfileID(workspace, string(tier))
		runnable, err := standardProfileRunnable(ctx, store, id)
		if err != nil {
			return err
		}
		if !runnable {
			continue
		}
		moved, err := store.ReplaceDefault(ctx, workspace, small, id, time.Now().UTC())
		if err != nil {
			return err
		}
		if moved {
			slog.InfoContext(ctx, "workspace AI default moved to a runnable standard profile", "workspace_id", workspace, "profile_id", id, "tier", tier)
		}
		return nil
	}
	return nil
}

func standardProfileRunnable(ctx context.Context, store *repository.AIStandardProfileRepository, id string) (bool, error) {
	p, err := store.Profile(ctx, id)
	if err != nil || p == nil || p.DeletedAt != nil {
		return false, err
	}
	c, err := store.ConnectionByID(ctx, p.Primary.ConnectionID)
	if err != nil || c == nil {
		return false, err
	}
	if c.SupersededBy != nil || c.Status != "connected" || c.Provider != p.Primary.Model.Provider {
		return false, nil
	}
	return len(c.EncryptedSecret) > 0 || (c.Endpoint != nil && c.Endpoint.AuthMode == "none"), nil
}

func (s *AIStandardProfiles) ensureConnection(ctx context.Context, store *repository.AIStandardProfileRepository, workspace, provider string) (*model.AIConnection, error) {
	id := model.StandardAIConnectionID(workspace, provider)
	c, err := store.ManagedConnection(ctx, workspace, provider)
	if err != nil {
		return nil, err
	}
	if c == nil {
		c, err = store.Connection(ctx, id)
		if err != nil {
			return nil, err
		}
	}
	fresh := c == nil
	if fresh {
		c = &model.AIConnection{ID: id, WorkspaceID: workspace, Scope: "workspace", Funding: s.funding, Name: provider + " (" + s.funding + ")", Provider: provider, Status: "unconfigured", CreatedAt: time.Now().UTC()}
	}
	if c.WorkspaceID != workspace || c.Scope != "workspace" || c.UserID != nil || c.Provider != provider || c.SupersededBy != nil {
		return nil, errors.New("standard connection identity conflicts with existing configuration")
	}
	// Only an untouched placeholder may change funding. Customer keys and
	// explicitly disconnected connections are never adopted or reconnected.
	// A connected customer connection whose key came from the environment may
	// follow a rotated environment key; a user-supplied key never does.
	placeholder := c.Status == "unconfigured" && len(c.EncryptedSecret) == 0
	environmentOwned := s.funding == "customer" && c.Funding == "customer" && c.ID == id && c.Status == "connected" &&
		derefString(c.CredentialSource) == model.AIConnectionCredentialSourceEnvironment
	if !fresh && !placeholder && !environmentOwned && (c.Funding != "managed" || s.funding != "managed" || c.Status != "connected") {
		return c, nil
	}
	changed := fresh
	if placeholder && c.Funding != s.funding && s.funding == "managed" {
		c.Funding = s.funding
		changed = true
	}
	if c.ID == id && c.Funding == "managed" && (c.Name == "Standard "+provider || c.Name == provider+" (customer)") {
		c.Name = provider + " (managed)"
		changed = true
	}
	sealed, err := s.sealEnvironmentCredential(c, provider)
	if err != nil {
		return nil, err
	}
	if sealed {
		changed = true
		slog.InfoContext(ctx, "standard AI connection credential set from environment", "workspace_id", workspace,
			"connection_id", c.ID, "provider", provider, "funding", c.Funding, "rotated", !placeholder && !fresh)
	}
	if changed {
		c.UpdatedAt = time.Now().UTC()
		if err := store.SaveConnection(ctx, c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// sealEnvironmentCredential stores the configured provider key when it differs
// from the sealed one. It reports whether the connection changed.
func (s *AIStandardProfiles) sealEnvironmentCredential(c *model.AIConnection, provider string) (bool, error) {
	apiKey := strings.TrimSpace(s.credentials[provider])
	if apiKey == "" || len(s.key) != 32 {
		return false, nil
	}
	if len(apiKey) > 65536 || strings.ContainsAny(apiKey, "\r\n") {
		return false, errors.New("invalid managed provider credential")
	}
	oldKey := ""
	if len(c.EncryptedSecret) > 0 {
		secret, err := (&AIConnectionService{key: s.key}).open(c)
		if err != nil {
			return false, errors.New("cannot decrypt standard managed connection")
		}
		oldKey = secret.APIKey
	}
	if oldKey == apiKey {
		return false, nil
	}
	raw, err := json.Marshal(aiConnectionSecret{APIKey: apiKey})
	if err != nil {
		return false, err
	}
	c.EncryptedSecret, err = crypto.EncryptWithAAD(raw, s.key, aiConnectionAAD(c))
	if err != nil {
		return false, err
	}
	source := model.AIConnectionCredentialSourceEnvironment
	c.CredentialSource = &source
	c.Status = "connected"
	return true, nil
}

func (s *AIStandardProfiles) WorkspaceCreated(ctx context.Context, workspace string) error {
	return s.EnsureWorkspace(ctx, workspace)
}
func (s *AIStandardProfiles) WorkspaceDeleting(context.Context, string) error { return nil }
