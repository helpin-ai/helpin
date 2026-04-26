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

func TestStageIntoStagesBuiltInSkillPackageReferences(t *testing.T) {
	agent := &model.Agent{
		RuntimeKind: "native_sdk",
		Skills:      model.AgentSkillRefs{{Key: "dependency_auditor"}},
	}
	destRoot := filepath.Join(t.TempDir(), "skills")

	resolution, err := StageInto(context.Background(), "ws_123", agent, []string{
		"update_plan",
		"list_directory",
		"read_file",
		"read_files",
		"read_file_range",
		"search_files",
		"ripgrep",
		"grep",
		"run_command",
		"web_search_exa",
		"create_task",
	}, nil, nil, destRoot)
	if err != nil {
		t.Fatalf("stage dependency auditor skill: %v", err)
	}
	if len(resolution.Definitions) != 1 || resolution.Definitions[0].Key != "dependency_auditor" {
		t.Fatalf("expected dependency_auditor definition, got %#v", resolution.Definitions)
	}
	for _, rel := range []string{
		filepath.Join("01-dependency_auditor", "SKILL.md"),
		filepath.Join("01-dependency_auditor", "ecosystems", "go.md"),
		filepath.Join("01-dependency_auditor", "ecosystems", "rust.md"),
		filepath.Join("01-dependency_auditor", "ecosystems", "python.md"),
		filepath.Join("01-dependency_auditor", "ecosystems", "node.md"),
		filepath.Join("01-dependency_auditor", "ecosystems", "java.md"),
		filepath.Join("01-dependency_auditor", "verification.md"),
	} {
		if _, err := os.Stat(filepath.Join(destRoot, rel)); err != nil {
			t.Fatalf("expected staged dependency auditor file %s: %v", rel, err)
		}
	}
}

func TestStageIntoStagesSecurityTriageBuiltInSkillPackage(t *testing.T) {
	agent := &model.Agent{
		RuntimeKind: "native_sdk",
		Skills:      model.AgentSkillRefs{{Key: "security_triage"}},
	}
	destRoot := filepath.Join(t.TempDir(), "skills")

	resolution, err := StageInto(context.Background(), "ws_123", agent, []string{
		"update_plan",
		"list_directory",
		"read_file",
		"read_files",
		"read_file_range",
		"search_files",
		"ripgrep",
		"grep",
		"run_command",
		"web_search_exa",
		"create_task",
	}, nil, nil, destRoot)
	if err != nil {
		t.Fatalf("stage security triage skill: %v", err)
	}
	if len(resolution.Definitions) != 1 || resolution.Definitions[0].Key != "security_triage" {
		t.Fatalf("expected security_triage definition, got %#v", resolution.Definitions)
	}
	payload, err := os.ReadFile(filepath.Join(destRoot, "01-security_triage", "SKILL.md"))
	if err != nil {
		t.Fatalf("read staged security triage SKILL.md: %v", err)
	}
	if !strings.Contains(string(payload), "gitleaks detect --source . --report-format json --no-git") {
		t.Fatalf("expected staged security triage skill to contain scanner command, got %q", string(payload))
	}
	if _, err := os.Stat(filepath.Join(destRoot, "01-security_triage", "semgrep", "helpin-security.yml")); err != nil {
		t.Fatalf("expected staged security triage semgrep rules: %v", err)
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
