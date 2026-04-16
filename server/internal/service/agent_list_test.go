package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestListAgentsSkipsSkillResolutionFailure(t *testing.T) {
	db := newAgentServiceTestDB(t)
	agentRepo := repository.NewAgentRepository(db)
	svc := &AgentService{agentRepo: agentRepo}

	now := time.Now().UTC()
	if err := db.Exec(`INSERT INTO agents (
		id, workspace_id, is_system, name, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets,
		approval_mode, max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-1", "ws-test", false, "Broken Skill Agent", "Custom Agent", "idle", "opencode",
		[]byte(`[{"key":"missing_workspace_skill"}]`), "manual", []byte("[]"), []byte("[]"), []byte("[]"),
		"never", 1, "autonomous", now, now,
	).Error; err != nil {
		t.Fatalf("insert agent: %v", err)
	}

	agents, err := svc.ListAgents(context.Background(), "ws-test")
	if err != nil {
		t.Fatalf("ListAgents returned error: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].ID != "agent-1" {
		t.Fatalf("expected agent-1, got %q", agents[0].ID)
	}
	if agents[0].ResolvedSkillInstructions != "" {
		t.Fatalf("expected no resolved skill instructions on failed resolution, got %q", agents[0].ResolvedSkillInstructions)
	}
}
