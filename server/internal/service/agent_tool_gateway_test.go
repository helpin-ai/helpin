package service

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/websocket"
	"github.com/helpin-ai/helpin/server/internal/worker"
)

type capturedGatewayEventPublisher struct {
	events []websocket.Event
}

func (p *capturedGatewayEventPublisher) Publish(event websocket.Event) {
	p.events = append(p.events, event)
}

func TestGatewayRequiresApprovalByAgentMode(t *testing.T) {
	tests := []struct {
		mode string
		want bool
	}{
		{mode: "never", want: false},
		{mode: "mutating_tools", want: true},
		{mode: "always", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			if got := gatewayRequiresApproval(&model.Agent{ApprovalMode: tt.mode}); got != tt.want {
				t.Fatalf("gatewayRequiresApproval() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestAgentToolGatewayPersistsPlannerArtifacts(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	ctx := context.Background()
	now := time.Now()

	allowedTools := mustMarshalTestJSON(t, []string{
		worker.ToolUpdatePlan,
		worker.ToolPublishTaskPlan,
		worker.ToolRequestApproval,
	})
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-gateway-1", "ws-1", true, "Atlas", model.AgentPresetEpicPlanner, "Planner", "idle", "codex",
		[]byte("[]"), "manual", allowedTools, []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)

	runRepo := repository.NewAgentRunRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	interactionRepo := repository.NewAgentRunInteractionRepository(db)
	run := &model.AgentRun{
		ID:             "run-gateway-1",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-gateway-1",
		TargetType:     "epic",
		TargetID:       "epic-1",
		RuntimeKind:    "codex",
		InvocationMode: model.InvocationModeInteractive,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusRunning,
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := runRepo.Create(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	jwtManager := auth.NewJWTManager("test-secret")
	token, err := jwtManager.GenerateAgentRunToolToken(run.ID, run.WorkspaceID, time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	wsPublisher := &capturedGatewayEventPublisher{}
	gateway := NewAgentToolGateway(
		runRepo,
		agentRepo,
		artifactRepo,
		interactionRepo,
		NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil),
		jwtManager,
		wsPublisher,
	)

	list, err := gateway.ListTools(ctx, token)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	names := make([]string, 0, len(list.Tools))
	for _, tool := range list.Tools {
		names = append(names, tool.Name)
	}
	for _, required := range []string{worker.ToolUpdatePlan, worker.ToolPublishTaskPlan, worker.ToolRequestApproval} {
		if !slices.Contains(names, required) {
			t.Fatalf("expected listed tools to include %s, got %v", required, names)
		}
	}

	if resp, err := gateway.CallTool(ctx, token, model.AgentRunToolCallRequest{
		ToolName: worker.ToolUpdatePlan,
		Input: json.RawMessage(`{"plan":[
			{"step":"Inspect repository context","status":"completed"},
			{"step":"Publish typed task plan","status":"in_progress"}
		]}`),
	}); err != nil || resp == nil || resp.IsError {
		t.Fatalf("update_plan response err=%v resp=%+v", err, resp)
	}
	if len(wsPublisher.events) != 1 {
		t.Fatalf("expected one live coding session event after update_plan, got %d", len(wsPublisher.events))
	}
	if wsPublisher.events[0].Entity != "coding_session_event" || wsPublisher.events[0].ParentID != run.ID {
		t.Fatalf("unexpected live event: %+v", wsPublisher.events[0])
	}
	var planEvent model.CodingSessionEvent
	if err := json.Unmarshal(wsPublisher.events[0].Data, &planEvent); err != nil {
		t.Fatalf("decode plan event: %v", err)
	}
	if planEvent.Type != "plan.updated" || planEvent.RuntimeKind != "codex" {
		t.Fatalf("unexpected plan event: %+v", planEvent)
	}
	payloadBytes, err := json.Marshal(planEvent.Payload)
	if err != nil {
		t.Fatalf("marshal plan payload: %v", err)
	}
	var planPayload struct {
		Content worker.RunPlanArtifact `json:"content"`
	}
	if err := json.Unmarshal(payloadBytes, &planPayload); err != nil {
		t.Fatalf("decode plan payload: %v", err)
	}
	if len(planPayload.Content.Plan) != 2 || planPayload.Content.Plan[1].Status != worker.PlanStepInProgress {
		t.Fatalf("unexpected plan payload: %+v", planPayload.Content)
	}

	taskPlanInput := json.RawMessage(`{
		"title":"Task Plan",
		"content":{
			"summary":"Add shared MCP artifact tools for planner runs.",
			"proposed_tasks":[
				{
					"ref":"task_1",
					"name":"Expose planner artifact tools",
					"description":"Make planner artifact tools available through the run-scoped MCP gateway.",
					"task_type":"feature",
					"acceptance_criteria":["Codex can publish a typed task plan artifact."],
					"dependency_refs":[],
					"slice_type":"vertical",
					"implementation_brief":{
						"approach":"Reuse canonical worker validation and persist the resulting preview.",
						"files_to_modify":[{"path":"server/internal/service/agent_tool_gateway.go","action":"modify","description":"Persist planner artifacts from MCP calls."}],
						"test_strategy":["Add a gateway persistence test."]
					}
				}
			],
			"open_questions":[],
			"risks":[]
		}
	}`)
	if resp, err := gateway.CallTool(ctx, token, model.AgentRunToolCallRequest{ToolName: worker.ToolPublishTaskPlan, Input: taskPlanInput}); err != nil || resp == nil || resp.IsError {
		t.Fatalf("publish_task_plan response err=%v resp=%+v", err, resp)
	}
	if resp, err := gateway.CallTool(ctx, token, model.AgentRunToolCallRequest{
		ToolName: worker.ToolRequestApproval,
		Input:    json.RawMessage(`{"phase":"tasks","preview_panel_key":"task_plan","title":"Approve task plan","summary":"Review the proposed implementation slices."}`),
	}); err != nil || resp == nil || resp.IsError {
		t.Fatalf("request_approval response err=%v resp=%+v", err, resp)
	}

	artifacts, err := artifactRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	var foundPlan, foundPreview, foundApproval bool
	for _, artifact := range artifacts {
		switch artifact.ArtifactType {
		case model.AgentRunArtifactTypeRunPlan:
			foundPlan = true
		case worker.RunPreviewArtifactType:
			if artifact.InlineContent == nil {
				t.Fatalf("run_preview artifact missing inline content")
			}
			var preview worker.PublishedPreview
			if err := json.Unmarshal([]byte(*artifact.InlineContent), &preview); err != nil {
				t.Fatalf("decode preview: %v", err)
			}
			foundPreview = preview.PanelKey == "task_plan" && preview.Format == worker.PreviewFormatJSON
		case model.AgentRunArtifactTypeHumanApprovalRequest:
			foundApproval = true
		}
	}
	if !foundPlan || !foundPreview || !foundApproval {
		t.Fatalf("expected run_plan=%v run_preview=%v human_approval_request=%v artifacts=%+v", foundPlan, foundPreview, foundApproval, artifacts)
	}

	interactions, err := interactionRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list interactions: %v", err)
	}
	if len(interactions) != 1 {
		t.Fatalf("expected one pending interaction, got %d", len(interactions))
	}
	if interactions[0].InteractionKind != model.AgentRunInteractionKindApprovalRequest || interactions[0].Status != model.AgentRunInteractionStatusPending {
		t.Fatalf("unexpected interaction: %+v", interactions[0])
	}
	var approval model.ApprovalRequest
	if err := json.Unmarshal(interactions[0].RequestPayload, &approval); err != nil {
		t.Fatalf("decode approval payload: %v", err)
	}
	if approval.Phase != "tasks" || approval.PreviewPanelKey != "task_plan" {
		t.Fatalf("unexpected approval payload: %+v", approval)
	}
}

func TestAgentToolGatewayPersistsUserInputWithSharedQuestionSchema(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	ctx := context.Background()
	interactionRepo := repository.NewAgentRunInteractionRepository(db)
	gateway := &AgentToolGateway{interactionRepo: interactionRepo}
	run := &model.AgentRun{
		ID:          "run-user-input-schema",
		WorkspaceID: "ws-1",
		RuntimeKind: "native_sdk",
	}

	interactionID, err := gateway.createRuntimeUserInputInteraction(ctx, &runToolState{run: run}, &worker.UserInputRequest{
		Questions: []worker.UserInputQuestion{
			{
				ID:       "metric_scope",
				Header:   "Metric scope",
				Question: "Which 3xx responses should this epic measure?",
				IsOther:  true,
				Options: []worker.UserInputOption{
					{Label: "All HTTP 3xx", Description: "Track all redirects."},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("createRuntimeUserInputInteraction returned error: %v", err)
	}

	interaction, err := interactionRepo.GetByID(ctx, run.WorkspaceID, run.ID, interactionID)
	if err != nil {
		t.Fatalf("get interaction: %v", err)
	}
	if interaction == nil {
		t.Fatal("expected interaction")
	}
	if interaction.RequestSchemaVersion != model.AgentRunInteractionSchemaVersionCodexV2 {
		t.Fatalf("expected shared user-input schema %q, got %q", model.AgentRunInteractionSchemaVersionCodexV2, interaction.RequestSchemaVersion)
	}
	var payload worker.UserInputRequest
	if err := json.Unmarshal(interaction.RequestPayload, &payload); err != nil {
		t.Fatalf("decode request payload: %v", err)
	}
	if len(payload.Questions) != 1 || payload.Questions[0].Question != "Which 3xx responses should this epic measure?" {
		t.Fatalf("unexpected user-input payload: %+v", payload)
	}
}

func TestAgentToolGatewayRejectsInvalidTaskPlanArtifact(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	ctx := context.Background()
	now := time.Now()
	mustExec(t, db, `INSERT INTO agents (
		id, workspace_id, is_system, name, preset_key, role, status, runtime_kind,
		skills, trigger_mode, allowed_tools, allowed_commands, allowed_targets, approval_mode,
		max_concurrent_runs, default_invocation_mode, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"agent-gateway-2", "ws-1", true, "Atlas", model.AgentPresetEpicPlanner, "Planner", "idle", "codex",
		[]byte("[]"), "manual", mustMarshalTestJSON(t, []string{worker.ToolPublishTaskPlan}), []byte("[]"), []byte("[]"), "never", 1, model.InvocationModeInteractive, now, now,
	)
	runRepo := repository.NewAgentRunRepository(db)
	run := &model.AgentRun{
		ID:             "run-gateway-2",
		WorkspaceID:    "ws-1",
		AgentID:        "agent-gateway-2",
		TargetType:     "epic",
		TargetID:       "epic-1",
		RuntimeKind:    "codex",
		InvocationMode: model.InvocationModeInteractive,
		ApprovalState:  "not_required",
		PauseReason:    model.AgentRunPauseReasonNone,
		Status:         model.AgentRunStatusRunning,
		Input:          json.RawMessage(`{}`),
		OutputSummary:  json.RawMessage(`{}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := runRepo.Create(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	artifactRepo := repository.NewAgentRunArtifactRepository(db)
	jwtManager := auth.NewJWTManager("test-secret")
	token, err := jwtManager.GenerateAgentRunToolToken(run.ID, run.WorkspaceID, time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	gateway := NewAgentToolGateway(
		runRepo,
		repository.NewAgentRepository(db),
		artifactRepo,
		repository.NewAgentRunInteractionRepository(db),
		NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil),
		jwtManager,
		nil,
	)

	resp, err := gateway.CallTool(ctx, token, model.AgentRunToolCallRequest{
		ToolName: worker.ToolPublishTaskPlan,
		Input:    json.RawMessage(`{"title":"Task Plan","content":{"summary":"missing tasks"}}`),
	})
	if err != nil {
		t.Fatalf("call invalid task plan: %v", err)
	}
	if resp == nil || !resp.IsError {
		t.Fatalf("expected tool error response, got %+v", resp)
	}
	artifacts, err := artifactRepo.ListByRun(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	for _, artifact := range artifacts {
		if artifact.ArtifactType == worker.RunPreviewArtifactType {
			t.Fatalf("invalid task plan should not create run_preview artifact: %+v", artifact)
		}
	}
}

func TestAgentToolGatewayKeepsRuntimeToolsWhenRunNarrowsAllowedTools(t *testing.T) {
	run := &model.AgentRun{
		Input: json.RawMessage(`{"allowed_tools":["read_file","run_command"]}`),
	}
	agent := &model.Agent{
		AllowedTools: mustMarshalTestJSON(t, []string{
			"read_file",
			"run_command",
			worker.ToolUpdatePlan,
			worker.ToolPublishTaskPlan,
			worker.ToolRequestApproval,
		}),
	}

	allowed := effectiveGatewayTools(run, agent)
	for _, required := range []string{worker.ToolUpdatePlan, worker.ToolPublishTaskPlan, worker.ToolRequestApproval} {
		if !allowed[required] {
			t.Fatalf("expected runtime tool %q to stay exposed, got %#v", required, allowed)
		}
	}
	if !allowed["read_file"] || !allowed["run_command"] {
		t.Fatalf("expected run-requested tools to stay exposed, got %#v", allowed)
	}
}
