package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/gorm"
)

func bootstrapFixture(t *testing.T) (*AIProfileBootstrapService, *AIConnectionService, *gorm.DB) {
	t.Helper()
	connections, db := setupAIConnectionTest(t)
	if err := db.AutoMigrate(&model.AIProfile{}, &model.AIWorkspaceSettings{}); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY, status TEXT); INSERT INTO workspaces VALUES ('workspace','active')`,
		`CREATE TABLE agents (id TEXT PRIMARY KEY, workspace_id TEXT, name TEXT, provider TEXT, model TEXT, execution_config TEXT, ai_profile_id TEXT)`,
		`INSERT INTO agents VALUES ('agent','workspace','Reviewer','openai','private-model','{"reasoning_effort":"high","native_context":{"enabled":true},"max_tool_steps":88}',NULL)`,
		`CREATE TABLE agent_versions (id TEXT, execution_config TEXT); INSERT INTO agent_versions VALUES ('historical','{"reasoning_effort":"low"}')`,
		`INSERT INTO agent_runs(id,workspace_id,input,status) VALUES ('old-run','workspace',CAST('{}' AS BLOB),'running'),('completed','workspace',CAST('{}' AS BLOB),'completed'),('new-run','workspace',CAST('{"model_connection_id":"personal"}' AS BLOB),'running')`,
	} {
		if err := db.Exec(query).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc, err := NewAIProfileBootstrapService(repository.NewAIProfileBootstrapRepository(db), strings.Repeat("k", 32))
	if err != nil {
		t.Fatal(err)
	}
	return svc, connections, db
}

func TestBootstrapPreviewAndApplyPreserveHistoricalState(t *testing.T) {
	svc, connections, db := bootstrapFixture(t)
	opts := AIProfileBootstrapOptions{WorkspaceID: "workspace", Funding: "managed", DryRun: true,
		Credentials: map[string]string{"openai": "explicit-secret"}}
	ctx := context.Background()
	preview, err := svc.Apply(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	if preview.AgentsMigrated != 1 || preview.ProfilesCreated != 5 || len(preview.RunsUsingRuntimeDefaults) != 1 || preview.RunsUsingRuntimeDefaults[0] != "old-run" {
		t.Fatalf("preview = %+v", preview)
	}
	var count int64
	if err := db.Model(&model.AIProfile{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("preview committed profiles")
	}
	opts.DryRun = false
	result, err := svc.Apply(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.UnconfiguredProviders) != 1 || result.UnconfiguredProviders[0] != "openrouter" {
		t.Fatalf("missing keys hidden: %+v", result)
	}
	var agent model.Agent
	if err := db.First(&agent, "id = ?", "agent").Error; err != nil {
		t.Fatal(err)
	}
	if agent.AIProfileID == nil || !strings.Contains(string(agent.ExecutionConfig), `"max_tool_steps":88`) || !strings.Contains(string(agent.ExecutionConfig), `"native_context"`) {
		t.Fatal("agent migration dropped independent controls")
	}
	var profile model.AIProfile
	if err := db.First(&profile, "id = ?", *agent.AIProfileID).Error; err != nil {
		t.Fatal(err)
	}
	if profile.Primary.Model.Model != "private-model" || derefString(profile.Primary.Model.Controls.ReasoningEffort) != "high" {
		t.Fatalf("changed route: %+v", profile.Primary)
	}
	c, err := connections.repo.Get(ctx, profile.Primary.ConnectionID)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := connections.open(c)
	if err != nil || secret.APIKey != "explicit-secret" || c.Funding != "managed" {
		t.Fatal("credential was not imported with workspace encryption")
	}
	raw, _ := json.Marshal(result)
	if strings.Contains(string(raw), "explicit-secret") {
		t.Fatal("bootstrap report leaked credential")
	}
	var historical struct{ ExecutionConfig string }
	if err := db.Table("agent_versions").Take(&historical).Error; err != nil {
		t.Fatal(err)
	}
	if historical.ExecutionConfig != `{"reasoning_effort":"low"}` {
		t.Fatal("historical version changed")
	}
	var run model.AgentRun
	if err := db.First(&run, "id = ?", "old-run").Error; err != nil {
		t.Fatal(err)
	}
	if string(run.Input) != "{}" {
		t.Fatal("active run identity changed")
	}
	// The operator may rerun bootstrap after a user edits and clears defaults.
	if err := db.Model(&profile).Update("name", "User's renamed profile").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Agent{}).Where("id = ?", "agent").UpdateColumn("ai_profile_id", nil).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.AIWorkspaceSettings{}).Where("workspace_id = ?", "workspace").UpdateColumn("default_profile_id", nil).Error; err != nil {
		t.Fatal(err)
	}
	result, err = svc.Apply(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.AgentsMigrated != 0 || result.ProfilesCreated != 0 {
		t.Fatal("rerun repeated migration")
	}
	agent = model.Agent{}
	if err := db.First(&agent, "id = ?", "agent").Error; err != nil {
		t.Fatal(err)
	}
	if agent.AIProfileID != nil {
		t.Fatal("rerun erased explicit workspace inheritance")
	}
	var settings model.AIWorkspaceSettings
	if err := db.First(&settings).Error; err != nil {
		t.Fatal(err)
	}
	if settings.DefaultProfileID != nil {
		t.Fatal("rerun restored cleared workspace default")
	}
}

func TestBootstrapCredentialRotationIsExplicitAndMigrationIsAtomic(t *testing.T) {
	svc, connections, db := bootstrapFixture(t)
	opts := AIProfileBootstrapOptions{WorkspaceID: "workspace", Funding: "customer", Credentials: map[string]string{"openai": "first-key"}}
	ctx := context.Background()
	if _, err := svc.Apply(ctx, opts); err != nil {
		t.Fatal(err)
	}
	opts.Credentials["openai"] = "second-key"
	if _, err := svc.Apply(ctx, opts); err == nil {
		t.Fatal("implicit credential rotation accepted")
	}
	opts.RotateCredentials = true
	if _, err := svc.Apply(ctx, opts); err != nil {
		t.Fatal(err)
	}
	var c model.AIConnection
	if err := db.Where("provider = ?", "openai").First(&c).Error; err != nil {
		t.Fatal(err)
	}
	secret, err := connections.open(&c)
	if err != nil || secret.APIKey != "second-key" {
		t.Fatal("explicit rotation did not replace the secret")
	}
	if err := db.Exec(`INSERT INTO agents VALUES ('invalid','workspace','Invalid',NULL,NULL,'{}',NULL)`).Error; err != nil {
		t.Fatal(err)
	}
	opts.Credentials["openai"] = "third-key"
	if _, err := svc.Apply(ctx, opts); err == nil {
		t.Fatal("implicit default route migration accepted")
	}
	if err := db.Where("provider = ?", "openai").First(&c).Error; err != nil {
		t.Fatal(err)
	}
	secret, err = connections.open(&c)
	if err != nil || secret.APIKey != "second-key" {
		t.Fatal("failed migration partially rotated credentials")
	}
}
