package service

import (
	"context"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestPMTriageReviewedSupportDraftCreatesExactlyReviewedWork(t *testing.T) {
	env := newTaskTestEnv(t)
	workflowID := seedTaskTeamWorkflow(t, env, env.teamID)
	ctx := context.Background()
	conversations := repository.NewSupportConversationRepository(env.db)
	messages := repository.NewSupportMessageRepository(env.db)
	associations := repository.NewCRMAssociationRepository(env.db)
	support := NewSupportInboxService(conversations, nil, messages, nil, associations, nil, nil, nil, nil, nil, nil, repository.NewUserRepository(env.db), nil, nil, nil)
	support.SetTaskService(env.svc)
	conversation := &model.SupportConversation{WorkspaceID: env.wsID, Subject: "Invoice CSV export fails", Status: "open", Priority: "medium"}
	if err := conversations.Create(ctx, conversation); err != nil {
		t.Fatal(err)
	}
	message := &model.SupportMessage{WorkspaceID: env.wsID, ConversationID: conversation.ID, SenderType: "customer", MessageType: "reply", Content: "Exporting archived invoices returns a 500 error. Please restore CSV export."}
	if err := messages.Create(ctx, message); err != nil {
		t.Fatal(err)
	}
	preview, err := support.PreviewTaskFromConversation(ctx, env.wsID, conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Installing a provider only after preview detects accidental regeneration at save.
	provider := &scriptedSupportRewriteLLM{}
	support.SetSupportAIService(&SupportAIService{llmProvider: provider})
	name, description, taskType, priority := "Reviewed invoice export fix", "<p>Keep this exact human-reviewed description.</p>", "bug", "low"
	created, err := support.CreateTaskFromConversation(ctx, env.wsID, conversation.ID, env.userID, model.CreateTaskFromConversationRequest{ReviewedDraft: true, SourceHash: preview.SourceHash, TeamID: &env.teamID, Name: &name, Description: &description, TaskType: &taskType, Priority: &priority})
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.lastReq.Messages) != 0 {
		t.Fatal("reviewed draft was regenerated")
	}
	task, err := repository.NewPMTaskRepository(env.db).GetRawByID(ctx, created.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if task.WorkflowID != workflowID || task.WorkflowStateID != workflowID+"-todo" {
		t.Fatalf("support task did not use team workflow: %s/%s", task.WorkflowID, task.WorkflowStateID)
	}
	if task.Name != name || derefString(task.Description) != description || task.TaskType != taskType || task.Priority != priority {
		t.Fatalf("reviewed fields changed: %+v", task)
	}
	linked, err := conversations.GetByID(ctx, env.wsID, conversation.ID, "", model.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if linked.LinkedTaskID == nil || *linked.LinkedTaskID != created.TaskID {
		t.Fatal("conversation evidence was not linked")
	}
}
func TestPMTriageReviewedSupportDraftRejectsChangedEvidence(t *testing.T) {
	env := newTaskTestEnv(t)
	ctx := context.Background()
	conversations := repository.NewSupportConversationRepository(env.db)
	messages := repository.NewSupportMessageRepository(env.db)
	support := NewSupportInboxService(conversations, nil, messages, nil, repository.NewCRMAssociationRepository(env.db), nil, nil, nil, nil, nil, nil, repository.NewUserRepository(env.db), nil, nil, nil)
	support.SetTaskService(env.svc)
	conversation := &model.SupportConversation{WorkspaceID: env.wsID, Subject: "Invoice CSV export fails", Status: "open", Priority: "medium"}
	if err := conversations.Create(ctx, conversation); err != nil {
		t.Fatal(err)
	}
	preview, err := support.PreviewTaskFromConversation(ctx, env.wsID, conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := messages.Create(ctx, &model.SupportMessage{WorkspaceID: env.wsID, ConversationID: conversation.ID, SenderType: "customer", MessageType: "reply", Content: "This was a different issue."}); err != nil {
		t.Fatal(err)
	}
	name, description, taskType := "Fix export", "Reviewed description", "bug"
	_, err = support.CreateTaskFromConversation(ctx, env.wsID, conversation.ID, env.userID, model.CreateTaskFromConversationRequest{ReviewedDraft: true, SourceHash: preview.SourceHash, TeamID: &env.teamID, Name: &name, Description: &description, TaskType: &taskType})
	if !errors.Is(err, ErrPMTriageStale) {
		t.Fatalf("expected stale refusal, got %v", err)
	}
	var count int64
	if err := env.db.Model(&model.PMTask{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("stale draft created work")
	}
}
