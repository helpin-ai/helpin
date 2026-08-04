package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestExecuteSpecialMCPUpdateTask(t *testing.T) {
	service, principal, actor, tasks, _ := setupMCPPMToolTest()
	result, err := service.executeSpecialMCPTool(
		context.Background(), principal, actor, "update_task",
		json.RawMessage(`{"task_id":"task-1","name":"Ship MCP","priority":"high","label_ids":["label-1"]}`),
	)
	if err != nil {
		t.Fatalf("update_task error = %v", err)
	}
	if result.Summary != "Task Ship MCP updated." || tasks.updateID != "task-1" || tasks.updateRequest.Name == nil || *tasks.updateRequest.Name != "Ship MCP" || len(tasks.updateRequest.LabelIDs) != 1 {
		t.Fatalf("update_task result = %#v, request %#v", result, tasks.updateRequest)
	}
}

func TestExecuteSpecialMCPCreateTaskBatch(t *testing.T) {
	service, principal, actor, _, _ := setupMCPPMToolTest()
	creator := &fakeMCPTaskBatchCreator{}
	service.taskBatches = creator
	result, err := service.executeSpecialMCPTool(
		context.Background(), principal, actor, "create_task_batch",
		json.RawMessage(`{"epic_id":"epic-1","tasks":[{"ref":"api","name":"Document API","task_type":"chore"}]}`),
	)
	if err != nil {
		t.Fatalf("create_task_batch error = %v", err)
	}
	if result.Summary != "Created 1 tasks." || creator.workspaceID != principal.WorkspaceID || creator.epicID != "epic-1" || len(creator.tasks) != 1 || creator.tasks[0].Name != "Document API" {
		t.Fatalf("create_task_batch result = %#v, creator = %#v", result, creator)
	}
}

func TestExecuteSpecialMCPChecklistLifecycle(t *testing.T) {
	service, principal, actor, _, checklists := setupMCPPMToolTest()
	result, err := service.executeSpecialMCPTool(
		context.Background(), principal, actor, "list_task_checklist", json.RawMessage(`{"task_id":"task-1"}`),
	)
	if err != nil || result.Summary != "Returned 1 checklist items." {
		t.Fatalf("list_task_checklist result = %#v, error = %v", result, err)
	}

	result, err = service.executeSpecialMCPTool(
		context.Background(), principal, actor, "create_task_checklist_item",
		json.RawMessage(`{"task_id":"task-1","text":"Run integration tests","position":2,"due_date":"2026-08-15"}`),
	)
	if err != nil || result.Summary != "Checklist item created." || checklists.createTaskID != "task-1" || checklists.createRequest.Text != "Run integration tests" || checklists.createRequest.DueDate == nil || checklists.createRequest.DueDate.Format("2006-01-02") != "2026-08-15" {
		t.Fatalf("create checklist result = %#v, request %#v, error = %v", result, checklists.createRequest, err)
	}

	result, err = service.executeSpecialMCPTool(
		context.Background(), principal, actor, "update_task_checklist_item",
		json.RawMessage(`{"task_id":"task-1","checklist_item_id":"check-1","completed":true,"due_date":"2026-08-22"}`),
	)
	if err != nil || result.Summary != "Checklist item updated." || checklists.updateID != "check-1" || checklists.updateRequest.Completed == nil || !*checklists.updateRequest.Completed || !checklists.updateRequest.DueDateSet || checklists.updateRequest.DueDate == nil || checklists.updateRequest.DueDate.Format("2006-01-02") != "2026-08-22" {
		t.Fatalf("update checklist result = %#v, request %#v, error = %v", result, checklists.updateRequest, err)
	}
}

func TestExecuteSpecialMCPChecklistRejectsCrossWorkspaceTask(t *testing.T) {
	service, principal, actor, tasks, checklists := setupMCPPMToolTest()
	tasks.tasks["task-other"] = &model.TaskDetail{Task: model.PMTask{ID: "task-other", WorkspaceID: "workspace-other", Name: "Other"}}
	_, err := service.executeSpecialMCPTool(
		context.Background(), principal, actor, "create_task_checklist_item",
		json.RawMessage(`{"task_id":"task-other","text":"Must not write"}`),
	)
	if !errors.Is(err, ErrMCPNotFound) {
		t.Fatalf("create checklist error = %v, want ErrMCPNotFound", err)
	}
	if checklists.createTaskID != "" {
		t.Fatalf("unexpected checklist write for task %q", checklists.createTaskID)
	}
}

func setupMCPPMToolTest() (*MCPService, *model.MCPPrincipal, *authorization.Actor, *fakeMCPPMTaskService, *fakeMCPPMChecklistService) {
	tasks := &fakeMCPPMTaskService{tasks: map[string]*model.TaskDetail{
		"task-1": {Task: model.PMTask{ID: "task-1", WorkspaceID: "workspace-1", Name: "Initial task"}},
	}}
	checklists := &fakeMCPPMChecklistService{items: map[string][]model.PMChecklistItem{
		"task-1": {{ID: "check-1", TaskID: "task-1", Text: "Review changes"}},
	}}
	service := &MCPService{tasks: tasks, checklists: checklists}
	principal := &model.MCPPrincipal{WorkspaceID: "workspace-1", UserID: "user-1"}
	actor := &authorization.Actor{WorkspaceID: "workspace-1", UserID: "user-1", Role: "admin"}
	return service, principal, actor, tasks, checklists
}

type fakeMCPPMTaskService struct {
	tasks         map[string]*model.TaskDetail
	updateID      string
	updateRequest model.UpdateTaskRequest
}

func (f *fakeMCPPMTaskService) GetByID(_ context.Context, taskID string) (*model.TaskDetail, error) {
	return f.tasks[taskID], nil
}

func (f *fakeMCPPMTaskService) Update(_ context.Context, taskID string, request model.UpdateTaskRequest, _ string) (*model.TaskDetail, error) {
	f.updateID = taskID
	f.updateRequest = request
	task := *f.tasks[taskID]
	if request.Name != nil {
		task.Task.Name = *request.Name
	}
	return &task, nil
}

type fakeMCPPMChecklistService struct {
	items         map[string][]model.PMChecklistItem
	createTaskID  string
	createRequest model.CreateChecklistItemRequest
	updateID      string
	updateRequest model.UpdateChecklistItemRequest
}

type fakeMCPTaskBatchCreator struct {
	workspaceID string
	epicID      string
	actorID     string
	tasks       []model.ProposedTask
}

func (f *fakeMCPTaskBatchCreator) CreateEpicTaskBatch(_ context.Context, workspaceID, epicID, actorID string, tasks []model.ProposedTask) ([]model.PMTask, error) {
	f.workspaceID = workspaceID
	f.epicID = epicID
	f.actorID = actorID
	f.tasks = tasks
	return []model.PMTask{{ID: "task-created", WorkspaceID: workspaceID, Name: tasks[0].Name}}, nil
}

func (f *fakeMCPPMChecklistService) List(_ context.Context, taskID, _ string) ([]model.PMChecklistItem, error) {
	return f.items[taskID], nil
}

func (f *fakeMCPPMChecklistService) Create(_ context.Context, taskID string, request model.CreateChecklistItemRequest, _, _ string) (*model.PMChecklistItem, error) {
	f.createTaskID = taskID
	f.createRequest = request
	return &model.PMChecklistItem{ID: "check-created", TaskID: taskID, Text: request.Text}, nil
}

func (f *fakeMCPPMChecklistService) Update(_ context.Context, itemID string, request model.UpdateChecklistItemRequest, _, _ string) (*model.PMChecklistItem, error) {
	f.updateID = itemID
	f.updateRequest = request
	return &model.PMChecklistItem{ID: itemID, TaskID: "task-1", Text: "Review changes", Completed: request.Completed != nil && *request.Completed}, nil
}
