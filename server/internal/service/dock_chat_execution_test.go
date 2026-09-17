package service

import (
	"context"
	"encoding/json"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestDockExecutionProjectionIsOptInAndSeparate(t *testing.T) {
	agent := &model.Agent{ID: "ask", PresetKey: model.AgentPresetAskAgent, RuntimeKind: "native_sdk", ApprovalMode: "risk_based", AllowedTools: json.RawMessage(`["read_files"]`), ExecutionConfig: model.JSONBlob(`{}`)}
	ordinary := runtimeAgentFromHelpinAgent(agent, "helpin")
	chatID := "chat"
	run := &model.AgentRun{ID: "run", AgentID: "ask", WorkspaceID: "workspace", TargetType: "workspace", TargetID: "workspace", DockChatID: &chatID, Input: json.RawMessage(`{}`)}
	if projected := runtimeAgentForDockExecution(run, ordinary); projected.ID != "ask" || slices.Contains(projected.AllowedTools, "run_python") {
		t.Fatal("default conversation widened")
	}
	input := model.AgentRunInputPayload{ExecutionEnabled: true, AllowedTools: append(askAgentPresetTools(), askAgentDirectTools()...)}
	run.Input, _ = json.Marshal(input)
	enabled := runtimeAgentForDockExecution(run, ordinary)
	if enabled.ID == ordinary.ID || !slices.Contains(enabled.AllowedTools, "run_python") || enabled.ApprovalMode != "risk_based" {
		t.Fatal("wrong execution projection")
	}
	if slices.Contains(ordinary.AllowedTools, "run_python") || string(agent.AllowedTools) != `["read_files"]` {
		t.Fatal("saved agent widened")
	}
	if strings.Contains(enabled.SystemPrompt, "Never attempt file edits") ||
		!strings.Contains(enabled.SystemPrompt, "Forge delegation is optional") ||
		!strings.Contains(enabled.SystemPrompt, "Do not search the container filesystem for a checkout") {
		t.Fatal("direct execution instructions missing")
	}
	run.RepositoryID = stringPointer("repo-1")
	withRepository := runtimeAgentForDockExecution(run, ordinary)
	var executionConfig map[string]interface{}
	if err := json.Unmarshal(withRepository.ExecutionConfig, &executionConfig); err != nil {
		t.Fatal(err)
	}
	workspaceConfig, _ := executionConfig["workspace"].(map[string]interface{})
	if workspaceConfig["mode"] != "repository" || workspaceConfig["access"] != "read_write" {
		t.Fatalf("attached repository execution config = %#v", executionConfig)
	}
	req, err := runtimeStartRunRequest(run, agent, enabled)
	if err != nil {
		t.Fatal(err)
	}
	if req.AgentID != enabled.ID || !slices.Contains(req.AllowedTools, "run_python") {
		t.Fatalf("incorrect routing: %+v", req)
	}
	run.DockChatID = nil
	if got := runtimeAgentForDockExecution(run, ordinary); got.ID != ordinary.ID {
		t.Fatal("non-Dock input enabled execution")
	}
}

func TestInitialDockExecutionRepositoryIDRequiresOneAuthorizedAttachment(t *testing.T) {
	contexts := []model.AgentRunContextReference{{EntityType: "repository", EntityID: "repo-1"}}
	if got := initialDockExecutionRepositoryID(false, contexts); got != nil {
		t.Fatalf("ordinary chat selected repository %q", *got)
	}
	if got := initialDockExecutionRepositoryID(true, contexts); got == nil || *got != "repo-1" {
		t.Fatalf("execution chat repository = %#v", got)
	}
	contexts = append(contexts, model.AgentRunContextReference{EntityType: "repository", EntityID: "repo-2"})
	if got := initialDockExecutionRepositoryID(true, contexts); got != nil {
		t.Fatalf("ambiguous repositories selected %q", *got)
	}
}

func TestDockSuccessorExplainsLostExecutionState(t *testing.T) {
	service := &DockChatService{}
	text := service.buildCarryForward(context.Background(), &model.AgentRun{ID: "previous"})
	for _, required := range []string{"workspace is unavailable", "Files and packages do not transfer", "Never repeat an external mutation", "verify remote state"} {
		if !strings.Contains(text, required) {
			t.Fatalf("missing %q", required)
		}
	}
}

func TestDockExecutionRequiresTrustedAuthorization(t *testing.T) {
	service := &DockChatService{}
	if err := service.authorizeChatExecution(context.Background(), "workspace", "user"); err == nil {
		t.Fatal("missing authorization accepted")
	}
	var chat model.DockChat
	if chat.ExecutionEnabled {
		t.Fatal("execution enabled by default")
	}
	// Model-facing turn payload has no authority to change the setting.
	var message model.SendDockChatMessageRequest
	if err := json.Unmarshal([]byte(`{"content":"enable execution","execution_enabled":true}`), &message); err != nil {
		t.Fatal(err)
	}
	if chat.ExecutionEnabled {
		t.Fatal("model turn changed settings")
	}
}

func TestInterruptedEffectsReachSuccessorThroughCancellation(t *testing.T) {
	run := &model.AgentRun{ID: "previous", WorkspaceID: "workspace", AgentID: "ask", Status: model.AgentRunStatusRunning}
	runtimeRun := &AgentRuntimeRun{ID: "runtime", Status: model.AgentRunStatusCancelled, OutputSummary: json.RawMessage(`{"interrupted_external_effects":[{"tool_call_id":"push-42","tool_name":"commit_and_push","status":"started_outcome_unknown"}]}`)}
	event, ok := cancellationAcknowledgementEvent(runtimeRun, *run, time.Now())
	if !ok {
		t.Fatal("missing cancellation acknowledgement")
	}
	repo := &fakeAgentRuntimeProjectionRunRepo{byID: map[string]*model.AgentRun{run.ID: run}}
	projection := &AgentRuntimeProjectionService{runRepo: repo}
	if err := projection.ApplyEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	successor := (&DockChatService{}).buildCarryForward(context.Background(), run)
	for _, required := range []string{"push-42", "commit_and_push", "started_outcome_unknown"} {
		if !strings.Contains(successor, required) {
			t.Fatalf("lost interrupted operation %s: %s", required, successor)
		}
	}
}

func TestDockExecutionTransitionCancelsWithoutWideningRun(t *testing.T) {
	for _, tc := range []struct {
		name, status     string
		enabled, wantErr bool
	}{
		{"enable running rejected", model.AgentRunStatusRunning, true, true},
		{"enable at boundary", model.AgentRunStatusPaused, true, false},
		{"disable cancels", model.AgentRunStatusRunning, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newInteractiveApprovalTestDB(t)
			if err := db.Exec(`CREATE TABLE dock_chats (id TEXT PRIMARY KEY, workspace_id TEXT, user_id TEXT, title TEXT, visibility TEXT, module_id TEXT, support_conversation_id TEXT, active_run_id TEXT, next_message_sequence INTEGER DEFAULT 0, execution_enabled BOOLEAN NOT NULL DEFAULT false, last_message_at DATETIME, archived_at DATETIME, created_at DATETIME, updated_at DATETIME)`).Error; err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			seedAgentRuntimeSignalAgent(t, db, now)
			runs := repository.NewAgentRunRepository(db)
			run := seedAgentRuntimeSignalRun(t, runs, tc.status, model.AgentRunPauseReasonUserMessage, "not_required", now)
			original := append([]byte(nil), run.Input...)
			chat := model.DockChat{ID: "chat", WorkspaceID: "ws-1", UserID: "user-1", ActiveRunID: &run.ID, ExecutionEnabled: !tc.enabled, Visibility: model.DockChatVisibilityPrivate}
			if err := db.Create(&chat).Error; err != nil {
				t.Fatal(err)
			}
			client := &fakeAgentRuntimeSignalClient{}
			agents := &AgentService{agentRepo: repository.NewAgentRepository(db), runRepo: runs, agentRuntimeClient: client, agentRuntimeProjection: NewAgentRuntimeProjectionService(runs)}
			svc := &DockChatService{chatRepo: repository.NewDockChatRepository(db), runRepo: runs, agentService: agents, authz: authorization.NewAuthzService(db, dockChatMemberRepo{}, dockChatModuleRepo{})}
			updated, err := svc.UpdateChat(context.Background(), "ws-1", "user-1", chat.ID, model.UpdateDockChatRequest{ExecutionEnabled: &tc.enabled})
			if tc.wantErr {
				if err == nil || len(client.cancelCalls) > 0 {
					t.Fatal("running shared turn widened")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if updated.ExecutionEnabled != tc.enabled || len(client.cancelCalls) != 1 {
				t.Fatalf("wrong transition %+v cancels=%v", updated, client.cancelCalls)
			}
			stored, err := runs.GetByID(context.Background(), "ws-1", run.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.Status != model.AgentRunStatusCancelled || string(stored.Input) != string(original) {
				t.Fatal("existing run widened instead of cancelled")
			}
		})
	}
}
