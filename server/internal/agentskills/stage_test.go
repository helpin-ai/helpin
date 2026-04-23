package agentskills

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

type stageTestLookup struct {
	byID  map[string]*model.WorkspaceSkill
	byKey map[string]*model.WorkspaceSkill
}

func (l *stageTestLookup) GetByID(_ context.Context, _ string, id string) (*model.WorkspaceSkill, error) {
	return l.byID[id], nil
}

func (l *stageTestLookup) GetActiveByKey(_ context.Context, _ string, key string) (*model.WorkspaceSkill, error) {
	return l.byKey[key], nil
}

type stageTestStore struct {
	objects map[string][]byte
}

func (s *stageTestStore) GetObject(_ context.Context, key string) ([]byte, error) {
	return s.objects[key], nil
}

func TestStageIntoStagesBuiltInSkillPackage(t *testing.T) {
	agent := &model.Agent{
		RuntimeKind: "codex",
		Skills:      model.AgentSkillRefs{{Key: "approval_protocol"}},
	}
	destRoot := filepath.Join(t.TempDir(), "skills")

	resolution, err := StageInto(context.Background(), "ws_123", agent, []string{worker.ToolRequestApproval, worker.ToolRequestReviewCheckpoint}, nil, nil, destRoot)
	if err != nil {
		t.Fatalf("stage skills: %v", err)
	}
	if len(resolution.Definitions) != 1 || resolution.Definitions[0].Key != "approval_protocol" {
		t.Fatalf("expected approval_protocol definition, got %#v", resolution.Definitions)
	}
	payload, err := os.ReadFile(filepath.Join(destRoot, "01-approval_protocol", "SKILL.md"))
	if err != nil {
		t.Fatalf("read staged SKILL.md: %v", err)
	}
	if !strings.Contains(string(payload), "approval_protocol") {
		t.Fatalf("expected staged skill markdown to contain skill key, got %q", string(payload))
	}
}

func TestStageIntoStagesWorkspaceSkillArchive(t *testing.T) {
	definition := worker.SkillDefinition{
		Key:          "workspace_review",
		Title:        "Workspace Review",
		Description:  "Review changes for the workspace.",
		Instructions: "Inspect the repo and produce a review summary.",
		SourceKind:   model.WorkspaceSkillSourceWorkspace,
	}
	archive, checksum, filename, err := worker.BuildSkillArchive(definition)
	if err != nil {
		t.Fatalf("build archive: %v", err)
	}
	skillID := "skill-123"
	versionKey := worker.SkillVersionForBytes(archive)
	objectKey := "workspaces/ws_123/skills/skill-123/" + filename
	lookup := &stageTestLookup{
		byID: map[string]*model.WorkspaceSkill{
			skillID: {
				ID:               skillID,
				WorkspaceID:      "ws_123",
				SourceKind:       model.WorkspaceSkillSourceWorkspace,
				Key:              definition.Key,
				VersionKey:       versionKey,
				Title:            definition.Title,
				Description:      &definition.Description,
				Instructions:     definition.Instructions,
				PackageObjectKey: objectKey,
				PackageChecksum:  checksum,
			},
		},
		byKey: map[string]*model.WorkspaceSkill{},
	}
	store := &stageTestStore{objects: map[string][]byte{objectKey: archive}}
	agent := &model.Agent{
		RuntimeKind: "opencode",
		Skills:      model.AgentSkillRefs{{SkillID: &skillID, Key: definition.Key, VersionKey: &versionKey}},
	}
	destRoot := filepath.Join(t.TempDir(), "skills")

	resolution, err := StageInto(context.Background(), "ws_123", agent, nil, lookup, store, destRoot)
	if err != nil {
		t.Fatalf("stage skills: %v", err)
	}
	if len(resolution.Refs) != 1 || resolution.Refs[0].SkillID == nil || *resolution.Refs[0].SkillID != skillID {
		t.Fatalf("expected canonical workspace skill ref, got %#v", resolution.Refs)
	}
	payload, err := os.ReadFile(filepath.Join(destRoot, "01-workspace_review", "SKILL.md"))
	if err != nil {
		t.Fatalf("read staged workspace SKILL.md: %v", err)
	}
	if !strings.Contains(string(payload), definition.Description) {
		t.Fatalf("expected staged workspace skill markdown to contain description, got %q", string(payload))
	}
}
