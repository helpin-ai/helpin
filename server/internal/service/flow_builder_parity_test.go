package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	flowtemplates "github.com/helpin-ai/helpin/server/internal/templates"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestFlowTemplateDependentReferences(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "flow.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`CREATE TABLE docs_spaces(id TEXT, workspace_id TEXT, type TEXT, deleted_at DATETIME)`,
		`CREATE TABLE docs_collections(id TEXT, workspace_id TEXT, space_id TEXT, deleted_at DATETIME)`,
		`INSERT INTO docs_spaces VALUES ('internal','ws','internal',NULL),('public','ws','external_capable',NULL),('deleted','ws','internal',CURRENT_TIMESTAMP)`,
		`INSERT INTO docs_collections VALUES ('collection','ws','internal',NULL),('deleted','ws','internal',CURRENT_TIMESTAMP)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := &DockChatService{chatRepo: repository.NewDockChatRepository(db)}
	c := flowBuilderCatalog{Workflows: []model.WorkflowWithStates{{Workflow: model.PMWorkflow{ID: "workflow", TeamID: stringPointer("team")}, States: []model.PMWorkflowState{{ID: "done"}}}}}
	tests := []struct {
		name    string
		input   flowtemplates.Input
		values  map[string]any
		wantErr bool
	}{
		{"team state", flowtemplates.Input{Key: "state", Type: "workflow_state", DependsOn: "team_id"}, map[string]any{"team_id": "team", "state": "done"}, false},
		{"wrong team", flowtemplates.Input{Key: "state", Type: "workflow_state", DependsOn: "team_id"}, map[string]any{"team_id": "other", "state": "done"}, true},
		{"workflow state", flowtemplates.Input{Key: "state", Type: "workflow_state", DependsOn: "workflow_id"}, map[string]any{"workflow_id": "workflow", "state": "done"}, false},
		{"missing dependency", flowtemplates.Input{Key: "state", Type: "workflow_state", DependsOn: "team_id"}, map[string]any{"state": "done"}, true},
		{"hidden state", flowtemplates.Input{Key: "state", Type: "workflow_state", DependsOn: "team_id", ShowIf: "create_task=true"}, map[string]any{"state": "old", "create_task": false}, false},
		{"deleted space", flowtemplates.Input{Key: "space", Type: "space"}, map[string]any{"space": "deleted"}, true},
		{"deleted collection", flowtemplates.Input{Key: "collection", Type: "collection", DependsOn: "space_id"}, map[string]any{"space_id": "internal", "collection": "deleted"}, true},
		{"internal space", flowtemplates.Input{Key: "space", Type: "space"}, map[string]any{"space": "internal"}, false},
		{"wrong space type", flowtemplates.Input{Key: "space", Type: "space"}, map[string]any{"space": "public"}, true},
		{"any space", flowtemplates.Input{Key: "space", Type: "space", SpaceType: "any"}, map[string]any{"space": "public"}, false},
		{"collection", flowtemplates.Input{Key: "collection", Type: "collection", DependsOn: "space_id"}, map[string]any{"space_id": "internal", "collection": "collection"}, false},
		{"wrong collection space", flowtemplates.Input{Key: "collection", Type: "collection", DependsOn: "space_id"}, map[string]any{"space_id": "public", "collection": "collection"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := flowtemplates.Template{Inputs: []flowtemplates.Input{tt.input}}
			for key := range tt.values {
				if key != tt.input.Key {
					tmpl.Inputs = append(tmpl.Inputs, flowtemplates.Input{Key: key, Type: "string"})
				}
			}
			err := svc.validateFlowTemplateReferences(context.Background(), "ws", tmpl, tt.values, c)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error=%v wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestFlowBuilderAgentOverrideContract(t *testing.T) {
	properties := flowBuilderDraftSchema()["properties"].(map[string]any)
	overrides := properties["agent_overrides"].(map[string]any)
	fields, ok := overrides["properties"].(map[string]any)
	if !ok {
		t.Fatal("agent overrides have no discoverable fields")
	}
	for _, key := range []string{"system_prompt", "allowed_tools", "skills", "allowed_targets", "approval_mode", "max_concurrent_runs"} {
		if fields[key] == nil {
			t.Errorf("missing agent setting %s", key)
		}
	}
}

func TestFlowScheduleReviewUsesWorkspaceTimezone(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	draft := model.FlowBuilderDraft{TriggerType: model.TriggerCron, TriggerConfig: json.RawMessage(`{"schedule":"0 4 * * 1-5"}`)}
	state := &model.FlowBuilderState{}
	if err := setFlowSchedulePreview(state, &draft, "Asia/Kolkata", now); err != nil {
		t.Fatal(err)
	}
	if state.Timezone != "Asia/Kolkata" || len(state.NextRuns) != 3 || state.NextRuns[0] != "2026-10-02T09:30:00+05:30" {
		t.Fatalf("incorrect local schedule: %+v", state)
	}
	draft.ScheduleTimezone = "Local"
	if err := setFlowSchedulePreview(state, &draft, "UTC", now); err == nil {
		t.Fatal("invalid timezone accepted")
	}
}

func TestFlowAgentOverridesRejectUnsupportedSettings(t *testing.T) {
	badMode := "invented"
	zero := 0
	for _, tt := range []struct {
		name      string
		overrides *model.CreateAgentFromTemplateOverrides
		creates   bool
		wantErr   bool
	}{
		{"defaults", nil, true, false},
		{"existing agent", &model.CreateAgentFromTemplateOverrides{SystemPrompt: stringPointer("Changed")}, false, true},
		{"invalid approval", &model.CreateAgentFromTemplateOverrides{ApprovalMode: &badMode}, true, true},
		{"zero runs", &model.CreateAgentFromTemplateOverrides{MaxConcurrentRuns: &zero}, true, true},
		{"unknown target", &model.CreateAgentFromTemplateOverrides{AllowedTargets: model.JSONBlob(`["invented"]`)}, true, true},
		{"unsupported model override", &model.CreateAgentFromTemplateOverrides{Model: stringPointer("other")}, true, true},
		{"supported approvals", &model.CreateAgentFromTemplateOverrides{ApprovalMode: stringPointer("always")}, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFlowAgentOverrides(model.FlowBuilderDraft{AgentOverrides: tt.overrides}, tt.creates)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error=%v wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestFlowBuilderEditingTemplatePreservesInstallerFields(t *testing.T) {
	c := flowBuilderCatalog{Agents: []flowBuilderAgent{{ID: "agent", Targets: []string{"repository"}}}, Repositories: []flowBuilderRepository{{ID: "repo", FullName: "example/app"}}}
	original := model.AutomationRule{ActionType: model.ActionStartAgentRun, ActionConfig: json.RawMessage(`{"agent_id":"agent","target_type":"repository","target_id":"repo","include_prerelease":true}`)}
	d := model.FlowBuilderDraft{Name: "Updated name", TriggerType: model.TriggerGitHubReleasePub, TriggerConfig: json.RawMessage(`{"repo_full_name":"example/app"}`), ActionType: model.ActionStartAgentRun, ActionConfig: original.ActionConfig}
	// Installer-owned parameters must not prevent a rename or unrelated edit.
	c.SourceRule = &original
	if err := validateFlowBuilderCatalog(d, c); err != nil {
		t.Fatal(err)
	}
	d.ActionConfig = json.RawMessage(`{"agent_id":"agent","target_type":"repository","target_id":"repo","invented_parameter":true}`)
	if err := validateFlowBuilderCatalog(d, c); err == nil {
		t.Fatal("invented action setting accepted")
	}
}

func TestFlowDraftConversationEditsPreserveAndClearSettings(t *testing.T) {
	previous := &model.FlowBuilderDraft{Name: "Original", Paused: true, Description: stringPointer("Keep this"), TriggerType: model.TriggerGitHubPROpened, TriggerConfig: json.RawMessage(`{"repo_full_name":"example/app","base_branch":"main","semantic_condition":{"text":"Authentication changes"}}`), SemanticCondition: "Authentication changes", ActionType: model.ActionStartAgentRun, ActionConfig: json.RawMessage(`{"agent_id":"agent","additional_context":"Keep context"}`)}
	for _, tc := range []struct {
		name, patch string
		condition   bool
	}{
		{"rename", `{"name":"Revised"}`, true},
		{"clear condition", `{"name":"Revised","semantic_condition":""}`, false},
		{"remove config condition", `{"name":"Revised","trigger_config":{"semantic_condition":null}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := decodeFlowDraft(json.RawMessage(tc.patch), previous)
			if err != nil {
				t.Fatal(err)
			}
			if d.Name != "Revised" || !d.Paused || derefString(d.Description) != "Keep this" || string(d.ActionConfig) != string(previous.ActionConfig) {
				t.Fatalf("unrelated settings lost: %+v", d)
			}
			var cfg map[string]any
			if err := json.Unmarshal(d.TriggerConfig, &cfg); err != nil {
				t.Fatal(err)
			}
			if (cfg["semantic_condition"] != nil) != tc.condition || (d.SemanticCondition != "") != tc.condition {
				t.Fatalf("incorrect condition: %s, %q", d.TriggerConfig, d.SemanticCondition)
			}
		})
	}
	if previous.Name != "Original" || previous.SemanticCondition == "" {
		t.Fatal("source was mutated")
	}
}

func TestFlowBuilderSupportsEveryFormerTrigger(t *testing.T) {
	c := flowBuilderCatalog{WorkspaceID: "ws", Agents: []flowBuilderAgent{{ID: "agent", Targets: []string{"repository", "workspace", "task"}}}, Repositories: []flowBuilderRepository{{ID: "repo", FullName: "example/app"}}, Workflows: []model.WorkflowWithStates{{Workflow: model.PMWorkflow{ID: "workflow"}, States: []model.PMWorkflowState{{ID: "from"}, {ID: "to"}}}}}
	engine := &AutomationRuleEngine{}
	for _, trigger := range []string{model.TriggerTaskStateEntered, model.TriggerAgentRunApproved, model.TriggerGitHubPush, model.TriggerGitLabPush, model.TriggerGitHubPROpened, model.TriggerGitHubPRMerged, model.TriggerGitHubPRClosed, model.TriggerGitHubPRReviewReq, model.TriggerGitLabMROpened, model.TriggerGitLabMRMerged, model.TriggerGitLabMRClosed, model.TriggerGitHubReleasePub, model.TriggerGitLabReleasePub, model.TriggerGitHubCheckSuite, model.TriggerGitLabPipeline, model.TriggerCron} {
		t.Run(trigger, func(t *testing.T) {
			d := model.FlowBuilderDraft{Name: "Configured flow", TriggerType: trigger, ActionType: model.ActionStartAgentRun, ActionConfig: json.RawMessage(`{"agent_id":"agent","target_type":"repository","target_id":"repo","base_branch":"main","working_branch":"flow/work","additional_context":"Review carefully"}`)}
			fields := map[string]any{}
			for _, key := range flowTriggerFields(trigger) {
				switch key {
				case "repo_full_name":
					fields[key] = "example/app"
				case "branch", "base_branch":
					fields[key] = "main"
				case "conclusion":
					fields[key] = "failure"
				case "tag_name":
					fields[key] = "v1.0"
				case "tag_pattern":
					fields[key] = "v*"
				case "release_kinds":
					fields[key] = []string{"release"}
				case "include_prerelease":
					fields[key] = false
				case "state_id":
					fields[key] = "from"
					d.WorkflowID = stringPointer("workflow")
					d.ActionConfig = json.RawMessage(`{"agent_id":"agent"}`)
				case "schedule":
					fields[key] = "30 9 * * 1-5"
					d.ActionConfig = json.RawMessage(`{"agent_id":"agent","target_type":"workspace","target_id":"ws"}`)
				}
			}
			var err error
			d.TriggerConfig, err = json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateFlowBuilderCatalog(d, c); err != nil {
				t.Fatalf("catalog: %v", err)
			}
			if err := engine.validateRuleRequest(d.TriggerType, d.TriggerConfig, d.ActionType, d.ActionConfig); err != nil {
				t.Fatalf("engine: %v", err)
			}
		})
	}
	for _, action := range []struct{ kind, config string }{{model.ActionMoveToState, `{"target_state_id":"to"}`}, {model.ActionMergeBranch, `{"target_branch":"main"}`}} {
		t.Run(action.kind, func(t *testing.T) {
			d := model.FlowBuilderDraft{Name: "Task flow", WorkflowID: stringPointer("workflow"), TriggerType: model.TriggerAgentRunApproved, TriggerConfig: json.RawMessage(`{"state_id":"from"}`), ActionType: action.kind, ActionConfig: json.RawMessage(action.config)}
			if err := validateFlowBuilderCatalog(d, c); err != nil {
				t.Fatal(err)
			}
		})
	}
}
