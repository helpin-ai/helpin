package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/agentcontract"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestPMToolCatalogExecutorParity(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	hostAliases := map[string]string{}
	hostPMAliases := map[string]string{}
	for _, def := range svc.ToolDefinitions() {
		if def.Tool == nil {
			continue
		}
		alias, category := strings.TrimSpace(def.Tool.Alias), strings.TrimSpace(def.Tool.Category)
		if def.Execute == nil {
			t.Fatalf("exposed command %q has no executor", def.Name)
		}
		if def.Tool.CommandName != def.Name {
			t.Fatalf("command %q metadata points to %q", def.Name, def.Tool.CommandName)
		}
		if previous, exists := hostAliases[alias]; exists {
			t.Fatalf("duplicate executor alias %q for %s and %s", alias, previous, def.Name)
		}
		hostAliases[alias] = def.Name
		if strings.HasPrefix(category, "PM /") || alias == "list_workspace_members" || alias == "list_workspace_teams" {
			hostPMAliases[alias] = def.Name
		}
	}
	catalogAliases := map[string]struct{}{}
	for _, tool := range agentcontract.ListToolCatalog().Tools {
		alias, category := strings.TrimSpace(tool.Name), strings.TrimSpace(tool.Category)
		if !strings.HasPrefix(category, "PM /") && alias != "list_workspace_members" && alias != "list_workspace_teams" {
			continue
		}
		if _, exists := catalogAliases[alias]; exists {
			t.Fatalf("duplicate catalog alias %q", alias)
		}
		catalogAliases[alias] = struct{}{}
	}
	nativeRuntimeAllowlist := map[string]struct{}{"list_epic_tasks": {}}
	for alias := range catalogAliases {
		if _, allowed := nativeRuntimeAllowlist[alias]; allowed {
			continue
		}
		if _, ok := hostAliases[alias]; !ok {
			t.Errorf("catalog alias %q has no executor", alias)
		}
	}
	for alias := range hostPMAliases {
		if _, ok := catalogAliases[alias]; !ok {
			t.Errorf("executor alias %q is absent from catalog", alias)
		}
	}
	if commandName := hostAliases["assign_task_agent"]; commandName == "" {
		t.Error("deprecated assign_task_agent compatibility alias is missing")
	}
}

func TestInternalCommandExecutionTreatsTargetAsContextNotAuthorization(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	var gotMeta model.InternalCommandContext
	svc.register(InternalCommandDefinition{
		Name:                 "test.target_context",
		SupportedTargetTypes: []string{"workspace"},
		Execute: func(_ context.Context, meta model.InternalCommandContext, _ json.RawMessage) (json.RawMessage, error) {
			gotMeta = meta
			return json.RawMessage(`{"ok":true}`), nil
		},
	})

	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "workspace-1",
		TargetType:  "repository",
		TargetID:    "repository-1",
	}, "test.target_context", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("repository-targeted command returned error: %v", err)
	}
	if string(output) != `{"ok":true}` {
		t.Fatalf("output = %s", output)
	}
	if gotMeta.TargetType != "repository" || gotMeta.TargetID != "repository-1" {
		t.Fatalf("target context was not preserved: %#v", gotMeta)
	}
}

func TestSafeOperationalToolExecutorParity(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	expected := []string{
		"update_task_delivery_target", "update_epic_delivery_target", "update_document_metadata",
		"get_crm_contact", "get_crm_company", "get_crm_deal", "list_crm_companies", "list_crm_pipelines", "list_crm_associations",
		"update_crm_contact", "update_crm_company", "update_crm_deal", "add_crm_activity", "link_crm_objects", "unlink_crm_association", "set_primary_contact_company",
		"list_support_conversations", "get_support_conversation", "list_support_tags", "list_support_inboxes", "list_support_assignees",
		"assign_support_conversation", "move_support_conversation", "add_support_conversation_tag", "remove_support_conversation_tag", "link_support_conversation_task", "link_support_conversation_contact", "update_support_conversation_subject",
	}
	aliases := map[string]InternalCommandDefinition{}
	for _, def := range svc.ToolDefinitions() {
		aliases[def.Tool.Alias] = def
	}
	for _, alias := range expected {
		def, ok := aliases[alias]
		if !ok || def.Execute == nil {
			t.Errorf("safe operational alias %q has no executor", alias)
		}
	}
}

func TestGroupDockCapabilitiesSupportsSelfExecutionDecision(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	summary := svc.groupDockCapabilities([]string{
		"find_skills", "read_skill",
		"checkout_repositories", "read_files", "repository_search",
		"create_document", "prepare_dock_execution",
	})
	if summary["can_load_skills"] != true {
		t.Fatalf("can_load_skills = %#v, want true", summary["can_load_skills"])
	}
	if summary["can_read_repositories"] != true {
		t.Fatalf("can_read_repositories = %#v, want true", summary["can_read_repositories"])
	}
	if summary["can_execute_product_mutations"] != true {
		t.Fatalf("can_execute_product_mutations = %#v, want true", summary["can_execute_product_mutations"])
	}
}

func TestWriteDocumentContentCommandSupportsDocumentTarget(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)

	def, ok := svc.Definition("docs.write_document_content")
	if !ok {
		t.Fatal("expected docs.write_document_content definition")
	}

	for _, required := range []string{"workspace", "document"} {
		if !slices.Contains(def.SupportedTargetTypes, required) {
			t.Fatalf("expected docs.write_document_content to support target type %q, got %#v", required, def.SupportedTargetTypes)
		}
	}
	proposal, ok := svc.Definition("docs.publish_document_change_proposal")
	if !ok || !slices.Contains(proposal.SupportedTargetTypes, "workspace") {
		t.Fatalf("expected document proposal to support workspace Dock target, got %#v", proposal.SupportedTargetTypes)
	}
	block, ok := svc.Definition("docs.update_document_block")
	if !ok || !slices.Contains(block.SupportedTargetTypes, "workspace") {
		t.Fatalf("expected block update to support workspace Dock target, got %#v", block.SupportedTargetTypes)
	}
	link, ok := svc.Definition("docs.link_document_to_object")
	if !ok || !slices.Contains(link.SupportedTargetTypes, "workspace") {
		t.Fatalf("expected document linking to support workspace Dock target, got %#v", link.SupportedTargetTypes)
	}
}

func TestDocumentationCoverageGapToolsAcceptCoverageTarget(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	commandsByAlias := make(map[string]InternalCommandDefinition)
	for _, def := range svc.ToolDefinitions() {
		commandsByAlias[def.Tool.Alias] = def
	}
	profile := agentcontract.GetRuntimeProfile(model.AgentPresetDocumentationAgent)
	for _, toolName := range profile.AllowedTools {
		def, ok := commandsByAlias[toolName]
		if !ok || len(def.SupportedTargetTypes) == 0 {
			continue
		}
		// Knowledge search is useful when Quill is launched on a specific
		// support conversation, but it cannot infer one conversation from an
		// aggregated coverage-gap target. The gap run receives its evidence in
		// launch context instead.
		if toolName == "search_knowledge" {
			if !slices.Contains(def.SupportedTargetTypes, "support_conversation") {
				t.Errorf("documentation tool %q (%s) rejects support_conversation", toolName, def.Name)
			}
			continue
		}
		if !slices.Contains(def.SupportedTargetTypes, "support_coverage_gap") {
			t.Errorf("documentation tool %q (%s) rejects support_coverage_gap", toolName, def.Name)
		}
	}
}

func TestCompleteSupportCoverageGapToolContract(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	def, ok := svc.Definition("support.complete_coverage_gap")
	if !ok || def.Tool == nil {
		t.Fatal("complete support coverage gap tool is not registered")
	}
	if !def.Mutating || def.Tool.Alias != agentcontract.ToolCompleteSupportCoverageGap {
		t.Fatalf("unexpected terminal tool metadata: %#v", def)
	}
	if !slices.Equal(def.SupportedTargetTypes, []string{"support_coverage_gap"}) {
		t.Fatalf("terminal tool targets = %#v", def.SupportedTargetTypes)
	}
	if def.Tool.InputSchema["additionalProperties"] != false {
		t.Fatalf("terminal tool schema is not strict: %#v", def.Tool.InputSchema)
	}
	required, _ := def.Tool.InputSchema["required"].([]string)
	for _, field := range []string{"outcome", "action", "source_status", "documentation_evidence", "summary"} {
		if !slices.Contains(required, field) {
			t.Fatalf("terminal tool schema does not require %q: %#v", field, def.Tool.InputSchema)
		}
	}
	properties, _ := def.Tool.InputSchema["properties"].(map[string]any)
	if properties["handoff_owner"] == nil || properties["source_evidence"] == nil {
		t.Fatalf("terminal tool schema is missing disposition evidence fields: %#v", properties)
	}
}

func TestEnsureTaskPlanDocumentToolContractAttachesAndReturnsDocumentID(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)

	def, ok := svc.Definition("docs.ensure_task_plan_doc")
	if !ok || def.Tool == nil {
		t.Fatal("expected docs.ensure_task_plan_doc runtime tool definition")
	}
	if !strings.Contains(def.Tool.Description, "attach it to that task") || !strings.Contains(def.Tool.Description, "document_id") {
		t.Fatalf("unexpected ensure task plan document description %q", def.Tool.Description)
	}
	schema := def.Tool.InputSchema
	if schema["additionalProperties"] != false {
		t.Fatalf("expected closed empty-object schema, got %#v", def.Tool.InputSchema)
	}
}

func TestEnsureEpicSpecDocumentToolContractAttachesAndReturnsDocumentID(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)

	def, ok := svc.Definition("docs.ensure_spec_doc")
	if !ok || def.Tool == nil {
		t.Fatal("expected docs.ensure_spec_doc runtime tool definition")
	}
	if !strings.Contains(def.Tool.Description, "attach it to that epic") || !strings.Contains(def.Tool.Description, "document_id") {
		t.Fatalf("unexpected ensure epic spec document description %q", def.Tool.Description)
	}
	if def.Tool.InputSchema["additionalProperties"] != false {
		t.Fatalf("expected closed empty-object schema, got %#v", def.Tool.InputSchema)
	}
}

func TestTaskDependencyGraphHasCycle(t *testing.T) {
	if taskDependencyGraphHasCycle(map[string][]string{"task-a": {"task-b"}, "task-b": {"task-c"}}) {
		t.Fatal("acyclic dependency graph reported a cycle")
	}
	if !taskDependencyGraphHasCycle(map[string][]string{"task-a": {"task-b"}, "task-b": {"task-c"}, "task-c": {"task-a"}}) {
		t.Fatal("cyclic dependency graph was accepted")
	}
}

func TestDocsOrganizationCommandsAreExposedToRuntimeAgents(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	for _, commandName := range []string{"docs.create_space", "docs.create_collection", "docs.update_space", "docs.update_collection", "docs.move_document"} {
		definition, ok := svc.Definition(commandName)
		if !ok {
			t.Fatalf("missing command %q", commandName)
		}
		if definition.Tool == nil || definition.Tool.Alias == "" || !definition.Mutating {
			t.Fatalf("command %q metadata = %#v", commandName, definition.Tool)
		}
	}
}

func TestWriteDocumentContentCommandRejectsEmptyContent(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)

	def, ok := svc.Definition("docs.write_document_content")
	if !ok {
		t.Fatal("expected docs.write_document_content definition")
	}

	_, err := def.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		TargetType:  "document",
		TargetID:    "doc-1",
	}, []byte(`{"document_id":"doc-1","content":{"type":"doc","content":[]}}`))
	if err == nil {
		t.Fatal("expected empty document content to be rejected")
	}
	if !strings.Contains(err.Error(), "content must not be empty") {
		t.Fatalf("expected empty content error, got %v", err)
	}
}

func TestCommandToolMetadataUsesExplicitAliasInsteadOfBoolean(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)

	def, ok := svc.Definition("docs.write_document_content")
	if !ok {
		t.Fatal("expected docs.write_document_content definition")
	}
	if !def.ExposesTool() {
		t.Fatal("expected docs.write_document_content to expose a runtime tool")
	}
	if def.Tool == nil || def.Tool.Alias != "write_document_content" || def.Tool.Category != "Docs" {
		t.Fatalf("unexpected tool metadata %#v", def.Tool)
	}
}

func TestCreateDocumentCommandMetadataAndTargets(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)

	def, ok := svc.Definition("docs.create_document")
	if !ok {
		t.Fatal("expected docs.create_document definition")
	}
	if !def.ExposesTool() {
		t.Fatal("expected docs.create_document to expose a runtime tool")
	}
	if def.Tool == nil || def.Tool.Alias != "create_document" || def.Tool.Category != "Docs" {
		t.Fatalf("unexpected tool metadata %#v", def.Tool)
	}
	for _, targetType := range def.SupportedTargetTypes {
		if targetType == "workspace" {
			return
		}
	}
	t.Fatalf("expected docs.create_document to support workspace target, got %#v", def.SupportedTargetTypes)
}

func TestCreateTaskCommandMetadataAndTargets(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)

	def, ok := svc.Definition("pm.create_task")
	if !ok {
		t.Fatal("expected pm.create_task definition")
	}
	if !def.ExposesTool() {
		t.Fatal("expected pm.create_task to expose a runtime tool")
	}
	if def.Tool == nil || def.Tool.Alias != "create_task" || def.Tool.Category != "PM / Tasks" {
		t.Fatalf("unexpected tool metadata %#v", def.Tool)
	}

	var supportsWorkspace bool
	var supportsEpic bool
	for _, targetType := range def.SupportedTargetTypes {
		if targetType == "workspace" {
			supportsWorkspace = true
		}
		if targetType == "epic" {
			supportsEpic = true
		}
	}
	if !supportsWorkspace || !supportsEpic {
		t.Fatalf("expected pm.create_task to support workspace and epic targets, got %#v", def.SupportedTargetTypes)
	}
}

func TestCreateTaskCommandTreatsEmptyOptionalIDsAsOmitted(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "actor-1", "actor@example.com", "Actor", "hash")
	seedWorkspace(t, db, "ws-1", "Workspace", "workspace", "actor-1")
	seedWorkspaceMember(t, db, "member-1", "ws-1", "actor-1", "actor@example.com", "Actor", "admin")
	now := time.Now()
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, team_type, default_task_type, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"team-1", "ws-1", "Marketing", "marketing", "chore", now, now)
	mustExec(t, db, `INSERT INTO pm_workflows (id, workspace_id, name, team_id, default_state_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"wf-1", "ws-1", "Marketing Workflow", "team-1", "state-1", now, now)
	mustExec(t, db, `INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"state-1", "wf-1", "To Do", model.PMStateTypeUnstarted, 0, true, now, now)

	taskRepo := repository.NewPMTaskRepository(db)
	taskService := NewPMTaskService(
		taskRepo,
		repository.NewWorkspaceRepository(db),
		repository.NewPMWorkflowRepository(db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		NewPMActivityService(repository.NewPMActivityRepository(db)),
		nil,
		nil,
		nil,
		nil,
	)
	svc := NewInternalCommandService(nil, taskService, nil, nil, nil, nil, taskRepo, nil)

	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		ActorID:     "actor-1",
		TargetType:  "workspace",
		TargetID:    "ws-1",
	}, "pm.create_task", json.RawMessage(`{
		"name":"Weekly competitor digest",
		"team_id":"team-1",
		"task_type":"chore",
		"epic_id":"",
		"workflow_id":"",
		"state_id":"",
		"owner_member_ids":[""],
		"label_ids":[""]
	}`))
	if err != nil {
		t.Fatalf("pm.create_task with empty optional IDs returned error: %v", err)
	}

	var result struct {
		TaskID     string  `json:"task_id"`
		WorkflowID string  `json:"workflow_id"`
		StateID    string  `json:"state_id"`
		EpicID     *string `json:"epic_id"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal create task output: %v", err)
	}
	if result.TaskID == "" || result.WorkflowID != "wf-1" || result.StateID != "state-1" || result.EpicID != nil {
		t.Fatalf("unexpected create task result: %#v", result)
	}

	var created model.PMTask
	if err := db.First(&created, "id = ?", result.TaskID).Error; err != nil {
		t.Fatalf("load created task: %v", err)
	}
	if created.EpicID != nil {
		t.Fatalf("epic_id = %q, want NULL", *created.EpicID)
	}
}

func TestListWorkspaceTeamsCommandMetadataAndOutput(t *testing.T) {
	db := newTestDB(t)
	now := time.Now()
	handle := "eng"
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, handle, team_type, default_task_type, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"team-1", "ws-1", "Engineering", handle, "engineering", "feature", now, now)
	mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, team_type, default_task_type, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"team-2", "ws-1", "Growth", "growth", "task", now, now)

	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetSettingsRepository(repository.NewSettingsRepository(db))
	def, ok := svc.Definition("workspace.list_teams")
	if !ok {
		t.Fatal("expected workspace.list_teams definition")
	}
	if !def.ExposesTool() {
		t.Fatal("expected workspace.list_teams to expose a runtime tool")
	}
	if def.Tool == nil || def.Tool.Alias != "list_workspace_teams" || def.Tool.Category != "Workspace" {
		t.Fatalf("unexpected tool metadata %#v", def.Tool)
	}

	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		ActorID:     "actor-1",
		TargetType:  "workspace",
		TargetID:    "ws-1",
	}, "workspace.list_teams", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("workspace.list_teams returned error: %v", err)
	}
	var result struct {
		Teams []struct {
			ID              string `json:"id"`
			Name            string `json:"name"`
			Handle          string `json:"handle"`
			TeamType        string `json:"team_type"`
			DefaultTaskType string `json:"default_task_type"`
		} `json:"teams"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, string(output))
	}
	if len(result.Teams) != 2 || result.Teams[0].ID != "team-1" || result.Teams[0].Handle != "eng" || result.Teams[1].Name != "Growth" {
		t.Fatalf("unexpected teams output %#v", result.Teams)
	}
}

func TestListWorkspaceTeamsCommandCapsDeterministicOutput(t *testing.T) {
	db := newTestDB(t)
	now := time.Now()
	for index := 0; index < 105; index++ {
		id := fmt.Sprintf("team-%03d", index)
		name := fmt.Sprintf("Team %03d", 104-index)
		mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, team_type, default_task_type, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, id, "ws-1", name, "engineering", "feature", now, now)
	}

	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetSettingsRepository(repository.NewSettingsRepository(db))
	output, err := svc.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws-1", ActorID: "actor-1", TargetType: "workspace", TargetID: "ws-1"}, "workspace.list_teams", json.RawMessage(`{"limit":100,"offset":0}`))
	if err != nil {
		t.Fatalf("list teams: %v", err)
	}
	var result struct {
		Teams []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"teams"`
		HasMore    bool `json:"has_more"`
		NextOffset *int `json:"next_offset"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode teams: %v", err)
	}
	if len(result.Teams) != 100 || result.Teams[0].Name != "Team 000" || result.Teams[99].Name != "Team 099" || !result.HasMore || result.NextOffset == nil || *result.NextOffset != 100 {
		t.Fatalf("bounded team output = %#v", result)
	}
}

func TestListAgentsCommandReturnsOnlyActorVisibleAgents(t *testing.T) {
	db := setupAgentScopeTestDB(t)
	seedAgentScopeAgent(t, db, model.Agent{
		ID:                    "quill-agent",
		WorkspaceID:           "ws-1",
		IsSystem:              true,
		Name:                  "Quill",
		PresetKey:             model.AgentPresetDocumentationAgent,
		Role:                  "Documentation Agent",
		Status:                "idle",
		RuntimeKind:           "codex",
		AllowedTargets:        json.RawMessage(`["workspace","document","repository"]`),
		AllowedTools:          json.RawMessage(`["list_documents","read_document","checkout_repositories"]`),
		AllowedCommands:       json.RawMessage(`[]`),
		Skills:                model.AgentSkillRefs{},
		ExecutionConfig:       model.JSONBlob(`{}`),
		ApprovalMode:          "never",
		DefaultInvocationMode: "interactive",
	})
	seedAgentScopeAgent(t, db, model.Agent{
		ID:                    "team-a-agent",
		WorkspaceID:           "ws-1",
		Name:                  "Team A Agent",
		Status:                "idle",
		RuntimeKind:           "native_sdk",
		AllowedTargets:        json.RawMessage(`["task"]`),
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		Skills:                model.AgentSkillRefs{},
		ExecutionConfig:       model.JSONBlob(`{}`),
		ApprovalMode:          "always",
		DefaultInvocationMode: "interactive",
		TeamIDs:               []string{"team-a"},
	})
	seedAgentScopeAgent(t, db, model.Agent{
		ID:                    "hidden-agent",
		WorkspaceID:           "ws-1",
		Name:                  "Hidden Team Agent",
		Status:                "idle",
		RuntimeKind:           "native_sdk",
		AllowedTargets:        json.RawMessage(`["task"]`),
		AllowedTools:          json.RawMessage(`[]`),
		AllowedCommands:       json.RawMessage(`[]`),
		Skills:                model.AgentSkillRefs{},
		ExecutionConfig:       model.JSONBlob(`{}`),
		ApprovalMode:          "always",
		DefaultInvocationMode: "interactive",
		TeamIDs:               []string{"team-b"},
	})

	agentService := &AgentService{agentRepo: repository.NewAgentRepository(db)}
	svc := NewInternalCommandService(agentService, nil, nil, nil, nil, nil, nil, nil)
	def, ok := svc.Definition("agents.list_agents")
	if !ok || def.Tool == nil || def.Tool.Alias != "list_agents" || def.Mutating {
		t.Fatalf("unexpected list_agents definition: %#v", def)
	}
	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID:  "ws-1",
		ActorID:      "actor-1",
		ActorRole:    "member",
		ActorTeamIDs: []string{"team-a"},
		TargetType:   "workspace",
		TargetID:     "ws-1",
	}, "agents.list_agents", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("agents.list_agents returned error: %v", err)
	}
	var directory compactAgentDirectoryResult
	if err := json.Unmarshal(output, &directory); err != nil {
		t.Fatalf("unmarshal agents: %v\n%s", err, string(output))
	}
	agents := directory.Agents
	if len(agents) != 2 || agents[0].Name != "Quill" || agents[1].Name != "Team A Agent" {
		t.Fatalf("agents = %#v, want Quill and Team A Agent", agents)
	}
	if agents[0].PresetKey != model.AgentPresetDocumentationAgent || !agents[0].IsSystem {
		t.Fatalf("expected grounded Quill metadata, got %#v", agents[0])
	}
}

func TestListTasksSupportsOptionalOwnerFilters(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "actor-1", "actor@example.com", "Actor", "hash")
	seedUser(t, db, "actor-2", "other@example.com", "Other", "hash")
	seedWorkspace(t, db, "ws-1", "Workspace", "workspace", "actor-1")
	seedWorkspaceMember(t, db, "member-1", "ws-1", "actor-1", "actor@example.com", "Actor", "admin")
	seedWorkspaceMember(t, db, "member-2", "ws-1", "actor-2", "other@example.com", "Other", "member")
	seedWorkflow(t, db, "wf-1", "ws-1", "state-1")
	now := time.Now()
	mustExec(t, db, `INSERT INTO pm_tasks (id, workspace_id, display_id, name, task_type, workflow_id, workflow_state_id, priority, severity, completed, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"task-owned", "ws-1", 1, "Owned by actor", model.PMTaskTypeFeature, "wf-1", "state-1", "medium", "normal", false, false, now, now)
	mustExec(t, db, `INSERT INTO pm_tasks (id, workspace_id, display_id, name, task_type, workflow_id, workflow_state_id, priority, severity, completed, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"task-other", "ws-1", 2, "Owned by other", model.PMTaskTypeFeature, "wf-1", "state-1", "medium", "normal", false, false, now, now)
	mustExec(t, db, `INSERT INTO pm_task_owners (task_id, user_id, created_at) VALUES (?, ?, ?)`, "task-owned", "actor-1", now)
	mustExec(t, db, `INSERT INTO pm_task_owners (task_id, user_id, created_at) VALUES (?, ?, ?)`, "task-other", "actor-2", now)

	taskRepo := repository.NewPMTaskRepository(db)
	taskService := NewPMTaskService(
		taskRepo,
		repository.NewWorkspaceRepository(db),
		repository.NewPMWorkflowRepository(db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		NewPMActivityService(repository.NewPMActivityRepository(db)),
		nil,
		nil,
		nil,
		nil,
	)
	svc := NewInternalCommandService(nil, taskService, nil, nil, nil, nil, taskRepo, nil)

	allOutput, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		ActorID:     "actor-1",
		TargetType:  "workspace",
		TargetID:    "ws-1",
	}, "pm.list_tasks", json.RawMessage(`{"open_only":true}`))
	if err != nil {
		t.Fatalf("pm.list_tasks without owner filter returned error: %v", err)
	}
	var allResult struct {
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal(allOutput, &allResult); err != nil {
		t.Fatalf("unmarshal all output: %v", err)
	}
	if allResult.Total != 2 {
		t.Fatalf("expected existing unfiltered behavior to return 2 tasks, got %d", allResult.Total)
	}

	ownedOutput, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		ActorID:     "actor-1",
		TargetType:  "workspace",
		TargetID:    "ws-1",
	}, "pm.list_tasks", json.RawMessage(`{"open_only":true,"owned_by_actor":true}`))
	if err != nil {
		t.Fatalf("pm.list_tasks owned_by_actor returned error: %v", err)
	}
	var ownedResult struct {
		Total int64 `json:"total"`
		Tasks []struct {
			Name string `json:"name"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(ownedOutput, &ownedResult); err != nil {
		t.Fatalf("unmarshal owned output: %v", err)
	}
	if ownedResult.Total != 1 || len(ownedResult.Tasks) != 1 || ownedResult.Tasks[0].Name != "Owned by actor" {
		t.Fatalf("expected actor-owned task only, got %#v", ownedResult)
	}
}

func TestNormalizeTaskDescriptionRichTextConvertsMarkdownToHTML(t *testing.T) {
	input := `<!-- sentinel:root_cause=test finding_ids=["sentinel:v1:test"] -->` + "\n\n## Summary\n\n- first\n- second"

	got := normalizeTaskDescriptionRichText(&input)
	if got == nil {
		t.Fatal("expected converted description")
	}
	if !strings.Contains(*got, "<h2") || !strings.Contains(*got, "<ul>") {
		t.Fatalf("expected markdown to be rendered as html, got %q", *got)
	}
	if !strings.HasPrefix(*got, `<!-- sentinel:root_cause=test`) {
		t.Fatalf("expected html comment marker to be preserved at the start, got %q", *got)
	}
}

func TestNormalizeTaskDescriptionRichTextPreservesHTML(t *testing.T) {
	input := "  <p><strong>Hello</strong> world</p>  "

	got := normalizeTaskDescriptionRichText(&input)
	if got == nil {
		t.Fatal("expected normalized description")
	}
	if *got != "<p><strong>Hello</strong> world</p>" {
		t.Fatalf("expected html to be preserved, got %q", *got)
	}
}

func TestAddTaskCommentCommandConvertsMarkdownToRichTextHTML(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "actor-1", "actor@example.com", "Actor", "hash")
	seedWorkspace(t, db, "ws-1", "Workspace", "workspace", "actor-1")
	seedWorkspaceMember(t, db, "member-1", "ws-1", "actor-1", "actor@example.com", "Actor", "admin")
	seedWorkflow(t, db, "wf-1", "ws-1", "state-1")
	now := time.Now()
	mustExec(t, db, `INSERT INTO pm_tasks (id, workspace_id, display_id, name, task_type, workflow_id, workflow_state_id, priority, severity, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"task-1", "ws-1", 1, "Fix secret", model.PMTaskTypeChore, "wf-1", "state-1", "high", "high", now, now)

	taskRepo := repository.NewPMTaskRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	activityService := NewPMActivityService(repository.NewPMActivityRepository(db))
	taskService := NewPMTaskService(
		taskRepo,
		workspaceRepo,
		repository.NewPMWorkflowRepository(db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		activityService,
		nil,
		nil,
		nil,
		nil,
	)
	commentService := NewPMCommentService(
		repository.NewPMCommentRepository(db),
		taskRepo,
		nil,
		activityService,
		nil,
		nil,
		workspaceRepo,
		nil,
	)
	svc := NewInternalCommandService(nil, taskService, nil, nil, nil, nil, taskRepo, nil)
	svc.SetPMCommentService(commentService)

	_, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		ActorID:     "actor-1",
		TargetType:  "task",
		TargetID:    "task-1",
	}, "pm.add_task_comment", json.RawMessage(`{"content":"<!-- sentinel:scan_update finding_ids=[\"sentinel:v1:test\"] -->\n\n## Scan update\n\n- CVE-1\n- CVE-2"}`))
	if err != nil {
		t.Fatalf("pm.add_task_comment returned error: %v", err)
	}

	var body string
	if err := db.Raw(`SELECT body FROM pm_comments WHERE entity_id = ?`, "task-1").Scan(&body).Error; err != nil {
		t.Fatalf("load comment body: %v", err)
	}
	if !strings.Contains(body, "<h2") || !strings.Contains(body, "Scan update") || !strings.Contains(body, "<ul>") || !strings.Contains(body, "<li>") {
		t.Fatalf("expected markdown comment to be stored as rich text html, got %q", body)
	}
	if strings.Contains(body, "## Scan update") {
		t.Fatalf("expected markdown syntax to be converted, got %q", body)
	}
	if !strings.HasPrefix(body, `<!-- sentinel:scan_update`) {
		t.Fatalf("expected comment marker to be preserved at the start, got %q", body)
	}
}

func TestAddTaskCommentCommandPersistsAgentAttribution(t *testing.T) {
	db := newTestDB(t)
	mustExec(t, db, `CREATE TABLE agents (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, name TEXT NOT NULL, icon_key TEXT, preset_key TEXT)`)
	mustExec(t, db, `CREATE TABLE agent_runs (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, external_runtime TEXT, external_runtime_id TEXT)`)
	seedUser(t, db, "actor-1", "actor@example.com", "Waqar Azeem", "hash")
	seedWorkspace(t, db, "ws-1", "Workspace", "workspace", "actor-1")
	seedWorkspaceMember(t, db, "member-1", "ws-1", "actor-1", "actor@example.com", "Waqar Azeem", "admin")
	seedWorkflow(t, db, "wf-1", "ws-1", "state-1")
	now := time.Now()
	mustExec(t, db, `INSERT INTO pm_tasks (id, workspace_id, display_id, name, task_type, workflow_id, workflow_state_id, priority, severity, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"task-1", "ws-1", 1, "Attributed task", model.PMTaskTypeChore, "wf-1", "state-1", "high", "high", now, now)
	mustExec(t, db, `INSERT INTO agents (id, workspace_id, name) VALUES (?, ?, ?)`, "agent-1", "ws-1", "Code Review Agent")
	mustExec(t, db, `INSERT INTO agent_runs (id, workspace_id, external_runtime, external_runtime_id) VALUES (?, ?, ?, ?)`,
		"0198c7d7-7654-7000-8000-000000000001", "ws-1", agentRuntimeName, "run_f1cb17f9f9fe4edc28403c7a")

	taskRepo := repository.NewPMTaskRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	activityService := NewPMActivityService(repository.NewPMActivityRepository(db))
	taskService := NewPMTaskService(taskRepo, workspaceRepo, repository.NewPMWorkflowRepository(db), nil, nil, nil, nil, nil, nil, activityService, nil, nil, nil, nil)
	commentService := NewPMCommentService(repository.NewPMCommentRepository(db), taskRepo, nil, activityService, nil, nil, workspaceRepo, nil)
	svc := NewInternalCommandService(&AgentService{agentRepo: repository.NewAgentRepository(db)}, taskService, nil, nil, nil, nil, taskRepo, nil)
	svc.agentRunRepo = repository.NewAgentRunRepository(db)
	svc.SetPMCommentService(commentService)

	_, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1", ActorID: "actor-1", AgentID: "agent-1", AgentScopeResolved: true, RunID: "run_f1cb17f9f9fe4edc28403c7a", TargetType: "task", TargetID: "task-1",
	}, "pm.add_task_comment", json.RawMessage(`{"content":"Reviewed by the agent"}`))
	if err != nil {
		t.Fatalf("pm.add_task_comment returned error: %v", err)
	}

	var comment model.PMComment
	if err := db.Where("entity_id = ?", "task-1").First(&comment).Error; err != nil {
		t.Fatalf("load comment: %v", err)
	}
	if comment.AuthorID != "actor-1" {
		t.Fatalf("author_id = %q, want initiating user", comment.AuthorID)
	}
	if comment.AgentID == nil || *comment.AgentID != "agent-1" || comment.AgentName != "Code Review Agent" || comment.AgentRunID == nil || *comment.AgentRunID != "0198c7d7-7654-7000-8000-000000000001" {
		t.Fatalf("agent attribution = id:%v name:%q run:%v", comment.AgentID, comment.AgentName, comment.AgentRunID)
	}
}

func TestListTasksCompactReturnsBoundedExcerptsWithHTMLComments(t *testing.T) {
	db := newTestDB(t)
	seedUser(t, db, "actor-1", "actor@example.com", "Actor", "hash")
	seedWorkspace(t, db, "ws-1", "Workspace", "workspace", "actor-1")
	seedWorkspaceMember(t, db, "member-1", "ws-1", "actor-1", "actor@example.com", "Actor", "admin")
	seedWorkflow(t, db, "wf-1", "ws-1", "state-1")
	now := time.Now()
	description := `<!-- sentinel:root_cause=dependency:npm:docs:next finding_ids=["sentinel:v1:dependency:npm:docs/package.json:next:CVE-1"] --><p>Fix the vulnerable dependency with enough detail to excerpt.</p>`
	mustExec(t, db, `INSERT INTO pm_tasks (id, workspace_id, display_id, name, description, task_type, workflow_id, workflow_state_id, priority, severity, completed, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"task-1", "ws-1", 1, "Fix Next.js CVE", description, model.PMTaskTypeChore, "wf-1", "state-1", "high", "critical", false, false, now, now)
	mustExec(t, db, `INSERT INTO pm_labels (id, workspace_id, name, color, archived, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"label-security", "ws-1", "security", "#dc2626", false, now, now)
	mustExec(t, db, `INSERT INTO pm_task_labels (task_id, label_id, created_at) VALUES (?, ?, ?)`, "task-1", "label-security", now)
	mustExec(t, db, `INSERT INTO pm_comments (id, workspace_id, entity_type, entity_id, author_id, body, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"comment-1", "ws-1", "task", "task-1", "actor-1", `<!-- sentinel:scan_update finding_ids=["sentinel:v1:dependency:npm:docs/package.json:next:CVE-2"] --><p>New scan evidence.</p>`, now, now)

	taskRepo := repository.NewPMTaskRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	activityService := NewPMActivityService(repository.NewPMActivityRepository(db))
	taskService := NewPMTaskService(
		taskRepo,
		workspaceRepo,
		repository.NewPMWorkflowRepository(db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		activityService,
		nil,
		nil,
		nil,
		nil,
	)
	commentService := NewPMCommentService(repository.NewPMCommentRepository(db), taskRepo, nil, activityService, nil, nil, workspaceRepo, nil)
	svc := NewInternalCommandService(nil, taskService, nil, nil, nil, nil, taskRepo, nil)
	svc.SetPMCommentService(commentService)

	output, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		ActorID:     "actor-1",
		TargetType:  "workspace",
		TargetID:    "ws-1",
	}, "pm.list_tasks", json.RawMessage(`{"label_id":"label-security","open_only":true,"detail_level":"compact","include_descriptions":true,"include_comments":true,"limit":100}`))
	if err != nil {
		t.Fatalf("pm.list_tasks returned error: %v", err)
	}

	var result struct {
		Compaction map[string]any `json:"_helpin_compaction"`
		Tasks      []struct {
			Description        string `json:"description,omitempty"`
			Comments           []any  `json:"comments,omitempty"`
			DescriptionExcerpt string `json:"description_excerpt"`
			CommentExcerpts    []struct {
				Excerpt string `json:"excerpt"`
			} `json:"comment_excerpts"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal list tasks output: %v\n%s", err, string(output))
	}
	if result.Compaction["mode"] != "bounded_index" {
		t.Fatalf("expected compaction hint, got %#v", result.Compaction)
	}
	if len(result.Tasks) != 1 {
		t.Fatalf("expected one task, got %#v", result.Tasks)
	}
	task := result.Tasks[0]
	if task.Description != "" || len(task.Comments) != 0 {
		t.Fatalf("compact should not include full description/comments, got %#v", task)
	}
	if !strings.Contains(task.DescriptionExcerpt, "Fix the vulnerable dependency") || len(task.CommentExcerpts) != 1 {
		t.Fatalf("expected compact excerpts, got %#v", task)
	}
	if !strings.Contains(task.DescriptionExcerpt, "<!-- sentinel:root_cause=dependency:npm:docs:next") || !strings.Contains(task.DescriptionExcerpt, "CVE-1") {
		t.Fatalf("expected description excerpt to preserve marker comment, got %q", task.DescriptionExcerpt)
	}
	if !strings.Contains(task.CommentExcerpts[0].Excerpt, "<!-- sentinel:scan_update") || !strings.Contains(task.CommentExcerpts[0].Excerpt, "CVE-2") {
		t.Fatalf("expected comment excerpt to preserve marker comment, got %#v", task.CommentExcerpts)
	}
	if strings.Contains(string(output), `"sentinel_root_cause"`) || strings.Contains(string(output), `"sentinel_finding_ids"`) {
		t.Fatalf("compact should not include structured Sentinel marker fields, got %s", string(output))
	}
}

func TestMarshalCompactTaskResponseDropsRowsToStayBounded(t *testing.T) {
	tasks := make([]map[string]any, 0, 100)
	for i := 0; i < 100; i++ {
		tasks = append(tasks, map[string]any{
			"task_id":             strings.Repeat("task-", 40),
			"task_key":            strings.Repeat("SEC-", 40),
			"name":                strings.Repeat("large task metadata ", 60),
			"state_name":          strings.Repeat("state ", 40),
			"description_excerpt": strings.Repeat("description ", 100),
			"comment_excerpts":    []map[string]any{{"excerpt": strings.Repeat("comment ", 100)}},
			"labels":              []map[string]any{{"label_id": strings.Repeat("label-", 40), "name": strings.Repeat("security ", 40)}},
		})
	}

	out, err := marshalCompactTaskResponse(tasks, int64(len(tasks)), len(tasks))
	if err != nil {
		t.Fatalf("marshal compact response: %v", err)
	}
	if len([]rune(string(out))) > 30000 {
		t.Fatalf("expected compact task output <= 30000 runes, got %d", len([]rune(string(out))))
	}
	var result struct {
		Bounded       bool             `json:"bounded"`
		HasMore       bool             `json:"has_more"`
		ReturnedTasks int              `json:"returned_tasks"`
		Tasks         []map[string]any `json:"tasks"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal compact response: %v", err)
	}
	if !result.Bounded || !result.HasMore || result.ReturnedTasks >= len(tasks) || len(result.Tasks) != result.ReturnedTasks {
		t.Fatalf("expected bounded response with fewer tasks, got %+v", result)
	}
}

func TestResolveTaskCreationWorkflowValidatesExplicitWorkflowTeamScope(t *testing.T) {
	db := newTestDB(t)
	workflowRepo := repository.NewPMWorkflowRepository(db)
	taskService := &PMTaskService{workflowRepo: workflowRepo}
	svc := NewInternalCommandService(nil, taskService, nil, nil, nil, nil, nil, nil)

	now := time.Now()
	mustExec(t, db, `INSERT INTO pm_workflows (id, workspace_id, name, team_id, default_state_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"wf-team-a", "ws-1", "Team A Workflow", "team-a", "state-team-a", now, now)
	mustExec(t, db, `INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"state-team-a", "wf-team-a", "To Do", model.PMStateTypeUnstarted, 0, true, now, now)
	mustExec(t, db, `INSERT INTO pm_workflows (id, workspace_id, name, team_id, default_state_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"wf-team-b", "ws-1", "Team B Workflow", "team-b", "state-team-b", now, now)
	mustExec(t, db, `INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"state-team-b", "wf-team-b", "To Do", model.PMStateTypeUnstarted, 0, true, now, now)

	workflowID := "wf-team-b"
	stateID := "state-team-b"
	_, _, err := svc.resolveTaskCreationWorkflow(context.Background(), "ws-1", "team-a", &workflowID, &stateID)
	if err == nil {
		t.Fatal("expected explicit workflow/state pair from another team to be rejected")
	}
	if !strings.Contains(err.Error(), "workflow_id does not belong to team_id") {
		t.Fatalf("expected team scope error, got %v", err)
	}

	workflowID = "wf-team-a"
	stateID = "state-team-a"
	resolvedWorkflowID, resolvedStateID, err := svc.resolveTaskCreationWorkflow(context.Background(), "ws-1", "team-a", &workflowID, &stateID)
	if err != nil {
		t.Fatalf("expected matching explicit workflow/state pair to resolve: %v", err)
	}
	if resolvedWorkflowID != "wf-team-a" || resolvedStateID != "state-team-a" {
		t.Fatalf("unexpected workflow/state resolution: %q %q", resolvedWorkflowID, resolvedStateID)
	}
}

func TestResolveTaskCreationWorkflowRejectsStateOutsideWorkflow(t *testing.T) {
	db := newTestDB(t)
	workflowRepo := repository.NewPMWorkflowRepository(db)
	taskService := &PMTaskService{workflowRepo: workflowRepo}
	svc := NewInternalCommandService(nil, taskService, nil, nil, nil, nil, nil, nil)

	now := time.Now()
	mustExec(t, db, `INSERT INTO pm_workflows (id, workspace_id, name, team_id, default_state_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"wf-team-a", "ws-1", "Team A Workflow", "team-a", "state-team-a", now, now)
	mustExec(t, db, `INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"state-team-a", "wf-team-a", "To Do", model.PMStateTypeUnstarted, 0, true, now, now)
	mustExec(t, db, `INSERT INTO pm_workflows (id, workspace_id, name, team_id, default_state_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"wf-team-b", "ws-1", "Team B Workflow", "team-b", "state-team-b", now, now)
	mustExec(t, db, `INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"state-team-b", "wf-team-b", "To Do", model.PMStateTypeUnstarted, 0, true, now, now)

	workflowID := "wf-team-a"
	stateID := "state-team-b"
	_, _, err := svc.resolveTaskCreationWorkflow(context.Background(), "ws-1", "team-a", &workflowID, &stateID)
	if err == nil {
		t.Fatal("expected explicit state from another workflow to be rejected")
	}
	if !strings.Contains(err.Error(), "state_id does not belong to workflow_id") {
		t.Fatalf("expected workflow/state mismatch error, got %v", err)
	}
}

func TestCreateFollowupTasksCommandIsBackendOnlyUntilToolExists(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)

	def, ok := svc.Definition("pm.create_followup_tasks")
	if !ok {
		t.Fatal("expected pm.create_followup_tasks definition")
	}
	if def.ExposesTool() {
		t.Fatalf("expected pm.create_followup_tasks to remain backend-only, got %#v", def.Tool)
	}
}

func TestDeliveryMergeBranchCommandUpdatesDeliveryStatusAfterSuccessfulMerge(t *testing.T) {
	db := newTestDB(t)
	seedGitDeliveryStatusFixture(t, db)
	app := &fakeGitHubAppClient{}
	gitSvc := newGitDeliveryStatusService(db, app)
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetGitService(gitSvc)

	_, err := svc.Execute(context.Background(), model.InternalCommandContext{
		WorkspaceID: "ws-1",
		TargetType:  "task",
		TargetID:    "task-1",
	}, "delivery.merge_branch", json.RawMessage(`{"target_branch":"main"}`))
	if err != nil {
		t.Fatalf("delivery.merge_branch returned error: %v", err)
	}
	if len(app.mergeCalls) != 1 {
		t.Fatalf("merge calls = %d, want 1", len(app.mergeCalls))
	}
	if app.mergeCalls[0].Base != "main" || app.mergeCalls[0].Head != "hel-31-fix-merge-status" {
		t.Fatalf("unexpected merge call: %#v", app.mergeCalls[0])
	}
	assertMergedDeliveryStatus(t, db)
}
