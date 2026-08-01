package agentskills

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
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
		Skills:      model.AgentSkillRefs{{Key: "prd_task_plan_approval"}},
	}
	destRoot := filepath.Join(t.TempDir(), "skills")

	resolution, err := StageInto(context.Background(), "ws_123", agent, []string{agentcontract.ToolRequestApproval, agentcontract.ToolRequestReviewCheckpoint}, nil, nil, destRoot)
	if err != nil {
		t.Fatalf("stage skills: %v", err)
	}
	if len(resolution.Definitions) != 1 || resolution.Definitions[0].Key != "prd_task_plan_approval" {
		t.Fatalf("expected prd_task_plan_approval definition, got %#v", resolution.Definitions)
	}
	payload, err := os.ReadFile(filepath.Join(destRoot, "01-prd_task_plan_approval", "SKILL.md"))
	if err != nil {
		t.Fatalf("read staged SKILL.md: %v", err)
	}
	if !strings.Contains(string(payload), "prd_task_plan_approval") {
		t.Fatalf("expected staged skill markdown to contain skill key, got %q", string(payload))
	}
	if !strings.Contains(string(payload), "`"+agentcontract.RuntimeToolNameForPrompt(agentcontract.ToolRequestApproval)+"`") {
		t.Fatalf("expected staged skill markdown to use runtime approval tool name, got %q", string(payload))
	}
	if strings.Contains(string(payload), "`"+agentcontract.ToolRequestApproval+"`") {
		t.Fatalf("expected staged skill markdown not to expose bare approval tool name, got %q", string(payload))
	}
}

func TestEffectiveRuntimeRefsFallsBackForBuiltInDefaultVersion(t *testing.T) {
	agent := &model.Agent{
		IsSystem:         true,
		PresetKey:        model.AgentPresetMarketer,
		PresetVersionKey: "marketer_default",
	}

	refs := EffectiveRuntimeRefs(agent)
	if len(refs) == 0 {
		t.Fatal("expected default system agent to fall back to built-in available skills")
	}
	if refs[0].Key != "marketing_context_setup" {
		t.Fatalf("expected first fallback skill %q, got %q", "marketing_context_setup", refs[0].Key)
	}
}

func TestEffectiveRuntimeRefsFallsBackToCoreSkillsForCoreOnlyBuiltInPreset(t *testing.T) {
	agent := &model.Agent{
		IsSystem:         true,
		PresetKey:        model.AgentPresetEpicPlanner,
		PresetVersionKey: "epic_planner_default",
	}

	refs := EffectiveRuntimeRefs(agent)
	if len(refs) == 0 {
		t.Fatal("expected default system agent to fall back to built-in runtime skills")
	}
	if refs[0].Key != "prd_task_plan_approval" {
		t.Fatalf("expected first fallback skill %q, got %q", "prd_task_plan_approval", refs[0].Key)
	}
}

func TestEffectiveRuntimeRefsMergesMissingCoreSkillsForBuiltInDefaultVersion(t *testing.T) {
	versionKey := "operating_rules_v1"
	agent := &model.Agent{
		IsSystem:         true,
		PresetKey:        model.AgentPresetTaskPlanner,
		PresetVersionKey: "task_planner_default",
		Skills: model.AgentSkillRefs{{
			Key:        "engineering_planner_operating_rules",
			VersionKey: &versionKey,
			Config:     model.JSONBlob(`{"strict":true}`),
		}},
	}

	refs := EffectiveRuntimeRefs(agent)
	if len(refs) != 3 {
		t.Fatalf("expected all three required Scribe skills, got %#v", refs)
	}
	wantKeys := []string{"coding_task_planning", "prd_task_plan_approval", "engineering_planner_operating_rules"}
	for index, wantKey := range wantKeys {
		if refs[index].Key != wantKey {
			t.Fatalf("expected skill %d to be %q, got %#v", index, wantKey, refs)
		}
	}
	if refs[2].VersionKey == nil || *refs[2].VersionKey != versionKey {
		t.Fatalf("expected explicit core skill version to be preserved, got %#v", refs[2])
	}
	if string(refs[2].Config) != `{"strict":true}` {
		t.Fatalf("expected explicit core skill config to be preserved, got %#v", refs[2])
	}
}

func TestEffectiveRuntimeRefsDoesNotLetWorkspaceSkillReplaceBuiltInCoreSkill(t *testing.T) {
	skillID := "skill_workspace_approval"
	agent := &model.Agent{
		IsSystem:         true,
		PresetKey:        model.AgentPresetTaskPlanner,
		PresetVersionKey: "task_planner_default",
		Skills: model.AgentSkillRefs{{
			SkillID: &skillID,
			Key:     "prd_task_plan_approval",
		}},
	}

	refs := EffectiveRuntimeRefs(agent)
	if len(refs) != 4 {
		t.Fatalf("expected three built-in core skills plus the workspace skill, got %#v", refs)
	}
	if refs[1].Key != "prd_task_plan_approval" || refs[1].SkillID != nil {
		t.Fatalf("expected required built-in approval skill, got %#v", refs[1])
	}
	if refs[3].SkillID == nil || *refs[3].SkillID != skillID {
		t.Fatalf("expected explicit workspace skill to remain available, got %#v", refs[3])
	}
}

func TestEffectiveRuntimeRefsDoesNotFallbackForWorkspaceVersion(t *testing.T) {
	agent := &model.Agent{
		IsSystem:         true,
		PresetKey:        model.AgentPresetMarketer,
		PresetVersionKey: "marketer_workspace_123",
	}

	refs := EffectiveRuntimeRefs(agent)
	if len(refs) != 0 {
		t.Fatalf("expected workspace version with explicit empty skills to stay empty, got %v", refs)
	}
}

func TestEffectiveRuntimeRefsDoesNotMergeCoreSkillsForWorkspaceVersion(t *testing.T) {
	agent := &model.Agent{
		IsSystem:         true,
		PresetKey:        model.AgentPresetTaskPlanner,
		PresetVersionKey: "task_planner_workspace_123",
		Skills: model.AgentSkillRefs{{
			Key: "engineering_planner_operating_rules",
		}},
	}

	refs := EffectiveRuntimeRefs(agent)
	if len(refs) != 1 || refs[0].Key != "engineering_planner_operating_rules" {
		t.Fatalf("expected workspace version to retain only its explicit skills, got %#v", refs)
	}
}

func TestStageIntoStagesBuiltInSkillPackageReferences(t *testing.T) {
	agent := &model.Agent{
		RuntimeKind: "native_sdk",
		Skills:      model.AgentSkillRefs{{Key: "dependency_audit"}},
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
		"fetch_url",
		"create_task",
	}, nil, nil, destRoot)
	if err != nil {
		t.Fatalf("stage dependency auditor skill: %v", err)
	}
	if len(resolution.Definitions) != 1 || resolution.Definitions[0].Key != "dependency_audit" {
		t.Fatalf("expected dependency_audit definition, got %#v", resolution.Definitions)
	}
	for _, rel := range []string{
		filepath.Join("01-dependency_audit", "SKILL.md"),
		filepath.Join("01-dependency_audit", "ecosystems", "go.md"),
		filepath.Join("01-dependency_audit", "ecosystems", "rust.md"),
		filepath.Join("01-dependency_audit", "ecosystems", "python.md"),
		filepath.Join("01-dependency_audit", "ecosystems", "node.md"),
		filepath.Join("01-dependency_audit", "ecosystems", "java.md"),
		filepath.Join("01-dependency_audit", "verification.md"),
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
	if !strings.Contains(string(payload), "`"+agentcontract.RuntimeToolNameForPrompt("scan_gitleaks")+"`") ||
		!strings.Contains(string(payload), "`"+agentcontract.RuntimeToolNameForPrompt("ensure_task_label")+"`") {
		t.Fatalf("expected staged security triage skill to contain scanner and label tools, got %q", string(payload))
	}
	if _, err := os.Stat(filepath.Join(destRoot, "01-security_triage", "semgrep", "helpin-security.yml")); err != nil {
		t.Fatalf("expected staged security triage semgrep rules: %v", err)
	}
}

func TestStageIntoStagesWorkspaceSkillArchive(t *testing.T) {
	definition := agentcontract.SkillDefinition{
		Key:          "workspace_review",
		Title:        "Workspace Review",
		Description:  "Review changes for the workspace.",
		Instructions: "Inspect the repo, call `update_plan`, and produce a review summary.",
		SourceKind:   model.WorkspaceSkillSourceWorkspace,
	}
	archive, checksum, filename, err := agentcontract.BuildSkillArchive(definition)
	if err != nil {
		t.Fatalf("build archive: %v", err)
	}
	skillID := "skill-123"
	versionKey := agentcontract.SkillVersionForBytes(archive)
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
	if !strings.Contains(string(payload), "`"+agentcontract.RuntimeToolNameForPrompt(agentcontract.ToolUpdatePlan)+"`") {
		t.Fatalf("expected staged workspace skill markdown to use runtime update_plan tool name, got %q", string(payload))
	}
	if strings.Contains(string(payload), "`"+agentcontract.ToolUpdatePlan+"`") {
		t.Fatalf("expected staged workspace skill markdown not to expose bare update_plan tool name, got %q", string(payload))
	}
}
