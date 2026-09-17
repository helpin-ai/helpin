package service

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/helpin-ai/agent-runtime-go"
	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestScheduledSupportFollowUpToolsHaveCatalogAndExecutableProvider(t *testing.T) {
	db := setupAgentRuntimeSupportRunTestDB(t)
	now := time.Now().UTC()
	seedAgentRuntimeSupportAgent(t, db, model.InvocationModeAutonomous, now)
	seedAgentRuntimeSupportConversation(t, db, now)
	mustExec(t, db, `UPDATE agents SET allowed_tools=? WHERE id='agent-1'`, []byte(`["get_support_conversation","list_conversation_messages","send_support_reply"]`))
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	agents := newDelegatedSupportRunService(t, db, runtimeClient)
	followUps := NewSupportFollowUpService(repository.NewSupportFollowUpRepository(db), &SupportChatService{
		agentService: agents, runRepo: repository.NewAgentRunRepository(db), supportAIService: &SupportAIService{},
	})
	settings := model.DefaultSupportInboxSettings()
	settings.AIAgentID = strPtr("agent-1")
	episode := model.SupportAIFollowUp{ID: "episode", WorkspaceID: "ws-1", ConversationID: "conv-1", SourceMessageID: "source", RunID: "reserved-run", CloseHours: 1}
	if err := followUps.launch(context.Background(), episode, &model.SupportConversation{ID: "conv-1", WorkspaceID: "ws-1"}, settings, now); err != nil {
		t.Fatal(err)
	}
	start := runtimeClient.startRunCalls[0]
	var saved model.Agent
	if err := db.First(&saved, "id = ?", "agent-1").Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved.AllowedTools), "finish_support_follow_up") {
		t.Fatal("scheduled projection changed saved agent permissions")
	}
	commands := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	host := NewAgentRuntimeHostService("helpin", nil, nil, nil, nil, nil, nil, nil, nil, nil, commands, nil)
	provider, err := host.ListProviderTools()
	if err != nil {
		t.Fatal(err)
	}
	catalog := agentcontract.ListToolCatalog()
	exposed := make(map[string]agentruntime.Tool)
	for _, tool := range provider.Tools {
		if slices.Contains(start.AllowedTools, tool.Name) {
			exposed[tool.Name] = tool
		}
	}
	for _, name := range []string{"get_support_conversation", "list_conversation_messages", "finish_support_follow_up"} {
		tool, ok := exposed[name]
		if !ok {
			t.Fatalf("scheduled run has no executable provider tool %q", name)
		}
		if len(tool.InputSchema) == 0 {
			t.Fatalf("tool %q missing executable schema", name)
		}
		if !slices.ContainsFunc(catalog.Tools, func(entry model.ToolCatalogEntry) bool { return entry.Name == name }) {
			t.Errorf("scheduled tool %q is missing from shared catalog", name)
		}
	}
	if len(exposed) != 3 || !exposed["finish_support_follow_up"].Mutating {
		t.Fatalf("unexpected scheduled executable tools: %+v", exposed)
	}
	// Dispatch through the real provider mapping and real command executor. An
	// unwired service must reach the executor's dependency guard, not disappear
	// as an unknown tool. Completion behavior is tested by the follow-up service.
	result, err := host.CallProviderTool(context.Background(), agentruntime.ProviderToolCallRequest{
		ToolName: "finish_support_follow_up", Input: json.RawMessage(`{"action":"skip"}`),
		Meta: agentruntime.CommandExecutionContext{AppID: "helpin", WorkspaceID: "ws-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError || len(result.Content) != 1 || !strings.Contains(result.Content[0].Text, "support follow-up service unavailable") {
		t.Fatalf("finish tool did not reach its executor: %+v", result)
	}
}

func TestSupportFollowUpRuntimeProjectionIsSystemScoped(t *testing.T) {
	for _, tc := range []struct {
		name, source, trigger, target string
		wantFinish                    bool
	}{
		{"scheduled", model.AgentRunTriggerSourceSystem, supportFollowUpTriggerType, "support_conversation", true},
		{"user forged trigger", "manual", supportFollowUpTriggerType, "support_conversation", false},
		{"customer message", model.AgentRunTriggerSourceSystem, supportChatTriggerType, "support_conversation", false},
		{"different target", model.AgentRunTriggerSourceSystem, supportFollowUpTriggerType, "workspace", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, err := buildAgentRunInputPayload(tc.target, "conv", &model.AgentRunTriggerContext{Source: tc.source, TriggerType: tc.trigger, Context: json.RawMessage(`{"follow_up_id":"episode"}`)}, nil, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			run := &model.AgentRun{TargetType: tc.target, TargetID: "conv", Input: input}
			// Custom and older agents may have the read tools but no newly shipped completion tool.
			agent := &model.Agent{PresetKey: "custom", AllowedTools: json.RawMessage(`["get_support_conversation","list_conversation_messages"]`)}
			projected, err := runtimeAgentForScheduledSupportFollowUp(run, runtimeAgentFromHelpinAgent(agent, "helpin"))
			if err != nil {
				t.Fatal(err)
			}
			if got := slices.Contains(projected.AllowedTools, "finish_support_follow_up"); got != tc.wantFinish {
				t.Fatalf("finish allowed = %v, want %v", got, tc.wantFinish)
			}
			if strings.Contains(string(agent.AllowedTools), "finish_support_follow_up") {
				t.Fatal("projection mutated saved agent")
			}
		})
	}
}

func TestSupportFollowUpRuntimeRejectsMissingRequiredTools(t *testing.T) {
	input, err := buildAgentRunInputPayload("support_conversation", "conv", &model.AgentRunTriggerContext{Source: model.AgentRunTriggerSourceSystem, TriggerType: supportFollowUpTriggerType, Context: json.RawMessage(`{"follow_up_id":"episode"}`)}, nil, nil, nil, []string{"get_support_conversation", "list_conversation_messages", "finish_support_follow_up"})
	if err != nil {
		t.Fatal(err)
	}
	run := &model.AgentRun{TargetType: "support_conversation", TargetID: "conv", Input: input}
	for _, missing := range []string{"get_support_conversation", "list_conversation_messages", "finish_support_follow_up"} {
		t.Run(missing, func(t *testing.T) {
			tools := []string{"get_support_conversation", "list_conversation_messages", "finish_support_follow_up"}
			tools = slices.DeleteFunc(tools, func(tool string) bool { return tool == missing })
			_, err := runtimeStartRunRequest(run, &model.Agent{}, AgentRuntimeAgent{AllowedTools: tools})
			if err == nil || !strings.Contains(err.Error(), missing) {
				t.Fatalf("expected missing %s to fail before launch, got %v", missing, err)
			}
		})
	}
	_, err = runtimeAgentForScheduledSupportFollowUp(run, AgentRuntimeAgent{AllowedTools: []string{"get_support_conversation"}})
	if err == nil || !strings.Contains(err.Error(), "list_conversation_messages") {
		t.Fatalf("missing read permission should fail projection: %v", err)
	}
}

func TestJevWaitingCustomerLaunchesRestrictedRuntimeWriter(t *testing.T) {
	db := setupAgentRuntimeSupportRunTestDB(t)
	now := time.Now().UTC()
	seedAgentRuntimeSupportAgent(t, db, model.InvocationModeAutonomous, now)
	seedAgentRuntimeSupportConversation(t, db, now)
	mustExec(t, db, `UPDATE agents SET allowed_tools=? WHERE id='agent-1'`, []byte(`["get_support_conversation","list_conversation_messages","send_support_reply"]`))
	runtimeClient := &fakeAgentRuntimeSignalClient{}
	agents := newDelegatedSupportRunService(t, db, runtimeClient)
	svc := NewSupportFollowUpService(repository.NewSupportFollowUpRepository(db), &SupportChatService{agentService: agents, runRepo: repository.NewAgentRunRepository(db), supportAIService: &SupportAIService{}})
	settings := model.DefaultSupportInboxSettings()
	settings.AIAgentID = strPtr("agent-1")
	episode := model.SupportAIFollowUp{ID: "episode", WorkspaceID: "ws-1", ConversationID: "conv-1", SourceMessageID: "source", RunID: "reserved-run", CloseHours: 1}
	if err := svc.launch(context.Background(), episode, &model.SupportConversation{ID: "conv-1", WorkspaceID: "ws-1"}, settings, now, "waiting_customer"); err != nil {
		t.Fatal(err)
	}
	if len(runtimeClient.startRunCalls) != 1 {
		t.Fatal("writer did not launch")
	}
	start := runtimeClient.startRunCalls[0]
	if len(start.AllowedTools) != 3 || !slices.Contains(start.AllowedTools, "finish_support_follow_up") || slices.Contains(start.AllowedTools, "send_support_reply") {
		t.Fatalf("unsafe writer tools: %v", start.AllowedTools)
	}
	saved, err := agents.runRepo.GetByID(context.Background(), "ws-1", "reserved-run")
	if err != nil || saved == nil {
		t.Fatalf("persisted writer missing: %v", err)
	}
	if !strings.Contains(string(saved.Input), "waiting_customer") || !strings.Contains(string(saved.Input), "contradictory evidence") {
		t.Fatalf("classification/safety instructions absent: %s", saved.Input)
	}
}
