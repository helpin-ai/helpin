package service

import (
	"context"
	"errors"
	"testing"
	"time"

	agentruntime "github.com/helpin-ai/agent-runtime-go"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestAgentRuntimeHostResolveActiveSkillByKey(t *testing.T) {
	db := newWorkspaceSkillTestDB(t)
	repo := repository.NewWorkspaceSkillRepository(db)
	store := &fakeSkillPackageStore{objects: make(map[string][]byte)}
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		SetWorkspaceSkillStore(repo, store)
	now := time.Now().UTC()
	skill := &model.WorkspaceSkill{
		ID:                "skill-1",
		WorkspaceID:       "ws-1",
		SourceKind:        model.WorkspaceSkillSourceWorkspace,
		Key:               "workspace_skill",
		VersionKey:        "v1",
		Title:             "Workspace Skill",
		Description:       stringPtr("Workspace-specific instructions."),
		Instructions:      "Follow workspace guidance.",
		RequiredTools:     model.JSONBlob(`["update_plan"]`),
		SupportedRuntimes: model.JSONBlob(`["native_sdk","codex"]`),
		InterfaceConfig:   model.JSONBlob(`{"interaction":"none"}`),
		PolicyConfig:      model.JSONBlob(`{"approval":"never"}`),
		PackageObjectKey:  "workspaces/ws-1/skills/skill-1/workspace_skill.zip",
		PackageFileName:   "workspace_skill.zip",
		PackageChecksum:   "checksum",
		PackageSize:       123,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := repo.Create(context.Background(), skill); err != nil {
		t.Fatalf("seed workspace skill: %v", err)
	}

	resolved, err := host.ResolveActiveSkillByKey(context.Background(), AgentRuntimeSkillLookupRequest{
		AppID: "helpin",
		RunID: "run-runtime-1",
		Target: agentruntime.TargetRef{
			Type:     "workspace",
			ID:       "ws-1",
			Metadata: map[string]interface{}{"workspace_id": "ws-1"},
		},
		Metadata: map[string]interface{}{"workspace_id": "ws-1"},
		Key:      "workspace_skill",
	})
	if err != nil {
		t.Fatalf("ResolveActiveSkillByKey returned error: %v", err)
	}
	if resolved.ID != "skill-1" || resolved.Key != "workspace_skill" || resolved.PackageObjectKey != skill.PackageObjectKey {
		t.Fatalf("unexpected resolved skill: %#v", resolved)
	}
	if len(resolved.RequiredTools) != 1 || resolved.RequiredTools[0] != "update_plan" {
		t.Fatalf("unexpected required tools: %#v", resolved.RequiredTools)
	}
	if string(resolved.Interface) != `{"interaction":"none"}` || string(resolved.Policy) != `{"approval":"never"}` {
		t.Fatalf("unexpected interface/policy: interface=%s policy=%s", resolved.Interface, resolved.Policy)
	}
}

func TestAgentRuntimeHostSkillLookupRejectsWrongApp(t *testing.T) {
	db := newWorkspaceSkillTestDB(t)
	repo := repository.NewWorkspaceSkillRepository(db)
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		SetWorkspaceSkillStore(repo, &fakeSkillPackageStore{})

	_, err := host.ResolveSkillByID(context.Background(), AgentRuntimeSkillLookupRequest{
		AppID:   "other-app",
		SkillID: "skill-1",
		Metadata: map[string]interface{}{
			"workspace_id": "ws-1",
		},
	})
	if !errors.Is(err, ErrAgentRuntimeHostForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestAgentRuntimeHostResolveHelpinBuiltInSkillByKey(t *testing.T) {
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resolved, err := host.ResolveActiveSkillByKey(context.Background(), AgentRuntimeSkillLookupRequest{
		AppID: "helpin",
		Key:   "marketing_context_setup",
	})
	if err != nil {
		t.Fatalf("ResolveActiveSkillByKey returned error: %v", err)
	}
	if resolved.ID != agentRuntimeHelpinBuiltInSkillIDPrefix+"marketing_context_setup" || resolved.SourceKind != model.WorkspaceSkillSourceBuiltIn {
		t.Fatalf("unexpected built-in skill identity: %#v", resolved)
	}
	if resolved.PackageObjectKey != agentRuntimeHelpinBuiltInSkillObjectPrefix+"marketing_context_setup.zip" || resolved.PackageChecksum == "" || resolved.PackageSize == 0 {
		t.Fatalf("expected built-in package metadata, got %#v", resolved)
	}
	payload, err := host.GetSkillPackageObject(context.Background(), resolved.PackageObjectKey)
	if err != nil {
		t.Fatalf("GetSkillPackageObject returned error: %v", err)
	}
	if len(payload) == 0 {
		t.Fatal("expected built-in skill archive payload")
	}
	byID, err := host.ResolveSkillByID(context.Background(), AgentRuntimeSkillLookupRequest{
		AppID:   "helpin",
		SkillID: resolved.ID,
	})
	if err != nil {
		t.Fatalf("ResolveSkillByID returned error: %v", err)
	}
	if byID.Key != resolved.Key || byID.PackageChecksum != resolved.PackageChecksum {
		t.Fatalf("unexpected by-id built-in skill: %#v", byID)
	}
}

func TestAgentRuntimeHostGetSkillPackageObjectVerifiesWorkspaceSkillObjectKey(t *testing.T) {
	db := newWorkspaceSkillTestDB(t)
	repo := repository.NewWorkspaceSkillRepository(db)
	objectKey := "workspaces/ws-1/skills/skill-1/workspace_skill.zip"
	store := &fakeSkillPackageStore{objects: map[string][]byte{objectKey: []byte("zip-bytes")}}
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		SetWorkspaceSkillStore(repo, store)
	now := time.Now().UTC()
	if err := repo.Create(context.Background(), &model.WorkspaceSkill{
		ID:                "skill-1",
		WorkspaceID:       "ws-1",
		SourceKind:        model.WorkspaceSkillSourceWorkspace,
		Key:               "workspace_skill",
		VersionKey:        "v1",
		Title:             "Workspace Skill",
		Instructions:      "Follow workspace guidance.",
		RequiredTools:     model.JSONBlob(`[]`),
		SupportedRuntimes: model.JSONBlob(`[]`),
		InterfaceConfig:   model.JSONBlob(`{}`),
		PolicyConfig:      model.JSONBlob(`{}`),
		PackageObjectKey:  objectKey,
		PackageFileName:   "workspace_skill.zip",
		PackageChecksum:   "checksum",
		PackageSize:       9,
		CreatedAt:         now,
		UpdatedAt:         now,
	}); err != nil {
		t.Fatalf("seed workspace skill: %v", err)
	}

	payload, err := host.GetSkillPackageObject(context.Background(), objectKey)
	if err != nil {
		t.Fatalf("GetSkillPackageObject returned error: %v", err)
	}
	if string(payload) != "zip-bytes" {
		t.Fatalf("unexpected package payload: %q", string(payload))
	}
	_, err = host.GetSkillPackageObject(context.Background(), "workspaces/ws-1/skills/missing.zip")
	if !errors.Is(err, ErrAgentRuntimeHostNotFound) {
		t.Fatalf("expected not found for unregistered object key, got %v", err)
	}
}
