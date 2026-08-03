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
		if _, err := svc.verifyDockApproval(context.Background(), meta, chatRun, interaction.ID, "launch", stepsHash(t, steps), dockApprovalPayloadKind); err != nil {
			t.Fatalf("verifyDockApproval() = %v, want nil", err)
		}
	})

	t.Run("runtime-shaped payload (phase + raw_input.action) passes", func(t *testing.T) {
		payload, err := json.Marshal(map[string]interface{}{
			"phase":   dockApprovalPayloadKind,
			"title":   "Confirm child agent run",
			"summary": "test",
			"raw_input": map[string]interface{}{
				"phase":  dockApprovalPayloadKind,
				"action": map[string]interface{}{"steps": steps},
			},
		})
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		interaction := seedDockApproval(t, db, chatRun.ID, func(i *model.AgentRunInteraction) {
			i.RequestPayload = payload
		})
		if _, err := svc.verifyDockApproval(context.Background(), meta, chatRun, interaction.ID, "launch", stepsHash(t, steps), dockApprovalPayloadKind); err != nil {
			t.Fatalf("verifyDockApproval() runtime shape = %v, want nil", err)
		}
	})

	t.Run("missing action rejected", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"phase": dockApprovalPayloadKind,
			"title": "Confirm child agent run",
		})
		interaction := seedDockApproval(t, db, chatRun.ID, func(i *model.AgentRunInteraction) {
			i.RequestPayload = payload
		})
		if _, err := svc.verifyDockApproval(context.Background(), meta, chatRun, interaction.ID, "launch", stepsHash(t, steps), dockApprovalPayloadKind); err == nil || !strings.Contains(err.Error(), "missing the structured action") {
			t.Fatalf("verifyDockApproval() without action = %v, want missing-action error", err)
		}
	})

	t.Run("single-step inline action form passes", func(t *testing.T) {
		interaction := seedDockApproval(t, db, chatRun.ID, func(i *model.AgentRunInteraction) {
			i.RequestPayload = dockApprovalRequestPayload(t, steps[0])
		})
		if _, err := svc.verifyDockApproval(context.Background(), meta, chatRun, interaction.ID, "launch", stepsHash(t, steps), dockApprovalPayloadKind); err != nil {
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
		if _, err := svc.verifyDockApproval(context.Background(), meta, chatRun, interaction.ID, "launch", stepsHash(t, tampered), dockApprovalPayloadKind); err == nil {
			t.Fatal("verifyDockApproval() = nil for tampered steps, want error")
		}
	})

	t.Run("pending interaction rejected", func(t *testing.T) {
		interaction := seedDockApproval(t, db, chatRun.ID, func(i *model.AgentRunInteraction) {
			i.Status = model.AgentRunInteractionStatusPending
			i.RequestPayload = dockApprovalRequestPayload(t, map[string]interface{}{"steps": steps})
		})
		if _, err := svc.verifyDockApproval(context.Background(), meta, chatRun, interaction.ID, "launch", stepsHash(t, steps), dockApprovalPayloadKind); err == nil || !strings.Contains(err.Error(), "not resolved") {
			t.Fatalf("verifyDockApproval() pending = %v, want not-resolved error", err)
		}
	})

	t.Run("rejected decision rejected", func(t *testing.T) {
		interaction := seedDockApproval(t, db, chatRun.ID, func(i *model.AgentRunInteraction) {
			i.ResponsePayload = json.RawMessage(`{"decision":"request_changes"}`)
			i.RequestPayload = dockApprovalRequestPayload(t, map[string]interface{}{"steps": steps})
		})
		if _, err := svc.verifyDockApproval(context.Background(), meta, chatRun, interaction.ID, "launch", stepsHash(t, steps), dockApprovalPayloadKind); err == nil || !strings.Contains(err.Error(), "did not approve") {
			t.Fatalf("verifyDockApproval() rejected = %v, want not-approved error", err)
		}
	})

	t.Run("wrong payload kind rejected", func(t *testing.T) {
		interaction := seedDockApproval(t, db, chatRun.ID, func(i *model.AgentRunInteraction) {
			payload, _ := json.Marshal(map[string]interface{}{"kind": "something_else", "action": map[string]interface{}{"steps": steps}})
			i.RequestPayload = payload
		})
		if _, err := svc.verifyDockApproval(context.Background(), meta, chatRun, interaction.ID, "launch", stepsHash(t, steps), dockApprovalPayloadKind); err == nil {
			t.Fatal("verifyDockApproval() = nil for wrong kind, want error")
		}
	})

	t.Run("consumed approval rejected", func(t *testing.T) {
		interaction := seedDockApproval(t, db, chatRun.ID, func(i *model.AgentRunInteraction) {
			i.RequestPayload = dockApprovalRequestPayload(t, map[string]interface{}{"steps": steps})
			i.RuntimeMetadata = json.RawMessage(`{"dock_action_consumed":true}`)
		})
		if _, err := svc.verifyDockApproval(context.Background(), meta, chatRun, interaction.ID, "launch", stepsHash(t, steps), dockApprovalPayloadKind); err == nil || !strings.Contains(err.Error(), "already used") {
			t.Fatalf("verifyDockApproval() consumed = %v, want already-used error", err)
		}
	})

	t.Run("missing interaction id rejected", func(t *testing.T) {
		if _, err := svc.verifyDockApproval(context.Background(), meta, chatRun, "", "launch", stepsHash(t, steps), dockApprovalPayloadKind); err == nil {
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

func TestNormalizeDockGetRunRequest(t *testing.T) {
	t.Run("defaults result page", func(t *testing.T) {
		got, err := normalizeDockGetRunRequest(dockGetRunRequest{RunID: " run-1 ", DetailLevel: "result"})
		if err != nil {
			t.Fatalf("normalize request: %v", err)
		}
		if got.RunID != "run-1" || got.ResultLimit != dockRunResultDefaultChars {
			t.Fatalf("normalized request = %+v", got)
		}
	})

	t.Run("rejects result paging in status mode", func(t *testing.T) {
		_, err := normalizeDockGetRunRequest(dockGetRunRequest{RunID: "run-1", ResultLimit: 10})
		if err == nil || !strings.Contains(err.Error(), "require detail_level=result") {
			t.Fatalf("error = %v, want result-mode guidance", err)
		}
	})

	t.Run("rejects plan result retrieval", func(t *testing.T) {
		_, err := normalizeDockGetRunRequest(dockGetRunRequest{PlanID: "plan-1", DetailLevel: "result"})
		if err == nil || !strings.Contains(err.Error(), "requires run_id") {
			t.Fatalf("error = %v, want run_id guidance", err)
		}
	})
}

func TestAskAgentPromptRecoversTruncatedResultWithoutRerun(t *testing.T) {
	prompt := askAgentSystemPrompt()
	for _, required := range []string{
		"summary_truncated=true",
		`{"run_id":"...","detail_level":"result"}`,
		"Never launch a replacement child merely to recover truncated output",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("Ask Agent prompt missing %q", required)
		}
	}
}

func TestGetAgentRunRetrievesOwnedPersistedResultWithoutNewPlan(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	if err := db.Exec(`CREATE TABLE command_bar_plans (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		actor_id TEXT,
		parent_chat_run_id TEXT,
		dock_chat_id TEXT,
		support_conversation_id TEXT,
		parent_notified_at DATETIME,
		status TEXT NOT NULL DEFAULT 'running',
		prompt TEXT NOT NULL,
		page_context BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
		steps BLOB NOT NULL DEFAULT (CAST('[]' AS BLOB)),
		run_ids_by_step BLOB NOT NULL DEFAULT (CAST('{}' AS BLOB)),
		current_step_index INTEGER NOT NULL DEFAULT 0,
		run_count INTEGER NOT NULL DEFAULT 0,
		error_message TEXT,
		cancelled_at DATETIME,
		completed_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create command_bar_plans: %v", err)
	}

	ctx := context.Background()
	workspaceID := "ws-1"
	actorID := "user-1"
	chatID := "chat-1"
	otherChatID := "chat-2"
	chatRunID := "chat-run-1"
	otherChatRunID := "chat-run-2"
	childRunID := "child-run-1"
	runRepo := repository.NewAgentRunRepository(db)
	messageRepo := repository.NewAgentRunMessageRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	planRepo := repository.NewCommandBarPlanRepository(db)

	createRun := func(run *model.AgentRun) {
		t.Helper()
		if err := runRepo.Create(ctx, run); err != nil {
			t.Fatalf("create run %s: %v", run.ID, err)
		}
	}
	baseRun := func(id string) *model.AgentRun {
		return &model.AgentRun{
			ID: id, WorkspaceID: workspaceID, AgentID: "agent-1", TargetType: "workspace", TargetID: workspaceID,
			RuntimeKind: "native_sdk", InvocationMode: model.InvocationModeAutonomous,
			ApprovalState: "not_required", PauseReason: model.AgentRunPauseReasonNone,
			Status: model.AgentRunStatusCompleted, Input: json.RawMessage(`{}`), OutputSummary: json.RawMessage(`{}`),
		}
	}
	chatRun := baseRun(chatRunID)
	chatRun.DockChatID = &chatID
	createRun(chatRun)
	otherChatRun := baseRun(otherChatRunID)
	otherChatRun.DockChatID = &otherChatID
	createRun(otherChatRun)
	createRun(baseRun(childRunID))

	actor := actorID
	if err := planRepo.Create(ctx, &model.CommandBarPlanRecord{
		ID: "plan-1", WorkspaceID: workspaceID, ActorID: &actor, ParentChatRunID: &chatRunID, DockChatID: &chatID,
		Status: model.CommandBarPlanStatusCompleted, Prompt: "research ClickHouse",
		PageContext: json.RawMessage(`{}`), Steps: json.RawMessage(`[]`),
		RunIDsByStep: json.RawMessage(`{"0":"child-run-1"}`), RunCount: 1,
	}); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	fullResponse := strings.Repeat("界", dockRunResultDefaultChars+5)
	if err := messageRepo.Create(ctx, &model.AgentRunMessage{
		WorkspaceID: workspaceID, RunID: childRunID, Role: "assistant", Content: fullResponse,
		MessageType: "message", SequenceNo: 1,
	}); err != nil {
		t.Fatalf("create result message: %v", err)
	}
	if err := artifactRepo.Create(ctx, &model.AgentRunArtifact{
		WorkspaceID: workspaceID, RunID: childRunID, ArtifactType: model.AgentRunArtifactTypeReviewFindings,
		Format: "json", StorageMode: "inline", SequenceNo: 1,
	}); err != nil {
		t.Fatalf("create artifact: %v", err)
	}

	agentService := &AgentService{runRepo: runRepo, runMessageRepo: messageRepo, artifactRepo: artifactRepo}
	commandBarService := NewCommandBarService(agentService, planRepo, nil)
	commandService := &InternalCommandService{
		agentService: agentService, agentRunRepo: runRepo, agentRunArtifactRepo: artifactRepo,
		commandBarService: commandBarService, definitions: map[string]InternalCommandDefinition{},
	}
	commandService.registerAgentOrchestrationCommands()
	definition, ok := commandService.Definition("agents.get_run")
	if !ok {
		t.Fatal("agents.get_run definition missing")
	}

	input := json.RawMessage(`{"run_id":"child-run-1","detail_level":"result"}`)
	before := int64(0)
	if err := db.Model(&model.CommandBarPlanRecord{}).Count(&before).Error; err != nil {
		t.Fatalf("count plans before retrieval: %v", err)
	}
	output, err := definition.Execute(ctx, model.InternalCommandContext{
		WorkspaceID: workspaceID, ActorID: actorID, RunID: chatRunID, TargetType: "workspace", TargetID: workspaceID,
	}, input)
	if err != nil {
		t.Fatalf("get_agent_run result: %v", err)
	}
	var response dockGetRunResponse
	if err := json.Unmarshal(output, &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.ResultAvailable || response.Result == nil || !response.Result.Truncated || response.Result.NextOffset == nil {
		t.Fatalf("unexpected result response: %+v", response)
	}
	if response.Result.CharCount != dockRunResultDefaultChars+5 || len([]rune(response.Result.Content)) != dockRunResultDefaultChars {
		t.Fatalf("unexpected result page: %+v", response.Result)
	}
	if len(response.Artifacts) != 1 || response.Artifacts[0].ArtifactType != model.AgentRunArtifactTypeReviewFindings {
		t.Fatalf("unexpected artifacts: %+v", response.Artifacts)
	}
	after := int64(0)
	if err := db.Model(&model.CommandBarPlanRecord{}).Count(&after).Error; err != nil {
		t.Fatalf("count plans after retrieval: %v", err)
	}
	if after != before {
		t.Fatalf("result retrieval created a new plan: before=%d after=%d", before, after)
	}

	_, err = definition.Execute(ctx, model.InternalCommandContext{
		WorkspaceID: workspaceID, ActorID: actorID, RunID: otherChatRunID, TargetType: "workspace", TargetID: workspaceID,
	}, input)
	if err == nil || !strings.Contains(err.Error(), "not launched from this chat") {
		t.Fatalf("cross-chat result lookup error = %v", err)
	}
}
