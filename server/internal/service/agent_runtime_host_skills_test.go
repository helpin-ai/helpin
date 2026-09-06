package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	agentruntime "github.com/helpin-ai/agent-runtime-go"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestAgentRuntimeHostCachedBuiltInSkillContracts(t *testing.T) {
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	uncached := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	uncached.builtInArchives = nil
	builds := 0
	host.builtInArchives = newRuntimeBuiltInSkillArchiveCache(func(def agentcontract.SkillDefinition) ([]byte, string, string, error) {
		builds++
		return agentcontract.BuildSkillArchive(def)
	})
	definitions := agentcontract.ListBuiltInSkills()
	for _, definition := range definitions {
		t.Run(definition.Key, func(t *testing.T) {
			req := AgentRuntimeSkillLookupRequest{AppID: "helpin", Key: definition.Key}
			want, err := uncached.ResolveActiveSkillByKey(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			wantBytes, err := uncached.GetSkillPackageObject(context.Background(), want.PackageObjectKey)
			if err != nil {
				t.Fatal(err)
			}
			for turn := 0; turn < 2; turn++ {
				byKey, err := host.ResolveActiveSkillByKey(context.Background(), req)
				if err != nil {
					t.Fatal(err)
				}
				byID, err := host.ResolveSkillByID(context.Background(), AgentRuntimeSkillLookupRequest{
					AppID: "helpin", SkillID: byKey.ID,
				})
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(byKey, want) || !reflect.DeepEqual(byID, want) {
					t.Errorf("turn %d changed runtime metadata, instructions, tools or policy", turn)
				}
				payload, err := host.GetSkillPackageObject(context.Background(), byKey.PackageObjectKey)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(payload, wantBytes) {
					t.Errorf("turn %d changed runtime package bytes", turn)
				}
				payload[0] ^= 0xff
			}
		})
	}
	if builds != len(definitions) {
		t.Errorf("built %d packages across repeated lookups; want %d", builds, len(definitions))
	}
}

func TestAgentRuntimeHostWarmSkillCacheStillRejectsWrongApp(t *testing.T) {
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	resolved, err := host.ResolveActiveSkillByKey(context.Background(), AgentRuntimeSkillLookupRequest{
		AppID: "helpin", Key: "marketing_context_setup",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = host.ResolveActiveSkillByKey(context.Background(), AgentRuntimeSkillLookupRequest{
		AppID: "other-app", Key: resolved.Key,
	})
	if !errors.Is(err, ErrAgentRuntimeHostForbidden) {
		t.Errorf("warm by-key cache bypassed app validation: %v", err)
	}
	_, err = host.ResolveSkillByID(context.Background(), AgentRuntimeSkillLookupRequest{
		AppID: "other-app", SkillID: resolved.ID,
	})
	if !errors.Is(err, ErrAgentRuntimeHostForbidden) {
		t.Errorf("warm by-ID cache bypassed app validation: %v", err)
	}
}

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

func TestAgentRuntimeHostResolveScribeApprovalSkillIncludesCompletionPolicy(t *testing.T) {
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resolved, err := host.ResolveActiveSkillByKey(context.Background(), AgentRuntimeSkillLookupRequest{
		AppID: "helpin",
		Key:   "prd_task_plan_approval",
	})
	if err != nil {
		t.Fatalf("ResolveActiveSkillByKey returned error: %v", err)
	}
	var policy struct {
		CompletionRequiresInteractionKinds []string `json:"completion_requires_interaction_kinds"`
	}
	if err := json.Unmarshal(resolved.Policy, &policy); err != nil {
		t.Fatalf("decode approval skill policy: %v", err)
	}
	if !containsString(policy.CompletionRequiresInteractionKinds, "approval_request") {
		t.Fatalf("expected approval_request completion policy, got %s", resolved.Policy)
	}
	if !containsString(resolved.RequiredTools, "request_approval") {
		t.Fatalf("expected request_approval required tool, got %#v", resolved.RequiredTools)
	}
}

func TestAgentRuntimeHostBuiltInSkillKeyIgnoresStalePersistedPackage(t *testing.T) {
	db := newWorkspaceSkillTestDB(t)
	repo := repository.NewWorkspaceSkillRepository(db)
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).
		SetWorkspaceSkillStore(repo, &fakeSkillPackageStore{})
	now := time.Now().UTC()
	stale := &model.WorkspaceSkill{
		ID:                "stale-approval-skill",
		WorkspaceID:       "ws-1",
		SourceKind:        model.WorkspaceSkillSourceBuiltIn,
		Key:               "prd_task_plan_approval",
		VersionKey:        "old-version",
		Title:             "Old approval protocol",
		Description:       stringPtr("Persisted before completion interactions were added."),
		Instructions:      "Ask for approval in prose.",
		RequiredTools:     model.JSONBlob(`[]`),
		SupportedRuntimes: model.JSONBlob(`["native_sdk","codex"]`),
		InterfaceConfig:   model.JSONBlob(`{}`),
		PolicyConfig:      model.JSONBlob(`{}`),
		PackageObjectKey:  "workspaces/ws-1/skills/stale-approval-skill/approval.zip",
		PackageFileName:   "approval.zip",
		PackageChecksum:   "old-checksum",
		PackageSize:       1,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := repo.Create(context.Background(), stale); err != nil {
		t.Fatalf("seed stale built-in skill: %v", err)
	}

	resolved, err := host.ResolveActiveSkillByKey(context.Background(), AgentRuntimeSkillLookupRequest{
		AppID:    "helpin",
		Key:      "prd_task_plan_approval",
		Metadata: map[string]interface{}{"workspace_id": "ws-1"},
	})
	if err != nil {
		t.Fatalf("ResolveActiveSkillByKey returned error: %v", err)
	}
	if resolved.ID != agentRuntimeHelpinBuiltInSkillIDPrefix+"prd_task_plan_approval" {
		t.Fatalf("expected current product-owned built-in, got %#v", resolved)
	}
	if !containsString(resolved.RequiredTools, "request_approval") {
		t.Fatalf("expected current approval tool contract, got %#v", resolved.RequiredTools)
	}
	var policy struct {
		CompletionRequiresInteractionKinds []string `json:"completion_requires_interaction_kinds"`
	}
	if err := json.Unmarshal(resolved.Policy, &policy); err != nil {
		t.Fatalf("decode current approval policy: %v", err)
	}
	if !containsString(policy.CompletionRequiresInteractionKinds, "approval_request") {
		t.Fatalf("expected current approval completion policy, got %s", resolved.Policy)
	}
}

func TestRuntimeSkillRefsFromHelpinAllowsNativeAuthoredSkillsOnCodex(t *testing.T) {
	refs := model.AgentSkillRefs{{Key: "marketing_context_setup"}}
	if got := runtimeSkillRefsFromHelpin(refs, "native_sdk"); len(got) != 1 || got[0].Key != "marketing_context_setup" {
		t.Fatalf("expected native runtime to keep marketing_context_setup, got %#v", got)
	}
	if got := runtimeSkillRefsFromHelpin(refs, "codex"); len(got) != 1 || got[0].Key != "marketing_context_setup" {
		t.Fatalf("expected Codex compatibility to keep native-authored marketing_context_setup, got %#v", got)
	}
	if got := runtimeSkillRefsFromHelpin(refs, "opencode"); len(got) != 0 {
		t.Fatalf("expected unsupported OpenCode runtime to drop marketing_context_setup, got %#v", got)
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
	// Custom packages must still be fetched live, and revocation must take
	// effect after a successful read rather than serving a cached object.
	store.objects[objectKey] = []byte("updated-zip-bytes")
	payload, err = host.GetSkillPackageObject(context.Background(), objectKey)
	if err != nil || string(payload) != "updated-zip-bytes" {
		t.Fatalf("custom package did not refresh: %q, %v", payload, err)
	}
	if err := repo.Archive(context.Background(), "ws-1", "skill-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := host.GetSkillPackageObject(context.Background(), objectKey); !errors.Is(err, ErrAgentRuntimeHostNotFound) {
		t.Fatalf("archived package remained available: %v", err)
	}
	_, err = host.GetSkillPackageObject(context.Background(), "workspaces/ws-1/skills/missing.zip")
	if !errors.Is(err, ErrAgentRuntimeHostNotFound) {
		t.Fatalf("expected not found for unregistered object key, got %v", err)
	}
}
