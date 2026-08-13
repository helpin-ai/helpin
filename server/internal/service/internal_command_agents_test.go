package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
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

func TestResolvedDockApprovalActionReturnsStoredLaunchInstructions(t *testing.T) {
	db := setupDockApprovalTestDB(t)
	svc := &InternalCommandService{agentRunInteractionRepo: repository.NewAgentRunInteractionRepository(db)}
	chatRun := &model.AgentRun{ID: "run-approved-action", WorkspaceID: "ws-1"}
	meta := model.InternalCommandContext{WorkspaceID: "ws-1", ActorID: "user-1"}
	approved := []dockLaunchStep{{AgentID: "agent-1", Instructions: "create the complete document", Target: dockLaunchTarget{Type: "workspace", ID: "ws-1"}}}
	interaction := seedDockApproval(t, db, chatRun.ID, func(i *model.AgentRunInteraction) {
		i.RequestPayload = dockApprovalRequestPayload(t, map[string]interface{}{"steps": approved})
	})

	_, action, err := svc.resolvedDockApprovalAction(context.Background(), meta, chatRun, interaction.ID, dockApprovalPayloadKind)
	if err != nil {
		t.Fatalf("resolvedDockApprovalAction() = %v", err)
	}
	var payload struct {
		Steps []dockLaunchStep `json:"steps"`
	}
	if err := json.Unmarshal(action, &payload); err != nil {
		t.Fatalf("decode approved action: %v", err)
	}
	if len(payload.Steps) != 1 || payload.Steps[0].Instructions != approved[0].Instructions {
		t.Fatalf("approved steps = %#v, want stored instructions", payload.Steps)
	}
}

func TestAgentOrchestrationToolDescriptionsUseSubAgentTerminology(t *testing.T) {
	svc := &InternalCommandService{definitions: map[string]InternalCommandDefinition{}}
	svc.registerAgentOrchestrationCommands()

	for _, name := range []string{"agents.start_run", "agents.start_plan", "agents.get_run", "agents.cancel_run", "agents.promote_run"} {
		definition, ok := svc.Definition(name)
		if !ok || definition.Tool == nil {
			t.Fatalf("%s definition missing", name)
		}
		lower := strings.ToLower(definition.Tool.Description)
		if strings.Contains(lower, "child agent") || strings.Contains(lower, "child run") || strings.Contains(lower, "one-shot") {
			t.Fatalf("%s uses legacy agent terminology: %q", name, definition.Tool.Description)
		}
		if !strings.Contains(lower, "sub-agent") {
			t.Fatalf("%s description does not use sub-agent terminology: %q", name, definition.Tool.Description)
		}
	}
}

func TestAgentLaunchContractsRequireExplicitDirectTargets(t *testing.T) {
	svc := &InternalCommandService{definitions: map[string]InternalCommandDefinition{}}
	svc.registerAgentOrchestrationCommands()

	startRun, ok := svc.Definition("agents.start_run")
	if !ok || startRun.Tool == nil {
		t.Fatal("agents.start_run definition missing")
	}
	if !strings.Contains(startRun.Tool.Description, "must explicitly target") {
		t.Fatalf("start run description omits explicit target guidance: %q", startRun.Tool.Description)
	}
	runProperties := startRun.Tool.InputSchema["properties"].(map[string]any)
	assertDockLaunchTargetSchema(t, runProperties["target"].(map[string]any))
	if alternatives, ok := startRun.Tool.InputSchema["anyOf"].([]map[string]any); !ok || len(alternatives) != 2 {
		t.Fatalf("start run direct/legacy alternatives = %#v", startRun.Tool.InputSchema["anyOf"])
	}

	startPlan, ok := svc.Definition("agents.start_plan")
	if !ok || startPlan.Tool == nil {
		t.Fatal("agents.start_plan definition missing")
	}
	planProperties := startPlan.Tool.InputSchema["properties"].(map[string]any)
	steps := planProperties["steps"].(map[string]any)
	if steps["minItems"] != 1 {
		t.Fatalf("start plan steps minItems = %#v, want 1", steps["minItems"])
	}
	stepSchema := steps["items"].(map[string]any)
	required := stepSchema["required"].([]string)
	if !slices.Contains(required, "instructions") || !slices.Contains(required, "target") {
		t.Fatalf("direct plan step required fields = %v, want instructions and target", required)
	}
}

func assertDockLaunchTargetSchema(t *testing.T, schema map[string]any) {
	t.Helper()
	required, _ := schema["required"].([]string)
	if !slices.Contains(required, "type") || schema["additionalProperties"] != false {
		t.Fatalf("launch target must require type and reject unknown fields: %#v", schema)
	}
	properties := schema["properties"].(map[string]any)
	typeSchema := properties["type"].(map[string]any)
	targetTypes, _ := typeSchema["enum"].([]string)
	for _, targetType := range []string{"workspace", "task", "epic", "repository"} {
		if !slices.Contains(targetTypes, targetType) {
			t.Fatalf("launch target enum missing %q: %v", targetType, targetTypes)
		}
	}
}

func TestValidateDirectDockLaunchSteps(t *testing.T) {
	tests := []struct {
		name    string
		steps   []dockLaunchStep
		wantErr string
	}{
		{name: "saved agent task target", steps: []dockLaunchStep{{AgentID: "scribe", Instructions: "plan it", Target: dockLaunchTarget{Type: "task", ID: "task-1"}}}},
		{name: "one-shot agent task target", steps: []dockLaunchStep{{UseCommandAgent: true, AllowedTools: []string{"get_task"}, Instructions: "inspect it", Target: dockLaunchTarget{Type: "task", ID: "task-1"}}}},
		{name: "workspace derives trusted id", steps: []dockLaunchStep{{Instructions: "research it", Target: dockLaunchTarget{Type: "workspace"}}}},
		{name: "missing target type", steps: []dockLaunchStep{{Instructions: "plan it"}}, wantErr: "target.type is required"},
		{name: "missing task id", steps: []dockLaunchStep{{Instructions: "plan it", Target: dockLaunchTarget{Type: "task"}}}, wantErr: `target.id is required for target.type "task"`},
		{name: "missing second step target", steps: []dockLaunchStep{{Instructions: "first", Target: dockLaunchTarget{Type: "task", ID: "task-1"}}, {Instructions: "second"}}, wantErr: "step 2: target.type is required"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateDirectDockLaunchSteps(test.steps)
			if test.wantErr == "" && err != nil {
				t.Fatalf("validate direct launch: %v", err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestAgentLaunchHandlersRejectImplicitWorkspaceTargets(t *testing.T) {
	svc := &InternalCommandService{definitions: map[string]InternalCommandDefinition{}}
	svc.registerAgentOrchestrationCommands()
	meta := model.InternalCommandContext{WorkspaceID: "ws-1", ActorID: "user-1"}

	startRun, _ := svc.Definition("agents.start_run")
	_, err := startRun.Execute(context.Background(), meta, json.RawMessage(`{"agent_id":"scribe","instructions":"plan the task"}`))
	if err == nil || !strings.Contains(err.Error(), "target.type is required") {
		t.Fatalf("start run error = %v, want explicit-target guidance", err)
	}

	startPlan, _ := svc.Definition("agents.start_plan")
	_, err = startPlan.Execute(context.Background(), meta, json.RawMessage(`{"steps":[{"agent_id":"scribe","instructions":"plan the task"}]}`))
	if err == nil || !strings.Contains(err.Error(), "target.type is required") {
		t.Fatalf("start plan error = %v, want explicit-target guidance", err)
	}
}

func TestRepairDockLaunchErrorExplainsRepositoryRecovery(t *testing.T) {
	err := repairDockLaunchError(fmt.Errorf("start task planner: %w", ErrTaskDeliveryTargetRequired))
	for _, required := range []string{"configured repository", `target.type="task"`, "list_repositories", "update_task_delivery_target", "same task-targeted launch"} {
		if !strings.Contains(err.Error(), required) {
			t.Fatalf("repair error missing %q: %v", required, err)
		}
	}
	original := fmt.Errorf("another launch error")
	if repairDockLaunchError(original) != original {
		t.Fatal("unrelated launch error was replaced")
	}
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
		"Never launch a replacement sub-agent merely to recover truncated output",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("Ask Agent prompt missing %q", required)
		}
	}
}

func TestAskAgentPromptUsesAttachedReferences(t *testing.T) {
	prompt := askAgentSystemPrompt()
	for _, required := range []string{
		"<references>[...]</references>",
		"supplemental entities the user explicitly attached",
		"consider every attached reference relevant",
		"never echo the raw block",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("Ask Agent prompt missing %q", required)
		}
	}
}

func TestAskAgentPromptLinksResolvedWorkspaceEntities(t *testing.T) {
	prompt := askAgentSystemPrompt()
	for _, required := range []string{
		"markdown_link",
		"machine-only values",
		"pass the appropriate ID verbatim to later tool calls",
		"never pass markdown_link as a tool argument",
		"do not show raw IDs unless the user explicitly asks for them",
		"presentation-only",
		"copy markdown_link verbatim into the response",
		"mandatory in prose, bullets, tables, summaries, and follow-up answers",
		"Never output the entity's plain key or name in place of an available markdown_link",
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
