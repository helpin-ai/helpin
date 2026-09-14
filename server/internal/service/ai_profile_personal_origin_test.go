package service

import (
	"context"
	"encoding/json"
	"testing"

	sdk "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestPersonalProfileSharedFallbackPreservesOwnerForChildrenAndRefresh(t *testing.T) {
	profiles, primary, fallback, db := setupAIProfileTestDB(t)
	ctx := context.Background()
	personal, err := profiles.connections.Create(ctx, "workspace", "owner", model.CreateAIConnectionRequest{Name: "My key", Provider: "openai", APIKey: "personal-key"})
	if err != nil {
		t.Fatal(err)
	}
	primary.ConnectionID = personal.Connection.ID
	profile, err := profiles.Save(ctx, "workspace", "owner", "", model.SaveAIProfileRequest{Name: "Personal with shared fallback", Scope: "personal", Primary: primary, Fallback: &fallback})
	if err != nil {
		t.Fatal(err)
	}
	if err := profiles.connections.repo.WithLocked(ctx, primary.ConnectionID, func(c *model.AIConnection) error { c.Status = "disconnected"; return nil }); err != nil {
		t.Fatal(err)
	}
	selection, credential, err := profiles.Resolve(ctx, "workspace", "owner", AIProfileSelectionRequest{ProfileID: profile.ID})
	if err != nil || credential.APIKey != "Fallback-key" || selection.ConnectionScope != "workspace" {
		t.Fatalf("fallback: %+v, %v", selection, err)
	}
	if owner, personal := selection.PersonalOwner(); !personal || owner != "owner" {
		t.Fatal("lost personal origin")
	}
	input, err := json.Marshal(model.AgentRunInputPayload{AISelection: selection, ModelConnectionID: fallback.ConnectionID, ModelProvider: fallback.Model.Provider, ModelName: fallback.Model.Model})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO agent_runs (id,workspace_id,triggered_by_user_id,input,status) VALUES (?,?,?,?,?)", "parent", "workspace", "owner", []byte(input), "running").Error; err != nil {
		t.Fatal(err)
	}
	service := (&AgentService{runRepo: repository.NewAgentRunRepository(db)}).SetAIConnectionService(profiles.connections).SetAIProfileService(profiles)
	params := createRunParams{workspaceID: "workspace", parentRunID: strPtr("parent"), actorID: strPtr("owner"), input: []byte(`{}`), agent: &model.Agent{}, trigger: &model.AgentRunTriggerContext{Source: "agent"}}
	_, credential, _, err = service.prepareAIProfileRun(ctx, &params)
	if err != nil || credential.APIKey != "Fallback-key" {
		t.Fatalf("child failed to inherit shared fallback: %v", err)
	}
	var inherited model.AgentRunInputPayload
	if err := json.Unmarshal(params.input, &inherited); err != nil {
		t.Fatal(err)
	}
	if inherited.AISelection.ProfileID != profile.ID || derefString(inherited.AISelection.ProfileOwnerID) != "owner" {
		t.Fatal("child changed originating selection")
	}
	params.actorID = strPtr("teammate")
	if _, _, _, err := service.prepareAIProfileRun(ctx, &params); err == nil {
		t.Fatal("another user inherited personal profile")
	}
	if _, err := profiles.Restore(ctx, "workspace", "teammate", selection); err == nil {
		t.Fatal("another user restored personal profile")
	}
	request := sdk.ModelCredentialRefreshRequest{AppID: "helpin", HostRunID: "parent", RunID: "runtime-parent", ConnectionID: fallback.ConnectionID, Provider: "openai"}
	if _, err := profiles.connections.RefreshRun(ctx, request); err != nil {
		t.Fatalf("authorized refresh: %v", err)
	}
	if err := db.Exec("UPDATE workspace_members SET status = 'removed' WHERE user_id = 'owner'").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := profiles.Restore(ctx, "workspace", "owner", selection); err == nil {
		t.Fatal("removed member restored shared fallback")
	}
	if _, err := profiles.connections.RefreshRun(ctx, request); err == nil {
		t.Fatal("removed member refreshed shared fallback")
	}
}
