package service

import (
	"context"
	"strings"
	"testing"

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
	if c.Funding != "managed" || c.Status != "connected" {
		t.Fatal("empty migration placeholder was not provisioned")
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
