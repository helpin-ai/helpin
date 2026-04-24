package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestWriteDocumentContentCommandSupportsDocumentTarget(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)

	def, ok := svc.Definition("docs.write_document_content")
	if !ok {
		t.Fatal("expected docs.write_document_content definition")
	}

	for _, targetType := range def.SupportedTargetTypes {
		if targetType == "document" {
			return
		}
	}

	t.Fatalf("expected docs.write_document_content to support target type document, got %#v", def.SupportedTargetTypes)
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

func TestNormalizeTaskDescriptionRichTextConvertsMarkdownToHTML(t *testing.T) {
	input := "## Summary\n\n- first\n- second"

	got := normalizeTaskDescriptionRichText(&input)
	if got == nil {
		t.Fatal("expected converted description")
	}
	if !strings.Contains(*got, "<h2") || !strings.Contains(*got, "<ul>") {
		t.Fatalf("expected markdown to be rendered as html, got %q", *got)
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
