package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDockApprovalTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:dock_approval_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE agent_run_interactions (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		run_id TEXT NOT NULL,
		runtime_kind TEXT NOT NULL,
		interaction_kind TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		request_schema_version TEXT NOT NULL,
		response_schema_version TEXT,
		request_id TEXT,
		thread_id TEXT,
		turn_id TEXT,
		item_id TEXT,
		approval_id TEXT,
		assistant_message_sequence_no INTEGER,
		title TEXT,
		summary TEXT,
		request_payload TEXT NOT NULL DEFAULT '{}',
		response_payload TEXT,
		runtime_metadata TEXT NOT NULL DEFAULT '{}',
		resolved_by TEXT,
		resolved_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create interactions table: %v", err)
	}
	return db
}

func seedDockApproval(t *testing.T, db *gorm.DB, runID string, mutate func(*model.AgentRunInteraction)) *model.AgentRunInteraction {
	t.Helper()
	interaction := &model.AgentRunInteraction{
		ID:                   fmt.Sprintf("interaction-%d", time.Now().UnixNano()),
		WorkspaceID:          "ws-1",
		RunID:                runID,
		RuntimeKind:          "native_sdk",
		InteractionKind:      model.AgentRunInteractionKindApprovalRequest,
		Status:               model.AgentRunInteractionStatusResolved,
		RequestSchemaVersion: "1",
		RequestPayload:       json.RawMessage(`{}`),
		ResponsePayload:      json.RawMessage(`{"decision":"approve"}`),
		RuntimeMetadata:      json.RawMessage(`{}`),
	}
	if mutate != nil {
		mutate(interaction)
	}
	if err := db.Create(interaction).Error; err != nil {
		t.Fatalf("create interaction: %v", err)
	}
	return interaction
}

func dockApprovalRequestPayload(t *testing.T, action interface{}) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(map[string]interface{}{
		"kind":    dockApprovalPayloadKind,
		"summary": "test",
		"action":  action,
	})
	if err != nil {
		t.Fatalf("marshal approval payload: %v", err)
	}
	return encoded
}

func TestVerifyDockApprovalLaunch(t *testing.T) {
	db := setupDockApprovalTestDB(t)
	svc := &InternalCommandService{agentRunInteractionRepo: repository.NewAgentRunInteractionRepository(db)}
	chatRun := &model.AgentRun{ID: "run-1", WorkspaceID: "ws-1"}
	meta := model.InternalCommandContext{WorkspaceID: "ws-1", ActorID: "user-1"}

	steps := []dockLaunchStep{{
		AgentID:      "agent-1",
		Target:       dockLaunchTarget{Type: "task", ID: "task-1"},
		Instructions: "review the task",
		AllowedTools: []string{"list_tasks"},
	}}
	stepsHash := func(t *testing.T, steps []dockLaunchStep) string {
		t.Helper()
		hash, err := dockActionHash(normalizeDockLaunchSteps(steps))
		if err != nil {
			t.Fatalf("hash steps: %v", err)
		}
		return hash
	}

	t.Run("matching action passes", func(t *testing.T) {
		interaction := seedDockApproval(t, db, chatRun.ID, func(i *model.AgentRunInteraction) {
			i.RequestPayload = dockApprovalRequestPayload(t, map[string]interface{}{"steps": steps})
		})
		if _, err := svc.verifyDockApproval(context.Background(), meta, chatRun, interaction.ID, "launch", stepsHash(t, steps)); err != nil {
			t.Fatalf("verifyDockApproval() = %v, want nil", err)
		}
	})

	t.Run("single-step inline action form passes", func(t *testing.T) {
		interaction := seedDockApproval(t, db, chatRun.ID, func(i *model.AgentRunInteraction) {
			i.RequestPayload = dockApprovalRequestPayload(t, steps[0])
		})
		if _, err := svc.verifyDockApproval(context.Background(), meta, chatRun, interaction.ID, "launch", stepsHash(t, steps)); err != nil {
			t.Fatalf("verifyDockApproval() inline form = %v, want nil", err)
		}
	})

	t.Run("mismatched instructions rejected", func(t *testing.T) {
		tampered := []dockLaunchStep{{
			AgentID:      "agent-1",
			Target:       dockLaunchTarget{Type: "task", ID: "task-1"},
			Instructions: "delete everything",
			AllowedTools: []string{"list_tasks"},
		}}
		interaction := seedDockApproval(t, db, chatRun.ID, func(i *model.AgentRunInteraction) {
			i.RequestPayload = dockApprovalRequestPayload(t, map[string]interface{}{"steps": steps})
		})
		if _, err := svc.verifyDockApproval(context.Background(), meta, chatRun, interaction.ID, "launch", stepsHash(t, tampered)); err == nil {
			t.Fatal("verifyDockApproval() = nil for tampered steps, want error")
		}
	})

	t.Run("pending interaction rejected", func(t *testing.T) {
		interaction := seedDockApproval(t, db, chatRun.ID, func(i *model.AgentRunInteraction) {
			i.Status = model.AgentRunInteractionStatusPending
			i.RequestPayload = dockApprovalRequestPayload(t, map[string]interface{}{"steps": steps})
		})
		if _, err := svc.verifyDockApproval(context.Background(), meta, chatRun, interaction.ID, "launch", stepsHash(t, steps)); err == nil || !strings.Contains(err.Error(), "not resolved") {
			t.Fatalf("verifyDockApproval() pending = %v, want not-resolved error", err)
		}
	})

	t.Run("rejected decision rejected", func(t *testing.T) {
		interaction := seedDockApproval(t, db, chatRun.ID, func(i *model.AgentRunInteraction) {
			i.ResponsePayload = json.RawMessage(`{"decision":"request_changes"}`)
			i.RequestPayload = dockApprovalRequestPayload(t, map[string]interface{}{"steps": steps})
		})
		if _, err := svc.verifyDockApproval(context.Background(), meta, chatRun, interaction.ID, "launch", stepsHash(t, steps)); err == nil || !strings.Contains(err.Error(), "did not approve") {
			t.Fatalf("verifyDockApproval() rejected = %v, want not-approved error", err)
		}
	})

	t.Run("wrong payload kind rejected", func(t *testing.T) {
		interaction := seedDockApproval(t, db, chatRun.ID, func(i *model.AgentRunInteraction) {
			payload, _ := json.Marshal(map[string]interface{}{"kind": "something_else", "action": map[string]interface{}{"steps": steps}})
			i.RequestPayload = payload
		})
		if _, err := svc.verifyDockApproval(context.Background(), meta, chatRun, interaction.ID, "launch", stepsHash(t, steps)); err == nil {
			t.Fatal("verifyDockApproval() = nil for wrong kind, want error")
		}
	})

	t.Run("consumed approval rejected", func(t *testing.T) {
		interaction := seedDockApproval(t, db, chatRun.ID, func(i *model.AgentRunInteraction) {
			i.RequestPayload = dockApprovalRequestPayload(t, map[string]interface{}{"steps": steps})
			i.RuntimeMetadata = json.RawMessage(`{"dock_action_consumed":true}`)
		})
		if _, err := svc.verifyDockApproval(context.Background(), meta, chatRun, interaction.ID, "launch", stepsHash(t, steps)); err == nil || !strings.Contains(err.Error(), "already used") {
			t.Fatalf("verifyDockApproval() consumed = %v, want already-used error", err)
		}
	})

	t.Run("missing interaction id rejected", func(t *testing.T) {
		if _, err := svc.verifyDockApproval(context.Background(), meta, chatRun, "", "launch", stepsHash(t, steps)); err == nil {
			t.Fatal("verifyDockApproval() = nil for empty id, want error")
		}
	})
}

func TestDockActionHashNormalization(t *testing.T) {
	base := []dockLaunchStep{{
		AgentID:      " agent-1 ",
		Target:       dockLaunchTarget{Type: " task ", ID: " task-1 "},
		Instructions: " do the thing ",
		AllowedTools: []string{"b_tool", "a_tool"},
	}}
	reordered := []dockLaunchStep{{
		AgentID:      "agent-1",
		Target:       dockLaunchTarget{Type: "task", ID: "task-1"},
		Instructions: "do the thing",
		AllowedTools: []string{"a_tool", "b_tool"},
	}}
	hashA, err := dockActionHash(normalizeDockLaunchSteps(base))
	if err != nil {
		t.Fatalf("hash base: %v", err)
	}
	hashB, err := dockActionHash(normalizeDockLaunchSteps(reordered))
	if err != nil {
		t.Fatalf("hash reordered: %v", err)
	}
	if hashA != hashB {
		t.Errorf("normalized hashes differ: %q vs %q", hashA, hashB)
	}
}
