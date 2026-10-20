package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/agentskills"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSkillEditRuntimeAndRestore(t *testing.T) {
	svc, repo, store := newWorkspaceSkillTestService(t)
	ctx := context.Background()
	key := "support_triage_response"
	updated, err := svc.UpdateWorkspaceSkill(ctx, "ws-1", key, model.UpdateWorkspaceSkillRequest{Instructions: stringPtr("Workspace-specific guidance")})
	if err != nil {
		t.Fatal(err)
	}
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).SetWorkspaceSkillStore(repo, store)
	for _, id := range []string{"", agentRuntimeHelpinBuiltInSkillIDPrefix + key, updated.ID} {
		req := AgentRuntimeSkillLookupRequest{AppID: "helpin", Key: key, SkillID: id, Metadata: map[string]interface{}{"workspace_id": "ws-1"}}
		var result *AgentRuntimeWorkspaceSkill
		if id == "" {
			result, err = host.ResolveActiveSkillByKey(ctx, req)
		} else {
			result, err = host.ResolveSkillByID(ctx, req)
		}
		if err != nil {
			t.Fatal(err)
		}
		if result.Instructions != updated.Instructions {
			t.Fatalf("runtime %q ignored edit", id)
		}
	}
	agent := &model.Agent{WorkspaceID: "ws-1", IsSystem: true, PresetKey: model.AgentPresetSupportAgent, RuntimeKind: "native_sdk", Skills: model.AgentSkillRefs{{Key: key}}}
	dest := filepath.Join(t.TempDir(), "skills")
	resolution, err := agentskills.StageInto(ctx, "ws-1", agent, nil, repo, store, dest)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(dest, "01-"+key, "SKILL.md"))
	if err != nil || !strings.Contains(string(payload), updated.Instructions) {
		t.Fatalf("staged skill missing edit: %v", err)
	}
	agent.Skills = resolution.Refs // Saved refs must keep working after edit/restore.
	if _, err := agentskills.StageInto(ctx, "ws-1", agent, nil, repo, store, dest); err != nil {
		t.Fatal("reloading system agent override:", err)
	}
	if _, err := svc.UpdateWorkspaceSkill(ctx, "ws-1", updated.ID, model.UpdateWorkspaceSkillRequest{RestoreDefault: true}); err != nil {
		t.Fatal(err)
	}
	resolution, err = agentskills.Resolve(ctx, "ws-1", agent.Skills, repo)
	if err != nil {
		t.Fatal(err)
	}
	original, _ := agentcontract.GetBuiltInSkill(key)
	if resolution.Definitions[0].Instructions != original.Instructions {
		t.Fatal("saved ref did not restore default")
	}
	if resolution.Refs[0].SkillID != nil {
		t.Fatal("restored built-in should follow future shipped defaults")
	}
}

func TestSkillCommandAuthorizationAndEdit(t *testing.T) {
	agentSvc, _, _ := newWorkspaceSkillTestService(t)
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.agentService = agentSvc
	svc.authz = authorization.NewAuthzService(nil, nil, nil)
	meta := model.InternalCommandContext{WorkspaceID: "ws-1", ActorID: "user-1", ActorRole: "viewer"}
	if _, err := svc.Execute(context.Background(), meta, "skills.update", json.RawMessage(`{"skill_id":"support_triage_response","instructions":"Bad edit","expected_version":"old"}`)); err == nil {
		t.Fatal("viewer edited skill")
	}
	meta.ActorRole = "member"
	read, err := svc.Execute(context.Background(), meta, "skills.list_workspace", json.RawMessage(`{"key":"support_triage_response"}`))
	if err != nil {
		t.Fatal(err)
	}
	var skill model.SkillCatalogEntry
	if err := json.Unmarshal(read, &skill); err != nil {
		t.Fatal(err)
	}
	request, _ := json.Marshal(map[string]any{"skill_id": skill.Key, "instructions": "Updated via Ask", "expected_version": skill.VersionKey})
	updated, err := svc.Execute(context.Background(), meta, "skills.update", request)
	if err != nil {
		t.Fatal(err)
	}
	var result model.WorkspaceSkillResponse
	if err := json.Unmarshal(updated, &result); err != nil {
		t.Fatal(err)
	}
	readByID, err := json.Marshal(map[string]string{"skill_id": result.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Execute(context.Background(), meta, "skills.list_workspace", readByID); err != nil {
		t.Fatal("cannot read edited built-in by returned ID:", err)
	}
	if _, err := svc.Execute(context.Background(), meta, "skills.update", request); err == nil {
		t.Fatal("stale edit overwrote skill")
	}
	meta.ActorRole = ""
	if _, err := svc.Execute(context.Background(), meta, "skills.update", request); err == nil {
		t.Fatal("unattributed actor edited skill")
	}
	if !containsString(askAgentPresetTools(), "update_skill") || !containsString(askAgentPresetTools(), "list_workspace_skills") {
		t.Fatal("Ask lacks skill tools")
	}
}

func TestCustomSkillEditRefreshesSavedVersion(t *testing.T) {
	svc, repo, _ := newWorkspaceSkillTestService(t)
	ctx := context.Background()
	created, err := svc.CreateWorkspaceSkill(ctx, "ws-1", "user-1", model.CreateWorkspaceSkillRequest{Key: "custom", Description: "Custom guidance", Instructions: "Original"})
	if err != nil {
		t.Fatal(err)
	}
	refs := model.AgentSkillRefs{{Key: created.Key, SkillID: &created.ID, VersionKey: &created.VersionKey}}
	if _, err := svc.UpdateWorkspaceSkill(ctx, "ws-1", created.ID, model.UpdateWorkspaceSkillRequest{Instructions: stringPtr("Updated")}); err != nil {
		t.Fatal(err)
	}
	resolved, err := agentskills.Resolve(ctx, "ws-1", refs, repo)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Definitions[0].Instructions != "Updated" || *resolved.Refs[0].VersionKey == created.VersionKey {
		t.Fatal("saved skill ref did not advance")
	}
}
