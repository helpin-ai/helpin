package service

import (
	"context"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSupportTagService(t *testing.T) {
	db := newTestDB(t)
	mustExec(t, db, `CREATE TABLE IF NOT EXISTS support_tags (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT NOT NULL,
		name TEXT NOT NULL,
		color TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`)
	mustExec(t, db, `CREATE TABLE IF NOT EXISTS support_conversation_tags (
		conversation_id TEXT NOT NULL,
		tag_id TEXT NOT NULL,
		created_at DATETIME,
		PRIMARY KEY (conversation_id, tag_id)
	)`)
	ctx := context.Background()
	workspaceID := "ws-support-tags"
	seedWorkspace(t, db, workspaceID, "Support Tags", "support-tags", "user-123")

	conversationRepo := repository.NewSupportConversationRepository(db)
	tagRepo := repository.NewSupportTagRepository(db)
	tagService := NewSupportTagService(tagRepo, conversationRepo, nil)

	conversation := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Billing question",
		Status:      model.SupportConversationStatusOpen,
		Priority:    "medium",
		Channel:     "email",
	}
	if err := conversationRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	color := "#ef4444"
	tag, err := tagService.Create(ctx, workspaceID, model.CreateSupportTagRequest{
		Name:  " Billing ",
		Color: &color,
	})
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if tag.Name != "Billing" {
		t.Fatalf("expected trimmed tag name, got %q", tag.Name)
	}

	if _, err := tagService.Create(ctx, workspaceID, model.CreateSupportTagRequest{Name: "billing"}); err == nil {
		t.Fatal("expected duplicate tag name to fail case-insensitively")
	}

	if err := tagService.AddConversationTag(ctx, workspaceID, conversation.ID, tag.ID); err != nil {
		t.Fatalf("add tag to conversation: %v", err)
	}

	tagsByConversation, err := tagRepo.ListByConversationIDs(ctx, workspaceID, []string{conversation.ID})
	if err != nil {
		t.Fatalf("list tags by conversation: %v", err)
	}
	tags := tagsByConversation[conversation.ID]
	if len(tags) != 1 || tags[0].Name != "Billing" {
		t.Fatalf("expected Billing tag on conversation, got %#v", tags)
	}
}

func idsFromConversations(conversations []model.SupportConversation) []string {
	ids := make([]string, 0, len(conversations))
	for _, conversation := range conversations {
		ids = append(ids, conversation.ID)
	}
	return ids
}

func TestSupportConversationTagsInList(t *testing.T) {
	db := newTestDB(t)
	mustExec(t, db, `CREATE TABLE IF NOT EXISTS support_tags (
		id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
		workspace_id TEXT NOT NULL,
		name TEXT NOT NULL,
		color TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`)
	mustExec(t, db, `CREATE TABLE IF NOT EXISTS support_conversation_tags (
		conversation_id TEXT NOT NULL,
		tag_id TEXT NOT NULL,
		created_at DATETIME,
		PRIMARY KEY (conversation_id, tag_id)
	)`)
	workspaceID := "ws-support-tag-list"
	ctx := authorization.WithActor(context.Background(), &authorization.Actor{
		WorkspaceID:       workspaceID,
		WorkspaceMemberID: "member-support-tag-list",
		Role:              model.RoleOwner,
	})
	seedWorkspace(t, db, workspaceID, "Support Tag List", "support-tag-list", "user-123")

	conversationRepo := repository.NewSupportConversationRepository(db)
	tagRepo := repository.NewSupportTagRepository(db)
	tagService := NewSupportTagService(tagRepo, conversationRepo, nil)
	inboxService := NewSupportInboxService(conversationRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	inboxService.SetSupportTagRepo(tagRepo)

	handoff := &model.SupportConversation{
		ID:          "handoff-conv",
		WorkspaceID: workspaceID,
		Subject:     "Needs help",
		Status:      model.SupportConversationStatusOpen,
		Priority:    "medium",
		Channel:     "widget",
		AIState:     strPtr("escalated"),
		FlowState:   strPtr(model.SupportConversationFlowStateWaitingForHuman),
	}
	resolvedByAI := &model.SupportConversation{
		ID:          "ai-resolved-conv",
		WorkspaceID: workspaceID,
		Subject:     "Solved by AI",
		Status:      model.SupportConversationStatusResolved,
		Priority:    "medium",
		Channel:     "widget",
		AIState:     strPtr("resolved"),
		FlowState:   strPtr(model.SupportConversationFlowStateResolvedByAI),
	}
	aiHandling := &model.SupportConversation{
		ID:          "ai-handling-conv",
		WorkspaceID: workspaceID,
		Subject:     "AI is handling",
		Status:      model.SupportConversationStatusOpen,
		Priority:    "medium",
		Channel:     "widget",
		AIState:     strPtr("pending"),
		FlowState:   strPtr(model.SupportConversationFlowStateAIHandling),
	}
	plain := &model.SupportConversation{
		ID:          "plain-conv",
		WorkspaceID: workspaceID,
		Subject:     "Plain",
		Status:      model.SupportConversationStatusOpen,
		Priority:    "medium",
		Channel:     "email",
	}
	for _, conversation := range []*model.SupportConversation{handoff, resolvedByAI, aiHandling, plain} {
		if err := conversationRepo.Create(ctx, conversation); err != nil {
			t.Fatalf("create conversation %s: %v", conversation.ID, err)
		}
	}

	tag, err := tagService.Create(ctx, workspaceID, model.CreateSupportTagRequest{Name: "Billing"})
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if err := tagService.AddConversationTag(ctx, workspaceID, handoff.ID, tag.ID); err != nil {
		t.Fatalf("add conversation tag: %v", err)
	}
	directTags, err := tagRepo.ListByConversationIDs(ctx, workspaceID, []string{handoff.ID})
	if err != nil {
		t.Fatalf("list direct tags: %v", err)
	}
	if got := directTags[handoff.ID]; len(got) != 1 || got[0].Name != "Billing" {
		t.Fatalf("expected direct Billing tag, got %#v for conversation id %q", got, handoff.ID)
	}

	resp, err := inboxService.ListConversationsWithMeta(ctx, SupportConversationListParams{
		WorkspaceID: workspaceID,
		Pagination:  model.PMPagination{Page: 1, PerPage: 50},
	})
	if err != nil {
		t.Fatalf("list conversations with tags: %v", err)
	}
	bySubject := map[string]model.SupportConversation{}
	for _, conversation := range resp.Data {
		bySubject[conversation.Subject] = conversation
	}
	handoffListConversation, ok := bySubject["Needs help"]
	if !ok {
		t.Fatalf("expected Needs help conversation in list, got %#v", bySubject)
	}
	if got := handoffListConversation.Tags; len(got) != 1 || got[0].Name != "Billing" {
		t.Fatalf("expected Billing user tag, got %#v", got)
	}
	if got := handoffListConversation.SystemTags; len(got) != 1 || got[0] != model.SupportSystemTagAIHandoff {
		t.Fatalf("expected AI handoff system tag, got %#v", got)
	}
	if got := bySubject["Solved by AI"].SystemTags; len(got) != 1 || got[0] != model.SupportSystemTagAIResolved {
		t.Fatalf("expected AI resolved system tag, got %#v", got)
	}

	userTagFiltered, _, err := conversationRepo.List(ctx, repository.ConversationRepositoryListParams{
		ConversationListParams: repository.ConversationListParams{
			WorkspaceID: workspaceID,
			TagIDs:      []string{tag.ID},
			Pagination:  model.PMPagination{Page: 1, PerPage: 50},
		},
		Role: model.RoleOwner,
	})
	if err != nil {
		t.Fatalf("filter by user tag: %v", err)
	}
	if len(userTagFiltered) != 1 || userTagFiltered[0].ID != handoff.ID {
		t.Fatalf("expected only handoff conversation by user tag, got %#v", userTagFiltered)
	}

	systemTagFiltered, _, err := conversationRepo.List(ctx, repository.ConversationRepositoryListParams{
		ConversationListParams: repository.ConversationListParams{
			WorkspaceID: workspaceID,
			SystemTags:  []string{model.SupportSystemTagAIResolved},
			Pagination:  model.PMPagination{Page: 1, PerPage: 50},
		},
		Role: model.RoleOwner,
	})
	if err != nil {
		t.Fatalf("filter by system tag: %v", err)
	}
	if len(systemTagFiltered) != 1 || systemTagFiltered[0].ID != resolvedByAI.ID {
		t.Fatalf("expected only AI resolved conversation by system tag, got %#v", systemTagFiltered)
	}

	aiFiltered, _, err := conversationRepo.List(ctx, repository.ConversationRepositoryListParams{
		ConversationListParams: repository.ConversationListParams{
			WorkspaceID: workspaceID,
			AIFilters:   []string{"handling", "handoff"},
			Pagination:  model.PMPagination{Page: 1, PerPage: 50},
		},
		Role: model.RoleOwner,
	})
	if err != nil {
		t.Fatalf("filter by AI state: %v", err)
	}
	assertContainsExactly(t, idsFromConversations(aiFiltered), []string{aiHandling.ID, handoff.ID})

	invalidSystemTagFiltered, _, err := conversationRepo.List(ctx, repository.ConversationRepositoryListParams{
		ConversationListParams: repository.ConversationListParams{
			WorkspaceID: workspaceID,
			SystemTags:  []string{"unknown_system_tag"},
			Pagination:  model.PMPagination{Page: 1, PerPage: 50},
		},
		Role: model.RoleOwner,
	})
	if err != nil {
		t.Fatalf("filter by invalid system tag: %v", err)
	}
	if len(invalidSystemTagFiltered) != 0 {
		t.Fatalf("expected invalid system tag to return no conversations, got %#v", invalidSystemTagFiltered)
	}
}
