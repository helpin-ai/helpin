package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestSupportConversationRepositoryListViews(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-support-views"
	userID := "user-support-views"
	seedWorkspace(t, db, workspaceID, "Support Views", "support-views", userID)
	repo := repository.NewSupportConversationRepository(db)

	createConversation := func(id, status string, fields map[string]any) {
		t.Helper()
		conversation := &model.SupportConversation{
			ID:          id,
			WorkspaceID: workspaceID,
			Subject:     id,
			Status:      status,
			Priority:    "medium",
			Channel:     "widget",
			Source:      "widget",
		}
		if err := repo.Create(ctx, conversation); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
		if len(fields) > 0 {
			if err := db.Model(&model.SupportConversation{}).Where("id = ?", id).Updates(fields).Error; err != nil {
				t.Fatalf("update %s: %v", id, err)
			}
		}
	}

	now := time.Now().UTC()
	createConversation("human-open", model.SupportConversationStatusOpen, nil)
	createConversation("ai-handling", model.SupportConversationStatusOpen, map[string]any{
		"ai_state":   "pending",
		"flow_state": model.SupportConversationFlowStateAIHandling,
	})
	createConversation("ai-escalated", model.SupportConversationStatusOpen, map[string]any{
		"ai_state":   "escalated",
		"flow_state": model.SupportConversationFlowStateAIHandling,
	})
	createConversation("requested-human", model.SupportConversationStatusOpen, map[string]any{
		"ai_state":                    "pending",
		"flow_state":                  model.SupportConversationFlowStateAIHandling,
		"customer_requested_human_at": now,
	})
	createConversation("waiting", model.SupportConversationStatusWaitingOnCustomer, nil)
	createConversation("human-resolved", model.SupportConversationStatusResolved, map[string]any{
		"flow_state": model.SupportConversationFlowStateResolvedByHuman,
	})
	createConversation("ai-resolved", model.SupportConversationStatusResolved, map[string]any{
		"ai_state":   "resolved",
		"flow_state": model.SupportConversationFlowStateResolvedByAI,
	})
	createConversation("spam", model.SupportConversationStatusSpam, nil)
	createConversation("assigned-mine", model.SupportConversationStatusOpen, map[string]any{
		"assigned_user_id": userID,
	})
	createConversation("opened-mine", model.SupportConversationStatusWaitingOnCustomer, map[string]any{
		"opened_by_user_id": userID,
	})
	createConversation("mentioned-mine", model.SupportConversationStatusOpen, nil)
	createConversation("resolved-mine", model.SupportConversationStatusResolved, map[string]any{
		"assigned_user_id": userID,
	})

	if err := db.Exec(`INSERT INTO support_messages
		(id, workspace_id, conversation_id, sender_type, message_type, content, is_internal, metadata, created_at, updated_at)
		VALUES (?, ?, ?, 'user', 'reply', 'Mentioning teammate', 1, ?, ?, ?)`,
		"msg-mention-user", workspaceID, "mentioned-mine", `{"mentioned_user_ids":["`+userID+`"]}`, now, now,
	).Error; err != nil {
		t.Fatalf("insert mention message: %v", err)
	}

	listIDs := func(filter string) []string {
		t.Helper()
		conversations, _, err := repo.List(ctx, repository.ConversationRepositoryListParams{
			ConversationListParams: repository.ConversationListParams{
				WorkspaceID: workspaceID,
				UserID:      userID,
				Filter:      filter,
				Pagination:  model.PMPagination{Page: 1, PerPage: 50},
			},
			Role: model.RoleOwner,
		})
		if err != nil {
			t.Fatalf("list %s: %v", filter, err)
		}
		ids := make([]string, 0, len(conversations))
		for _, conversation := range conversations {
			ids = append(ids, conversation.ID)
		}
		return ids
	}

	assertContainsExactly(t, listIDs(model.SupportConversationListFilterInbox), []string{
		"human-open",
		"ai-escalated",
		"requested-human",
		"assigned-mine",
		"mentioned-mine",
	})
	assertContainsExactly(t, listIDs(model.SupportConversationListFilterMine), []string{
		"assigned-mine",
		"opened-mine",
		"mentioned-mine",
	})
	assertContainsExactly(t, listIDs(model.SupportConversationListFilterMentions), []string{
		"assigned-mine",
		"opened-mine",
		"mentioned-mine",
	})
	assertContainsExactly(t, listIDs(model.SupportConversationListFilterResolved), []string{
		"human-resolved",
		"resolved-mine",
	})
}

func TestSupportConversationRepositoryListAssignmentAndSortFilters(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-support-list-filters"
	userID := "user-support-list-filters"
	otherUserID := "user-support-list-filters-other"
	seedWorkspace(t, db, workspaceID, "Support List Filters", "support-list-filters", userID)
	repo := repository.NewSupportConversationRepository(db)

	createConversation := func(id string, updatedAt time.Time, fields map[string]any) {
		t.Helper()
		conversation := &model.SupportConversation{
			ID:          id,
			WorkspaceID: workspaceID,
			Subject:     id,
			Status:      model.SupportConversationStatusOpen,
			Priority:    "medium",
			Channel:     "widget",
			Source:      "widget",
			UpdatedAt:   updatedAt,
		}
		if err := repo.Create(ctx, conversation); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
		updates := map[string]any{"updated_at": updatedAt}
		for key, value := range fields {
			updates[key] = value
		}
		if err := db.Model(&model.SupportConversation{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			t.Fatalf("update %s: %v", id, err)
		}
	}

	now := time.Now().UTC()
	createConversation("assigned-me", now.Add(-3*time.Minute), map[string]any{"assigned_user_id": userID})
	createConversation("assigned-other", now.Add(-2*time.Minute), map[string]any{"assigned_user_id": otherUserID})
	createConversation("unassigned", now.Add(-1*time.Minute), nil)
	createConversation("agent-owned", now, map[string]any{"assigned_agent_id": "agent-support-list-filters"})

	listIDs := func(assignedTo, sortOrder string) []string {
		t.Helper()
		conversations, _, err := repo.List(ctx, repository.ConversationRepositoryListParams{
			ConversationListParams: repository.ConversationListParams{
				WorkspaceID: workspaceID,
				UserID:      userID,
				AssignedTo:  assignedTo,
				Sort:        sortOrder,
				Pagination:  model.PMPagination{Page: 1, PerPage: 50},
			},
			Role: model.RoleOwner,
		})
		if err != nil {
			t.Fatalf("list assigned_to=%s sort=%s: %v", assignedTo, sortOrder, err)
		}
		ids := make([]string, 0, len(conversations))
		for _, conversation := range conversations {
			ids = append(ids, conversation.ID)
		}
		return ids
	}

	assertContainsExactly(t, listIDs("me", ""), []string{"assigned-me"})
	assertContainsExactly(t, listIDs("unassigned", ""), []string{"unassigned"})
	assertContainsExactly(t, listIDs("others", ""), []string{"assigned-other"})

	if got, want := listIDs("", "oldest"), []string{"assigned-me", "assigned-other", "unassigned", "agent-owned"}; len(got) != len(want) {
		t.Fatalf("got ids %v, want %v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got ids %v, want %v", got, want)
			}
		}
	}
}

func TestSupportConversationRepositoryListStatusFilters(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-support-status-filters"
	userID := "user-support-status-filters"
	seedWorkspace(t, db, workspaceID, "Support Status Filters", "support-status-filters", userID)
	repo := repository.NewSupportConversationRepository(db)

	for _, item := range []struct {
		id     string
		status string
	}{
		{id: "open", status: model.SupportConversationStatusOpen},
		{id: "waiting", status: model.SupportConversationStatusWaitingOnCustomer},
		{id: "resolved", status: model.SupportConversationStatusResolved},
		{id: "spam", status: model.SupportConversationStatusSpam},
	} {
		conversation := &model.SupportConversation{
			ID:          item.id,
			WorkspaceID: workspaceID,
			Subject:     item.id,
			Status:      item.status,
			Priority:    "medium",
			Channel:     "widget",
			Source:      "widget",
		}
		if err := repo.Create(ctx, conversation); err != nil {
			t.Fatalf("create %s: %v", item.id, err)
		}
	}

	conversations, _, err := repo.List(ctx, repository.ConversationRepositoryListParams{
		ConversationListParams: repository.ConversationListParams{
			WorkspaceID: workspaceID,
			UserID:      userID,
			Statuses:    []string{model.SupportConversationStatusOpen, model.SupportConversationStatusWaitingOnCustomer},
			Pagination:  model.PMPagination{Page: 1, PerPage: 50},
		},
		Role: model.RoleOwner,
	})
	if err != nil {
		t.Fatalf("list statuses: %v", err)
	}
	ids := make([]string, 0, len(conversations))
	for _, conversation := range conversations {
		ids = append(ids, conversation.ID)
	}
	assertContainsExactly(t, ids, []string{"open", "waiting"})
}

func assertContainsExactly(t *testing.T, got []string, want []string) {
	t.Helper()
	gotSet := make(map[string]bool, len(got))
	for _, id := range got {
		gotSet[id] = true
	}
	if len(gotSet) != len(want) {
		t.Fatalf("got ids %v, want %v", got, want)
	}
	for _, id := range want {
		if !gotSet[id] {
			t.Fatalf("got ids %v, want %v", got, want)
		}
	}
}
