package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/tiptap"
	"gorm.io/gorm"
)

// executeMCPCommandTool runs a command-backed public tool exactly as the MCP
// boundary does: a workspace principal with no agent run or target context.
func executeMCPCommandTool(t *testing.T, commands *InternalCommandService, workspaceID, userID, toolName, arguments string) (*MCPToolResult, error) {
	t.Helper()
	return executeMCPCommandToolWith(t, &MCPService{commands: commands}, workspaceID, userID, toolName, arguments)
}

func executeMCPCommandToolWith(t *testing.T, service *MCPService, workspaceID, userID, toolName, arguments string) (*MCPToolResult, error) {
	t.Helper()
	var tool MCPToolDefinition
	for _, candidate := range service.buildToolCatalog() {
		if candidate.Name == toolName {
			tool = candidate
		}
	}
	if tool.CommandName == "" {
		t.Fatalf("tool %q is not command-backed", toolName)
	}
	principal := &model.MCPPrincipal{WorkspaceID: workspaceID, UserID: userID}
	actor := &authorization.Actor{WorkspaceID: workspaceID, UserID: userID, Role: model.RoleAdmin}
	ctx := authorization.WithActor(context.Background(), actor)
	return service.executeAuthorizedMCPTool(ctx, principal, actor, tool, json.RawMessage(arguments))
}

func TestMCPAddTaskCommentWithoutRunContext(t *testing.T) {
	env := newPMCommandTestEnv(t)
	seedPMCommandTask(t, env.db, "task-a", env.workspaceID, "team-a", "wf-a", "state-a", "", "", 1)
	result, err := executeMCPCommandTool(t, env.service, env.workspaceID, env.actorID, "add_task_comment", `{"task_id":"task-a","content":"One-line comment"}`)
	if err != nil {
		t.Fatalf("add_task_comment error = %v", err)
	}
	encoded, _ := json.Marshal(result.Data)
	if !jsonContains(encoded, `"comment_id"`) {
		t.Fatalf("result = %s", encoded)
	}
}

func TestMCPPMCommandToolsWithoutRunContext(t *testing.T) {
	tests := []struct {
		tool      string
		arguments string
	}{
		{"create_task", `{"name":"From MCP","team_id":"team-a"}`},
		{"update_task_state", `{"task_id":"task-a","state_id":"state-a"}`},
		{"set_task_dependencies", `{"dependencies":[{"source_task_id":"task-a","target_task_id":"task-a2"}]}`},
		{"add_task_comment", `{"task_id":"task-a","content":"Hi"}`},
		{"list_tasks", `{"limit":5}`},
		{"create_epic", `{"name":"MCP epic","team_id":"team-a"}`},
		{"get_epic", `{"epic_id":"epic-a"}`},
		{"update_epic", `{"epic_id":"epic-a","name":"Renamed"}`},
		{"list_epics", `{}`},
		{"list_sprints", `{}`},
		{"get_sprint", `{"sprint_id":"sprint-a"}`},
		{"list_sprint_tasks", `{"sprint_id":"sprint-a"}`},
		{"update_sprint", `{"sprint_id":"sprint-a","name":"Renamed"}`},
		{"list_pm_labels", `{}`},
		{"ensure_task_label", `{"name":"security"}`},
		{"list_team_workflows_with_stages", `{}`},
		{"list_workspace_members", `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			env := newPMCommandTestEnv(t)
			seedPMCommandTask(t, env.db, "task-a", env.workspaceID, "team-a", "wf-a", "state-a", "epic-a", "sprint-a", 1)
			seedPMCommandTask(t, env.db, "task-a2", env.workspaceID, "team-a", "wf-a", "state-a", "", "", 2)
			if _, err := executeMCPCommandTool(t, env.service, env.workspaceID, env.actorID, tt.tool, tt.arguments); err != nil {
				t.Fatalf("%s error = %v", tt.tool, err)
			}
		})
	}
}

func assertMCPToolError(t *testing.T, err error, wantCode, wantMessage string) {
	t.Helper()
	var toolErr *MCPToolError
	if !errors.As(err, &toolErr) {
		t.Fatalf("error = %v (%T), want MCPToolError %s", err, err, wantCode)
	}
	if toolErr.Code != wantCode {
		t.Fatalf("code = %q (%s), want %q", toolErr.Code, toolErr.Message, wantCode)
	}
	if wantMessage != "" && toolErr.Message != wantMessage {
		t.Fatalf("message = %q, want %q", toolErr.Message, wantMessage)
	}
}

func TestMCPCommandToolErrorsAreTyped(t *testing.T) {
	tests := []struct {
		name        string
		tool        string
		arguments   string
		wantCode    string
		wantMessage string
	}{
		{name: "unknown task", tool: "add_task_comment", arguments: `{"task_id":"missing-task","content":"Hi"}`, wantCode: MCPErrorCodeNotFound, wantMessage: "Task not found."},
		{name: "task in another workspace", tool: "add_task_comment", arguments: `{"task_id":"task-other","content":"Hi"}`, wantCode: MCPErrorCodeNotFound, wantMessage: "Task not found."},
		{name: "blank comment", tool: "add_task_comment", arguments: `{"task_id":"task-a","content":"   "}`, wantCode: MCPErrorCodeInvalidInput, wantMessage: "Content is required."},
		{name: "state from another workflow", tool: "update_task_state", arguments: `{"task_id":"task-a","state_id":"state-b"}`, wantCode: MCPErrorCodeInvalidInput, wantMessage: "State_id must belong to task workflow."},
		{name: "unknown task state move", tool: "update_task_state", arguments: `{"task_id":"missing-task","state_id":"state-a"}`, wantCode: MCPErrorCodeNotFound},
		{name: "self dependency", tool: "set_task_dependencies", arguments: `{"dependencies":[{"source_task_id":"task-a","target_task_id":"task-a"}]}`, wantCode: MCPErrorCodeInvalidInput},
		{name: "foreign dependency", tool: "set_task_dependencies", arguments: `{"dependencies":[{"source_task_id":"task-a","target_task_id":"task-other"}]}`, wantCode: MCPErrorCodeNotFound},
		{name: "unknown epic", tool: "get_epic", arguments: `{"epic_id":"missing-epic"}`, wantCode: MCPErrorCodeNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newPMCommandTestEnv(t)
			seedPMCommandTask(t, env.db, "task-a", env.workspaceID, "team-a", "wf-a", "state-a", "", "", 1)
			seedPMCommandTask(t, env.db, "task-other", "ws-2", "team-other", "wf-other", "state-other", "", "", 1)
			_, err := executeMCPCommandTool(t, env.service, env.workspaceID, env.actorID, tt.tool, tt.arguments)
			assertMCPToolError(t, err, tt.wantCode, tt.wantMessage)
		})
	}
}

func setupMCPDocumentCommandTest(t *testing.T) (*MCPService, model.InternalCommandContext) {
	t.Helper()
	commands, meta := documentWorkflowFixture(t, "First paragraph.\n\nSecond paragraph.")
	service := &MCPService{
		commands:  commands,
		documents: &fakeMCPDocsDocumentService{documents: map[string]*model.DocsDocument{meta.TargetID: {ID: meta.TargetID, WorkspaceID: meta.WorkspaceID, SpaceID: "space-1"}}},
		spaces:    &fakeMCPDocsSpaceService{getResults: map[string]*model.DocsSpaceWithTeams{"space-1": {DocsSpace: model.DocsSpace{ID: "space-1", WorkspaceID: meta.WorkspaceID}}}},
	}
	return service, meta
}

func TestMCPEditDocumentStaleVersionIsVersionConflict(t *testing.T) {
	service, meta := setupMCPDocumentCommandTest(t)
	arguments := fmt.Sprintf(`{"document_id":%q,"expected_version":"stale-version","operations":[{"type":"insert","position":"end","content":"Added."}]}`, meta.TargetID)
	_, err := executeMCPCommandToolWith(t, service, meta.WorkspaceID, meta.ActorID, "edit_document", arguments)
	assertMCPToolError(t, err, MCPErrorCodeVersionConflict, "The document changed; read it again and retry with the new version.")
}

func TestMCPEditDocumentUnknownBlockIsInvalidInput(t *testing.T) {
	service, meta := setupMCPDocumentCommandTest(t)
	content, err := service.commands.docsContentService.Get(context.Background(), meta.TargetID)
	if err != nil {
		t.Fatalf("load content: %v", err)
	}
	arguments := fmt.Sprintf(`{"document_id":%q,"expected_version":%q,"operations":[{"type":"replace_text","block_id":"missing-block","old_text":"a","new_text":"b"}]}`, meta.TargetID, tiptap.DocumentVersion(content.Content))
	_, err = executeMCPCommandToolWith(t, service, meta.WorkspaceID, meta.ActorID, "edit_document", arguments)
	assertMCPToolError(t, err, MCPErrorCodeInvalidInput, "Block missing-block not found; reread the section.")
}

func TestMCPEditDocumentUnknownDocumentIsNotFound(t *testing.T) {
	service, meta := setupMCPDocumentCommandTest(t)
	_, err := executeMCPCommandToolWith(t, service, meta.WorkspaceID, meta.ActorID, "edit_document",
		`{"document_id":"missing-document","expected_version":"v","operations":[{"type":"insert","position":"end","content":"Added."}]}`)
	if !errors.Is(err, ErrMCPNotFound) {
		t.Fatalf("error = %v, want ErrMCPNotFound", err)
	}
}

func TestMCPCommandError(t *testing.T) {
	raw := errors.New(`pq: relation "secret_table" does not exist`)
	tests := []struct {
		name        string
		err         error
		wantCode    string
		wantMessage string
	}{
		{name: "docs content conflict", err: fmt.Errorf("save: %w", ErrDocsContentConflict), wantCode: MCPErrorCodeVersionConflict, wantMessage: _mcpVersionConflictMessage},
		{name: "stale block revision", err: ErrDocsStaleBlockRevision, wantCode: MCPErrorCodeVersionConflict, wantMessage: _mcpVersionConflictMessage},
		{name: "locked document", err: ErrDocsDocumentLocked, wantCode: MCPErrorCodeDocumentLocked},
		{name: "invalid content", err: fmt.Errorf("%w: bad node", ErrDocsInvalidContent), wantCode: MCPErrorCodeInvalidInput, wantMessage: _mcpInvalidContentMessage},
		{name: "command not found", err: fmt.Errorf("wrap: %w", errCommandNotFound("sprint")), wantCode: MCPErrorCodeNotFound, wantMessage: "Sprint not found."},
		{name: "command input", err: errCommandInput("%s_id is required", "task"), wantCode: MCPErrorCodeInvalidInput, wantMessage: "Task_id is required."},
		{name: "forbidden", err: &model.ErrForbidden{Message: "agent does not have access to team-x"}, wantCode: MCPErrorCodeForbidden, wantMessage: _mcpForbiddenMessage},
		{name: "record not found", err: fmt.Errorf("load: %w", gorm.ErrRecordNotFound), wantCode: MCPErrorCodeNotFound, wantMessage: _mcpNotFoundMessage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertMCPToolError(t, mcpCommandError(tt.err), tt.wantCode, tt.wantMessage)
		})
	}
	t.Run("unclassified errors are not forwarded as typed errors", func(t *testing.T) {
		var toolErr *MCPToolError
		if got := mcpCommandError(raw); errors.As(got, &toolErr) {
			t.Fatalf("raw error became typed: %v", got)
		}
	})
	t.Run("boundary errors pass through", func(t *testing.T) {
		if got := mcpCommandError(ErrMCPForbidden); !errors.Is(got, ErrMCPForbidden) {
			t.Fatalf("got %v", got)
		}
	})
}
