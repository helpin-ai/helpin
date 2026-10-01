package service

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestFlowBuilderToolsCannotExecuteWork(t *testing.T) {
	tools := flowBuilderTools()
	for _, forbidden := range []string{"start_agent_run", "create_custom_agent", "run_command", "prepare_dock_execution", "create_task"} {
		if slices.Contains(tools, forbidden) {
			t.Fatalf("builder grants %s", forbidden)
		}
	}
	for _, required := range []string{"get_flow_builder_context", "preview_flow", "request_user_input"} {
		if !slices.Contains(tools, required) {
			t.Fatalf("missing %s", required)
		}
	}
}

func TestFlowBuilderRejectsUnknownReferencesAndUnsupportedConfig(t *testing.T) {
	catalog := flowBuilderCatalog{
		Agents:       []flowBuilderAgent{{ID: "agent", Name: "Reviewer", Targets: []string{"repository"}}},
		Repositories: []flowBuilderRepository{{ID: "repo", FullName: "example/app"}},
	}
	valid := model.FlowBuilderDraft{Name: "Review pull requests", TriggerType: "github.pull_request_opened", TriggerConfig: json.RawMessage(`{"repo_full_name":"example/app"}`), ActionType: "start_agent_run", ActionConfig: json.RawMessage(`{"agent_id":"agent","target_type":"repository","target_id":"repo"}`)}
	tests := []struct {
		name    string
		change  func(*model.FlowBuilderDraft)
		wantErr bool
	}{
		{"valid", func(*model.FlowBuilderDraft) {}, false},
		{"unknown agent", func(d *model.FlowBuilderDraft) {
			d.ActionConfig = json.RawMessage(`{"agent_id":"other","target_type":"repository","target_id":"repo"}`)
		}, true},
		{"unknown repository", func(d *model.FlowBuilderDraft) {
			d.TriggerConfig = json.RawMessage(`{"repo_full_name":"other/private"}`)
		}, true},
		{"invented condition", func(d *model.FlowBuilderDraft) {
			d.TriggerConfig = json.RawMessage(`{"repo_full_name":"example/app","file_path":"auth/**"}`)
		}, true},
		{"empty name", func(d *model.FlowBuilderDraft) { d.Name = " " }, true},
		{"unsupported action", func(d *model.FlowBuilderDraft) { d.ActionType = "run_command" }, true},
		{"unknown team", func(d *model.FlowBuilderDraft) { d.TeamID = stringPointer("other-team") }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := valid
			tt.change(&d)
			err := validateFlowBuilderCatalog(d, catalog)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestFlowBuilderPausedCreationStaysPaused(t *testing.T) {
	db := setupRuleEngineTestDB(t)
	engine := NewAutomationRuleEngine(repository.NewAutomationRuleRepository(db), nil, nil, nil, nil, nil, nil, nil)
	rule, err := engine.createRuleForActor(context.Background(), "ws", "owner", model.CreateAutomationRuleRequest{Name: "Paused", TriggerType: model.TriggerTaskStateEntered, TriggerConfig: json.RawMessage(`{"state_id":"todo"}`), ActionType: model.ActionMoveToState, ActionConfig: json.RawMessage(`{"target_state_id":"done"}`)}, "flow", false)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := engine.GetRule(context.Background(), "ws", rule.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rule.Enabled || saved.Enabled {
		t.Fatal("paused flow became active")
	}
}

func TestFlowBuilderEditPreservesIdentityAndRejectsConcurrentChange(t *testing.T) {
	db := setupRuleEngineTestDB(t)
	repo := repository.NewAutomationRuleRepository(db)
	engine := NewAutomationRuleEngine(repo, nil, nil, nil, nil, nil, nil, nil)
	current, err := engine.createRuleForActor(context.Background(), "ws", "owner", model.CreateAutomationRuleRequest{Name: "Original", TriggerType: model.TriggerTaskStateEntered, TriggerConfig: json.RawMessage(`{"state_id":"todo"}`), ActionType: model.ActionMoveToState, ActionConfig: json.RawMessage(`{"target_state_id":"done"}`)}, "existing-flow", true)
	if err != nil {
		t.Fatal(err)
	}
	originalTime := current.UpdatedAt
	name := "Revised"
	saved, err := engine.updateRule(context.Background(), "ws", current.ID, model.UpdateAutomationRuleRequest{Name: &name}, &originalTime)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID != current.ID || string(saved.ActionConfig) != string(current.ActionConfig) {
		t.Fatal("edit changed identity or untouched configuration")
	}
	staleName := "Stale change"
	if _, err := engine.updateRule(context.Background(), "ws", current.ID, model.UpdateAutomationRuleRequest{Name: &staleName}, &originalTime); err == nil {
		t.Fatal("stale edit overwrote changes")
	}
	latest, _ := engine.GetRule(context.Background(), "ws", current.ID)
	if latest.Name != name {
		t.Fatal("stale edit was persisted")
	}
}

func TestFlowBuilderApprovalMustMatchOwnerAndLatestDraft(t *testing.T) {
	db := newInteractiveApprovalTestDB(t)
	repo := repository.NewAgentRunInteractionRepository(db)
	commands := &InternalCommandService{agentRunInteractionRepo: repo}
	state := &model.FlowBuilderState{Revision: "current", UserMessageID: "request", Draft: &model.FlowBuilderDraft{Name: "Reviewed"}}
	for _, tc := range []struct {
		name, status, decision, owner, revision, phase, latest string
		wantErr                                                bool
	}{
		{"approved", "resolved", "approve", "owner", "current", "flow_confirm", "request", false},
		{"pending", "pending", "approve", "owner", "current", "flow_confirm", "request", true},
		{"rejected", "resolved", "reject", "owner", "current", "flow_confirm", "request", true},
		{"changes requested", "resolved", "request_changes", "owner", "current", "flow_confirm", "request", true},
		{"another user", "resolved", "approve", "other", "current", "flow_confirm", "request", true},
		{"old revision", "resolved", "approve", "owner", "old", "flow_confirm", "request", true},
		{"unrelated approval", "resolved", "approve", "owner", "current", "dock_plan_confirm", "request", true},
		{"new user request", "resolved", "approve", "owner", "current", "flow_confirm", "new-request", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request, _ := json.Marshal(map[string]any{"phase": tc.phase, "action": map[string]string{"revision": tc.revision}})
			response, _ := json.Marshal(map[string]string{"decision": tc.decision})
			interaction := &model.AgentRunInteraction{ID: tc.name, WorkspaceID: "ws", RunID: "run", RuntimeKind: "native_sdk", InteractionKind: "approval_request", Status: tc.status, RequestSchemaVersion: "v1", RequestPayload: request, ResponsePayload: response, ResolvedBy: &tc.owner, RuntimeMetadata: json.RawMessage(`{}`)}
			if err := db.Create(interaction).Error; err != nil {
				t.Fatal(err)
			}
			got, action, err := commands.resolvedDockApprovalAction(context.Background(), model.InternalCommandContext{WorkspaceID: "ws"}, &model.AgentRun{ID: "run"}, tc.name, "flow_confirm")
			if err == nil {
				err = validateFlowApproval(state, "owner", got, action, tc.latest)
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestFlowBuilderRequiresAutomationAdminAndPrivateOwner(t *testing.T) {
	chat := &model.DockChat{ID: "chat", WorkspaceID: "ws", UserID: "owner", FlowBuilder: &model.FlowBuilderState{}}
	svc := &DockChatService{flowBuilder: &flowBuilderDependencies{}, authz: authorization.NewAuthzService(nil, dockChatAdminMemberRepo{}, dockChatModuleRepo{})}
	if _, err := svc.authorizeFlowBuilder(context.Background(), chat, "owner"); err != nil {
		t.Fatalf("admin owner rejected: %v", err)
	}
	if _, err := svc.authorizeFlowBuilder(context.Background(), chat, "other"); err == nil {
		t.Fatal("another user accepted")
	}
	svc.authz = authorization.NewAuthzService(nil, dockChatMemberRepo{}, dockChatModuleRepo{})
	if _, err := svc.authorizeFlowBuilder(context.Background(), chat, "owner"); err == nil {
		t.Fatal("non-admin accepted")
	}
}
