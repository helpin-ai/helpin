package service

import (
	"context"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

type conversationTaskTestEnv struct {
	taskTestEnv
	svc          *SupportInboxService
	convRepo     *repository.SupportConversationRepository
	messageRepo  *repository.SupportMessageRepository
	assocRepo    *repository.CRMAssociationRepository
	conversation *model.SupportConversation
}

func newConversationTaskTestEnv(t *testing.T) conversationTaskTestEnv {
	t.Helper()
	env := newTaskTestEnv(t)
	convRepo := repository.NewSupportConversationRepository(env.db)
	messageRepo := repository.NewSupportMessageRepository(env.db)
	assocRepo := repository.NewCRMAssociationRepository(env.db)
	svc := NewSupportInboxService(convRepo, nil, messageRepo, nil, assocRepo, nil, nil, nil, nil, nil, nil, repository.NewUserRepository(env.db), nil, nil, nil)
	svc.SetTaskService(env.svc)

	conversation := &model.SupportConversation{WorkspaceID: env.wsID, Subject: "Invoice sync is failing", Status: "open"}
	if err := convRepo.Create(context.Background(), conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	return conversationTaskTestEnv{taskTestEnv: env, svc: svc, convRepo: convRepo, messageRepo: messageRepo, assocRepo: assocRepo, conversation: conversation}
}

func (env conversationTaskTestEnv) createTask(t *testing.T, name string) *model.TaskDetail {
	t.Helper()
	detail, err := env.taskTestEnv.svc.Create(context.Background(), model.CreateTaskRequest{WorkspaceID: env.wsID, Name: name, TeamID: &env.teamID}, env.userID)
	if err != nil {
		t.Fatalf("create task %q: %v", name, err)
	}
	return detail
}

func (env conversationTaskTestEnv) taskLines(t *testing.T) []model.SupportMessage {
	t.Helper()
	messages, err := env.messageRepo.ListByConversation(context.Background(), env.wsID, env.conversation.ID, true)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	var lines []model.SupportMessage
	for _, message := range messages {
		if message.SystemEventType != nil && strings.HasPrefix(*message.SystemEventType, "task_") {
			lines = append(lines, message)
		}
	}
	return lines
}

func (env conversationTaskTestEnv) linkedTaskIDs(t *testing.T) map[string]bool {
	t.Helper()
	tasks, err := env.svc.conversationTaskAssociations(context.Background(), env.wsID, env.conversation.ID)
	if err != nil {
		t.Fatalf("list task associations: %v", err)
	}
	ids := make(map[string]bool, len(tasks))
	for id := range tasks {
		ids[id] = true
	}
	return ids
}

func (env conversationTaskTestEnv) mainTaskID(t *testing.T) string {
	t.Helper()
	conversation, err := env.svc.loadConversationUnscoped(context.Background(), env.wsID, env.conversation.ID)
	if err != nil || conversation == nil {
		t.Fatalf("load conversation: %v", err)
	}
	return derefString(conversation.LinkedTaskID)
}

func TestUpdateConversationLinkedTask_KeepsOtherTasksAndNotesThread(t *testing.T) {
	env := newConversationTaskTestEnv(t)
	ctx := context.Background()
	sidebarTask := env.createTask(t, "Archived invoices break import")
	mainTask := env.createTask(t, "Xero sync returns 500")
	if err := env.assocRepo.Create(ctx, &model.CRMAssociation{WorkspaceID: env.wsID, FromObjectType: model.CRMObjectSupportConversation, FromObjectID: env.conversation.ID, ToObjectType: model.CRMObjectTask, ToObjectID: sidebarTask.Task.ID}); err != nil {
		t.Fatalf("link sidebar task: %v", err)
	}

	for range 2 {
		if err := env.svc.LinkConversationTask(ctx, env.wsID, env.conversation.ID, mainTask.Task.ID, env.userID); err != nil {
			t.Fatalf("LinkConversationTask: %v", err)
		}
	}
	if got := env.mainTaskID(t); got != mainTask.Task.ID {
		t.Fatalf("main task = %q, want %q", got, mainTask.Task.ID)
	}
	if linked := env.linkedTaskIDs(t); !linked[sidebarTask.Task.ID] || !linked[mainTask.Task.ID] {
		t.Fatalf("linked tasks = %v, want both tasks", linked)
	}
	lines := env.taskLines(t)
	if len(lines) != 1 || *lines[0].SystemEventType != string(model.SystemEventTaskLinked) || !strings.Contains(lines[0].Content, "Story linked task") || !strings.Contains(lines[0].Content, "Xero sync returns 500") {
		t.Fatalf("thread lines after link = %+v, want one linked line", lines)
	}

	if err := env.svc.UpdateConversationLinkedTask(ctx, env.wsID, env.conversation.ID, nil, env.userID); err != nil {
		t.Fatalf("clear main task: %v", err)
	}
	if got := env.mainTaskID(t); got != "" {
		t.Fatalf("main task = %q, want cleared", got)
	}
	if linked := env.linkedTaskIDs(t); !linked[sidebarTask.Task.ID] || linked[mainTask.Task.ID] {
		t.Fatalf("linked tasks = %v, want only the sidebar task", linked)
	}
	lines = env.taskLines(t)
	if len(lines) != 2 || *lines[1].SystemEventType != string(model.SystemEventTaskUnlinked) || !strings.Contains(lines[1].Content, "Story unlinked task") {
		t.Fatalf("thread lines after clear = %+v, want an unlinked line", lines)
	}
}

func TestCRMAssociationService_NotesConversationTaskLinks(t *testing.T) {
	env := newConversationTaskTestEnv(t)
	ctx := authorization.WithActor(context.Background(), &authorization.Actor{UserID: env.userID, WorkspaceID: env.wsID, Role: model.RoleAdmin})
	associations := NewCRMAssociationService(env.assocRepo).SetConversationTaskLinkRecorder(env.svc)
	task := env.createTask(t, "Xero sync returns 500")
	request := model.CreateCRMAssociationRequest{WorkspaceID: env.wsID, FromObjectType: model.CRMObjectSupportConversation, FromObjectID: env.conversation.ID, ToObjectType: model.CRMObjectTask, ToObjectID: task.Task.ID}

	assoc, err := associations.Create(ctx, request)
	if err != nil {
		t.Fatalf("link task: %v", err)
	}
	if _, err := associations.Create(ctx, request); err != nil {
		t.Fatalf("link task again: %v", err)
	}
	lines := env.taskLines(t)
	if len(lines) != 1 || *lines[0].SystemEventType != string(model.SystemEventTaskLinked) || derefString(lines[0].SenderUserID) != env.userID {
		t.Fatalf("thread lines after sidebar link = %+v, want one linked line by the teammate", lines)
	}

	if err := env.svc.LinkConversationTask(ctx, env.wsID, env.conversation.ID, task.Task.ID, env.userID); err != nil {
		t.Fatalf("make it the main task: %v", err)
	}
	if err := associations.DeleteScoped(ctx, env.wsID, assoc.ID); err != nil {
		t.Fatalf("unlink task: %v", err)
	}
	if got := env.mainTaskID(t); got != "" {
		t.Fatalf("main task = %q, want cleared after the sidebar unlink", got)
	}
	lines = env.taskLines(t)
	if len(lines) != 2 || *lines[1].SystemEventType != string(model.SystemEventTaskUnlinked) {
		t.Fatalf("thread lines after sidebar unlink = %+v, want an unlinked line", lines)
	}
}

func TestLinkCreatedConversationTask_AgentOnItsOwnIsHelpinAI(t *testing.T) {
	env := newConversationTaskTestEnv(t)
	ctx := withCommandAgent(context.Background(), "agent-1")
	task := env.createTask(t, "Xero sync returns 500")

	if err := env.svc.LinkCreatedConversationTask(ctx, env.wsID, env.conversation.ID, task.Task.ID, ""); err != nil {
		t.Fatalf("LinkCreatedConversationTask: %v", err)
	}
	if got := env.mainTaskID(t); got != task.Task.ID {
		t.Fatalf("main task = %q, want %q", got, task.Task.ID)
	}
	lines := env.taskLines(t)
	if len(lines) != 1 {
		t.Fatalf("thread lines = %+v, want one created line", lines)
	}
	line := lines[0]
	if *line.SystemEventType != string(model.SystemEventTaskCreated) || line.SenderType != "ai" || derefString(line.SenderAgentID) != "agent-1" || !strings.HasPrefix(line.Content, "Helpin AI created task") {
		t.Fatalf("created line = %+v, want Helpin AI as the sender", line)
	}
}

func TestSupportTaskEventContent(t *testing.T) {
	tests := []struct {
		name    string
		taskKey string
		task    string
		want    string
	}{
		{name: "key and name", taskKey: "HLP-12", task: "Fix  login", want: "Sam linked task #HLP-12: Fix login."},
		{name: "key only", taskKey: "#HLP-12", want: "Sam linked task #HLP-12."},
		{name: "name only", task: "Fix login", want: "Sam linked task: Fix login."},
		{name: "neither", want: "Sam linked a task."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := supportTaskEventContent("Sam", "linked", tt.taskKey, tt.task); got != tt.want {
				t.Errorf("supportTaskEventContent() = %q, want %q", got, tt.want)
			}
		})
	}
}
