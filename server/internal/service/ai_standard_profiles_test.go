package service

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func standardProfilesFixture(t *testing.T) (*AIStandardProfiles, *AIConnectionService, *gorm.DB) {
	t.Helper()
	connections, db := setupAIConnectionTest(t)
	if err := db.AutoMigrate(&model.AIProfile{}, &model.AIWorkspaceSettings{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE workspaces(id TEXT PRIMARY KEY); INSERT INTO workspaces VALUES ('workspace'),('other')`).Error; err != nil {
		t.Fatal(err)
	}
	profiles, err := NewAIStandardProfiles(repository.NewAIStandardProfileRepository(db), strings.Repeat("k", 32), "managed", map[string]string{"openrouter": "first-fixture-key"})
	if err != nil {
		t.Fatal(err)
	}
	return profiles, connections, db
}

func TestStandardAIProfilesProvisionAndPreserveChoices(t *testing.T) {
	s, connections, db := standardProfilesFixture(t)
	ctx := context.Background()
	if err := s.EnsureAll(ctx); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.AIProfile{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 8 {
		t.Fatalf("want four profiles per workspace, got %d", count)
	}
	c, err := connections.repo.Get(ctx, model.StandardAIConnectionID("workspace", "openrouter"))
	if err != nil {
		t.Fatal(err)
	}
	secret, err := connections.open(c)
	if err != nil || secret.APIKey != "first-fixture-key" || c.Funding != "managed" {
		t.Fatal("managed credential was not sealed for its workspace")
	}
	small := model.StandardAIProfileID("workspace", "small")
	if err := db.Model(&model.AIProfile{}).Where("id = ?", small).Updates(map[string]any{"name": "My small profile", "revision": 2}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.AIWorkspaceSettings{}).Where("workspace_id = ?", "workspace").UpdateColumn("default_profile_id", nil).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureAll(ctx); err != nil {
		t.Fatal(err)
	}
	var p model.AIProfile
	if err := db.First(&p, "id = ?", small).Error; err != nil {
		t.Fatal(err)
	}
	if p.Name != "My small profile" || p.Revision != 2 {
		t.Fatal("provisioning overwrote edited profile")
	}
	var settings model.AIWorkspaceSettings
	if err := db.First(&settings, "workspace_id = ?", "workspace").Error; err != nil {
		t.Fatal(err)
	}
	if settings.DefaultProfileID != nil {
		t.Fatal("provisioning restored a cleared default")
	}
}

func TestStandardAIProfilesUseDirectProviderCredentials(t *testing.T) {
	s, connections, db := standardProfilesFixture(t)
	s.credentials["openai"] = "openai-fixture-key"
	s.credentials["anthropic"] = "anthropic-fixture-key"
	ctx := context.Background()
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ tier, provider, model string }{
		{"large", "openai", "gpt-5.6-terra"},
		{"flagship", "anthropic", "claude-sonnet-5"},
	} {
		var profile model.AIProfile
		if err := db.First(&profile, "id = ?", model.StandardAIProfileID("workspace", test.tier)).Error; err != nil {
			t.Fatal(err)
		}
		if profile.Primary.Model.Provider != test.provider || profile.Primary.Model.Model != test.model ||
			profile.Primary.ConnectionID != model.StandardAIConnectionID("workspace", test.provider) {
			t.Fatalf("%s has the wrong default route", test.tier)
		}
		connection, err := connections.repo.Get(ctx, profile.Primary.ConnectionID)
		if err != nil || connection == nil {
			t.Fatalf("%s connection unavailable: %v", test.tier, err)
		}
		secret, err := connections.open(connection)
		if err != nil || secret.APIKey != s.credentials[test.provider] || connection.Status != "connected" {
			t.Fatalf("%s did not receive its own provider credential", test.tier)
		}
	}
}

func TestStandardAIProfilesManagedRotationAndDisconnect(t *testing.T) {
	s, connections, db := standardProfilesFixture(t)
	ctx := context.Background()
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	s.credentials["openrouter"] = "second-fixture-key"
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	id := model.StandardAIConnectionID("workspace", "openrouter")
	c, err := connections.repo.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := connections.open(c)
	if err != nil || secret.APIKey != "second-fixture-key" {
		t.Fatal("managed rotation failed")
	}
	if err := db.Model(&model.AIConnection{}).Where("id = ?", id).UpdateColumn("status", "disconnected").Error; err != nil {
		t.Fatal(err)
	}
	s.credentials["openrouter"] = "third-fixture-key"
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	c, err = connections.repo.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	secret, err = connections.open(c)
	if err != nil || secret.APIKey != "second-fixture-key" || c.Status != "disconnected" {
		t.Fatal("provisioning reconnected a revoked connection")
	}
}

func TestStandardAIProfilesDoNotAdoptCustomerCredentials(t *testing.T) {
	s, connections, _ := standardProfilesFixture(t)
	ctx := context.Background()
	s.funding = "customer"
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	s.funding = "managed"
	s.credentials["openrouter"] = "platform-fixture-key"
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	c, err := connections.repo.Get(ctx, model.StandardAIConnectionID("workspace", "openrouter"))
	if err != nil {
		t.Fatal(err)
	}
	secret, err := connections.open(c)
	if err != nil || secret.APIKey != "first-fixture-key" || c.Funding != "customer" {
		t.Fatal("customer credential was adopted by platform")
	}
}

func TestStandardAIProfilesAdoptOnlyEmptyPlaceholder(t *testing.T) {
	s, connections, _ := standardProfilesFixture(t)
	ctx := context.Background()
	platformKey := s.credentials["openrouter"]
	s.funding = "customer"
	s.credentials = nil
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	s.funding = "managed"
	s.credentials = map[string]string{"openrouter": platformKey}
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	c, err := connections.repo.Get(ctx, model.StandardAIConnectionID("workspace", "openrouter"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Funding != "managed" || c.Status != "connected" || c.Name != "openrouter (managed)" {
		t.Fatal("empty migration placeholder was not provisioned")
	}
}

// customerStandardProfilesFixture mirrors Community: customer funding with
// environment provider keys sealed into the standard connections.
func customerStandardProfilesFixture(t *testing.T, credentials map[string]string) (*AIStandardProfiles, *AIConnectionService, *gorm.DB) {
	t.Helper()
	s, connections, db := standardProfilesFixture(t)
	s.funding = "customer"
	s.credentials = credentials
	connections.SetAuthorizationService(authorization.NewAuthzService(db, connectionMembers{}, nil))
	return s, connections, db
}

type standardRouteWant struct{ provider, model string }

func assertStandardProfile(t *testing.T, db *gorm.DB, tier string, want standardRouteWant, connectionID string) {
	t.Helper()
	var p model.AIProfile
	if err := db.First(&p, "id = ?", model.StandardAIProfileID("workspace", tier)).Error; err != nil {
		t.Fatal(err)
	}
	if p.Primary.Model.Provider != want.provider || p.Primary.Model.Model != want.model || p.Primary.ConnectionID != connectionID {
		t.Fatalf("%s = %s/%s on %s, want %s/%s on %s", tier, p.Primary.Model.Provider, p.Primary.Model.Model,
			p.Primary.ConnectionID, want.provider, want.model, connectionID)
	}
}

func workspaceDefault(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var settings model.AIWorkspaceSettings
	if err := db.First(&settings, "workspace_id = ?", "workspace").Error; err != nil {
		t.Fatal(err)
	}
	return derefString(settings.DefaultProfileID)
}

func TestStandardProfilesFollowConfiguredProviders(t *testing.T) {
	fixedLarge := standardRouteWant{"openai", "gpt-5.6-terra"}
	fixedFlagship := standardRouteWant{"anthropic", "claude-sonnet-5"}
	tests := []struct {
		name        string
		credentials map[string]string
		want        map[string]standardRouteWant
	}{
		{name: "only anthropic", credentials: map[string]string{"anthropic": "anthropic-key"}, want: map[string]standardRouteWant{
			"small": {"anthropic", "claude-haiku-4-5"}, "medium": {"anthropic", "claude-haiku-4-5"},
			"large": {"anthropic", "claude-haiku-4-5"}, "flagship": fixedFlagship,
		}},
		{name: "only openai", credentials: map[string]string{"openai": "openai-key"}, want: map[string]standardRouteWant{
			"small": {"openai", "gpt-5.6-luna"}, "medium": {"openai", "gpt-5-mini"},
			"large": fixedLarge, "flagship": {"openai", "gpt-5.5"},
		}},
		{name: "only openrouter", credentials: map[string]string{"openrouter": "openrouter-key"}, want: map[string]standardRouteWant{
			"small": {"openrouter", "deepseek/deepseek-v4.1-flash:nitro"}, "medium": {"openrouter", "google/gemini-3.8-flash"},
			"large": {"openrouter", "openai/gpt-5.6-terra"}, "flagship": {"openrouter", "anthropic/claude-sonnet-5"},
		}},
		{name: "openai and anthropic", credentials: map[string]string{"openai": "openai-key", "anthropic": "anthropic-key"}, want: map[string]standardRouteWant{
			"small": {"openai", "gpt-5.6-luna"}, "medium": {"openai", "gpt-5-mini"},
			"large": fixedLarge, "flagship": fixedFlagship,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, connections, db := customerStandardProfilesFixture(t, tt.credentials)
			ctx := context.Background()
			if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
				t.Fatal(err)
			}
			for tier, want := range tt.want {
				connectionID := model.StandardAIConnectionID("workspace", want.provider)
				assertStandardProfile(t, db, tier, want, connectionID)
				c, err := connections.repo.Get(ctx, connectionID)
				if err != nil || !usableStandardConnection(c) {
					t.Fatalf("%s runs on an unusable connection: %v", tier, err)
				}
			}
			if workspaceDefault(t, db) != model.StandardAIProfileID("workspace", "small") {
				t.Fatal("runnable Small was not kept as the default")
			}
		})
	}
}

func TestStandardProfilesRemapWhenSharedConnectionsChange(t *testing.T) {
	s, connections, db := customerStandardProfilesFixture(t, nil)
	connections.SetChangeObserver(s)
	ctx := context.Background()
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	fixedSmall := standardRouteWant{"openrouter", "deepseek/deepseek-v4.1-flash:nitro"}
	assertStandardProfile(t, db, "small", fixedSmall, model.StandardAIConnectionID("workspace", "openrouter"))

	login, err := connections.Create(ctx, "workspace", "owner", model.CreateAIConnectionRequest{Name: "Team OpenAI", Scope: "workspace", Provider: "openai", APIKey: "team-key"})
	if err != nil {
		t.Fatal(err)
	}
	id := login.Connection.ID
	assertStandardProfile(t, db, "small", standardRouteWant{"openai", "gpt-5.6-luna"}, id)
	assertStandardProfile(t, db, "large", standardRouteWant{"openai", "gpt-5.6-terra"}, id)
	assertStandardProfile(t, db, "flagship", standardRouteWant{"openai", "gpt-5.5"}, id)
	var p model.AIProfile
	if err := db.First(&p, "id = ?", model.StandardAIProfileID("workspace", "small")).Error; err != nil || p.Revision != 1 {
		t.Fatalf("re-mapping changed the revision: %d, %v", p.Revision, err)
	}

	if err := connections.Disconnect(ctx, "workspace", "owner", id); err != nil {
		t.Fatal(err)
	}
	assertStandardProfile(t, db, "small", fixedSmall, model.StandardAIConnectionID("workspace", "openrouter"))
	assertStandardProfile(t, db, "large", standardRouteWant{"openai", "gpt-5.6-terra"}, model.StandardAIConnectionID("workspace", "openai"))

	if _, err := connections.Create(ctx, "workspace", "owner", model.CreateAIConnectionRequest{Name: "Mine", Provider: "anthropic", APIKey: "personal-key"}); err != nil {
		t.Fatal(err)
	}
	assertStandardProfile(t, db, "small", fixedSmall, model.StandardAIConnectionID("workspace", "openrouter"))
}

func TestStandardProfilesNeverOverwriteEditedProfiles(t *testing.T) {
	s, _, db := customerStandardProfilesFixture(t, nil)
	ctx := context.Background()
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	large := model.StandardAIProfileID("workspace", "large")
	if err := db.Model(&model.AIProfile{}).Where("id = ?", large).UpdateColumn("revision", 2).Error; err != nil {
		t.Fatal(err)
	}
	s.credentials = map[string]string{"anthropic": "anthropic-key"}
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	assertStandardProfile(t, db, "large", standardRouteWant{"openai", "gpt-5.6-terra"}, model.StandardAIConnectionID("workspace", "openai"))
	assertStandardProfile(t, db, "small", standardRouteWant{"anthropic", "claude-haiku-4-5"}, model.StandardAIConnectionID("workspace", "anthropic"))
}

func TestStandardProfilesRepointDefaultOnlyFromUnrunnableSmall(t *testing.T) {
	s, _, db := customerStandardProfilesFixture(t, nil)
	ctx := context.Background()
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	small := model.StandardAIProfileID("workspace", "small")
	if workspaceDefault(t, db) != small {
		t.Fatal("Small is not the initial default")
	}
	// A user-edited Small stays on the unconfigured OpenRouter placeholder.
	if err := db.Model(&model.AIProfile{}).Where("id = ?", small).UpdateColumn("revision", 2).Error; err != nil {
		t.Fatal(err)
	}
	s.credentials = map[string]string{"anthropic": "anthropic-key"}
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	if got := workspaceDefault(t, db); got != model.StandardAIProfileID("workspace", "medium") {
		t.Fatalf("default = %s, want the runnable Medium profile", got)
	}

	flagship := model.StandardAIProfileID("workspace", "flagship")
	if err := db.Model(&model.AIWorkspaceSettings{}).Where("workspace_id = ?", "workspace").UpdateColumn("default_profile_id", flagship).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	if workspaceDefault(t, db) != flagship {
		t.Fatal("a user-selected default was moved")
	}
}

func TestStandardProfilesKeepDefaultWhenNothingRuns(t *testing.T) {
	s, _, db := customerStandardProfilesFixture(t, nil)
	if err := s.EnsureWorkspace(context.Background(), "workspace"); err != nil {
		t.Fatal(err)
	}
	if workspaceDefault(t, db) != model.StandardAIProfileID("workspace", "small") {
		t.Fatal("default moved although no profile can run")
	}
}

func TestStandardProfilesRotateOnlyEnvironmentCredentials(t *testing.T) {
	s, connections, db := customerStandardProfilesFixture(t, map[string]string{"openai": "first-env-key"})
	connections.SetChangeObserver(s)
	ctx := context.Background()
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	id := model.StandardAIConnectionID("workspace", "openai")
	storedKey := func() (string, *model.AIConnection) {
		t.Helper()
		c, err := connections.repo.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		secret, err := connections.open(c)
		if err != nil {
			t.Fatal(err)
		}
		return secret.APIKey, c
	}
	key, c := storedKey()
	if key != "first-env-key" || derefString(c.CredentialSource) != model.AIConnectionCredentialSourceEnvironment {
		t.Fatal("environment key was not sealed with its source")
	}
	s.credentials["openai"] = "second-env-key"
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	if key, _ = storedKey(); key != "second-env-key" {
		t.Fatal("rotated environment key did not reach the workspace")
	}
	if _, err := connections.Reconnect(ctx, "workspace", "owner", id, "user-key"); err != nil {
		t.Fatal(err)
	}
	s.credentials["openai"] = "third-env-key"
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	key, c = storedKey()
	if key != "user-key" || c.CredentialSource != nil {
		t.Fatal("environment rotation overwrote a user-supplied key")
	}
	// Legacy rows have no recorded source and are never rotated.
	if err := db.Model(&model.AIConnection{}).Where("id = ?", id).UpdateColumn("credential_source", nil).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureWorkspace(ctx, "workspace"); err != nil {
		t.Fatal(err)
	}
	if key, _ = storedKey(); key != "user-key" {
		t.Fatal("legacy connection adopted an environment key")
	}
}

func TestAgentCreationUsesConnectedProviderRoute(t *testing.T) {
	db := newAgentServiceTestDB(t)
	if err := db.AutoMigrate(&model.AIConnection{}, &model.AIProfile{}, &model.AIWorkspaceSettings{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE workspaces(id TEXT PRIMARY KEY); INSERT INTO workspaces VALUES ('ws-test')`).Error; err != nil {
		t.Fatal(err)
	}
	connections, err := NewAIConnectionService(repository.NewAIConnectionRepository(db), nil, &AgentRuntimeClient{}, AIConnectionConfig{EncryptionKey: strings.Repeat("k", 32)})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	shared := &model.AIConnection{ID: "shared-openai", WorkspaceID: "ws-test", Scope: "workspace", Funding: "customer", Provider: "openai", Name: "Team OpenAI", Status: "connected"}
	if err := connections.seal(shared, aiConnectionSecret{APIKey: "team-key"}); err != nil {
		t.Fatal(err)
	}
	if err := connections.repo.Create(ctx, shared); err != nil {
		t.Fatal(err)
	}
	svc := (&AgentService{agentRepo: repository.NewAgentRepository(db)}).SetAIProfileService(NewAIProfileService(repository.NewAIProfileRepository(db), connections))
	agent, err := svc.CreateAgent(ctx, modelCreateAgentRequest(nil), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if derefString(agent.Provider) != "openai" || derefString(agent.Model) != "gpt-5.6-luna" {
		t.Fatalf("agent route = %s/%s, want openai/gpt-5.6-luna", derefString(agent.Provider), derefString(agent.Model))
	}
	var p model.AIProfile
	if err := db.First(&p, "id = ?", model.StandardAIProfileID("ws-test", "small")).Error; err != nil || p.Primary.ConnectionID != shared.ID {
		t.Fatalf("Small does not run on the shared connection: %v", err)
	}
}

func TestAgentCreationAssignsStandardProfiles(t *testing.T) {
	db := newAgentServiceTestDB(t)
	if err := db.AutoMigrate(&model.AIConnection{}, &model.AIProfile{}, &model.AIWorkspaceSettings{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE workspaces(id TEXT PRIMARY KEY); INSERT INTO workspaces VALUES ('ws-test')`).Error; err != nil {
		t.Fatal(err)
	}
	connections, err := NewAIConnectionService(repository.NewAIConnectionRepository(db), nil, &AgentRuntimeClient{}, AIConnectionConfig{EncryptionKey: strings.Repeat("k", 32)})
	if err != nil {
		t.Fatal(err)
	}
	svc := (&AgentService{agentRepo: repository.NewAgentRepository(db)}).SetAIProfileService(NewAIProfileService(repository.NewAIProfileRepository(db), connections))
	ctx := context.Background()
	custom, err := svc.CreateAgent(ctx, modelCreateAgentRequest(nil), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if derefString(custom.AIProfileID) != model.StandardAIProfileID("ws-test", "small") || custom.ModelTier != "small" {
		t.Fatal("custom agent did not start on Small")
	}
	for _, preset := range ListAgentPresets() {
		agent, err := svc.ensureBuiltInAgent(ctx, "ws-test", "user-1", preset.Key)
		if err != nil {
			t.Fatal(err)
		}
		if derefString(agent.AIProfileID) != model.StandardAIProfileID("ws-test", preset.ModelTier) {
			t.Fatalf("wrong profile for preset %s", preset.Key)
		}
		if preset.Key == model.AgentPresetAskAgent && derefString(agent.AIProfileID) != model.StandardAIProfileID("ws-test", "small") {
			t.Fatal("Ask Agent must default to Small")
		}
	}
	medium := model.StandardAIProfileID("ws-test", "medium")
	req := modelCreateAgentRequest(nil)
	req.Name = "Explicit profile"
	req.AIProfileID = &medium
	explicit, err := svc.CreateAgent(ctx, req, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if derefString(explicit.AIProfileID) != medium {
		t.Fatal("explicit profile was replaced by Small")
	}
	// Provisioning on restart must not change agents that later inherit the default.
	if err := db.Model(&model.Agent{}).Where("id = ?", custom.ID).UpdateColumn("ai_profile_id", nil).Error; err != nil {
		t.Fatal(err)
	}
	standard, err := NewAIStandardProfiles(repository.NewAIStandardProfileRepository(db), "", "customer", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := standard.EnsureAll(ctx); err != nil {
		t.Fatal(err)
	}
	var saved model.Agent
	if err := db.First(&saved, "id = ?", custom.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.AIProfileID != nil {
		t.Fatal("startup reassigned an agent")
	}
}
