package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	flowtemplates "github.com/helpin-ai/helpin/server/internal/templates"
)

// Exercise the actual conversation tools through durable preview, approval,
// persistence, retry and editing. No model or public preview transport is used.
func TestFlowBuilderToolsCreateTemplateAndEdit(t *testing.T) {
	for _, mode := range []string{"custom", "template", "edit"} {
		t.Run(mode, func(t *testing.T) {
			db := setupRuleEngineTestDB(t)
			for _, sql := range []string{
				`CREATE TABLE pm_activity_log(id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),workspace_id TEXT,entity_type TEXT,entity_id TEXT,actor_id TEXT,event_type TEXT,action TEXT,field_name TEXT,old_value TEXT,new_value TEXT,metadata BLOB,created_at DATETIME)`,
				`CREATE TABLE dock_chats(id TEXT PRIMARY KEY,workspace_id TEXT,user_id TEXT,title TEXT,visibility TEXT,flow_builder TEXT,active_run_id TEXT,archived_at DATETIME,updated_at DATETIME)`,
				`CREATE TABLE agents(id TEXT,workspace_id TEXT,is_system BOOLEAN,created_at DATETIME)`,
				`CREATE TABLE agent_runs(id TEXT PRIMARY KEY,workspace_id TEXT,dock_chat_id TEXT,external_runtime TEXT,external_runtime_id TEXT)`,
				`CREATE TABLE agent_run_messages(id TEXT,workspace_id TEXT,dock_chat_id TEXT,role TEXT,actor_user_id TEXT,message_type TEXT,dock_chat_sequence INTEGER,created_at DATETIME)`,
				`CREATE TABLE agent_run_interactions(id TEXT PRIMARY KEY,workspace_id TEXT,run_id TEXT,interaction_kind TEXT,status TEXT,request_payload BLOB,response_payload BLOB,resolved_by TEXT,runtime_metadata BLOB)`,
				`CREATE TABLE workspace_teams(id TEXT,workspace_id TEXT,name TEXT)`,
				`CREATE TABLE pm_workflows(id TEXT,workspace_id TEXT,name TEXT,team_id TEXT,created_at DATETIME)`,
				`INSERT INTO workspaces(id,name,timezone) VALUES('ws','Example','Asia/Kolkata')`,
				`INSERT INTO pm_workflows VALUES('workflow','ws','Delivery',NULL,CURRENT_TIMESTAMP)`,
				`INSERT INTO pm_workflow_states(id,workflow_id,name,state_type,position) VALUES('from','workflow','Review','started',1),('to','workflow','Done','done',2)`,
				`INSERT INTO agent_runs VALUES('run','ws','chat','','')`,
				`INSERT INTO agent_run_messages VALUES('request','ws','chat','user','owner','prompt',1,CURRENT_TIMESTAMP)`,
			} {
				if err := db.Exec(sql).Error; err != nil {
					t.Fatal(err)
				}
			}
			registry, err := flowtemplates.LoadSystemRegistry()
			if err != nil {
				t.Fatal(err)
			}
			rules := repository.NewAutomationRuleRepository(db)
			engine := NewAutomationRuleEngine(rules, nil, nil, nil, nil, nil, nil, nil)
			engine.workflowRepo = repository.NewPMWorkflowRepository(db)
			engine.gitService = &GitService{repoRepo: repository.NewGitRepositoryRepository(db)}
			chats := repository.NewDockChatRepository(db)
			commands := &InternalCommandService{dockChatRepo: chats, agentRunRepo: repository.NewAgentRunRepository(db), agentRunInteractionRepo: repository.NewAgentRunInteractionRepository(db), workspaceRepo: repository.NewWorkspaceRepository(db)}
			svc := (&DockChatService{chatRepo: chats, commandService: commands, agentService: &AgentService{agentRepo: repository.NewAgentRepository(db)}, authz: authorization.NewAuthzService(nil, dockChatAdminMemberRepo{}, dockChatModuleRepo{})}).SetFlowBuilder(engine, registry, flowtemplates.NewInstaller(db, registry))
			commands.SetFlowBuilderChat(svc)
			state := &model.FlowBuilderState{}
			input := json.RawMessage(`{"name":"Move approved work","workflow_id":"workflow","trigger_type":"agent_run.approved","trigger_config":{"state_id":"from"},"action_type":"move_to_state","action_config":{"target_state_id":"to"},"paused":true}`)
			wantID := "chat"
			if mode == "template" {
				state.TemplateKey = "advance_on_approval"
				input = json.RawMessage(`{"template_inputs":{"workflow_id":"workflow","from_state_id":"from","to_state_id":"to"},"summary":"Approved tasks move to **Done**.","paused":true}`)
			}
			if mode == "edit" {
				current, err := engine.createRuleForActor(context.Background(), "ws", "owner", model.CreateAutomationRuleRequest{Name: "Original", WorkflowID: stringPointer("workflow"), TriggerType: model.TriggerAgentRunApproved, TriggerConfig: json.RawMessage(`{"state_id":"from"}`), ActionType: model.ActionMoveToState, ActionConfig: json.RawMessage(`{"target_state_id":"to"}`)}, "existing", true)
				if err != nil {
					t.Fatal(err)
				}
				state.SourceRule = current
				state.Draft = &model.FlowBuilderDraft{Name: current.Name, WorkflowID: current.WorkflowID, TriggerType: current.TriggerType, TriggerConfig: current.TriggerConfig, ActionType: current.ActionType, ActionConfig: current.ActionConfig}
				input = json.RawMessage(`{"name":"Renamed","paused":true}`)
				wantID = "existing"
			}
			raw, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Exec(`INSERT INTO dock_chats VALUES('chat','ws','owner','Flow','private',?,'run',NULL,CURRENT_TIMESTAMP)`, string(raw)).Error; err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			meta := model.InternalCommandContext{WorkspaceID: "ws", ActorID: "owner", RunID: "run"}
			result, err := commands.executeFlowBuilderContext(ctx, meta, json.RawMessage(`{"include_agent_options":true}`))
			if err != nil {
				t.Fatal(err)
			}
			var contextResult struct {
				Catalog flowBuilderCatalog `json:"catalog"`
			}
			if err := json.Unmarshal(result, &contextResult); err != nil {
				t.Fatal(err)
			}
			if contextResult.Catalog.Timezone != "Asia/Kolkata" || len(contextResult.Catalog.Tools) == 0 || len(contextResult.Catalog.Skills) == 0 {
				t.Fatal("missing real context options")
			}
			if _, err := commands.executePreviewFlow(ctx, meta, input); err != nil {
				t.Fatalf("preview: %v", err)
			}
			chat, err := chats.GetByID(ctx, "ws", "chat")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := commands.executeCreateFlow(ctx, meta, json.RawMessage(`{}`)); err == nil {
				t.Fatal("saved without approval")
			}
			approval, err := json.Marshal(map[string]any{"phase": "flow_confirm", "action": map[string]string{"revision": chat.FlowBuilder.Revision}})
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Exec(`INSERT INTO agent_run_interactions VALUES('approval','ws','run','approval_request','resolved',?,CAST('{"decision":"approve"}' AS BLOB),'owner',CAST('{}' AS BLOB))`, []byte(approval)).Error; err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if _, err := commands.executeCreateFlow(ctx, meta, json.RawMessage(`{"approval_interaction_id":"approval"}`)); err != nil {
					t.Fatalf("save attempt %d: %v", i, err)
				}
			}
			saved, err := rules.GetByID(ctx, "ws", wantID)
			if err != nil {
				t.Fatal(err)
			}
			if saved == nil || saved.Enabled {
				t.Fatalf("wrong persisted rule: %+v", saved)
			}
			if mode == "edit" && saved.Name != "Renamed" {
				t.Fatal("edit not saved")
			}
			var count int64
			if err := db.Model(&model.AutomationRule{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatalf("retry duplicated flow: %d", count)
			}
		})
	}
}
