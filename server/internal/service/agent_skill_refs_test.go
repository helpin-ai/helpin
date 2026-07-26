package service

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"

	worker "github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCreateAgentResolvesBuiltInSkillRefs(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := &AgentService{agentRepo: agentRepo, activitySvc: activitySvc}

	req := modelCreateAgentRequest(nil)
	req.Skills = model.AgentSkillRefs{{Key: "prd_task_plan_approval"}}
	req.AllowedTools = mustJSONStringSlice([]string{worker.ToolRequestApproval, worker.ToolPublishPRDDraft, worker.ToolPublishTaskPlan})

	created, err := svc.CreateAgent(context.Background(), req, "user-1")
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}
	if len(created.Skills) != 1 || created.Skills[0].Key != "prd_task_plan_approval" {
		t.Fatalf("expected canonical built-in skill ref, got %+v", created.Skills)
	}
	if created.ResolvedSkillInstructions == "" {
		t.Fatal("expected resolved skill instructions")
	}
}

func TestCreateAgentResolvesWorkspaceSkillRefs(t *testing.T) {
	db := newAgentServiceTestDB(t)
	addWorkspaceSkillTable(t, db)
	now := time.Now().UTC()
	if err := db.Exec(`INSERT INTO workspace_skills (
		id, workspace_id, source_kind, key, version_key, title, description, instructions,
		required_tools, supported_runtimes, interface_config, policy_config,
		package_object_key, package_file_name, package_size, package_checksum,
		is_archived, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"skill-1", "ws-test", model.WorkspaceSkillSourceWorkspace, "workspace_skill", "v1", "Workspace Skill", "Use for workspace-specific planning.", "Follow workspace-specific planning instructions.",
		[]byte("[]"), []byte(`["native_sdk"]`), []byte(`{}`), []byte(`{}`), "workspaces/ws-test/skills/skill-1/workspace_skill.zip", "workspace_skill.zip", 1, "checksum", false, now, now,
	).Error; err != nil {
		t.Fatalf("insert workspace skill: %v", err)
	}

	agentRepo := repository.NewAgentRepository(db)
	workspaceSkillRepo := repository.NewWorkspaceSkillRepository(db)
	activitySvc := NewPMActivityService(repository.NewPMActivityRepository(db))
	svc := (&AgentService{agentRepo: agentRepo, activitySvc: activitySvc}).SetWorkspaceSkillStore(workspaceSkillRepo, nil)

	req := modelCreateAgentRequest(nil)
	req.Skills = model.AgentSkillRefs{{Key: "workspace_skill"}}

	created, err := svc.CreateAgent(context.Background(), req, "user-1")
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}
	if len(created.Skills) != 1 {
		t.Fatalf("expected 1 skill ref, got %+v", created.Skills)
	}
	if created.Skills[0].SkillID == nil || *created.Skills[0].SkillID != "skill-1" {
		t.Fatalf("expected workspace skill id to be canonicalized, got %+v", created.Skills[0])
	}
	if created.ResolvedSkillInstructions == "" {
		t.Fatal("expected resolved workspace skill instructions")
	}
}

func addWorkspaceSkillTable(t *testing.T, db *gorm.DB) {
	t.Helper()
	statement := `CREATE TABLE workspace_skills (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		source_kind TEXT NOT NULL,
		source_runtime TEXT,
		key TEXT NOT NULL,
		version_key TEXT NOT NULL,
		title TEXT NOT NULL,
		description TEXT,
		instructions TEXT NOT NULL,
		required_tools BLOB NOT NULL DEFAULT '[]',
		supported_runtimes BLOB NOT NULL DEFAULT '[]',
		interface_config BLOB NOT NULL DEFAULT '{}',
		policy_config BLOB NOT NULL DEFAULT '{}',
		package_object_key TEXT NOT NULL,
		package_file_name TEXT NOT NULL,
		package_size INTEGER NOT NULL DEFAULT 0,
		package_checksum TEXT NOT NULL,
		is_archived BOOLEAN NOT NULL DEFAULT 0,
		created_by TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		UNIQUE(workspace_id, key, is_archived)
	)`
	if err := db.Exec(statement).Error; err != nil {
		t.Fatalf("create workspace_skills table: %v", err)
	}
}
