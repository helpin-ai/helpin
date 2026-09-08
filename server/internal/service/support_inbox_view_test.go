package service

import (
	"context"
	"slices"
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
		"waiting",
		"assigned-mine",
		"opened-mine",
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
		"ai-resolved",
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

	createConversation := func(id string, createdAt, updatedAt time.Time, fields map[string]any) {
		t.Helper()
		conversation := &model.SupportConversation{
			ID:          id,
			WorkspaceID: workspaceID,
			Subject:     id,
			Status:      model.SupportConversationStatusOpen,
			Priority:    "medium",
			Channel:     "widget",
			Source:      "widget",
			CreatedAt:   createdAt,
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

	base := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	// Empty conversations sort by creation time, not by unrelated record updates.
	// Keep updated_at in the opposite order to catch regressions to the old sort.
	createConversation("assigned-me", base.Add(-3*time.Minute), base.Add(10*time.Minute), map[string]any{"assigned_user_id": userID})
	createConversation("assigned-other", base.Add(-2*time.Minute), base.Add(9*time.Minute), map[string]any{"assigned_user_id": otherUserID})
	createConversation("unassigned", base.Add(-time.Minute), base.Add(8*time.Minute), nil)
	createConversation("agent-owned", base, base.Add(7*time.Minute), map[string]any{"assigned_agent_id": "agent-support-list-filters"})
	createConversation("opened-by-me", base.Add(time.Minute), base.Add(6*time.Minute), map[string]any{"opened_by_user_id": userID})
	// The oldest conversation has the newest activity because of its internal note.
	createConversation("mentioned-me", base.Add(-4*time.Minute), base.Add(5*time.Minute), nil)
	mentionAt := base.Add(2 * time.Minute)

	if err := db.Exec(`INSERT INTO support_messages
		(id, workspace_id, conversation_id, sender_type, message_type, content, is_internal, metadata, created_at, updated_at)
		VALUES (?, ?, ?, 'user', 'reply', 'Mentioning teammate', 1, ?, ?, ?)`,
		"msg-assignment-filter-mention", workspaceID, "mentioned-me", `{"mentioned_user_ids":["`+userID+`"]}`, mentionAt, mentionAt,
	).Error; err != nil {
		t.Fatalf("insert mention message: %v", err)
	}

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
	assertContainsExactly(t, listIDs("unassigned", ""), []string{"unassigned", "opened-by-me", "mentioned-me"})
	assertContainsExactly(t, listIDs("others", ""), []string{"assigned-other"})
	assertContainsExactly(t, listIDs("opened_by_me", ""), []string{"opened-by-me"})
	assertContainsExactly(t, listIDs("mentioned_me", ""), []string{"mentioned-me"})
	assertContainsExactly(t, listIDs("me,mentioned_me,opened_by_me", ""), []string{"assigned-me", "mentioned-me", "opened-by-me"})
	assertContainsExactly(t, listIDs("me,unassigned", ""), []string{"assigned-me", "unassigned", "opened-by-me", "mentioned-me"})
	assertContainsExactly(t, listIDs("unknown", ""), []string{})
	assertContainsExactly(t, listIDs("none", ""), []string{})

	for _, tt := range []struct {
		sort string
		want []string
	}{
		{
			sort: "oldest",
			want: []string{"assigned-me", "assigned-other", "unassigned", "agent-owned", "opened-by-me", "mentioned-me"},
		},
		{
			sort: "newest",
			want: []string{"mentioned-me", "opened-by-me", "agent-owned", "unassigned", "assigned-other", "assigned-me"},
		},
	} {
		t.Run(tt.sort, func(t *testing.T) {
			if got := listIDs("", tt.sort); !slices.Equal(got, tt.want) {
				t.Fatalf("got ids %v, want %v", got, tt.want)
			}
		})
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

func TestSupportConversationRepositoryListMailboxIDsFilter(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-support-mailbox-filters"
	userID := "user-support-mailbox-filters"
	seedWorkspace(t, db, workspaceID, "Support Mailbox Filters", "support-mailbox-filters", userID)
	repo := repository.NewSupportConversationRepository(db)

	for _, mailbox := range []model.SupportMailbox{
		{ID: "mailbox-billing-filter", WorkspaceID: workspaceID, Name: "Billing", Handle: "billing", Icon: "inbox", Active: true, CreatedByID: userID},
		{ID: "mailbox-sales-filter", WorkspaceID: workspaceID, Name: "Sales", Handle: "sales", Icon: "inbox", Active: true, CreatedByID: userID},
	} {
		if err := db.Create(&mailbox).Error; err != nil {
			t.Fatalf("create mailbox %s: %v", mailbox.ID, err)
		}
	}

	for _, item := range []struct {
		id        string
		mailboxID *string
	}{
		{id: "shared", mailboxID: nil},
		{id: "billing", mailboxID: stringPtr("mailbox-billing-filter")},
		{id: "sales", mailboxID: stringPtr("mailbox-sales-filter")},
	} {
		conversation := &model.SupportConversation{
			ID:          item.id,
			WorkspaceID: workspaceID,
			Subject:     item.id,
			Status:      model.SupportConversationStatusOpen,
			Priority:    "medium",
			Channel:     "widget",
			Source:      "widget",
			MailboxID:   item.mailboxID,
		}
		if err := repo.Create(ctx, conversation); err != nil {
			t.Fatalf("create %s: %v", item.id, err)
		}
	}

	conversations, _, err := repo.List(ctx, repository.ConversationRepositoryListParams{
		ConversationListParams: repository.ConversationListParams{
			WorkspaceID: workspaceID,
			UserID:      userID,
			MailboxIDs:  []string{"mailbox-billing-filter", "mailbox-sales-filter"},
			Pagination:  model.PMPagination{Page: 1, PerPage: 50},
		},
		Role: model.RoleOwner,
	})
	if err != nil {
		t.Fatalf("list mailbox ids: %v", err)
	}
	ids := make([]string, 0, len(conversations))
	for _, conversation := range conversations {
		ids = append(ids, conversation.ID)
	}
	assertContainsExactly(t, ids, []string{"billing", "sales"})
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
