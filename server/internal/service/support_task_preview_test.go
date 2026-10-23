package service

import (
	"context"
	"errors"
	"strings"
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
	seedUser(t, env.db, "second-owner", "second@test.com", "Second owner", "hash")
	seedWorkspaceMember(t, env.db, "second-member", env.wsID, "second-owner", "second@test.com", "Second owner", model.RoleMember)
	agentID := "agent-without-runtime"
	name, description, taskType, priority := "Reviewed invoice export fix", "<p>Keep this exact human-reviewed description.</p>", "bug", "low"
	created, err := support.CreateTaskFromConversation(ctx, env.wsID, conversation.ID, env.userID, model.CreateTaskFromConversationRequest{ReviewedDraft: true, SourceHash: preview.SourceHash, TeamID: &env.teamID, Name: &name, Description: &description, TaskType: &taskType, Priority: &priority, OwnerMemberIDs: []string{"member-story-001", "second-member"}, AssignedAgentID: &agentID, RunOnCreate: true})
	if err != nil {
		t.Fatal(err)
	}
	if created.AgentRunError == nil || *created.AgentRunError != "agent service is not configured" {
		t.Fatalf("expected saved-task agent failure, got %+v", created)
	}
	detail, err := env.svc.GetByID(ctx, created.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Task.OwnerMemberIDs) != 2 || derefString(detail.Task.AssignedAgentID) != agentID {
		t.Fatalf("support creation dropped PM assignees: %+v", detail.Task)
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

func TestSupportTaskDraftPreviewUsesDestinationTeamAndRejectsUnknownTeam(t *testing.T) {
	env := newTaskTestEnv(t)
	ctx := context.Background()
	conversations := repository.NewSupportConversationRepository(env.db)
	messages := repository.NewSupportMessageRepository(env.db)
	support := NewSupportInboxService(conversations, nil, messages, nil, repository.NewCRMAssociationRepository(env.db), nil, nil, nil, nil, nil, nil, repository.NewUserRepository(env.db), nil, nil, nil)
	support.workspaceRepo = repository.NewWorkspaceRepository(env.db)
	if err := env.db.Model(&model.WorkspaceTeam{}).Where("id = ?", env.teamID).Updates(map[string]interface{}{"name": "Marketing", "team_type": "marketing", "description": "Campaigns and partnerships"}).Error; err != nil {
		t.Fatal(err)
	}
	conversation := &model.SupportConversation{WorkspaceID: env.wsID, Subject: "Partner newsletter promotion", Status: "open", Priority: "low"}
	if err := conversations.Create(ctx, conversation); err != nil {
		t.Fatal(err)
	}
	provider := &recordingSupportTaskDraftProvider{response: `{"title":"Prepare partner newsletter promotion","summary":"Partner requests a newsletter promotion.","description_markdown":"Prepare a newsletter promotion for the partner launch.","task_type":"chore","priority":"low"}`}
	support.SetSupportAIService(&SupportAIService{llmProvider: provider})
	preview, err := support.PreviewTaskFromConversation(ctx, env.wsID, conversation.ID, env.teamID)
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 || preview.SourceHash == "" {
		t.Fatalf("unexpected draft generation: calls=%d preview=%+v", provider.calls, preview)
	}
	last := provider.request.Messages[len(provider.request.Messages)-1].Content
	if !strings.Contains(last, `"name":"Marketing"`) || !strings.Contains(last, `"description":"Campaigns and partnerships"`) {
		t.Fatalf("missing destination context: %s", last)
	}
	if !strings.Contains(provider.request.SystemPrompt, "120 words") {
		t.Fatal("missing concise draft guidance")
	}
	if _, err := support.PreviewTaskFromConversation(ctx, env.wsID, conversation.ID, "other-workspace-team"); err == nil {
		t.Fatal("unknown team accepted")
	}
	if provider.calls != 1 {
		t.Fatal("invalid team triggered AI")
	}
}
