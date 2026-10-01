package templates

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestTemplatePreviewDoesNotCreateAgentOrFlow(t *testing.T) {
	db := setupInstallerTestDB(t)
	installer := NewInstaller(db, mustTestRegistry(t))
	var before int64
	if err := db.Model(&model.Agent{}).Count(&before).Error; err != nil {
		t.Fatal(err)
	}
	preview, err := installer.Preview(context.Background(), InstallRequest{WorkspaceID: "ws-1", TemplateKey: "release_notes_writer", ActorID: "user-1", Inputs: map[string]any{"repository_id": "repo-1", "destination_space_id": "space-1", "destination_collection_id": "collection-1", "include_prerelease": true}})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Agent == nil || preview.Rule == nil {
		t.Fatal("incomplete preview")
	}
	var agents, flows int64
	db.Model(&model.Agent{}).Count(&agents)
	db.Model(&model.AutomationRule{}).Count(&flows)
	if agents != before || flows != 0 {
		t.Fatalf("preview wrote agents=%d flows=%d", agents-before, flows)
	}
}

func TestTemplateInstallPausedMatchesPreview(t *testing.T) {
	db := setupInstallerTestDB(t)
	installer := NewInstaller(db, mustTestRegistry(t))
	req := InstallRequest{WorkspaceID: "ws-1", TemplateKey: "release_notes_writer", ActorID: "user-1", RuleID: "approved-flow", Paused: true, Inputs: map[string]any{"repository_id": "repo-1", "destination_space_id": "space-1", "destination_collection_id": "collection-1", "include_prerelease": true}}
	preview, err := installer.Preview(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := installer.Install(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	var rule model.AutomationRule
	if err := db.First(&rule, "id = ?", saved.Rule.ID).Error; err != nil {
		t.Fatal(err)
	}
	if preview.Rule.Enabled || saved.Rule.Enabled || rule.Enabled || rule.ID != "approved-flow" {
		t.Fatal("installation did not preserve approved identity and paused state")
	}
	if rule.Name != preview.Rule.Name || string(rule.TriggerConfig) != string(preview.Rule.TriggerConfig) {
		t.Fatal("installation differs from preview")
	}
}
