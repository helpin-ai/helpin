package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func supportConversationListParams(workspaceID, status, priority string, pagination model.PMPagination, workspaceMemberID, role string, mailboxID *string, flowState, search string, aiState ...string) repository.ConversationRepositoryListParams {
	return repository.ConversationRepositoryListParams{
		ConversationListParams: repository.ConversationListParams{
			WorkspaceID: workspaceID,
			Status:      status,
			Priority:    priority,
			Pagination:  pagination,
			MailboxID:   mailboxID,
			FlowState:   flowState,
			Search:      search,
			AIState:     aiState,
		},
		WorkspaceMemberID: workspaceMemberID,
		Role:              role,
	}
}

// ---------------------------------------------------------------------------
// Repository-level tests
// ---------------------------------------------------------------------------

func TestSupportConversationRepository(t *testing.T) {
	db := newTestDB(t)

	workspaceID := "ws-test-123"
	seedWorkspace(t, db, workspaceID, "Test Workspace", "test-ws", "user-123")

	repo := repository.NewSupportConversationRepository(db)

	t.Run("Create and Get conversation", func(t *testing.T) {
		ctx := context.Background()

		conversation := &model.SupportConversation{
			WorkspaceID:   workspaceID,
			Subject:       "Test conversation",
			Status:        "open",
			Priority:      "medium",
			Channel:       "widget",
			CustomerEmail: strPtr("test@example.com"),
			CustomerName:  strPtr("Test User"),
		}

		err := repo.Create(ctx, conversation)
		if err != nil {
			t.Fatalf("create conversation: %v", err)
		}

		if conversation.ID == "" {
			t.Error("expected conversation ID to be set")
		}

		if conversation.DisplayID == 0 {
			t.Error("expected display_id to be auto-allocated")
		}

		// Get the conversation
		fetched, err := repo.GetByID(ctx, workspaceID, conversation.ID, "", model.RoleOwner)
		if err != nil {
			t.Fatalf("get conversation: %v", err)
		}
		if fetched == nil {
			t.Fatal("expected conversation to be found")
		}
		if fetched.Subject != "Test conversation" {
			t.Errorf("expected subject 'Test conversation', got %q", fetched.Subject)
		}
	})

	t.Run("List conversations", func(t *testing.T) {
		ctx := context.Background()

		// Create another conversation
		conversation2 := &model.SupportConversation{
			WorkspaceID: workspaceID,
			Subject:     "Second conversation",
			Status:      model.SupportConversationStatusResolved,
			Priority:    "high",
		}
		err := repo.Create(ctx, conversation2)
		if err != nil {
			t.Fatalf("create second conversation: %v", err)
		}

		// List all
		conversations, total, err := repo.List(ctx, supportConversationListParams(workspaceID, "", "", model.PMPagination{}, "", model.RoleOwner, nil, "", ""))
		if err != nil {
			t.Fatalf("list conversations: %v", err)
		}
		if total != 2 {
			t.Errorf("expected 2 conversations, got %d", total)
		}
		if len(conversations) != 2 {
			t.Errorf("expected 2 conversations in slice, got %d", len(conversations))
		}

		// Filter by status
		openConvs, totalOpen, err := repo.List(ctx, supportConversationListParams(workspaceID, "open", "", model.PMPagination{}, "", model.RoleOwner, nil, "", ""))
		if err != nil {
			t.Fatalf("list open conversations: %v", err)
		}
		if totalOpen != 1 {
			t.Errorf("expected 1 open conversation, got %d", totalOpen)
		}
		if openConvs[0].Status != "open" {
			t.Errorf("expected open conversation, got %q", openConvs[0].Status)
		}

		// Filter by priority
		_, totalHigh, err := repo.List(ctx, supportConversationListParams(workspaceID, "", "high", model.PMPagination{}, "", model.RoleOwner, nil, "", ""))
		if err != nil {
			t.Fatalf("list high priority: %v", err)
		}
		if totalHigh != 1 {
			t.Errorf("expected 1 high priority, got %d", totalHigh)
		}
	})

	t.Run("List conversations paginates older results and search ignores recency", func(t *testing.T) {
		ctx := context.Background()
		searchWorkspaceID := "ws-test-support-search"
		seedWorkspace(t, db, searchWorkspaceID, "Search Workspace", "support-search", "user-123")

		oldConversation := &model.SupportConversation{
			WorkspaceID:   searchWorkspaceID,
			Subject:       "Legacy email thread",
			Status:        model.SupportConversationStatusOpen,
			Priority:      "medium",
			Channel:       "email",
			CustomerEmail: strPtr("matta.trisha@gmail.com"),
		}
		if err := repo.Create(ctx, oldConversation); err != nil {
			t.Fatalf("create old conversation: %v", err)
		}
		oldUpdatedAt := time.Now().UTC().AddDate(0, 0, -10)
		if err := db.Model(&model.SupportConversation{}).
			Where("id = ?", oldConversation.ID).
			Update("updated_at", oldUpdatedAt).Error; err != nil {
			t.Fatalf("age old conversation: %v", err)
		}

		for i := 0; i < 55; i++ {
			conversation := &model.SupportConversation{
				WorkspaceID: searchWorkspaceID,
				Subject:     fmt.Sprintf("Recent conversation %d", i),
				Status:      model.SupportConversationStatusOpen,
				Priority:    "medium",
			}
			if err := repo.Create(ctx, conversation); err != nil {
				t.Fatalf("create recent conversation %d: %v", i, err)
			}
			recentUpdatedAt := time.Now().UTC().Add(-time.Duration(i) * time.Minute)
			if err := db.Model(&model.SupportConversation{}).
				Where("id = ?", conversation.ID).
				Update("updated_at", recentUpdatedAt).Error; err != nil {
				t.Fatalf("age recent conversation %d: %v", i, err)
			}
		}

		pageTwo, total, err := repo.List(ctx, supportConversationListParams(searchWorkspaceID, "", "", model.PMPagination{Page: 2, PerPage: 50}, "", model.RoleOwner, nil, "", ""))
		if err != nil {
			t.Fatalf("list second page: %v", err)
		}
		if total != 56 {
			t.Fatalf("expected 56 conversations, got %d", total)
		}
		if len(pageTwo) == 0 {
			t.Fatal("expected older conversations on the second page")
		}
		foundOnPageTwo := false
		for _, conversation := range pageTwo {
			if conversation.ID == oldConversation.ID {
				foundOnPageTwo = true
				break
			}
		}
		if !foundOnPageTwo {
			t.Fatal("expected second page to include the older conversation")
		}

		searchResults, searchTotal, err := repo.List(ctx, supportConversationListParams(searchWorkspaceID, "", "", model.PMPagination{Page: 1, PerPage: 50}, "", model.RoleOwner, nil, "", "matta.trisha@gmail.com"))
		if err != nil {
			t.Fatalf("search conversations: %v", err)
		}
		if searchTotal != 1 || len(searchResults) != 1 {
			t.Fatalf("expected one search result, got total=%d len=%d", searchTotal, len(searchResults))
		}
		if searchResults[0].ID != oldConversation.ID {
			t.Fatalf("expected search result %q, got %q", oldConversation.ID, searchResults[0].ID)
		}
	})

	t.Run("List conversations includes latest visitor country", func(t *testing.T) {
		ctx := context.Background()

		conversation := &model.SupportConversation{
			WorkspaceID: workspaceID,
			Subject:     "Country conversation",
			Status:      model.SupportConversationStatusOpen,
			AnonymousID: strPtr("anon-country"),
		}
		if err := repo.Create(ctx, conversation); err != nil {
			t.Fatalf("create conversation: %v", err)
		}

		sessionRepo := repository.NewSupportInboxSessionRepository(db)
		oldSession := &model.SupportWidgetSession{
			WorkspaceID:    workspaceID,
			ConversationID: &conversation.ID,
			SessionToken:   "country-old",
			AnonymousID:    "anon-country",
			IsAnonymous:    true,
			CountryCode:    strPtr("CA"),
			CountryName:    strPtr("Canada"),
			ExpiresAt:      time.Now().Add(24 * time.Hour),
			CreatedAt:      time.Now().Add(-2 * time.Hour),
		}
		if err := sessionRepo.Create(ctx, oldSession); err != nil {
			t.Fatalf("create old session: %v", err)
		}

		newSession := &model.SupportWidgetSession{
			WorkspaceID:    workspaceID,
			ConversationID: &conversation.ID,
			SessionToken:   "country-new",
			AnonymousID:    "anon-country",
			IsAnonymous:    true,
			CountryCode:    strPtr("DE"),
			CountryName:    strPtr("Germany"),
			ExpiresAt:      time.Now().Add(24 * time.Hour),
			CreatedAt:      time.Now().Add(-1 * time.Hour),
		}
		if err := sessionRepo.Create(ctx, newSession); err != nil {
			t.Fatalf("create new session: %v", err)
		}

		conversations, total, err := repo.List(ctx, supportConversationListParams(workspaceID, "", "", model.PMPagination{}, "", model.RoleOwner, nil, "", ""))
		if err != nil {
			t.Fatalf("list conversations: %v", err)
		}
		if total < 1 {
			t.Fatalf("expected conversations, got %d", total)
		}

		var found *model.SupportConversation
		for i := range conversations {
			if conversations[i].ID == conversation.ID {
				found = &conversations[i]
				break
			}
		}
		if found == nil {
			t.Fatal("expected seeded conversation in list")
		}
		if found.CountryCode == nil || *found.CountryCode != "DE" {
			t.Fatalf("country_code = %v, want %q", found.CountryCode, "DE")
		}
		if found.CountryName == nil || *found.CountryName != "Germany" {
			t.Fatalf("country_name = %v, want %q", found.CountryName, "Germany")
		}
	})

	t.Run("Update conversation", func(t *testing.T) {
		ctx := context.Background()

		conversation := &model.SupportConversation{
			WorkspaceID: workspaceID,
			Subject:     "Original subject",
			Status:      "open",
		}
		err := repo.Create(ctx, conversation)
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		// Update
		conversation.Status = "resolved"
		err = repo.Update(ctx, conversation)
		if err != nil {
			t.Fatalf("update: %v", err)
		}

		// Verify
		fetched, err := repo.GetByID(ctx, workspaceID, conversation.ID, "", model.RoleOwner)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if fetched.Status != "resolved" {
			t.Errorf("expected status 'resolved', got %q", fetched.Status)
		}
	})

	t.Run("GetByID not found returns nil", func(t *testing.T) {
		ctx := context.Background()

		fetched, err := repo.GetByID(ctx, workspaceID, "non-existent-id", "", model.RoleOwner)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if fetched != nil {
			t.Error("expected nil for non-existent conversation")
		}
	})

	t.Run("Display ID auto-increments sequentially", func(t *testing.T) {
		ctx := context.Background()

		c1 := &model.SupportConversation{WorkspaceID: workspaceID, Subject: "Seq 1", Status: "open"}
		c2 := &model.SupportConversation{WorkspaceID: workspaceID, Subject: "Seq 2", Status: "open"}

		if err := repo.Create(ctx, c1); err != nil {
			t.Fatalf("create c1: %v", err)
		}
		if err := repo.Create(ctx, c2); err != nil {
			t.Fatalf("create c2: %v", err)
		}

		if c2.DisplayID != c1.DisplayID+1 {
			t.Errorf("expected sequential display IDs: c1=%d, c2=%d", c1.DisplayID, c2.DisplayID)
		}
	})

	t.Run("Cross-workspace isolation", func(t *testing.T) {
		ctx := context.Background()

		otherWS := "ws-other-999"
		seedWorkspace(t, db, otherWS, "Other Workspace", "other-ws", "user-456")

		otherRepo := repository.NewSupportConversationRepository(db)
		otherConv := &model.SupportConversation{
			WorkspaceID: otherWS,
			Subject:     "Other workspace conversation",
			Status:      "open",
		}
		if err := otherRepo.Create(ctx, otherConv); err != nil {
			t.Fatalf("create in other ws: %v", err)
		}

		// Should not be visible from original workspace
		fetched, err := repo.GetByID(ctx, workspaceID, otherConv.ID, "", model.RoleOwner)
		if err != nil {
			t.Fatalf("get cross-workspace: %v", err)
		}
		if fetched != nil {
			t.Error("conversation from other workspace should not be visible")
		}
	})

	t.Run("Pagination", func(t *testing.T) {
		ctx := context.Background()

		paginationWS := "ws-pagination-test"
		seedWorkspace(t, db, paginationWS, "Pagination WS", "pagination-ws", "user-123")
		pRepo := repository.NewSupportConversationRepository(db)

		// Create 5 conversations
		for i := 0; i < 5; i++ {
			c := &model.SupportConversation{
				WorkspaceID: paginationWS,
				Subject:     "Pagination test",
				Status:      "open",
			}
			if err := pRepo.Create(ctx, c); err != nil {
				t.Fatalf("create %d: %v", i, err)
			}
		}

		// Page 1, 2 per page
		page1, total, err := pRepo.List(ctx, supportConversationListParams(paginationWS, "", "", model.PMPagination{Page: 1, PerPage: 2}, "", model.RoleOwner, nil, "", ""))
		if err != nil {
			t.Fatalf("page 1: %v", err)
		}
		if total != 5 {
			t.Errorf("expected total 5, got %d", total)
		}
		if len(page1) != 2 {
			t.Errorf("expected 2 results on page 1, got %d", len(page1))
		}

		// Page 3, 2 per page → 1 result
		page3, _, err := pRepo.List(ctx, supportConversationListParams(paginationWS, "", "", model.PMPagination{Page: 3, PerPage: 2}, "", model.RoleOwner, nil, "", ""))
		if err != nil {
			t.Fatalf("page 3: %v", err)
		}
		if len(page3) != 1 {
			t.Errorf("expected 1 result on page 3, got %d", len(page3))
		}
	})

	t.Run("Linked team members can access mailbox conversations", func(t *testing.T) {
		ctx := context.Background()

		linkedWorkspaceID := "ws-linked-team-access"
		ownerUserID := "user-linked-owner"
		teamUserID := "user-linked-team"
		outsiderUserID := "user-linked-outsider"
		teamID := "team-linked-support"

		seedUser(t, db, ownerUserID, "linked-owner@example.com", "Linked Owner", "hash")
		seedUser(t, db, teamUserID, "linked-team@example.com", "Linked Team", "hash")
		seedUser(t, db, outsiderUserID, "linked-outsider@example.com", "Linked Outsider", "hash")
		seedWorkspace(t, db, linkedWorkspaceID, "Linked Team Access", "linked-team-access", ownerUserID)
		seedWorkspaceMember(t, db, "wm-linked-owner", linkedWorkspaceID, ownerUserID, "linked-owner@example.com", "Linked Owner", model.RoleAdmin)
		seedWorkspaceMember(t, db, "wm-linked-team", linkedWorkspaceID, teamUserID, "linked-team@example.com", "Linked Team", model.RoleMember)
		seedWorkspaceMember(t, db, "wm-linked-outsider", linkedWorkspaceID, outsiderUserID, "linked-outsider@example.com", "Linked Outsider", model.RoleMember)

		now := time.Now()
		mustExec(t, db, `INSERT INTO workspace_teams (id, workspace_id, name, handle, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
			teamID, linkedWorkspaceID, "Linked Support", "linked-support", now, now)
		mustExec(t, db, `INSERT INTO team_workspace_memberships (id, team_id, workspace_member_id, role, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
			"twm-linked-team", teamID, "wm-linked-team", "member", now, now)

		mailboxRepo := repository.NewSupportMailboxRepository(db)
		mailbox := &model.SupportMailbox{
			WorkspaceID:    linkedWorkspaceID,
			Name:           "Billing",
			Handle:         "billing",
			Icon:           "inbox",
			LinkedTeamID:   strPtr(teamID),
			VisibilityMode: "members_only",
			AssignmentMode: "round_robin",
			Active:         true,
			CreatedByID:    ownerUserID,
		}
		if err := mailboxRepo.Create(ctx, mailbox); err != nil {
			t.Fatalf("create mailbox: %v", err)
		}

		privateConversation := &model.SupportConversation{
			WorkspaceID: linkedWorkspaceID,
			MailboxID:   &mailbox.ID,
			Subject:     "Linked team private conversation",
			Status:      "open",
		}
		if err := repo.Create(ctx, privateConversation); err != nil {
			t.Fatalf("create private conversation: %v", err)
		}

		accessibleMailboxes, err := mailboxRepo.ListAccessible(ctx, linkedWorkspaceID, "wm-linked-team", model.RoleMember, false)
		if err != nil {
			t.Fatalf("list accessible mailboxes: %v", err)
		}
		if len(accessibleMailboxes) != 1 || accessibleMailboxes[0].ID != mailbox.ID {
			t.Fatalf("expected linked team member to see mailbox %q, got %#v", mailbox.ID, accessibleMailboxes)
		}

		isMember, err := mailboxRepo.IsMember(ctx, mailbox.ID, "wm-linked-team")
		if err != nil {
			t.Fatalf("is member: %v", err)
		}
		if !isMember {
			t.Fatal("expected linked team member to have mailbox access")
		}

		teamConversations, total, err := repo.List(ctx, supportConversationListParams(linkedWorkspaceID, "", "", model.PMPagination{}, "wm-linked-team", model.RoleMember, nil, "", ""))
		if err != nil {
			t.Fatalf("list conversations for linked team member: %v", err)
		}
		if total != 1 || len(teamConversations) != 1 || teamConversations[0].ID != privateConversation.ID {
			t.Fatalf("expected linked team member to see private conversation, got total=%d conversations=%#v", total, teamConversations)
		}

		outsiderConversations, outsiderTotal, err := repo.List(ctx, supportConversationListParams(linkedWorkspaceID, "", "", model.PMPagination{}, "wm-linked-outsider", model.RoleMember, nil, "", ""))
		if err != nil {
			t.Fatalf("list conversations for outsider: %v", err)
		}
		if outsiderTotal != 0 || len(outsiderConversations) != 0 {
			t.Fatalf("expected outsider to see no private conversations, got total=%d conversations=%#v", outsiderTotal, outsiderConversations)
		}

		memberUserIDs, err := mailboxRepo.ListActiveMemberUserIDs(ctx, linkedWorkspaceID, mailbox.ID)
		if err != nil {
			t.Fatalf("list active mailbox member user ids: %v", err)
		}
		if len(memberUserIDs) != 1 || memberUserIDs[0] != teamUserID {
			t.Fatalf("expected linked team user id %q, got %#v", teamUserID, memberUserIDs)
		}

		ownerID, err := mailboxRepo.SelectRoundRobinOwnerUserID(ctx, linkedWorkspaceID, mailbox.ID)
		if err != nil {
			t.Fatalf("select round robin owner: %v", err)
		}
		if ownerID == nil || *ownerID != teamUserID {
			t.Fatalf("expected round robin owner %q, got %#v", teamUserID, ownerID)
		}
	})

	t.Run("GetUnreadStats excludes AI-managed conversations from human inbox buckets", func(t *testing.T) {
		ctx := context.Background()
		now := time.Now()
		customerMessageAt := now.Add(-time.Minute)

		baseStats, err := repo.GetUnreadStats(ctx, workspaceID, "user-123", "", model.RoleOwner, nil)
		if err != nil {
			t.Fatalf("get baseline unread stats: %v", err)
		}

		humanConv := &model.SupportConversation{
			WorkspaceID:     workspaceID,
			Subject:         "Human owned conversation",
			Status:          "open",
			OpenedByUserID:  strPtr("user-123"),
			AssignedUserID:  strPtr("user-123"),
			TeamLastSeenAt:  &now,
			AssignedAgentID: nil,
		}
		if err := repo.Create(ctx, humanConv); err != nil {
			t.Fatalf("create human conversation: %v", err)
		}
		if err := db.Exec(
			`UPDATE support_conversations SET team_last_seen_at = ?, updated_at = ? WHERE id = ?`,
			now.Add(-2*time.Minute), now, humanConv.ID,
		).Error; err != nil {
			t.Fatalf("seed human read cursor: %v", err)
		}
		if err := db.Exec(
			`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content, message_type, is_internal, created_at, updated_at) VALUES (?, ?, ?, 'customer', ?, 'reply', 0, ?, ?)`,
			"msg-human-unread", workspaceID, humanConv.ID, "Need help from a person", customerMessageAt, customerMessageAt,
		).Error; err != nil {
			t.Fatalf("seed human unread message: %v", err)
		}
		mustExec(t, db, `INSERT INTO support_conversation_user_states (workspace_id, conversation_id, user_id, unread_customer_message_count, relevance_mask, version, created_at, updated_at) VALUES (?, ?, ?, 1, 3, 1, ?, ?)`, workspaceID, humanConv.ID, "user-123", now, now)

		aiPending := "pending"
		aiConv := &model.SupportConversation{
			WorkspaceID:    workspaceID,
			Subject:        "AI owned conversation",
			Status:         "open",
			OpenedByUserID: strPtr("user-123"),
			AIState:        &aiPending,
			FlowState:      strPtr(model.SupportConversationFlowStateAIHandling),
			TeamLastSeenAt: &now,
		}
		if err := repo.Create(ctx, aiConv); err != nil {
			t.Fatalf("create AI conversation: %v", err)
		}
		if err := db.Exec(
			`UPDATE support_conversations SET team_last_seen_at = ?, updated_at = ?, ai_state = ?, flow_state = ? WHERE id = ?`,
			now.Add(-2*time.Minute), now, aiPending, model.SupportConversationFlowStateAIHandling, aiConv.ID,
		).Error; err != nil {
			t.Fatalf("seed AI state: %v", err)
		}
		if err := db.Exec(
			`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content, message_type, is_internal, created_at, updated_at) VALUES (?, ?, ?, 'customer', ?, 'reply', 0, ?, ?)`,
			"msg-ai-unread", workspaceID, aiConv.ID, "AI can take this", customerMessageAt, customerMessageAt,
		).Error; err != nil {
			t.Fatalf("seed AI unread message: %v", err)
		}
		mustExec(t, db, `INSERT INTO support_conversation_user_states (workspace_id, conversation_id, user_id, unread_customer_message_count, relevance_mask, version, created_at, updated_at) VALUES (?, ?, ?, 1, 2, 1, ?, ?)`, workspaceID, aiConv.ID, "user-123", now, now)

		aiEscalated := "escalated"
		escalatedConv := &model.SupportConversation{
			WorkspaceID:    workspaceID,
			Subject:        "Escalated AI conversation",
			Status:         "open",
			AIState:        &aiEscalated,
			FlowState:      strPtr(model.SupportConversationFlowStateWaitingForHuman),
			TeamLastSeenAt: &now,
		}
		if err := repo.Create(ctx, escalatedConv); err != nil {
			t.Fatalf("create escalated conversation: %v", err)
		}
		if err := db.Exec(
			`UPDATE support_conversations SET team_last_seen_at = ?, updated_at = ?, ai_state = ?, flow_state = ?, assigned_agent_id = NULL, assigned_user_id = NULL, opened_by_user_id = NULL WHERE id = ?`,
			now.Add(-2*time.Minute), now, aiEscalated, model.SupportConversationFlowStateWaitingForHuman, escalatedConv.ID,
		).Error; err != nil {
			t.Fatalf("seed escalated state: %v", err)
		}
		if err := db.Exec(
			`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content, message_type, is_internal, created_at, updated_at) VALUES (?, ?, ?, 'customer', ?, 'reply', 0, ?, ?)`,
			"msg-escalated-unread", workspaceID, escalatedConv.ID, "Need a human now", customerMessageAt, customerMessageAt,
		).Error; err != nil {
			t.Fatalf("seed escalated unread message: %v", err)
		}

		waitingConv := &model.SupportConversation{
			WorkspaceID:    workspaceID,
			Subject:        "Waiting conversation",
			Status:         model.SupportConversationStatusWaitingOnCustomer,
			TeamLastSeenAt: &now,
		}
		if err := repo.Create(ctx, waitingConv); err != nil {
			t.Fatalf("create waiting conversation: %v", err)
		}
		if err := db.Exec(
			`UPDATE support_conversations SET team_last_seen_at = ?, updated_at = ? WHERE id = ?`,
			now.Add(-2*time.Minute), now, waitingConv.ID,
		).Error; err != nil {
			t.Fatalf("seed waiting read cursor: %v", err)
		}
		if err := db.Exec(
			`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content, message_type, is_internal, created_at, updated_at) VALUES (?, ?, ?, 'customer', ?, 'reply', 0, ?, ?)`,
			"msg-waiting-unread", workspaceID, waitingConv.ID, "I have replied", customerMessageAt, customerMessageAt,
		).Error; err != nil {
			t.Fatalf("seed waiting unread message: %v", err)
		}

		stats, err := repo.GetUnreadStats(ctx, workspaceID, "user-123", "", model.RoleOwner, nil)
		if err != nil {
			t.Fatalf("get unread stats: %v", err)
		}

		if got := stats.Inbox - baseStats.Inbox; got != 1 {
			t.Fatalf("expected inbox unread count delta 1, got %d", got)
		}
		if got := stats.Mine - baseStats.Mine; got != 1 {
			t.Fatalf("expected mine unread count delta 1, got %d", got)
		}
		if got := stats.Waiting - baseStats.Waiting; got != 0 {
			t.Fatalf("expected waiting unread count delta 0, got %d", got)
		}
		if got := stats.Total - baseStats.Total; got != 2 {
			t.Fatalf("expected total unread support count delta 2, got %d", got)
		}
		if got := stats.MyInbox - baseStats.MyInbox; got != 1 {
			t.Fatalf("expected my inbox count delta 1, got %d", got)
		}
		if got := stats.Unassigned - baseStats.Unassigned; got != 0 {
			t.Fatalf("expected unassigned count delta 0, got %d", got)
		}
		if got := stats.AIActive - baseStats.AIActive; got != 1 {
			t.Fatalf("expected AI active unread count delta 1, got %d", got)
		}
		if got := stats.InboxTotal - baseStats.InboxTotal; got != 3 {
			t.Fatalf("expected inbox workload count delta 3, got %d", got)
		}
		if got := stats.MineTotal - baseStats.MineTotal; got != 1 {
			t.Fatalf("expected mine workload count delta 1, got %d", got)
		}
		if got := stats.WaitingTotal - baseStats.WaitingTotal; got != 1 {
			t.Fatalf("expected waiting workload count delta 1, got %d", got)
		}
		if got := stats.AIActiveTotal - baseStats.AIActiveTotal; got != 1 {
			t.Fatalf("expected AI active workload count delta 1, got %d", got)
		}
	})

	t.Run("Mailbox workload counts match human inbox list scope", func(t *testing.T) {
		ctx := context.Background()
		now := time.Now()
		customerMessageAt := now.Add(-time.Minute)
		mailboxRepo := repository.NewSupportMailboxRepository(db)

		billingMailbox := &model.SupportMailbox{
			WorkspaceID:    workspaceID,
			Name:           "Billing",
			Handle:         "billing",
			Icon:           "inbox",
			TriageEligible: true,
			VisibilityMode: "members_only",
			AssignmentMode: "manual",
			Active:         true,
			CreatedByID:    "user-123",
		}
		if err := mailboxRepo.Create(ctx, billingMailbox); err != nil {
			t.Fatalf("create billing mailbox: %v", err)
		}

		humanConv := &model.SupportConversation{
			WorkspaceID: workspaceID,
			MailboxID:   &billingMailbox.ID,
			Subject:     "Billing refund",
			Status:      "open",
		}
		if err := repo.Create(ctx, humanConv); err != nil {
			t.Fatalf("create human billing conversation: %v", err)
		}
		if err := db.Exec(
			`UPDATE support_conversations SET team_last_seen_at = ?, updated_at = ? WHERE id = ?`,
			now.Add(-2*time.Minute), now, humanConv.ID,
		).Error; err != nil {
			t.Fatalf("seed human billing read cursor: %v", err)
		}
		if err := db.Exec(
			`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content, message_type, is_internal, created_at, updated_at) VALUES (?, ?, ?, 'customer', ?, 'reply', 0, ?, ?)`,
			"msg-billing-human-unread", workspaceID, humanConv.ID, "Need a refund", customerMessageAt, customerMessageAt,
		).Error; err != nil {
			t.Fatalf("seed human billing unread message: %v", err)
		}

		aiPending := "pending"
		aiPendingConv := &model.SupportConversation{
			WorkspaceID: workspaceID,
			MailboxID:   &billingMailbox.ID,
			Subject:     "AI pending billing conversation",
			Status:      "open",
			AIState:     &aiPending,
			FlowState:   strPtr(model.SupportConversationFlowStateAIHandling),
		}
		if err := repo.Create(ctx, aiPendingConv); err != nil {
			t.Fatalf("create AI pending billing conversation: %v", err)
		}
		if err := db.Exec(
			`UPDATE support_conversations SET team_last_seen_at = ?, updated_at = ?, ai_state = ?, flow_state = ? WHERE id = ?`,
			now.Add(-2*time.Minute), now, aiPending, model.SupportConversationFlowStateAIHandling, aiPendingConv.ID,
		).Error; err != nil {
			t.Fatalf("seed AI pending billing state: %v", err)
		}
		if err := db.Exec(
			`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content, message_type, is_internal, created_at, updated_at) VALUES (?, ?, ?, 'customer', ?, 'reply', 0, ?, ?)`,
			"msg-billing-ai-pending-unread", workspaceID, aiPendingConv.ID, "Can AI handle this billing question?", customerMessageAt, customerMessageAt,
		).Error; err != nil {
			t.Fatalf("seed AI pending billing unread message: %v", err)
		}

		aiEscalated := "escalated"
		aiEscalatedConv := &model.SupportConversation{
			WorkspaceID: workspaceID,
			MailboxID:   &billingMailbox.ID,
			Subject:     "Escalated billing conversation",
			Status:      "open",
			AIState:     &aiEscalated,
			FlowState:   strPtr(model.SupportConversationFlowStateWaitingForHuman),
		}
		if err := repo.Create(ctx, aiEscalatedConv); err != nil {
			t.Fatalf("create escalated billing conversation: %v", err)
		}
		if err := db.Exec(
			`UPDATE support_conversations SET team_last_seen_at = ?, updated_at = ?, ai_state = ?, flow_state = ? WHERE id = ?`,
			now.Add(-2*time.Minute), now, aiEscalated, model.SupportConversationFlowStateWaitingForHuman, aiEscalatedConv.ID,
		).Error; err != nil {
			t.Fatalf("seed escalated billing state: %v", err)
		}
		if err := db.Exec(
			`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content, message_type, is_internal, created_at, updated_at) VALUES (?, ?, ?, 'customer', ?, 'reply', 0, ?, ?)`,
			"msg-billing-ai-escalated-unread", workspaceID, aiEscalatedConv.ID, "Need a human for billing", customerMessageAt, customerMessageAt,
		).Error; err != nil {
			t.Fatalf("seed escalated billing unread message: %v", err)
		}

		aiResolved := "resolved"
		aiResolvedConv := &model.SupportConversation{
			WorkspaceID: workspaceID,
			MailboxID:   &billingMailbox.ID,
			Subject:     "AI resolved billing conversation",
			Status:      "resolved",
			AIState:     &aiResolved,
			FlowState:   strPtr(model.SupportConversationFlowStateResolvedByAI),
		}
		if err := repo.Create(ctx, aiResolvedConv); err != nil {
			t.Fatalf("create AI resolved billing conversation: %v", err)
		}
		if err := db.Exec(
			`UPDATE support_conversations SET team_last_seen_at = ?, updated_at = ?, ai_state = ?, flow_state = ? WHERE id = ?`,
			now.Add(-2*time.Minute), now, aiResolved, model.SupportConversationFlowStateResolvedByAI, aiResolvedConv.ID,
		).Error; err != nil {
			t.Fatalf("seed AI resolved billing state: %v", err)
		}
		if err := db.Exec(
			`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content, message_type, is_internal, created_at, updated_at) VALUES (?, ?, ?, 'customer', ?, 'reply', 0, ?, ?)`,
			"msg-billing-ai-resolved-unread", workspaceID, aiResolvedConv.ID, "Thanks, that fixed billing", customerMessageAt, customerMessageAt,
		).Error; err != nil {
			t.Fatalf("seed AI resolved billing unread message: %v", err)
		}

		count, err := mailboxRepo.CountUnread(ctx, workspaceID, &billingMailbox.ID)
		if err != nil {
			t.Fatalf("count billing mailbox unread: %v", err)
		}
		if count != 2 {
			t.Fatalf("expected billing mailbox unread count 2, got %d", count)
		}

		workloadCount, err := mailboxRepo.CountWorkload(ctx, workspaceID, &billingMailbox.ID)
		if err != nil {
			t.Fatalf("count billing mailbox workload: %v", err)
		}
		if workloadCount != 2 {
			t.Fatalf("expected billing mailbox workload count 2, got %d", workloadCount)
		}
	})
}

func TestSupportConversationRepositoryGetByIDIncludesUnreadCount(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-detail-unread"
	seedWorkspace(t, db, workspaceID, "Detail Unread", "detail-unread", "user-detail-unread")
	repo := repository.NewSupportConversationRepository(db)
	conversation := &model.SupportConversation{WorkspaceID: workspaceID, Subject: "Unread detail", Status: model.SupportConversationStatusOpen}
	if err := repo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	message := &model.SupportMessage{WorkspaceID: workspaceID, ConversationID: conversation.ID, SenderType: "customer", MessageType: "reply", Content: "Unread customer reply"}
	if err := db.Create(message).Error; err != nil {
		t.Fatalf("create customer message: %v", err)
	}

	fetched, err := repo.GetByID(ctx, workspaceID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if fetched.UnreadCount != 1 {
		t.Fatalf("UnreadCount = %d, want 1", fetched.UnreadCount)
	}
}

func TestSupportInboxServiceCreateConversationWithMessageAssignsCreatorAndStoresEmailRecipients(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	workspaceID := "ws-new-conversation"
	actorID := "user-new-conversation"
	seedUser(t, db, actorID, "agent@example.com", "Agent User", "hash")
	seedWorkspace(t, db, workspaceID, "New Conversation Workspace", "new-conversation", actorID)
	seedWorkspaceMember(t, db, "wm-new-conversation", workspaceID, actorID, "agent@example.com", "Agent User", model.RoleAdmin)

	now := time.Now()
	mustExec(t, db, `INSERT INTO support_tags (id, workspace_id, name, color, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"tag-new-conversation", workspaceID, "VIP", "#2563eb", now, now)

	conversationRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	svc := NewSupportInboxService(
		conversationRepo,
		repository.NewSupportMailboxRepository(db),
		messageRepo,
		repository.NewAgentRepository(db),
		repository.NewCRMAssociationRepository(db),
		repository.NewSupportInboxInstallationRepository(db),
		repository.NewSupportInboxSessionRepository(db),
		repository.NewSupportCannedResponseRepository(db),
		nil,
		nil,
		repository.NewCRMContactRepository(db),
		repository.NewUserRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db, false),
		repository.NewDocsHelpcenterRepository(db, false),
	).SetSupportTagRepo(repository.NewSupportTagRepository(db))

	result, err := svc.CreateConversationWithMessage(ctx, model.CreateConversationWithMessageRequest{
		WorkspaceID:   workspaceID,
		Subject:       "Renewal question",
		CustomerName:  strPtr("Jane Customer"),
		CustomerEmail: strPtr("jane@example.com"),
		Channels:      []string{"chat", "email"},
		Content:       "Hi Jane, following up here.",
		CCEmails:      []string{"finance@example.com"},
		BCCEmails:     []string{"audit@example.com"},
		TagIDs:        []string{"tag-new-conversation"},
	}, actorID)
	if err != nil {
		t.Fatalf("CreateConversationWithMessage: %v", err)
	}
	if result.Conversation == nil || result.Message == nil {
		t.Fatalf("expected conversation and message, got %#v", result)
	}
	if result.Conversation.Subject != "Renewal question" {
		t.Fatalf("subject = %q", result.Conversation.Subject)
	}
	if result.Conversation.AssignedUserID == nil || *result.Conversation.AssignedUserID != actorID {
		t.Fatalf("assigned_user_id = %#v, want %q", result.Conversation.AssignedUserID, actorID)
	}
	if result.Message.ConversationID != result.Conversation.ID || result.Message.Content != "Hi Jane, following up here." {
		t.Fatalf("unexpected first message: %#v", result.Message)
	}
	var metadata struct {
		DeliveryChannels []string `json:"delivery_channels"`
		CCEmails         []string `json:"email_cc"`
		BCCEmails        []string `json:"email_bcc"`
	}
	if err := json.Unmarshal([]byte(result.Message.Metadata), &metadata); err != nil {
		t.Fatalf("parse message metadata %q: %v", result.Message.Metadata, err)
	}
	if !slices.Equal(metadata.DeliveryChannels, []string{"chat", "email"}) {
		t.Fatalf("delivery_channels = %#v", metadata.DeliveryChannels)
	}
	if !slices.Equal(metadata.CCEmails, []string{"finance@example.com"}) {
		t.Fatalf("cc metadata = %#v", metadata.CCEmails)
	}
	if !slices.Equal(metadata.BCCEmails, []string{"audit@example.com"}) {
		t.Fatalf("bcc metadata = %#v", metadata.BCCEmails)
	}
	if !slices.Equal([]string(result.Conversation.EmailCC), []string{"finance@example.com"}) {
		t.Fatalf("conversation email_cc = %#v", result.Conversation.EmailCC)
	}

	tags, err := repository.NewSupportTagRepository(db).ListByConversationIDs(ctx, workspaceID, []string{result.Conversation.ID})
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	if len(tags[result.Conversation.ID]) != 1 || tags[result.Conversation.ID][0].ID != "tag-new-conversation" {
		t.Fatalf("conversation tags = %#v", tags[result.Conversation.ID])
	}
}

func TestSupportInboxServiceUpdateConversationEmailRecipientsSwitchesPrimaryAndKeepsPreviousAsCC(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	workspaceID := "ws-recipient-switch"
	actorID := "user-recipient-switch"
	seedUser(t, db, actorID, "owner@example.com", "Owner", "hashed")
	seedWorkspace(t, db, workspaceID, "Recipient Switch WS", "recipient-switch", actorID)
	seedWorkspaceMember(t, db, "wm-recipient-switch", workspaceID, actorID, "owner@example.com", "Owner", model.RoleOwner)

	convRepo := repository.NewSupportConversationRepository(db)
	msgRepo := repository.NewSupportMessageRepository(db)
	svc := NewSupportInboxService(
		convRepo,
		repository.NewSupportMailboxRepository(db),
		msgRepo,
		nil,
		nil,
		repository.NewSupportInboxInstallationRepository(db),
		repository.NewSupportInboxSessionRepository(db),
		nil,
		nil,
		nil,
		repository.NewCRMContactRepository(db),
		repository.NewUserRepository(db),
		nil,
		nil,
		nil,
	).SetWorkspaceRepo(repository.NewWorkspaceRepository(db))

	currentEmail := "teammate@company.com"
	currentName := "Alex Teammate"
	suggestedEmail := "jane@example.com"
	suggestedName := "Jane Persona"
	conversation := &model.SupportConversation{
		WorkspaceID:                    workspaceID,
		DisplayID:                      1,
		Subject:                        "Copied thread",
		Status:                         model.SupportConversationStatusOpen,
		FlowState:                      strPtr(model.SupportConversationFlowStateWaitingForHuman),
		Priority:                       "medium",
		Channel:                        "email",
		CustomerName:                   &currentName,
		CustomerEmail:                  &currentEmail,
		Source:                         "email",
		PrimaryRecipientState:          model.SupportPrimaryRecipientStateUnconfirmed,
		SuggestedPrimaryRecipientEmail: &suggestedEmail,
		SuggestedPrimaryRecipientName:  &suggestedName,
		EmailThreadParticipants:        model.DocsStringArray{suggestedEmail},
	}
	if err := convRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	updated, err := svc.UpdateConversationEmailRecipients(ctx, workspaceID, conversation.ID, model.UpdateConversationEmailRecipientsRequest{
		PrimaryRecipientEmail: &suggestedEmail,
		PrimaryRecipientName:  &suggestedName,
		ConfirmPrimary:        boolPtr(true),
	}, actorID)
	if err != nil {
		t.Fatalf("UpdateConversationEmailRecipients: %v", err)
	}

	if updated.CustomerEmail == nil || *updated.CustomerEmail != suggestedEmail {
		t.Fatalf("customer_email = %#v, want %q", updated.CustomerEmail, suggestedEmail)
	}
	if updated.CustomerName == nil || *updated.CustomerName != suggestedName {
		t.Fatalf("customer_name = %#v, want %q", updated.CustomerName, suggestedName)
	}
	if updated.PrimaryRecipientState != model.SupportPrimaryRecipientStateConfirmed {
		t.Fatalf("primary_recipient_state = %q", updated.PrimaryRecipientState)
	}
	if updated.SuggestedPrimaryRecipientEmail != nil || updated.SuggestedPrimaryRecipientName != nil {
		t.Fatalf("suggested recipient not cleared: email=%#v name=%#v", updated.SuggestedPrimaryRecipientEmail, updated.SuggestedPrimaryRecipientName)
	}
	if len(updated.EmailCC) != 1 || updated.EmailCC[0] != currentEmail {
		t.Fatalf("email_cc = %#v, want previous primary as cc", updated.EmailCC)
	}
	messages, err := msgRepo.ListByConversation(ctx, workspaceID, conversation.ID, true)
	if err != nil {
		t.Fatalf("list messages after recipient switch: %v", err)
	}
	if len(messages) != 1 || messages[0].SystemEventType == nil || *messages[0].SystemEventType != model.SystemEventEmailRecipientsUpdated {
		t.Fatalf("expected email recipient system event, got %#v", messages)
	}
	if !strings.Contains(messages[0].Content, "Owner made jane@example.com the primary recipient.") {
		t.Fatalf("recipient switch system message = %q", messages[0].Content)
	}
	if !strings.Contains(messages[0].Content, "Owner added teammate@company.com to Cc.") {
		t.Fatalf("recipient cc add system message = %q", messages[0].Content)
	}

	updated, err = svc.UpdateConversationEmailRecipients(ctx, workspaceID, conversation.ID, model.UpdateConversationEmailRecipientsRequest{
		CCEmails: []string{},
	}, actorID)
	if err != nil {
		t.Fatalf("remove cc UpdateConversationEmailRecipients: %v", err)
	}
	if len(updated.EmailCC) != 0 {
		t.Fatalf("email_cc after remove = %#v", updated.EmailCC)
	}
	messages, err = msgRepo.ListByConversation(ctx, workspaceID, conversation.ID, true)
	if err != nil {
		t.Fatalf("list messages after recipient remove: %v", err)
	}
	lastMessage := messages[len(messages)-1]
	if lastMessage.SystemEventType == nil || *lastMessage.SystemEventType != model.SystemEventEmailRecipientsUpdated {
		t.Fatalf("expected email recipient remove system event, got %#v", lastMessage.SystemEventType)
	}
	if !strings.Contains(lastMessage.Content, "Owner removed teammate@company.com from Cc.") {
		t.Fatalf("recipient cc remove system message = %q", lastMessage.Content)
	}
}

func TestSupportInboxServiceCreateConversationMessageBlocksEmailReplyUntilPrimaryRecipientConfirmed(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	workspaceID := "ws-unconfirmed-send"
	actorID := "user-unconfirmed-send"
	seedUser(t, db, actorID, "agent@example.com", "Agent User", "hash")
	seedWorkspace(t, db, workspaceID, "Unconfirmed Send Workspace", "unconfirmed-send", actorID)
	seedWorkspaceMember(t, db, "wm-unconfirmed-send", workspaceID, actorID, "agent@example.com", "Agent User", model.RoleAdmin)

	convRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	svc := NewSupportInboxService(
		convRepo,
		repository.NewSupportMailboxRepository(db),
		messageRepo,
		repository.NewAgentRepository(db),
		repository.NewCRMAssociationRepository(db),
		repository.NewSupportInboxInstallationRepository(db),
		repository.NewSupportInboxSessionRepository(db),
		repository.NewSupportCannedResponseRepository(db),
		nil,
		nil,
		repository.NewCRMContactRepository(db),
		repository.NewUserRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db, false),
		repository.NewDocsHelpcenterRepository(db, false),
	)

	primaryEmail := "teammate@company.com"
	suggestedEmail := "jane@example.com"
	conversation := &model.SupportConversation{
		WorkspaceID:                    workspaceID,
		DisplayID:                      1,
		Subject:                        "Copied thread",
		Status:                         model.SupportConversationStatusOpen,
		FlowState:                      strPtr(model.SupportConversationFlowStateWaitingForHuman),
		Priority:                       "medium",
		Channel:                        "email",
		CustomerEmail:                  &primaryEmail,
		Source:                         "email",
		PrimaryRecipientState:          model.SupportPrimaryRecipientStateUnconfirmed,
		SuggestedPrimaryRecipientEmail: &suggestedEmail,
	}
	if err := convRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	_, err := svc.CreateConversationMessage(ctx, workspaceID, conversation.ID, model.CreateMessageRequest{
		Content: "Looping back over email.",
		Channels: []string{
			"email",
		},
	}, "user", &actorID, nil, nil)
	if err == nil {
		t.Fatalf("expected unconfirmed recipient error")
	}
	if !strings.Contains(err.Error(), "primary recipient") {
		t.Fatalf("error = %q, want primary recipient confirmation", err.Error())
	}

	_, err = svc.CreateConversationMessage(ctx, workspaceID, conversation.ID, model.CreateMessageRequest{
		Content:    "Internal context is still allowed.",
		IsInternal: true,
	}, "user", &actorID, nil, nil)
	if err != nil {
		t.Fatalf("internal note should not be blocked: %v", err)
	}
	if err := db.Model(&model.SupportConversation{}).Where("id = ?", conversation.ID).
		Update("primary_recipient_state", model.SupportPrimaryRecipientStateConfirmed).Error; err != nil {
		t.Fatalf("confirm recipient for AI-assisted reply: %v", err)
	}

	aiAssisted, err := svc.CreateConversationMessage(ctx, workspaceID, conversation.ID, model.CreateMessageRequest{
		Content:         "Here is the AI-polished response.",
		ClientMessageID: "optimistic-conversation-1",
		AIAssisted:      true,
	}, "user", &actorID, nil, nil)
	if err != nil {
		t.Fatalf("AI-assisted reply: %v", err)
	}
	if !strings.Contains(aiAssisted.Metadata, `"ai_assisted":true`) {
		t.Fatalf("AI-assisted reply metadata = %q", aiAssisted.Metadata)
	}
	if aiAssisted.ClientMessageID != "optimistic-conversation-1" {
		t.Fatalf("client_message_id = %q, want optimistic-conversation-1", aiAssisted.ClientMessageID)
	}
	messages, err := messageRepo.ListByConversation(ctx, workspaceID, conversation.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	var joined *model.SupportMessage
	for i := range messages {
		if messages[i].SystemEventType != nil && *messages[i].SystemEventType == model.SystemEventTeammateJoined {
			joined = &messages[i]
			break
		}
	}
	if joined == nil {
		t.Fatal("expected first-reply joined event")
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(joined.Metadata), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["reply_client_message_id"] != aiAssisted.ClientMessageID {
		t.Fatalf("joined event lost reply correlation: %s", joined.Metadata)
	}
	saved, err := messageRepo.GetByID(ctx, aiAssisted.ID)
	if err != nil {
		t.Fatal(err)
	}
	var savedMetadata map[string]any
	if err := json.Unmarshal([]byte(saved.Metadata), &savedMetadata); err != nil {
		t.Fatal(err)
	}
	if savedMetadata["client_message_id"] != aiAssisted.ClientMessageID {
		t.Fatalf("saved reply lost client identity: %s", saved.Metadata)
	}

	if joined.ClientMessageID != "" {
		t.Fatal("joined event must not share the reply's deduplication ID")
	}

}

func TestAgentServiceRunConversationAgentSkipsUnconfirmedPrimaryRecipient(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)

	workspaceID := "ws-agent-unconfirmed-recipient"
	seedWorkspace(t, db, workspaceID, "Agent Unconfirmed Workspace", "agent-unconfirmed", "owner-agent-unconfirmed")

	primaryEmail := "teammate@company.com"
	suggestedEmail := "jane@example.com"
	conversation := &model.SupportConversation{
		WorkspaceID:                    workspaceID,
		DisplayID:                      1,
		Subject:                        "Copied thread",
		Status:                         model.SupportConversationStatusOpen,
		FlowState:                      strPtr(model.SupportConversationFlowStateWaitingForHuman),
		Priority:                       "medium",
		Channel:                        "email",
		CustomerEmail:                  &primaryEmail,
		Source:                         "email",
		PrimaryRecipientState:          model.SupportPrimaryRecipientStateUnconfirmed,
		SuggestedPrimaryRecipientEmail: &suggestedEmail,
	}
	convRepo := repository.NewSupportConversationRepository(db)
	if err := convRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	svc := &AgentService{conversationRepo: convRepo}
	_, err := svc.RunConversationAgentAuto(ctx, workspaceID, conversation.ID)
	if err == nil {
		t.Fatalf("expected unconfirmed recipient error")
	}
	if !strings.Contains(err.Error(), "primary recipient") {
		t.Fatalf("error = %q, want primary recipient confirmation", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Message Repository Tests
// ---------------------------------------------------------------------------

func TestSupportMessageRepository(t *testing.T) {
	db := newTestDB(t)

	workspaceID := "ws-msg-test"
	seedWorkspace(t, db, workspaceID, "Message Test WS", "msg-test-ws", "user-123")

	convRepo := repository.NewSupportConversationRepository(db)
	msgRepo := repository.NewSupportMessageRepository(db)
	ctx := context.Background()

	// Create a conversation to attach messages to.
	conv := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Message test conv",
		Status:      "open",
	}
	if err := convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	t.Run("Create and list messages by conversation_id", func(t *testing.T) {
		msg := &model.SupportMessage{
			WorkspaceID:    workspaceID,
			ConversationID: conv.ID,
			SenderType:     "customer",
			Content:        "Hello, need help",
			MessageType:    "reply",
		}
		if err := msgRepo.Create(ctx, msg); err != nil {
			t.Fatalf("create message: %v", err)
		}
		if msg.ID == "" {
			t.Error("expected message ID to be set")
		}

		messages, err := msgRepo.ListByConversation(ctx, workspaceID, conv.ID, true)
		if err != nil {
			t.Fatalf("list messages: %v", err)
		}
		if len(messages) != 1 {
			t.Errorf("expected 1 message, got %d", len(messages))
		}
		if messages[0].Content != "Hello, need help" {
			t.Errorf("unexpected content: %q", messages[0].Content)
		}
	})

	t.Run("List messages by conversation_id only", func(t *testing.T) {
		// After migration 039, ticket_id column is dropped. Only conversation_id is used.
		mustExec(t, db, `INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content, message_type, is_internal) VALUES (?, ?, ?, 'user', 'Second message', 'reply', 0)`,
			"msg-002", workspaceID, conv.ID)

		messages, err := msgRepo.ListByConversation(ctx, workspaceID, conv.ID, true)
		if err != nil {
			t.Fatalf("list messages: %v", err)
		}
		if len(messages) < 2 {
			t.Errorf("expected at least 2 messages, got %d", len(messages))
		}
	})

	t.Run("Internal notes filtered when includeInternal is false", func(t *testing.T) {
		// Create an internal note
		note := &model.SupportMessage{
			WorkspaceID:    workspaceID,
			ConversationID: conv.ID,
			SenderType:     "user",
			Content:        "Internal note for team",
			MessageType:    "reply",
			IsInternal:     true,
		}
		if err := msgRepo.Create(ctx, note); err != nil {
			t.Fatalf("create internal note: %v", err)
		}

		// With internal
		allMsgs, err := msgRepo.ListByConversation(ctx, workspaceID, conv.ID, true)
		if err != nil {
			t.Fatalf("list all: %v", err)
		}

		// Without internal
		publicMsgs, err := msgRepo.ListByConversation(ctx, workspaceID, conv.ID, false)
		if err != nil {
			t.Fatalf("list public: %v", err)
		}

		if len(publicMsgs) >= len(allMsgs) {
			t.Errorf("expected fewer public messages (%d) than all messages (%d)", len(publicMsgs), len(allMsgs))
		}

		for _, m := range publicMsgs {
			if m.IsInternal {
				t.Error("found internal note in public-only list")
			}
		}
	})

	t.Run("Messages ordered by created_at ASC", func(t *testing.T) {
		orderWS := "ws-msg-order"
		seedWorkspace(t, db, orderWS, "Order WS", "order-ws", "user-123")
		oConv := &model.SupportConversation{
			WorkspaceID: orderWS,
			Subject:     "Order test",
			Status:      "open",
		}
		if err := convRepo.Create(ctx, oConv); err != nil {
			t.Fatalf("create: %v", err)
		}

		// Create messages in order
		for _, content := range []string{"First", "Second", "Third"} {
			msg := &model.SupportMessage{
				WorkspaceID:    orderWS,
				ConversationID: oConv.ID,
				SenderType:     "customer",
				Content:        content,
				MessageType:    "reply",
			}
			if err := msgRepo.Create(ctx, msg); err != nil {
				t.Fatalf("create %s: %v", content, err)
			}
		}

		messages, err := msgRepo.ListByConversation(ctx, orderWS, oConv.ID, true)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(messages) != 3 {
			t.Fatalf("expected 3 messages, got %d", len(messages))
		}
		if messages[0].Content != "First" || messages[2].Content != "Third" {
			t.Errorf("messages not in ASC order: [%q, %q, %q]", messages[0].Content, messages[1].Content, messages[2].Content)
		}
	})
}

func TestCreateConversationMessage_CustomerReplyCreatesOwnedSupportNotification(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := time.Now()
	ensureSupportModuleGrantsTable(t, db)

	workspaceID := "ws-support-notifs"
	ownerUserID := "user-owner"

	seedUser(t, db, ownerUserID, "owner@example.com", "Owner User", "hash")
	seedWorkspace(t, db, workspaceID, "Support Notifications WS", "support-notifications-ws", ownerUserID)
	seedWorkspaceMember(t, db, "wm-owner", workspaceID, ownerUserID, "owner@example.com", "Owner User", model.RoleAdmin)

	mustExec(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-owner", ownerUserID, true, "immediate", "09:00", 1, false, "all", "UTC", now, now)

	convRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)

	conv := &model.SupportConversation{
		WorkspaceID:    workspaceID,
		Subject:        "Billing question",
		Status:         "open",
		OpenedByUserID: &ownerUserID,
	}
	if err := convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	emailer := &stubEmailSender{}
	notificationService := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		repository.NewFollowerRepository(db),
		repository.NewUserRepository(db),
		repository.NewWorkspaceRepository(db),
		nil,
		emailer,
		"",
	)

	svc := NewSupportInboxService(
		convRepo,
		repository.NewSupportMailboxRepository(db),
		messageRepo,
		repository.NewAgentRepository(db),
		repository.NewCRMAssociationRepository(db),
		repository.NewSupportInboxInstallationRepository(db),
		repository.NewSupportInboxSessionRepository(db),
		repository.NewSupportCannedResponseRepository(db),
		nil,
		nil,
		repository.NewCRMContactRepository(db),
		repository.NewUserRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db, false),
		repository.NewDocsHelpcenterRepository(db, false),
	)
	svc.SetNotificationService(notificationService, repository.NewWorkspaceRepository(db))

	customerName := "Sarah"
	if _, err := svc.CreateConversationMessage(
		ctx,
		workspaceID,
		conv.ID,
		model.CreateMessageRequest{Content: "I still need help with billing", MessageType: "reply"},
		"customer",
		nil,
		nil,
		&customerName,
	); err != nil {
		t.Fatalf("CreateConversationMessage: %v", err)
	}

	var notifications []model.Notification
	if err := db.WithContext(ctx).Order("recipient_id ASC").Find(&notifications).Error; err != nil {
		t.Fatalf("load notifications: %v", err)
	}
	if len(notifications) != 1 {
		t.Fatalf("notification count = %d, want 1", len(notifications))
	}
	if notifications[0].RecipientID != ownerUserID {
		t.Fatalf("recipient_id = %q, want %q", notifications[0].RecipientID, ownerUserID)
	}
	if notifications[0].EventType != "support_conversation.customer_reply" {
		t.Fatalf("event_type = %q, want support_conversation.customer_reply", notifications[0].EventType)
	}
	wantTitle := fmt.Sprintf("Sarah replied to conversation #%d", conv.DisplayID)
	if notifications[0].Title != wantTitle {
		t.Fatalf("title = %q, want %q", notifications[0].Title, wantTitle)
	}
	if notifications[0].LatestEventCategory != model.NotifCategorySupportReplies {
		t.Fatalf("latest_event_category = %q, want %q", notifications[0].LatestEventCategory, model.NotifCategorySupportReplies)
	}

	deliveries := loadNotificationServiceDeliveries(t, db)
	if len(deliveries) != 2 {
		t.Fatalf("deliveries = %+v, want in_app + delayed email rows", deliveries)
	}
	if deliveries[0].Channel != "in_app" || deliveries[0].Status != "delivered" {
		t.Fatalf("first delivery = %+v, want delivered in_app", deliveries[0])
	}
	if deliveries[1].Channel != "support_reply_email" || deliveries[1].Status != "pending" {
		t.Fatalf("second delivery = %+v, want pending support_reply_email", deliveries[1])
	}
	if len(emailer.sent) != 0 {
		t.Fatalf("sent email count = %d, want 0", len(emailer.sent))
	}
}

func TestCreateConversationMessage_CustomerReplySkipsUsersWithoutSupportAccess(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	ensureSupportModuleGrantsTable(t, db)

	workspaceID := "ws-support-access-notifs"
	ownerUserID := "zzz-owner-support-access"
	supportUserID := "bbb-support-access"
	marketingUserID := "aaa-marketing-access"

	seedUser(t, db, ownerUserID, "owner-access@example.com", "Owner User", "hash")
	seedUser(t, db, supportUserID, "support-access@example.com", "Support User", "hash")
	seedUser(t, db, marketingUserID, "marketing-access@example.com", "Marketing User", "hash")
	seedWorkspace(t, db, workspaceID, "Support Access Notifications", "support-access-notifs", ownerUserID)
	seedWorkspaceMember(t, db, "wm-owner-support-access", workspaceID, ownerUserID, "owner-access@example.com", "Owner User", model.RoleAdmin)
	seedWorkspaceMember(t, db, "wm-support-access", workspaceID, supportUserID, "support-access@example.com", "Support User", model.RoleMember)
	seedWorkspaceMember(t, db, "wm-marketing-access", workspaceID, marketingUserID, "marketing-access@example.com", "Marketing User", model.RoleMember)
	seedSupportModuleGrant(t, db, "grant-support-access", workspaceID, model.ModuleGrantSubjectWorkspaceMember, "wm-support-access")

	convRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	conv := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Routing question",
		Status:      "open",
	}
	if err := convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	notificationService := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		repository.NewFollowerRepository(db),
		repository.NewUserRepository(db),
		repository.NewWorkspaceRepository(db),
		nil,
		&stubEmailSender{},
		"",
	)

	svc := NewSupportInboxService(
		convRepo,
		repository.NewSupportMailboxRepository(db),
		messageRepo,
		repository.NewAgentRepository(db),
		repository.NewCRMAssociationRepository(db),
		repository.NewSupportInboxInstallationRepository(db),
		repository.NewSupportInboxSessionRepository(db),
		repository.NewSupportCannedResponseRepository(db),
		nil,
		nil,
		repository.NewCRMContactRepository(db),
		repository.NewUserRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db),
		repository.NewDocsHelpcenterRepository(db),
	)
	svc.SetNotificationService(notificationService, repository.NewWorkspaceRepository(db))

	customerName := "Customer"
	if _, err := svc.CreateConversationMessage(
		ctx,
		workspaceID,
		conv.ID,
		model.CreateMessageRequest{Content: "Can someone help?", MessageType: "reply"},
		"customer",
		nil,
		nil,
		&customerName,
	); err != nil {
		t.Fatalf("CreateConversationMessage: %v", err)
	}

	var notifications []model.Notification
	if err := db.WithContext(ctx).Order("recipient_id ASC").Find(&notifications).Error; err != nil {
		t.Fatalf("load notifications: %v", err)
	}
	if len(notifications) != 1 {
		t.Fatalf("notification count = %d, want 1", len(notifications))
	}
	if notifications[0].RecipientID != supportUserID {
		t.Fatalf("recipient_id = %q, want %q", notifications[0].RecipientID, supportUserID)
	}
	if notifications[0].RecipientID == marketingUserID {
		t.Fatalf("marketing user received support notification")
	}
}

func TestCreateConversationMessage_SupportMentionSkipsUsersWithoutSupportAccess(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	ensureSupportModuleGrantsTable(t, db)

	workspaceID := "ws-support-mention-access"
	actorID := "user-mention-actor"
	supportUserID := "user-mention-support"
	marketingUserID := "user-mention-marketing"

	seedUser(t, db, actorID, "actor-mention@example.com", "Admin Actor", "hash")
	seedUser(t, db, supportUserID, "support-mention@example.com", "Support Mention", "hash")
	seedUser(t, db, marketingUserID, "marketing-mention@example.com", "Marketing Mention", "hash")
	seedWorkspace(t, db, workspaceID, "Support Mention Access", "support-mention-access", actorID)
	seedWorkspaceMember(t, db, "wm-mention-actor", workspaceID, actorID, "actor-mention@example.com", "Admin Actor", model.RoleAdmin)
	seedWorkspaceMember(t, db, "wm-mention-support", workspaceID, supportUserID, "support-mention@example.com", "Support Mention", model.RoleMember)
	seedWorkspaceMember(t, db, "wm-mention-marketing", workspaceID, marketingUserID, "marketing-mention@example.com", "Marketing Mention", model.RoleMember)
	seedSupportModuleGrant(t, db, "grant-mention-support", workspaceID, model.ModuleGrantSubjectWorkspaceMember, "wm-mention-support")

	convRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	conv := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Mention question",
		Status:      "open",
	}
	if err := convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	workspaceRepo := repository.NewWorkspaceRepository(db)
	notificationService := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		repository.NewFollowerRepository(db),
		repository.NewUserRepository(db),
		workspaceRepo,
		nil,
		nil,
		"",
	)

	svc := NewSupportInboxService(
		convRepo,
		repository.NewSupportMailboxRepository(db),
		messageRepo,
		repository.NewAgentRepository(db),
		repository.NewCRMAssociationRepository(db),
		repository.NewSupportInboxInstallationRepository(db),
		repository.NewSupportInboxSessionRepository(db),
		repository.NewSupportCannedResponseRepository(db),
		nil,
		nil,
		repository.NewCRMContactRepository(db),
		repository.NewUserRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db),
		repository.NewDocsHelpcenterRepository(db),
	)
	svc.SetNotificationService(notificationService, workspaceRepo)

	if _, err := svc.CreateConversationMessage(
		ctx,
		workspaceID,
		conv.ID,
		model.CreateMessageRequest{Content: "@support-mention @marketing-mention please look", MessageType: "note", IsInternal: true},
		"user",
		&actorID,
		nil,
		nil,
	); err != nil {
		t.Fatalf("CreateConversationMessage: %v", err)
	}

	var notifications []model.Notification
	if err := db.WithContext(ctx).Order("recipient_id ASC").Find(&notifications).Error; err != nil {
		t.Fatalf("load notifications: %v", err)
	}
	if len(notifications) != 1 {
		t.Fatalf("notification count = %d, want 1", len(notifications))
	}
	if notifications[0].RecipientID != supportUserID {
		t.Fatalf("recipient_id = %q, want %q", notifications[0].RecipientID, supportUserID)
	}
	if notifications[0].EventType != "support_conversation.mentioned" {
		t.Fatalf("event_type = %q, want support_conversation.mentioned", notifications[0].EventType)
	}
}

func TestMarkConversationRead_MarksSupportReplyNotificationsRead(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := time.Now()

	workspaceID := "ws-support-read"
	userID := "user-owner"

	seedUser(t, db, userID, "owner@example.com", "Owner User", "hash")
	seedWorkspace(t, db, workspaceID, "Support Read WS", "support-read-ws", userID)
	seedWorkspaceMember(t, db, "wm-owner", workspaceID, userID, "owner@example.com", "Owner User", model.RoleAdmin)
	mustExec(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"settings-owner", userID, true, "immediate", "09:00", 1, false, "all", "UTC", now, now)

	convRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	notificationService := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		repository.NewFollowerRepository(db),
		repository.NewUserRepository(db),
		repository.NewWorkspaceRepository(db),
		nil,
		&stubEmailSender{},
		"",
	)

	conv := &model.SupportConversation{
		WorkspaceID:    workspaceID,
		Subject:        "Billing question",
		Status:         "open",
		OpenedByUserID: &userID,
	}
	if err := convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	svc := NewSupportInboxService(
		convRepo,
		repository.NewSupportMailboxRepository(db),
		messageRepo,
		repository.NewAgentRepository(db),
		repository.NewCRMAssociationRepository(db),
		repository.NewSupportInboxInstallationRepository(db),
		repository.NewSupportInboxSessionRepository(db),
		repository.NewSupportCannedResponseRepository(db),
		nil,
		nil,
		repository.NewCRMContactRepository(db),
		repository.NewUserRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db, false),
		repository.NewDocsHelpcenterRepository(db, false),
	)
	svc.SetNotificationService(notificationService, repository.NewWorkspaceRepository(db))

	customerName := "Customer"
	if _, err := svc.CreateConversationMessage(
		ctx,
		workspaceID,
		conv.ID,
		model.CreateMessageRequest{Content: "I still need help", MessageType: "reply"},
		"customer",
		nil,
		nil,
		&customerName,
	); err != nil {
		t.Fatalf("CreateConversationMessage: %v", err)
	}

	if err := svc.MarkConversationRead(ctx, workspaceID, conv.ID, userID); err != nil {
		t.Fatalf("MarkConversationRead: %v", err)
	}

	var notification model.Notification
	if err := db.WithContext(ctx).
		Where("workspace_id = ? AND recipient_id = ? AND entity_id = ?", workspaceID, userID, conv.ID).
		First(&notification).Error; err != nil {
		t.Fatalf("load notification: %v", err)
	}
	if notification.Status != "read" {
		t.Fatalf("notification status = %q, want read", notification.Status)
	}
	if notification.ReadAt == nil {
		t.Fatal("expected notification read_at to be set")
	}
}

func TestCreateConversationMessage_PublicMentionsNotifyWorkspaceMembers(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	workspaceID := "ws-support-mentions"
	senderUserID := "user-sender"
	mentionedUserID := "user-mentioned"

	seedUser(t, db, senderUserID, "sender@example.com", "Sender User", "hash")
	seedUser(t, db, mentionedUserID, "mentioned@example.com", "Teammate Mentioned", "hash")
	seedWorkspace(t, db, workspaceID, "Support Mentions", "support-mentions", senderUserID)
	seedWorkspaceMember(t, db, "wm-sender", workspaceID, senderUserID, "sender@example.com", "Sender User", model.RoleAdmin)
	seedWorkspaceMember(t, db, "wm-mentioned", workspaceID, mentionedUserID, "mentioned@example.com", "Teammate Mentioned", model.RoleMember)
	seedSupportModuleGrant(t, db, "grant-mentioned-support", workspaceID, model.ModuleGrantSubjectWorkspaceMember, "wm-mentioned")
	for _, userID := range []string{senderUserID, mentionedUserID} {
		mustExec(t, db, `INSERT INTO user_notification_settings (id, user_id, email_enabled, email_digest_frequency, email_digest_time, email_digest_day, do_not_disturb, badge_mode, timezone, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"settings-"+userID, userID, false, "daily", "09:00", 1, false, "all", "UTC", now, now)
	}

	convRepo := repository.NewSupportConversationRepository(db)
	messageRepo := repository.NewSupportMessageRepository(db)
	workspaceRepo := repository.NewWorkspaceRepository(db)
	notificationService := NewNotificationService(
		repository.NewNotificationRepository(db),
		repository.NewNotificationPreferenceRepository(db),
		repository.NewUserNotificationSettingsRepository(db),
		repository.NewFollowerRepository(db),
		repository.NewUserRepository(db),
		workspaceRepo,
		nil,
		nil,
		"",
	)

	conv := &model.SupportConversation{
		WorkspaceID:    workspaceID,
		Subject:        "Public mention",
		Status:         model.SupportConversationStatusOpen,
		OpenedByUserID: &senderUserID,
	}
	if err := convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	svc := NewSupportInboxService(
		convRepo,
		repository.NewSupportMailboxRepository(db),
		messageRepo,
		repository.NewAgentRepository(db),
		repository.NewCRMAssociationRepository(db),
		repository.NewSupportInboxInstallationRepository(db),
		repository.NewSupportInboxSessionRepository(db),
		repository.NewSupportCannedResponseRepository(db),
		nil,
		nil,
		repository.NewCRMContactRepository(db),
		repository.NewUserRepository(db),
		repository.NewDocsSpaceRepository(db),
		repository.NewDocsCollectionRepository(db),
		repository.NewDocsHelpcenterRepository(db),
	).SetNotificationService(notificationService, workspaceRepo)

	msg, err := svc.CreateConversationMessage(
		ctx,
		workspaceID,
		conv.ID,
		model.CreateMessageRequest{Content: "@teammate.mentioned can you take this one?", MessageType: "reply"},
		"user",
		&senderUserID,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("CreateConversationMessage: %v", err)
	}

	if !strings.Contains(msg.Metadata, mentionedUserID) {
		t.Fatalf("message metadata = %q, want mentioned user id %q", msg.Metadata, mentionedUserID)
	}

	var notifications []model.Notification
	if err := db.WithContext(ctx).
		Where("workspace_id = ? AND entity_type = ? AND entity_id = ?", workspaceID, "support_conversation", conv.ID).
		Order("recipient_id ASC").
		Find(&notifications).Error; err != nil {
		t.Fatalf("load notifications: %v", err)
	}
	if len(notifications) != 1 {
		t.Fatalf("notification count = %d, want 1", len(notifications))
	}
	if notifications[0].RecipientID != mentionedUserID {
		t.Fatalf("recipient_id = %q, want %q", notifications[0].RecipientID, mentionedUserID)
	}
	if notifications[0].EventType != "support_conversation.mentioned" {
		t.Fatalf("event_type = %q, want support_conversation.mentioned", notifications[0].EventType)
	}
	wantTitle := fmt.Sprintf("Sender User mentioned you in conversation #%d", conv.DisplayID)
	if notifications[0].Title != wantTitle {
		t.Fatalf("title = %q, want %q", notifications[0].Title, wantTitle)
	}
	if notifications[0].LatestEventCategory != model.NotifCategorySupportMentions {
		t.Fatalf("latest_event_category = %q, want %q", notifications[0].LatestEventCategory, model.NotifCategorySupportMentions)
	}
}

func TestAssignConversationUserDisablesMembersWithoutInboxAccess(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	mustExec(t, db, `CREATE TABLE IF NOT EXISTS workspace_module_grants (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		module TEXT NOT NULL,
		subject_type TEXT NOT NULL,
		subject_id TEXT NOT NULL,
		access_level TEXT NOT NULL,
		created_by_id TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`)

	workspaceID := "ws-support-assign"
	ownerID := "user-owner"
	eligibleID := "user-eligible"
	blockedID := "user-blocked"
	noSupportID := "user-no-support"

	seedUser(t, db, ownerID, "owner@example.com", "Owner User", "hash")
	seedUser(t, db, eligibleID, "eligible@example.com", "Eligible User", "hash")
	seedUser(t, db, blockedID, "blocked@example.com", "Blocked User", "hash")
	seedUser(t, db, noSupportID, "nosupport@example.com", "No Support User", "hash")
	seedWorkspace(t, db, workspaceID, "Support Assign WS", "support-assign-ws", ownerID)

	workspaceRepo := repository.NewWorkspaceRepository(db)
	ownerMember, err := workspaceRepo.AddMember(ctx, workspaceID, ownerID, model.RoleOwner)
	if err != nil {
		t.Fatalf("add owner member: %v", err)
	}
	eligibleMember, err := workspaceRepo.AddMember(ctx, workspaceID, eligibleID, model.RoleMember)
	if err != nil {
		t.Fatalf("add eligible member: %v", err)
	}
	blockedMember, err := workspaceRepo.AddMember(ctx, workspaceID, blockedID, model.RoleMember)
	if err != nil {
		t.Fatalf("add blocked member: %v", err)
	}
	_, err = workspaceRepo.AddMember(ctx, workspaceID, noSupportID, model.RoleMember)
	if err != nil {
		t.Fatalf("add no-support member: %v", err)
	}

	mailboxRepo := repository.NewSupportMailboxRepository(db)
	mailbox := &model.SupportMailbox{
		WorkspaceID:    workspaceID,
		Name:           "Billing",
		Handle:         "billing",
		Icon:           "inbox",
		VisibilityMode: "members_only",
		AssignmentMode: "manual",
		Active:         true,
		CreatedByID:    ownerID,
	}
	if err := mailboxRepo.Create(ctx, mailbox); err != nil {
		t.Fatalf("create mailbox: %v", err)
	}
	if err := mailboxRepo.AddMembers(ctx, mailbox.ID, []string{eligibleMember.ID}); err != nil {
		t.Fatalf("add mailbox member: %v", err)
	}

	for _, grant := range []struct {
		id       string
		memberID string
	}{
		{id: "grant-eligible", memberID: eligibleMember.ID},
		{id: "grant-blocked", memberID: blockedMember.ID},
	} {
		mustExec(t, db, `INSERT INTO workspace_module_grants (id, workspace_id, module, subject_type, subject_id, access_level, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			grant.id, workspaceID, model.ModuleSupport, model.ModuleGrantSubjectWorkspaceMember, grant.memberID, "member", now, now)
	}

	convRepo := repository.NewSupportConversationRepository(db)
	conv := &model.SupportConversation{
		WorkspaceID:    workspaceID,
		MailboxID:      &mailbox.ID,
		Subject:        "Need help",
		Status:         "open",
		OpenedByUserID: &ownerID,
	}
	if err := convRepo.Create(ctx, conv); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	svc := NewSupportInboxService(
		convRepo,
		mailboxRepo,
		repository.NewSupportMessageRepository(db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		repository.NewUserRepository(db),
		nil,
		nil,
		nil,
	).SetWorkspaceRepo(workspaceRepo).SetAuthzService(
		authorization.NewAuthzService(
			db,
			authorization.NewGORMMemberRepository(db),
			repository.NewWorkspaceModuleGrantRepository(db),
		),
	)

	assignable, err := svc.ListConversationAssignableUsers(ctx, workspaceID, conv.ID)
	if err != nil {
		t.Fatalf("list conversation assignable users: %v", err)
	}
	assignableByUserID := make(map[string]model.AssignableMember, len(assignable))
	for _, member := range assignable {
		if member.UserID != nil {
			assignableByUserID[*member.UserID] = member
		}
	}
	for _, userID := range []string{ownerID, eligibleID, blockedID, noSupportID} {
		if _, ok := assignableByUserID[userID]; !ok {
			t.Fatalf("expected %s in assignable users, got %#v", userID, assignableByUserID)
		}
	}
	for _, userID := range []string{blockedID, noSupportID} {
		if assignableByUserID[userID].AssignmentDisabledReason == nil {
			t.Fatalf("expected %s to be disabled", userID)
		}
	}
	if assignableByUserID[eligibleID].AssignmentDisabledReason != nil {
		t.Fatal("inbox member should be selectable")
	}

	if err := svc.AssignConversationUser(ctx, workspaceID, conv.ID, &noSupportID, ownerID); err == nil {
		t.Fatal("expected user without support access to be rejected")
	}

	if err := svc.AssignConversationUser(ctx, workspaceID, conv.ID, &blockedID, ownerID); err == nil {
		t.Fatal("assignment allowed a member who cannot access the inbox")
	}

	if err := svc.AssignConversationUser(ctx, workspaceID, conv.ID, &eligibleID, ownerID); err != nil {
		t.Fatalf("assign eligible user: %v", err)
	}

	updated, err := convRepo.GetByID(ctx, workspaceID, conv.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if updated.AssignedUserID == nil || *updated.AssignedUserID != eligibleID {
		t.Fatalf("assigned_user_id = %#v, want %q", updated.AssignedUserID, eligibleID)
	}
	if updated.OpenedByUserID == nil || *updated.OpenedByUserID != ownerID {
		t.Fatalf("opened_by_user_id = %#v, want %q", updated.OpenedByUserID, ownerID)
	}

	_ = ownerMember
}

func TestSupportInboxServiceListContactConversationsReturnsUnpagedTotal(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	workspaceID := "ws-contact-conversation-total"
	contactID := "contact-conversation-total"
	userID := "owner-contact-total"
	seedWorkspace(t, db, workspaceID, "Contact Conversation Total", "contact-conversation-total", userID)
	seedWorkspaceMember(t, db, "member-contact-total", workspaceID, userID, "owner-contact-total@example.com", "Owner Contact Total", model.RoleOwner)

	repo := repository.NewSupportConversationRepository(db)
	svc := NewSupportInboxService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	ctx = authorization.WithActor(ctx, &authorization.Actor{
		UserID:            userID,
		WorkspaceID:       workspaceID,
		WorkspaceMemberID: "member-contact-total",
		Role:              model.RoleOwner,
	})
	now := time.Now().UTC()
	resolvedConversationID := ""

	for i := 0; i < 3; i++ {
		conversation := &model.SupportConversation{
			WorkspaceID:   workspaceID,
			Subject:       fmt.Sprintf("Linked conversation %d", i+1),
			Status:        model.SupportConversationStatusOpen,
			Priority:      "medium",
			Channel:       "widget",
			CRMContactID:  strPtr(contactID),
			CustomerEmail: strPtr(fmt.Sprintf("customer-%d@example.com", i+1)),
		}
		if err := repo.Create(ctx, conversation); err != nil {
			t.Fatalf("create linked conversation %d: %v", i+1, err)
		}
		if i == 1 {
			resolvedConversationID = conversation.ID
		}
		if err := db.Model(&model.SupportConversation{}).
			Where("id = ?", conversation.ID).
			Updates(map[string]any{
				"created_at": now.Add(-time.Duration(i) * time.Minute),
				"updated_at": now.Add(-time.Duration(i) * time.Minute),
			}).Error; err != nil {
			t.Fatalf("timestamp linked conversation %d: %v", i+1, err)
		}
	}

	conversations, total, err := svc.ListContactConversations(ctx, workspaceID, contactID, "", "", model.PMPagination{Page: 1, PerPage: 2})
	if err != nil {
		t.Fatalf("list contact conversations: %v", err)
	}
	if len(conversations) != 2 {
		t.Fatalf("len(conversations) = %d, want 2", len(conversations))
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}

	if err := db.Model(&model.SupportConversation{}).
		Where("id = ?", resolvedConversationID).
		Update("status", model.SupportConversationStatusResolved).Error; err != nil {
		t.Fatalf("resolve filtered conversation: %v", err)
	}
	filtered, filteredTotal, err := svc.ListContactConversations(ctx, workspaceID, contactID, "resolved", "conversation 2", model.PMPagination{Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("filter contact conversations: %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != resolvedConversationID || filteredTotal != 1 {
		t.Fatalf("filtered conversations = %#v, total = %d", filtered, filteredTotal)
	}
}

func TestSupportInboxServiceUpdateConversationStatus_KeepsResolvedEventsInternal(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	workspaceID := "ws-status-events"
	seedWorkspace(t, db, workspaceID, "Status Events", "status-events", "user-123")

	convRepo := repository.NewSupportConversationRepository(db)
	msgRepo := repository.NewSupportMessageRepository(db)
	svc := NewSupportInboxService(
		convRepo,
		nil,
		msgRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	conversation := &model.SupportConversation{
		WorkspaceID: workspaceID,
		Subject:     "Widget visibility",
		Status:      model.SupportConversationStatusOpen,
	}
	if err := convRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	if _, err := svc.UpdateConversationStatus(ctx, workspaceID, conversation.ID, model.SupportConversationStatusResolved, ""); err != nil {
		t.Fatalf("resolve conversation: %v", err)
	}
	if _, err := svc.UpdateConversationStatus(ctx, workspaceID, conversation.ID, model.SupportConversationStatusOpen, ""); err != nil {
		t.Fatalf("reopen conversation: %v", err)
	}

	allMessages, err := msgRepo.ListByConversation(ctx, workspaceID, conversation.ID, true)
	if err != nil {
		t.Fatalf("list all messages: %v", err)
	}
	if len(allMessages) != 2 {
		t.Fatalf("expected 2 system messages, got %d", len(allMessages))
	}

	wantEvents := []model.SupportSystemEventType{
		model.SystemEventResolved,
		model.SystemEventReopened,
	}
	for i, wantEvent := range wantEvents {
		msg := allMessages[i]
		if !msg.IsInternal {
			t.Fatalf("message %d is public; want internal system event", i)
		}
		if msg.MessageType != "system" {
			t.Fatalf("message %d type = %q, want system", i, msg.MessageType)
		}
		if msg.SystemEventType == nil || *msg.SystemEventType != wantEvent {
			t.Fatalf("message %d system_event_type = %v, want %q", i, msg.SystemEventType, wantEvent)
		}
	}

	publicMessages, err := msgRepo.ListByConversation(ctx, workspaceID, conversation.ID, false)
	if err != nil {
		t.Fatalf("list public messages: %v", err)
	}
	if len(publicMessages) != 0 {
		t.Fatalf("expected no widget-visible messages, got %d", len(publicMessages))
	}
}

func TestSupportInboxServiceUpdateConversationCustomerName(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	workspaceID := "ws-update-customer-name"
	seedWorkspace(t, db, workspaceID, "Update Customer Name WS", "update-customer-name", "user-123")

	convRepo := repository.NewSupportConversationRepository(db)
	svc := NewSupportInboxService(
		convRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	email := "casey@example.com"
	conversation := &model.SupportConversation{
		WorkspaceID:   workspaceID,
		Subject:       "Need help",
		Status:        model.SupportConversationStatusOpen,
		Priority:      "medium",
		Channel:       "email",
		Source:        "email",
		CustomerEmail: &email,
	}
	if err := convRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	updated, err := svc.UpdateConversationCustomerName(ctx, workspaceID, conversation.ID, "  Casey Newton  ", "user-123")
	if err != nil {
		t.Fatalf("update customer name: %v", err)
	}
	if updated.CustomerName == nil || *updated.CustomerName != "Casey Newton" {
		t.Fatalf("customer_name = %#v, want Casey Newton", updated.CustomerName)
	}

	reloaded, err := convRepo.GetByID(ctx, workspaceID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if reloaded.CustomerName == nil || *reloaded.CustomerName != "Casey Newton" {
		t.Fatalf("persisted customer_name = %#v, want Casey Newton", reloaded.CustomerName)
	}
}

func TestBuildSupportConversationStatusEventIncludesStatusPayload(t *testing.T) {
	flowState := model.SupportConversationFlowStateAssignedToHuman
	mailboxID := "mailbox-billing"
	updatedAt := time.Date(2026, 6, 4, 9, 30, 0, 0, time.UTC)
	conversation := &model.SupportConversation{
		ID:          "conv-reopen",
		WorkspaceID: "ws-reopen",
		MailboxID:   &mailboxID,
		Status:      model.SupportConversationStatusOpen,
		FlowState:   &flowState,
		UpdatedAt:   updatedAt,
	}

	event := buildSupportConversationStatusEvent(conversation, model.SupportConversationStatusResolved, "user-1")

	if event.Entity != "support_conversation" || event.Action != "updated" {
		t.Fatalf("event = %s/%s, want support_conversation/updated", event.Entity, event.Action)
	}
	var payload struct {
		OldStatus string  `json:"old_status"`
		Status    string  `json:"status"`
		FlowState *string `json:"flow_state"`
		UpdatedAt string  `json:"updated_at"`
		MailboxID *string `json:"mailbox_id"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		t.Fatalf("unmarshal event data: %v", err)
	}
	if payload.OldStatus != model.SupportConversationStatusResolved {
		t.Fatalf("old_status = %q, want resolved", payload.OldStatus)
	}
	if payload.Status != model.SupportConversationStatusOpen {
		t.Fatalf("status = %q, want open", payload.Status)
	}
	if payload.FlowState == nil || *payload.FlowState != flowState {
		t.Fatalf("flow_state = %v, want %q", payload.FlowState, flowState)
	}
	if payload.MailboxID == nil || *payload.MailboxID != mailboxID {
		t.Fatalf("mailbox_id = %v, want %q", payload.MailboxID, mailboxID)
	}
	if payload.UpdatedAt != updatedAt.Format(time.RFC3339) {
		t.Fatalf("updated_at = %q, want %q", payload.UpdatedAt, updatedAt.Format(time.RFC3339))
	}
}

func TestSupportInboxServiceCreateTaskFromConversation_CreatesLinkedTaskAndCopiesAssociations(t *testing.T) {
	env := newTaskTestEnv(t)
	ctx := context.Background()

	convRepo := repository.NewSupportConversationRepository(env.db)
	messageRepo := repository.NewSupportMessageRepository(env.db)
	assocRepo := repository.NewCRMAssociationRepository(env.db)

	svc := NewSupportInboxService(
		convRepo,
		nil,
		messageRepo,
		nil,
		assocRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		repository.NewUserRepository(env.db),
		nil,
		nil,
		nil,
	)
	svc.SetTaskService(env.svc)
	svc.SetSupportAIService(&SupportAIService{
		llmProvider: &scriptedSupportRewriteLLM{
			response: llm.ChatResponse{
				Content: `{"title":"Xero invoice import fails after sync attempt","summary":"Customer cannot sync invoices from Xero after the latest import attempt.","description_markdown":"## Problem\nXero invoice import fails after a sync attempt.\n\n## Impact\nThe customer cannot import invoices into the workspace.\n\n## Requested Outcome\nRestore invoice import for the affected account.\n\n## Reproduction\n- Start a Xero sync\n- Observe a 500 error when archived invoices are present","task_type":"bug","priority":"high"}`,
			},
		},
	})

	conversation := &model.SupportConversation{
		WorkspaceID:   env.wsID,
		Subject:       "Invoice sync is failing",
		Status:        "open",
		Priority:      model.PMTaskPriorityUrgent,
		CustomerName:  strPtr("Casey Customer"),
		CustomerEmail: strPtr("casey@example.com"),
		CRMContactID:  strPtr("contact-1"),
		CRMCompanyID:  strPtr("company-1"),
	}
	if err := convRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	customerName := "Casey Customer"
	for _, msg := range []model.SupportMessage{
		{
			WorkspaceID:       env.wsID,
			ConversationID:    conversation.ID,
			SenderType:        "customer",
			MessageType:       "reply",
			SenderDisplayName: &customerName,
			Content:           "After the latest sync attempt, invoice imports from Xero fail with a 500 error.",
		},
		{
			WorkspaceID:    env.wsID,
			ConversationID: conversation.ID,
			SenderType:     "user",
			MessageType:    "reply",
			SenderUserID:   &env.userID,
			Content:        "We can reproduce it when the account has archived invoices.",
			IsInternal:     true,
		},
	} {
		message := msg
		if err := messageRepo.Create(ctx, &message); err != nil {
			t.Fatalf("create message: %v", err)
		}
	}

	for _, assoc := range []model.CRMAssociation{
		{
			WorkspaceID:    env.wsID,
			FromObjectType: model.CRMObjectSupportConversation,
			FromObjectID:   conversation.ID,
			ToObjectType:   model.CRMObjectContact,
			ToObjectID:     "contact-1",
		},
		{
			WorkspaceID:    env.wsID,
			FromObjectType: model.CRMObjectSupportConversation,
			FromObjectID:   conversation.ID,
			ToObjectType:   model.CRMObjectDeal,
			ToObjectID:     "deal-1",
		},
	} {
		association := assoc
		if err := assocRepo.Create(ctx, &association); err != nil {
			t.Fatalf("create association: %v", err)
		}
	}

	resp, err := svc.CreateTaskFromConversation(ctx, env.wsID, conversation.ID, env.userID, model.CreateTaskFromConversationRequest{TeamID: &env.teamID})
	if err != nil {
		t.Fatalf("CreateTaskFromConversation: %v", err)
	}

	if resp.TaskID == "" {
		t.Fatal("expected created task id")
	}
	if resp.TaskName != "Xero invoice import fails after sync attempt" {
		t.Fatalf("task_name = %q, want %q", resp.TaskName, "Xero invoice import fails after sync attempt")
	}
	if resp.Summary == "" {
		t.Fatal("expected summary in response")
	}
	if resp.CopiedContactAssociations != 1 {
		t.Fatalf("copied_contact_associations = %d, want 1", resp.CopiedContactAssociations)
	}
	if resp.CopiedCompanyAssociations != 1 {
		t.Fatalf("copied_company_associations = %d, want 1", resp.CopiedCompanyAssociations)
	}
	messages, err := messageRepo.ListByConversation(ctx, env.wsID, conversation.ID, true)
	if err != nil {
		t.Fatalf("list messages after task creation: %v", err)
	}
	var taskEvent *model.SupportMessage
	for i := range messages {
		if messages[i].SystemEventType != nil && *messages[i].SystemEventType == model.SystemEventTaskCreated {
			taskEvent = &messages[i]
		}
	}
	if taskEvent == nil {
		t.Fatalf("expected task_created system event, got %#v", messages)
	}
	if !strings.Contains(taskEvent.Content, "created task #"+resp.TaskKey+": "+resp.TaskName) {
		t.Fatalf("task system message = %q", taskEvent.Content)
	}
	var taskEventMetadata map[string]string
	if err := json.Unmarshal([]byte(taskEvent.Metadata), &taskEventMetadata); err != nil {
		t.Fatalf("decode task system metadata: %v", err)
	}
	if taskEventMetadata["task_id"] != resp.TaskID {
		t.Fatalf("task system metadata task_id = %q, want %q", taskEventMetadata["task_id"], resp.TaskID)
	}
	if resp.CopiedDealAssociations != 1 {
		t.Fatalf("copied_deal_associations = %d, want 1", resp.CopiedDealAssociations)
	}

	updatedConversation, err := convRepo.GetByID(ctx, env.wsID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if updatedConversation == nil || updatedConversation.LinkedTaskID == nil || *updatedConversation.LinkedTaskID != resp.TaskID {
		t.Fatalf("linked_task_id = %v, want %q", updatedConversation.LinkedTaskID, resp.TaskID)
	}

	taskDetail, err := env.svc.GetByID(ctx, resp.TaskID)
	if err != nil {
		t.Fatalf("load task detail: %v", err)
	}
	if taskDetail == nil || taskDetail.Task.Description == nil {
		t.Fatal("expected task description to be stored")
	}
	if !strings.Contains(*taskDetail.Task.Description, "<h2") || !strings.Contains(*taskDetail.Task.Description, "Problem") {
		t.Fatalf("task description = %q, want rendered html with structured sections", *taskDetail.Task.Description)
	}

	taskAssociations, err := assocRepo.ListByObject(ctx, env.wsID, model.CRMObjectTask, resp.TaskID)
	if err != nil {
		t.Fatalf("list task associations: %v", err)
	}

	seen := map[string]string{}
	for _, assoc := range taskAssociations {
		otherType, otherID := supportAssociationPeer(assoc, model.CRMObjectTask, resp.TaskID)
		seen[otherType] = otherID
	}

	if seen[model.CRMObjectSupportConversation] != conversation.ID {
		t.Fatalf("support conversation association = %q, want %q", seen[model.CRMObjectSupportConversation], conversation.ID)
	}
	if seen[model.CRMObjectContact] != "contact-1" {
		t.Fatalf("contact association = %q, want %q", seen[model.CRMObjectContact], "contact-1")
	}
	if seen[model.CRMObjectCompany] != "company-1" {
		t.Fatalf("company association = %q, want %q", seen[model.CRMObjectCompany], "company-1")
	}
	if seen[model.CRMObjectDeal] != "deal-1" {
		t.Fatalf("deal association = %q, want %q", seen[model.CRMObjectDeal], "deal-1")
	}
}

func TestSupportInboxServiceCreateTaskFromConversation_NormalizesGenericActionTitle(t *testing.T) {
	env := newTaskTestEnv(t)
	ctx := context.Background()

	convRepo := repository.NewSupportConversationRepository(env.db)
	messageRepo := repository.NewSupportMessageRepository(env.db)

	svc := NewSupportInboxService(
		convRepo,
		nil,
		messageRepo,
		nil,
		repository.NewCRMAssociationRepository(env.db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		repository.NewUserRepository(env.db),
		nil,
		nil,
		nil,
	)
	svc.SetTaskService(env.svc)
	svc.SetSupportAIService(&SupportAIService{
		llmProvider: &scriptedSupportRewriteLLM{
			response: llm.ChatResponse{
				Content: `{"title":"Investigate invoice sync failure for archived invoices","summary":"Archived invoices cause Xero sync imports to fail for the customer.","description_markdown":"## Problem\nInvoice sync fails when archived invoices are present.\n\n## Impact\nThe customer cannot import invoices.\n\n## Requested Outcome\nRestore invoice import when archived invoices exist.","task_type":"bug","priority":"high"}`,
			},
		},
	})

	conversation := &model.SupportConversation{
		WorkspaceID:   env.wsID,
		Subject:       "Re: Invoice sync failure for archived invoices",
		Status:        "open",
		Priority:      model.PMTaskPriorityHigh,
		CustomerName:  strPtr("Casey Customer"),
		CustomerEmail: strPtr("casey@example.com"),
	}
	if err := convRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	customerName := "Casey Customer"
	message := model.SupportMessage{
		WorkspaceID:       env.wsID,
		ConversationID:    conversation.ID,
		SenderType:        "customer",
		MessageType:       "reply",
		SenderDisplayName: &customerName,
		Content:           "Invoice sync fails when archived invoices are included in the Xero import.",
	}
	if err := messageRepo.Create(ctx, &message); err != nil {
		t.Fatalf("create message: %v", err)
	}

	resp, err := svc.CreateTaskFromConversation(ctx, env.wsID, conversation.ID, env.userID, model.CreateTaskFromConversationRequest{TeamID: &env.teamID})
	if err != nil {
		t.Fatalf("CreateTaskFromConversation: %v", err)
	}

	if resp.TaskName != "Invoice sync failure for archived invoices" {
		t.Fatalf("task_name = %q, want %q", resp.TaskName, "Invoice sync failure for archived invoices")
	}

	taskDetail, err := env.svc.GetByID(ctx, resp.TaskID)
	if err != nil {
		t.Fatalf("load task detail: %v", err)
	}
	if taskDetail == nil || taskDetail.Task.Description == nil {
		t.Fatal("expected task description to be stored")
	}
	if !strings.Contains(*taskDetail.Task.Description, "Requested Outcome") {
		t.Fatalf("task description = %q, want structured html", *taskDetail.Task.Description)
	}
}

// Reproduces conversation CON-136 (ticket af8f06af-…) where the LLM returned
// valid JSON but left `title` and `description_markdown` empty. Current code
// silently overwrites both with the deterministic fallback, so the task ends
// up with the raw error message as its name, "thanks in advance…" as its
// Impact, and the canned Requested Outcome sentence. The LLM is working — the
// service just swallows partial responses without logging. This test will
// fail until the fallback/observability is fixed.
func TestSupportInboxServiceCreateTaskFromConversation_DoesNotFallBackSilentlyWhenLLMReturnsEmptyFields(t *testing.T) {
	env := newTaskTestEnv(t)
	ctx := context.Background()

	convRepo := repository.NewSupportConversationRepository(env.db)
	messageRepo := repository.NewSupportMessageRepository(env.db)

	svc := NewSupportInboxService(
		convRepo,
		nil,
		messageRepo,
		nil,
		repository.NewCRMAssociationRepository(env.db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		repository.NewUserRepository(env.db),
		nil,
		nil,
		nil,
	)
	svc.SetTaskService(env.svc)
	// Simulate the production failure mode: LLM responds with schema-valid JSON
	// but empty title + empty description_markdown.
	svc.SetSupportAIService(&SupportAIService{
		llmProvider: &scriptedSupportRewriteLLM{
			response: llm.ChatResponse{
				Content: `{"title":"","summary":"SERP Analyzer token is rejected during content generation.","description_markdown":"","task_type":"bug","priority":"medium"}`,
			},
		},
	})

	rawErrorSubject := "Error: Content generation failed: SERP analysis failed: SERP Analyzer failed: Token is not valid; SERP Knowledge failed: Token is not valid"
	conversation := &model.SupportConversation{
		WorkspaceID:   env.wsID,
		Subject:       rawErrorSubject,
		Status:        "open",
		Priority:      model.PMTaskPriorityMedium,
		CustomerName:  strPtr("Fiorenzo Minnelli"),
		CustomerEmail: strPtr("minnellif@example.com"),
	}
	if err := convRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	customerName := "Fiorenzo Minnelli"
	agentName := "Support Agent"
	seedMessages := []model.SupportMessage{
		{
			WorkspaceID:       env.wsID,
			ConversationID:    conversation.ID,
			SenderType:        "customer",
			MessageType:       "reply",
			SenderDisplayName: &customerName,
			Content:           rawErrorSubject + " I 'm facing for the second time with this error",
		},
		{
			WorkspaceID:       env.wsID,
			ConversationID:    conversation.ID,
			SenderType:        "agent",
			MessageType:       "reply",
			SenderDisplayName: &agentName,
			Content:           "Let me connect you with a team member who can help further.",
		},
		{
			WorkspaceID:       env.wsID,
			ConversationID:    conversation.ID,
			SenderType:        "customer",
			MessageType:       "reply",
			SenderDisplayName: &customerName,
			Content:           "thanks in advance i tried twice and same error",
		},
	}
	for i := range seedMessages {
		if err := messageRepo.Create(ctx, &seedMessages[i]); err != nil {
			t.Fatalf("create message %d: %v", i, err)
		}
	}

	resp, err := svc.CreateTaskFromConversation(ctx, env.wsID, conversation.ID, env.userID, model.CreateTaskFromConversationRequest{TeamID: &env.teamID})
	if err != nil {
		t.Fatalf("CreateTaskFromConversation: %v", err)
	}

	// Title must not be the raw multi-line error string. An 80-char-plus
	// stack-trace-like title is never a useful PM task name.
	if resp.TaskName == rawErrorSubject {
		t.Errorf("task_name fell back to raw error subject verbatim: %q", resp.TaskName)
	}
	if len(resp.TaskName) > 120 {
		t.Errorf("task_name is %d chars, want <=120: %q", len(resp.TaskName), resp.TaskName)
	}
	if strings.Contains(resp.TaskName, "SERP Analyzer failed: Token is not valid; SERP Knowledge failed") {
		t.Errorf("task_name still contains the full concatenated error chain: %q", resp.TaskName)
	}

	taskDetail, err := env.svc.GetByID(ctx, resp.TaskID)
	if err != nil {
		t.Fatalf("load task detail: %v", err)
	}
	if taskDetail == nil || taskDetail.Task.Description == nil {
		t.Fatal("expected task description to be stored")
	}
	desc := *taskDetail.Task.Description

	// Impact should not lift the customer's sign-off verbatim. The phrase
	// may legitimately appear in the Conversation Notes transcript dump at
	// the bottom, so scope the check to just the Impact section.
	if impact := extractSupportDescriptionSection(desc, "impact"); strings.Contains(strings.ToLower(impact), "thanks in advance") {
		t.Errorf("Impact section picked up the customer sign-off: %q", impact)
	}

	// Requested Outcome should not be the boilerplate default when the LLM
	// gave us no real outcome. Either drop the section or derive something
	// specific — never emit the canned filler sentence.
	boilerplateOutcome := "Determine the next internal product or support action needed to resolve the customer issue."
	if strings.Contains(desc, boilerplateOutcome) {
		t.Errorf("description contains boilerplate Requested Outcome filler: %q", desc)
	}
}

func TestSupportInboxServiceCreateTaskFromConversation_UsesStructuredLLMOutput(t *testing.T) {
	env := newTaskTestEnv(t)
	ctx := context.Background()

	convRepo := repository.NewSupportConversationRepository(env.db)
	messageRepo := repository.NewSupportMessageRepository(env.db)

	svc := NewSupportInboxService(
		convRepo,
		nil,
		messageRepo,
		nil,
		repository.NewCRMAssociationRepository(env.db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		repository.NewUserRepository(env.db),
		nil,
		nil,
		nil,
	)
	svc.SetTaskService(env.svc)

	fake := &recordingSupportTaskDraftProvider{
		response: `{
			"title":"SERP Analyzer token rejected during content generation",
			"summary":"The SERP Analyzer integration rejects the stored token, blocking content generation end-to-end.",
			"description_markdown":"## Problem\nSERP token is rejected.\n\n## Impact\nCustomer cannot generate content.\n",
			"task_type":"bug",
			"priority":"high"
		}`,
	}
	aiSvc := &SupportAIService{llmProvider: fake}
	svc.SetSupportAIService(aiSvc)

	conversation := &model.SupportConversation{
		WorkspaceID:   env.wsID,
		Subject:       "Error: SERP analysis failed",
		Status:        "open",
		Priority:      model.PMTaskPriorityMedium,
		CustomerName:  strPtr("Fiorenzo Minnelli"),
		CustomerEmail: strPtr("minnellif@example.com"),
	}
	if err := convRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	customerName := "Fiorenzo Minnelli"
	msg := model.SupportMessage{
		WorkspaceID:       env.wsID,
		ConversationID:    conversation.ID,
		SenderType:        "customer",
		MessageType:       "reply",
		SenderDisplayName: &customerName,
		Content:           "SERP Analyzer keeps returning Token is not valid on every generation attempt.",
	}
	if err := messageRepo.Create(ctx, &msg); err != nil {
		t.Fatalf("create message: %v", err)
	}

	resp, err := svc.CreateTaskFromConversation(ctx, env.wsID, conversation.ID, env.userID, model.CreateTaskFromConversationRequest{TeamID: &env.teamID})
	if err != nil {
		t.Fatalf("CreateTaskFromConversation: %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("expected LLM provider to be called exactly once, got %d", fake.calls)
	}
	if fake.request.Model == "" {
		t.Error("expected request to include a resolved model name")
	}
	if !fake.request.JSONMode || !fake.request.JSONSchemaStrict {
		t.Fatalf("expected strict structured output request, got %#v", fake.request)
	}
	properties, ok := fake.request.JSONSchema["properties"].(map[string]any)
	if !ok || properties["description_markdown"] == nil {
		t.Fatalf("expected task draft response schema, got %#v", fake.request.JSONSchema)
	}
	if resp.TaskName != "SERP Analyzer token rejected during content generation" {
		t.Errorf("task_name = %q, want the injected draft title", resp.TaskName)
	}
}

type recordingSupportTaskDraftProvider struct {
	response string
	err      error
	calls    int
	request  llm.ChatRequest
}

func (f *recordingSupportTaskDraftProvider) ChatCompletion(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	f.calls++
	f.request = req
	if f.err != nil {
		return nil, f.err
	}
	return &llm.ChatResponse{Content: f.response}, nil
}

func TestSupportInboxServiceCreateTaskFromConversation_FailsWhenContextIsTooWeak(t *testing.T) {
	env := newTaskTestEnv(t)
	ctx := context.Background()

	convRepo := repository.NewSupportConversationRepository(env.db)
	messageRepo := repository.NewSupportMessageRepository(env.db)

	svc := NewSupportInboxService(
		convRepo,
		nil,
		messageRepo,
		nil,
		repository.NewCRMAssociationRepository(env.db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		repository.NewUserRepository(env.db),
		nil,
		nil,
		nil,
	)
	svc.SetTaskService(env.svc)

	conversation := &model.SupportConversation{
		WorkspaceID:   env.wsID,
		Subject:       "New convo",
		Status:        "open",
		Priority:      model.PMTaskPriorityMedium,
		CustomerName:  strPtr("Untidy"),
		CustomerEmail: strPtr("support@untidy.com"),
	}
	if err := convRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	customerName := "Untidy"
	message := model.SupportMessage{
		WorkspaceID:       env.wsID,
		ConversationID:    conversation.ID,
		SenderType:        "customer",
		MessageType:       "reply",
		SenderDisplayName: &customerName,
		Content:           "what about me",
	}
	if err := messageRepo.Create(ctx, &message); err != nil {
		t.Fatalf("create message: %v", err)
	}

	resp, err := svc.CreateTaskFromConversation(ctx, env.wsID, conversation.ID, env.userID, model.CreateTaskFromConversationRequest{TeamID: &env.teamID})
	if err == nil {
		t.Fatalf("expected error, got response %#v", resp)
	}
	if !errors.Is(err, ErrSupportTaskInsufficientContext) {
		t.Fatalf("error = %v, want ErrSupportTaskInsufficientContext", err)
	}

	var taskCount int64
	if err := env.db.WithContext(ctx).Model(&model.PMTask{}).Count(&taskCount).Error; err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if taskCount != 0 {
		t.Fatalf("task_count = %d, want 0", taskCount)
	}

	updatedConversation, err := convRepo.GetByID(ctx, env.wsID, conversation.ID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("reload conversation: %v", err)
	}
	if updatedConversation == nil {
		t.Fatal("expected conversation to exist")
	}
	if updatedConversation.LinkedTaskID != nil {
		t.Fatalf("linked_task_id = %v, want nil", updatedConversation.LinkedTaskID)
	}
}

func TestSupportInboxServiceCreateTaskFromConversation_UsesInternalNotesAsFallbackContext(t *testing.T) {
	env := newTaskTestEnv(t)
	ctx := context.Background()

	convRepo := repository.NewSupportConversationRepository(env.db)
	messageRepo := repository.NewSupportMessageRepository(env.db)

	svc := NewSupportInboxService(
		convRepo,
		nil,
		messageRepo,
		nil,
		repository.NewCRMAssociationRepository(env.db),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		repository.NewUserRepository(env.db),
		nil,
		nil,
		nil,
	)
	svc.SetTaskService(env.svc)

	conversation := &model.SupportConversation{
		WorkspaceID:   env.wsID,
		Subject:       "New convo",
		Status:        "open",
		Priority:      model.PMTaskPriorityMedium,
		CustomerName:  strPtr("Untidy"),
		CustomerEmail: strPtr("support@untidy.com"),
	}
	if err := convRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	customerName := "Untidy"
	for _, msg := range []model.SupportMessage{
		{
			WorkspaceID:       env.wsID,
			ConversationID:    conversation.ID,
			SenderType:        "customer",
			MessageType:       "reply",
			SenderDisplayName: &customerName,
			Content:           "what about me",
		},
		{
			WorkspaceID:    env.wsID,
			ConversationID: conversation.ID,
			SenderType:     "user",
			MessageType:    "reply",
			SenderUserID:   &env.userID,
			IsInternal:     true,
			Content:        "The customer is having an issue with incorrect attribution. Google Analytics shows 200 conversions for March 15-17 while Usermaven shows 150, which is throwing off paid ads attribution.",
		},
	} {
		message := msg
		if err := messageRepo.Create(ctx, &message); err != nil {
			t.Fatalf("create message: %v", err)
		}
	}

	resp, err := svc.CreateTaskFromConversation(ctx, env.wsID, conversation.ID, env.userID, model.CreateTaskFromConversationRequest{TeamID: &env.teamID})
	if err != nil {
		t.Fatalf("CreateTaskFromConversation: %v", err)
	}

	if resp.TaskID == "" {
		t.Fatal("expected created task id")
	}
	if strings.EqualFold(resp.TaskName, "New convo") || strings.EqualFold(resp.TaskName, "What about me") {
		t.Fatalf("task_name = %q, want internal-note-derived issue title", resp.TaskName)
	}
	if !strings.Contains(strings.ToLower(resp.TaskName), "incorrect attribution") {
		t.Fatalf("task_name = %q, want internal note context", resp.TaskName)
	}
	if !strings.Contains(strings.ToLower(resp.Summary), "google analytics") {
		t.Fatalf("summary = %q, want internal note context", resp.Summary)
	}
}

// ---------------------------------------------------------------------------
// Widget Installation Repository Tests
// ---------------------------------------------------------------------------

func TestWidgetInstallationRepository(t *testing.T) {
	db := newTestDB(t)

	workspaceID := "ws-widget-test"
	seedWorkspace(t, db, workspaceID, "Widget Test WS", "widget-test-ws", "user-123")

	repo := repository.NewSupportInboxInstallationRepository(db)
	ctx := context.Background()

	t.Run("Create and GetByWorkspace", func(t *testing.T) {
		inst := &model.SupportWidgetInstallation{
			WorkspaceID: workspaceID,
			WidgetKey:   "wk_test123",
			SecretKey:   "sk_secret456",
			Active:      true,
		}
		if err := repo.Create(ctx, inst); err != nil {
			t.Fatalf("create: %v", err)
		}

		fetched, err := repo.GetByWorkspace(ctx, workspaceID)
		if err != nil {
			t.Fatalf("get by workspace: %v", err)
		}
		if fetched == nil {
			t.Fatal("expected installation to be found")
		}
		if fetched.WidgetKey != "wk_test123" {
			t.Errorf("expected widget_key 'wk_test123', got %q", fetched.WidgetKey)
		}
	})

	t.Run("GetByWidgetKey", func(t *testing.T) {
		fetched, err := repo.GetByWidgetKey(ctx, "wk_test123")
		if err != nil {
			t.Fatalf("get by widget key: %v", err)
		}
		if fetched == nil {
			t.Fatal("expected installation to be found by widget key")
		}
		if fetched.WorkspaceID != workspaceID {
			t.Errorf("expected workspace %q, got %q", workspaceID, fetched.WorkspaceID)
		}
	})

	t.Run("GetByWidgetKey returns nil for inactive", func(t *testing.T) {
		inactiveWS := "ws-inactive-widget"
		seedWorkspace(t, db, inactiveWS, "Inactive WS", "inactive-ws", "user-123")

		inst := &model.SupportWidgetInstallation{
			WorkspaceID: inactiveWS,
			WidgetKey:   "wk_inactive",
			SecretKey:   "sk_inactive",
			Active:      true,
		}
		if err := repo.Create(ctx, inst); err != nil {
			t.Fatalf("create: %v", err)
		}

		// Deactivate via update
		inst.Active = false
		if err := repo.Update(ctx, inst); err != nil {
			t.Fatalf("deactivate: %v", err)
		}

		fetched, err := repo.GetByWidgetKey(ctx, "wk_inactive")
		if err != nil {
			t.Fatalf("get inactive: %v", err)
		}
		if fetched != nil {
			t.Error("expected nil for inactive widget installation")
		}
	})

	t.Run("GetByWidgetKey returns nil for unknown key", func(t *testing.T) {
		fetched, err := repo.GetByWidgetKey(ctx, "wk_nonexistent")
		if err != nil {
			t.Fatalf("get unknown: %v", err)
		}
		if fetched != nil {
			t.Error("expected nil for unknown widget key")
		}
	})

	t.Run("Update installation", func(t *testing.T) {
		fetched, _ := repo.GetByWorkspace(ctx, workspaceID)
		fetched.Active = false
		if err := repo.Update(ctx, fetched); err != nil {
			t.Fatalf("update: %v", err)
		}

		refetched, _ := repo.GetByWorkspace(ctx, workspaceID)
		if refetched.Active {
			t.Error("expected active to be false after update")
		}

		// Restore for other tests
		refetched.Active = true
		_ = repo.Update(ctx, refetched)
	})
}

// ---------------------------------------------------------------------------
// Widget Session Repository Tests
// ---------------------------------------------------------------------------

func TestWidgetSessionRepository(t *testing.T) {
	db := newTestDB(t)

	workspaceID := "ws-session-test"
	seedWorkspace(t, db, workspaceID, "Session Test WS", "session-test-ws", "user-123")

	repo := repository.NewSupportInboxSessionRepository(db)
	ctx := context.Background()

	t.Run("Create and GetByToken", func(t *testing.T) {
		session := &model.SupportWidgetSession{
			WorkspaceID:   workspaceID,
			SessionToken:  "token_abc123",
			CustomerName:  strPtr("Jane Doe"),
			CustomerEmail: strPtr("jane@example.com"),
			ExpiresAt:     time.Now().Add(24 * time.Hour),
		}
		if err := repo.Create(ctx, session); err != nil {
			t.Fatalf("create session: %v", err)
		}
		if session.ID == "" {
			t.Error("expected session ID to be set")
		}

		fetched, err := repo.GetByToken(ctx, "token_abc123")
		if err != nil {
			t.Fatalf("get by token: %v", err)
		}
		if fetched == nil {
			t.Fatal("expected session to be found")
		}
		if *fetched.CustomerEmail != "jane@example.com" {
			t.Errorf("expected email 'jane@example.com', got %q", *fetched.CustomerEmail)
		}
	})

	t.Run("GetByToken returns nil for unknown token", func(t *testing.T) {
		fetched, err := repo.GetByToken(ctx, "token_nonexistent")
		if err != nil {
			t.Fatalf("get unknown: %v", err)
		}
		if fetched != nil {
			t.Error("expected nil for unknown session token")
		}
	})

	t.Run("Update session with conversation_id", func(t *testing.T) {
		session := &model.SupportWidgetSession{
			WorkspaceID:  workspaceID,
			SessionToken: "token_update_test",
			ExpiresAt:    time.Now().Add(24 * time.Hour),
		}
		if err := repo.Create(ctx, session); err != nil {
			t.Fatalf("create: %v", err)
		}

		convID := "conv-id-789"
		session.ConversationID = &convID
		if err := repo.Update(ctx, session); err != nil {
			t.Fatalf("update: %v", err)
		}

		fetched, _ := repo.GetByToken(ctx, "token_update_test")
		if fetched.ConversationID == nil || *fetched.ConversationID != convID {
			t.Error("expected conversation_id to be set after update")
		}
	})

	t.Run("GetLatestActivityByAnonymousID uses last_active_at across sessions", func(t *testing.T) {
		older := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
		newer := older.Add(45 * time.Minute)
		for _, session := range []*model.SupportWidgetSession{
			{
				WorkspaceID:  workspaceID,
				SessionToken: "token_activity_old",
				AnonymousID:  "anon-activity",
				LastActiveAt: &older,
				ExpiresAt:    time.Now().Add(24 * time.Hour),
			},
			{
				WorkspaceID:  workspaceID,
				SessionToken: "token_activity_new",
				AnonymousID:  "anon-activity",
				LastActiveAt: &newer,
				ExpiresAt:    time.Now().Add(24 * time.Hour),
			},
		} {
			if err := repo.Create(ctx, session); err != nil {
				t.Fatalf("create activity session: %v", err)
			}
		}

		got, err := repo.GetLatestActivityByAnonymousID(ctx, workspaceID, "anon-activity")
		if err != nil {
			t.Fatalf("get latest activity by anonymous id: %v", err)
		}
		if got == nil || !got.Equal(newer) {
			t.Fatalf("latest activity = %v, want %v", got, newer)
		}
	})
}

func TestSupportInboxServiceVisitorContextLastActivity(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	workspaceID := "ws-visitor-activity"
	seedWorkspace(t, db, workspaceID, "Visitor Activity WS", "visitor-activity-ws", "user-123")

	conversationRepo := repository.NewSupportConversationRepository(db)
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	svc := NewSupportInboxService(
		conversationRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		sessionRepo,
		nil,
		nil,
		nil,
		contactRepo,
		nil,
		nil,
		nil,
		nil,
	)

	contactID := "contact-visitor-activity"
	if err := db.Exec(
		`INSERT INTO crm_contacts (id, workspace_id, display_id, first_name, email, lifecycle_stage, lead_status, custom_properties, email_status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		contactID,
		workspaceID,
		"CON-activity",
		"Ada",
		"ada@example.com",
		"lead",
		"new",
		"{}",
		"valid",
		time.Now(),
		time.Now(),
	).Error; err != nil {
		t.Fatalf("insert contact: %v", err)
	}

	selected := &model.SupportConversation{
		WorkspaceID:  workspaceID,
		Subject:      "Selected conversation",
		Status:       model.SupportConversationStatusOpen,
		Priority:     "medium",
		Channel:      "widget",
		AnonymousID:  strPtr("anon-selected-activity"),
		CRMContactID: &contactID,
	}
	other := &model.SupportConversation{
		WorkspaceID:  workspaceID,
		Subject:      "Other contact conversation",
		Status:       model.SupportConversationStatusOpen,
		Priority:     "medium",
		Channel:      "widget",
		AnonymousID:  strPtr("anon-other-activity"),
		CRMContactID: &contactID,
	}
	if err := conversationRepo.Create(ctx, selected); err != nil {
		t.Fatalf("create selected conversation: %v", err)
	}
	if err := conversationRepo.Create(ctx, other); err != nil {
		t.Fatalf("create other conversation: %v", err)
	}

	selectedActivity := time.Date(2026, 6, 16, 9, 30, 0, 0, time.UTC)
	contactActivity := selectedActivity.Add(2 * time.Hour)
	for _, session := range []*model.SupportWidgetSession{
		{
			WorkspaceID:    workspaceID,
			ConversationID: &selected.ID,
			SessionToken:   "token_selected_activity",
			AnonymousID:    "anon-selected-activity",
			LastActiveAt:   &selectedActivity,
			ExpiresAt:      time.Now().Add(24 * time.Hour),
		},
		{
			WorkspaceID:    workspaceID,
			ConversationID: &other.ID,
			SessionToken:   "token_contact_activity",
			AnonymousID:    "anon-other-activity",
			LastActiveAt:   &contactActivity,
			ExpiresAt:      time.Now().Add(24 * time.Hour),
		},
	} {
		if err := sessionRepo.Create(ctx, session); err != nil {
			t.Fatalf("create widget session: %v", err)
		}
	}

	resp, err := svc.GetVisitorContext(ctx, workspaceID, selected.ID)
	if err != nil {
		t.Fatalf("get visitor context: %v", err)
	}
	if resp.LastActiveAt == nil {
		t.Fatal("expected last_active_at")
	}
	if *resp.LastActiveAt != contactActivity.Format(time.RFC3339) {
		t.Fatalf("last_active_at = %q, want %q", *resp.LastActiveAt, contactActivity.Format(time.RFC3339))
	}
	if resp.LastActiveSource == nil || *resp.LastActiveSource != "crm_contact" {
		t.Fatalf("last_active_source = %v, want crm_contact", resp.LastActiveSource)
	}
}

func TestSupportInboxServiceUpdateConversationCRMCompany(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	const workspaceID = "ws-update-conversation-company"
	seedWorkspace(t, db, workspaceID, "Update Conversation Company", "update-conversation-company", "user-123")
	conversationRepo := repository.NewSupportConversationRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	companyRepo := repository.NewCRMCompanyRepository(db)
	assocRepo := repository.NewCRMAssociationRepository(db)
	contact := &model.CRMContact{WorkspaceID: workspaceID, DisplayID: "CON-1", FirstName: "Ada", LifecycleStage: model.CRMLifecycleLead, LeadStatus: model.CRMLeadStatusNew}
	if err := contactRepo.Create(ctx, contact); err != nil {
		t.Fatalf("create contact: %v", err)
	}
	company := &model.CRMCompany{WorkspaceID: workspaceID, DisplayID: "COM-1", Name: "Acme"}
	if err := companyRepo.Create(ctx, company); err != nil {
		t.Fatalf("create company: %v", err)
	}
	conversation := &model.SupportConversation{WorkspaceID: workspaceID, Subject: "Company correction", Status: model.SupportConversationStatusOpen, CRMContactID: &contact.ID}
	if err := conversationRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	svc := NewSupportInboxService(conversationRepo, nil, nil, nil, assocRepo, nil, nil, nil, nil, nil, contactRepo, nil, nil, nil, nil)
	svc.SetCRMCompanyRepository(companyRepo)

	updated, err := svc.UpdateConversationCRMCompany(ctx, workspaceID, conversation.ID, &company.ID, "user-123")
	if err != nil {
		t.Fatalf("UpdateConversationCRMCompany: %v", err)
	}
	if updated.CRMCompanyID == nil || *updated.CRMCompanyID != company.ID {
		t.Fatalf("crm_company_id = %v, want %q", updated.CRMCompanyID, company.ID)
	}
	assocs, err := assocRepo.ListByObject(ctx, workspaceID, model.CRMObjectContact, contact.ID)
	if err != nil {
		t.Fatalf("list contact associations: %v", err)
	}
	if len(assocs) != 1 {
		t.Fatalf("contact company memberships = %d, want 1", len(assocs))
	}
	if !isPrimaryCompanyAssociationLabel(assocs[0].AssociationLabel) {
		t.Fatalf("first company membership label = %v, want primary", assocs[0].AssociationLabel)
	}

	secondCompany := &model.CRMCompany{WorkspaceID: workspaceID, DisplayID: "COM-2", Name: "Second Account"}
	if err := companyRepo.Create(ctx, secondCompany); err != nil {
		t.Fatalf("create second company: %v", err)
	}
	if _, err := svc.UpdateConversationCRMCompany(ctx, workspaceID, conversation.ID, &secondCompany.ID, "user-123"); err != nil {
		t.Fatalf("set second conversation company: %v", err)
	}
	assocs, err = assocRepo.ListByObject(ctx, workspaceID, model.CRMObjectContact, contact.ID)
	if err != nil {
		t.Fatalf("list contact associations after switch: %v", err)
	}
	if len(assocs) != 2 {
		t.Fatalf("contact company memberships after switch = %d, want 2", len(assocs))
	}
	primaryID := ""
	for _, assoc := range assocs {
		_, associatedCompanyID := otherAssociationSide(assoc, model.CRMObjectContact, contact.ID)
		if isPrimaryCompanyAssociationLabel(assoc.AssociationLabel) {
			primaryID = associatedCompanyID
		}
	}
	if primaryID != company.ID {
		t.Fatalf("primary company after manual switch = %q, want original %q", primaryID, company.ID)
	}

	const otherWorkspaceID = "ws-update-conversation-company-other"
	seedWorkspace(t, db, otherWorkspaceID, "Other Company Workspace", "update-conversation-company-other", "user-123")
	foreignCompany := &model.CRMCompany{WorkspaceID: otherWorkspaceID, DisplayID: "COM-1", Name: "Foreign Company"}
	if err := companyRepo.Create(ctx, foreignCompany); err != nil {
		t.Fatalf("create foreign company: %v", err)
	}
	if _, err := svc.UpdateConversationCRMCompany(ctx, workspaceID, conversation.ID, &foreignCompany.ID, "user-123"); err == nil {
		t.Fatal("expected company from another workspace to be rejected")
	}

	cleared, err := svc.UpdateConversationCRMCompany(ctx, workspaceID, conversation.ID, nil, "user-123")
	if err != nil {
		t.Fatalf("clear conversation company: %v", err)
	}
	if cleared.CRMCompanyID != nil {
		t.Fatalf("cleared crm_company_id = %v, want nil", cleared.CRMCompanyID)
	}
}

func TestSupportInboxServiceVisitorContextIncludesLiveCompany(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	const workspaceID = "ws-visitor-company"
	seedWorkspace(t, db, workspaceID, "Visitor Company", "visitor-company", "user-123")
	conversationRepo := repository.NewSupportConversationRepository(db)
	sessionRepo := repository.NewSupportInboxSessionRepository(db)
	contactRepo := repository.NewCRMContactRepository(db)
	companyRepo := repository.NewCRMCompanyRepository(db)
	assocRepo := repository.NewCRMAssociationRepository(db)
	contact := &model.CRMContact{
		WorkspaceID: workspaceID, DisplayID: "CON-1", FirstName: "Ada",
		LifecycleStage: model.CRMLifecycleCustomer, LeadStatus: model.CRMLeadStatusOpen,
		CustomProperties: model.JSONB{"score": float64(92), "vip": true},
	}
	if err := contactRepo.Create(ctx, contact); err != nil {
		t.Fatalf("create contact: %v", err)
	}
	company := &model.CRMCompany{
		WorkspaceID: workspaceID, DisplayID: "COM-1", Name: "Acme",
		CustomProperties: model.JSONB{"plan": "enterprise", "seats_used": float64(12), "priority_support": true},
	}
	if err := companyRepo.Create(ctx, company); err != nil {
		t.Fatalf("create company: %v", err)
	}
	if err := assocRepo.Create(ctx, &model.CRMAssociation{WorkspaceID: workspaceID, FromObjectType: model.CRMObjectContact, FromObjectID: contact.ID, ToObjectType: model.CRMObjectCompany, ToObjectID: company.ID}); err != nil {
		t.Fatalf("create association: %v", err)
	}
	conversation := &model.SupportConversation{WorkspaceID: workspaceID, Subject: "Live company", Status: model.SupportConversationStatusOpen, CRMContactID: &contact.ID, CRMCompanyID: &company.ID}
	if err := conversationRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	svc := NewSupportInboxService(conversationRepo, nil, nil, nil, assocRepo, nil, sessionRepo, nil, nil, nil, contactRepo, nil, nil, nil, nil)
	svc.SetCRMCompanyRepository(companyRepo)

	resp, err := svc.GetVisitorContext(ctx, workspaceID, conversation.ID)
	if err != nil {
		t.Fatalf("GetVisitorContext: %v", err)
	}
	if resp.CompanyContextStatus != model.VisitorCompanyContextOK || resp.Company == nil {
		t.Fatalf("company context = %q / %#v, want ok company", resp.CompanyContextStatus, resp.Company)
	}
	if resp.Company.ID != company.ID || resp.Company.CustomProperties["seats_used"] != float64(12) {
		t.Fatalf("company = %#v, want live typed properties", resp.Company)
	}
	if len(resp.CompanyOptions) != 1 || resp.CompanyOptions[0].ID != company.ID {
		t.Fatalf("company options = %#v, want selected membership", resp.CompanyOptions)
	}
	if resp.Contact == nil || resp.Contact.CustomProperties["score"] != float64(92) || resp.Contact.CustomProperties["vip"] != true {
		t.Fatalf("contact custom properties = %#v, want typed scalars", resp.Contact)
	}

	company.CustomProperties["plan"] = "growth"
	if err := companyRepo.Update(ctx, company); err != nil {
		t.Fatalf("update company: %v", err)
	}
	resp, err = svc.GetVisitorContext(ctx, workspaceID, conversation.ID)
	if err != nil {
		t.Fatalf("GetVisitorContext after update: %v", err)
	}
	if resp.Company.CustomProperties["plan"] != "growth" {
		t.Fatalf("company plan = %#v, want current growth", resp.Company.CustomProperties["plan"])
	}
}

// ---------------------------------------------------------------------------
// Canned Response Repository Tests
// ---------------------------------------------------------------------------

func TestSupportCannedResponseRepository(t *testing.T) {
	db := newTestDB(t)

	workspaceID := "ws-test-canned-search"
	seedWorkspace(t, db, workspaceID, "Test Workspace Search", "test-ws-canned-search", "user-123")

	repo := repository.NewSupportCannedResponseRepository(db)

	t.Run("Create and List canned responses", func(t *testing.T) {
		ctx := context.Background()

		response := &model.SupportCannedResponse{
			WorkspaceID: workspaceID,
			ShortCode:   "greeting",
			Content:     "Hello! How can we help you today?",
			CreatedByID: "user-123",
		}

		err := repo.Create(ctx, response)
		if err != nil {
			t.Fatalf("create canned response: %v", err)
		}

		// List all
		responses, err := repo.List(ctx, workspaceID)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(responses) != 1 {
			t.Errorf("expected 1 response, got %d", len(responses))
		}

		// Get by ID
		fetched, err := repo.GetByID(ctx, workspaceID, response.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if fetched == nil {
			t.Fatal("expected to find response")
		}
		if fetched.ShortCode != "greeting" {
			t.Errorf("expected short_code 'greeting', got %q", fetched.ShortCode)
		}
		if fetched.Tag != "General" {
			t.Errorf("expected default category 'General', got %q", fetched.Tag)
		}
	})

	t.Run("Search canned responses", func(t *testing.T) {
		ctx := context.Background()

		// Create multiple responses
		responses := []struct {
			shortCode string
			content   string
		}{
			{"greetshort", "Hello there! How can we help?"},
			{"thankshort", "Thank you for reaching out!"},
			{"closingshort", "Is there anything else?"},
		}

		for _, r := range responses {
			err := repo.Create(ctx, &model.SupportCannedResponse{
				WorkspaceID: workspaceID,
				ShortCode:   r.shortCode,
				Content:     r.content,
			})
			if err != nil {
				t.Fatalf("create %s: %v", r.shortCode, err)
			}
		}

		// Search by short_code - use unique term to avoid matching previous test data
		results, err := repo.Search(ctx, workspaceID, "greetshort")
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 result for 'greetshort', got %d", len(results))
		}

		// Search by another short_code
		results, err = repo.Search(ctx, workspaceID, "thankshort")
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 result for 'thankshort', got %d", len(results))
		}

		// Search by content
		results, err = repo.Search(ctx, workspaceID, "reaching")
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 result for 'reaching', got %d", len(results))
		}
	})

	t.Run("Search ignores deprecated title field", func(t *testing.T) {
		ctx := context.Background()

		err := repo.Create(ctx, &model.SupportCannedResponse{
			WorkspaceID: workspaceID,
			ShortCode:   "not-title-searchable",
			Content:     "Body does not contain the deprecated search token",
			Tag:         "Support",
		})
		if err != nil {
			t.Fatalf("create title-only response: %v", err)
		}

		results, err := repo.Search(ctx, workspaceID, "UniqueDeprecatedTitleOnly")
		if err != nil {
			t.Fatalf("search title-only token: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected title-only search to return 0 results, got %d", len(results))
		}
	})

	t.Run("Update canned response", func(t *testing.T) {
		ctx := context.Background()

		response := &model.SupportCannedResponse{
			WorkspaceID: workspaceID,
			ShortCode:   "test",
			Content:     "Original content",
		}
		err := repo.Create(ctx, response)
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		// Update
		response.Content = "Updated content"
		err = repo.Update(ctx, response)
		if err != nil {
			t.Fatalf("update: %v", err)
		}

		// Verify
		fetched, err := repo.GetByID(ctx, workspaceID, response.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if fetched.Content != "Updated content" {
			t.Errorf("expected content 'Updated content', got %q", fetched.Content)
		}
	})

	t.Run("Delete canned response", func(t *testing.T) {
		ctx := context.Background()

		response := &model.SupportCannedResponse{
			WorkspaceID: workspaceID,
			ShortCode:   "delete-me",
			Content:     "This will be deleted",
		}
		err := repo.Create(ctx, response)
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		// Delete
		err = repo.Delete(ctx, workspaceID, response.ID)
		if err != nil {
			t.Fatalf("delete: %v", err)
		}

		// Verify deleted
		fetched, err := repo.GetByID(ctx, workspaceID, response.ID)
		if err != nil {
			t.Fatalf("get after delete: %v", err)
		}
		if fetched != nil {
			t.Error("expected response to be nil after delete")
		}
	})

	t.Run("Cross-workspace isolation for canned responses", func(t *testing.T) {
		ctx := context.Background()

		otherWS := "ws-canned-other"
		seedWorkspace(t, db, otherWS, "Other Canned WS", "other-canned-ws", "user-456")

		// Create in other workspace
		err := repo.Create(ctx, &model.SupportCannedResponse{
			WorkspaceID: otherWS,
			ShortCode:   "isolated",
			Content:     "Only in other workspace",
		})
		if err != nil {
			t.Fatalf("create in other ws: %v", err)
		}

		// Search from original workspace should not find it
		results, err := repo.Search(ctx, workspaceID, "isolated")
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results from cross-workspace search, got %d", len(results))
		}
	})
}

func TestSupportInboxServiceSeedWorkspaceDefaultsSeedsStarterShortcuts(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	workspaceID := "ws-support-shortcut-defaults"
	ownerID := "user-shortcut-defaults"
	seedWorkspace(t, db, workspaceID, "Shortcut Defaults", "shortcut-defaults", ownerID)

	cannedRepo := repository.NewSupportCannedResponseRepository(db)
	svc := NewSupportInboxService(
		nil,
		nil,
		nil,
		nil,
		nil,
		repository.NewSupportInboxInstallationRepository(db),
		nil,
		cannedRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	if err := svc.SeedWorkspaceDefaults(ctx, workspaceID, ownerID); err != nil {
		t.Fatalf("seed workspace defaults: %v", err)
	}

	responses, err := cannedRepo.List(ctx, workspaceID)
	if err != nil {
		t.Fatalf("list canned responses: %v", err)
	}
	if len(responses) != 12 {
		t.Fatalf("expected 12 starter shortcuts, got %d", len(responses))
	}

	byCode := map[string]model.SupportCannedResponse{}
	for _, response := range responses {
		byCode[response.ShortCode] = response
	}
	if byCode["!hello"].Tag != "General" {
		t.Fatalf("expected !hello in General, got %q", byCode["!hello"].Tag)
	}
	if !strings.Contains(byCode["!hello"].Content, `{{customer.first_name | fallback: "there"}}`) {
		t.Fatalf("expected !hello to include customer first-name fallback, got %q", byCode["!hello"].Content)
	}
	if !strings.Contains(byCode["!hello"].Content, "\n\nThanks for reaching out.") {
		t.Fatalf("expected !hello to separate greeting from message body, got %q", byCode["!hello"].Content)
	}
	if !strings.Contains(byCode["!followup"].Content, "\n\nJust checking in") {
		t.Fatalf("expected !followup to separate greeting from message body, got %q", byCode["!followup"].Content)
	}
	if byCode["!demo"].Tag != "Sales" {
		t.Fatalf("expected !demo in Sales, got %q", byCode["!demo"].Tag)
	}

	if err := svc.SeedWorkspaceDefaults(ctx, workspaceID, ownerID); err != nil {
		t.Fatalf("seed workspace defaults again: %v", err)
	}
	responses, err = cannedRepo.List(ctx, workspaceID)
	if err != nil {
		t.Fatalf("list canned responses after second seed: %v", err)
	}
	if len(responses) != 12 {
		t.Fatalf("expected second seed to avoid duplicates, got %d shortcuts", len(responses))
	}
}

func TestSupportInboxServiceSeedWorkspaceDefaultsKeepsExistingShortcutSet(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	workspaceID := "ws-support-existing-shortcuts"
	ownerID := "user-existing-shortcuts"
	seedWorkspace(t, db, workspaceID, "Existing Shortcuts", "existing-shortcuts", ownerID)

	cannedRepo := repository.NewSupportCannedResponseRepository(db)
	if err := cannedRepo.Create(ctx, &model.SupportCannedResponse{
		WorkspaceID: workspaceID,
		ShortCode:   "!custom",
		Content:     "Custom saved reply",
		Tag:         "General",
		CreatedByID: ownerID,
	}); err != nil {
		t.Fatalf("create custom shortcut: %v", err)
	}

	svc := NewSupportInboxService(
		nil,
		nil,
		nil,
		nil,
		nil,
		repository.NewSupportInboxInstallationRepository(db),
		nil,
		cannedRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	if err := svc.SeedWorkspaceDefaults(ctx, workspaceID, ownerID); err != nil {
		t.Fatalf("seed workspace defaults: %v", err)
	}

	responses, err := cannedRepo.List(ctx, workspaceID)
	if err != nil {
		t.Fatalf("list canned responses: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected existing shortcut set to remain unchanged, got %d shortcuts", len(responses))
	}
	if responses[0].ShortCode != "!custom" {
		t.Fatalf("expected custom shortcut to remain, got %q", responses[0].ShortCode)
	}
}

// ---------------------------------------------------------------------------
// Pure Function Tests
// ---------------------------------------------------------------------------

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "short string unchanged",
			input:  "Hello",
			maxLen: 100,
			want:   "Hello",
		},
		{
			name:   "exact length unchanged",
			input:  "Hello",
			maxLen: 5,
			want:   "Hello",
		},
		{
			name:   "long string truncated with ellipsis",
			input:  "This is a very long message that exceeds the limit",
			maxLen: 20,
			want:   "This is a very lo...",
		},
		{
			name:   "UTF-8 multi-byte characters preserved",
			input:  "こんにちは世界のみなさん",
			maxLen: 8,
			want:   "こんにちは...",
		},
		{
			name:   "emoji preserved",
			input:  "Hello 🌍🌎🌏 World",
			maxLen: 10,
			want:   "Hello 🌍...",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestValidConversationStatuses(t *testing.T) {
	valid := []string{
		model.SupportConversationStatusOpen,
		model.SupportConversationStatusWaitingOnCustomer,
		model.SupportConversationStatusResolved,
		model.SupportConversationStatusSpam,
	}
	for _, s := range valid {
		if !validConversationStatuses[s] {
			t.Errorf("expected %q to be valid", s)
		}
	}

	invalid := []string{"", "pending", "in_progress", "waiting", "closed", "OPEN", "Closed", "deleted"}
	for _, s := range invalid {
		if validConversationStatuses[s] {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

func TestGenerateSecureToken(t *testing.T) {
	t.Run("generates correct length hex string", func(t *testing.T) {
		token, err := generateSecureToken(32)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// 32 bytes = 64 hex characters
		if len(token) != 64 {
			t.Errorf("expected 64 char hex string, got %d chars", len(token))
		}
	})

	t.Run("generates unique tokens", func(t *testing.T) {
		token1, _ := generateSecureToken(32)
		token2, _ := generateSecureToken(32)
		if token1 == token2 {
			t.Error("expected unique tokens, got duplicates")
		}
	})
}

// extractSupportDescriptionSection pulls the body text out of a single
// <h2 id="..."> section in the rendered task description HTML. Returns "" if
// the section isn't present.
func extractSupportDescriptionSection(html, sectionID string) string {
	needle := `<h2 id="` + sectionID + `">`
	start := strings.Index(html, needle)
	if start < 0 {
		return ""
	}
	start += len(needle)
	if closeH2 := strings.Index(html[start:], "</h2>"); closeH2 >= 0 {
		start += closeH2 + len("</h2>")
	}
	if end := strings.Index(html[start:], "<h2 "); end >= 0 {
		return html[start : start+end]
	}
	return html[start:]
}

func TestSupportInboxServiceCreateConversationWithMessageWithoutCustomerLanguage(t *testing.T) {
	env, existing, provider := translationFixture(t)
	ctx := context.Background()
	setTranslationWorkspaceSettings(t, env, existing.WorkspaceID, func(settings *model.SupportInboxSettings) {
		settings.TranslationCustomerLanguage = ""
		settings.TranslationIncomingEnabled = false
		settings.TranslationOutgoingEnabled = true
	})
	provider.fail = true // With no evidence, even a provider outage must not block.
	body := "Hello, following up on your cancellation request."
	result, err := env.service.supportInboxService.CreateConversationWithMessage(ctx, model.CreateConversationWithMessageRequest{
		WorkspaceID: existing.WorkspaceID, Subject: "Following up", Content: body,
		CustomerEmail: strPtr("recipient@example.com"), Channels: []string{"email"},
	}, "22222222-2222-2222-2222-222222222222")
	if err != nil {
		t.Fatalf("first outbound send: %v", err)
	}
	if result == nil || result.Conversation == nil || result.Message == nil {
		t.Fatal("missing conversation or first message")
	}
	if result.Message.Content != body || result.Message.TranslationID != "" || provider.calls != 0 {
		t.Fatalf("first message was translated: content=%q translation=%q calls=%d", result.Message.Content, result.Message.TranslationID, provider.calls)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		saved, err := env.messageRepo.GetByID(ctx, result.Message.ID)
		if err != nil {
			t.Fatal(err)
		}
		if saved == nil || saved.Content != body {
			t.Fatalf("first message not recorded: %+v", saved)
		}
		if saved.CancellableUntil != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("first outbound email was not queued")
		}
		time.Sleep(5 * time.Millisecond)
	}
	queued, err := env.redis.LRange(ctx, env.service.msgListKey(result.Conversation.ID), 0, -1).Result()
	if err != nil || len(queued) != 1 || queued[0] != result.Message.ID {
		t.Fatalf("queued email=%v err=%v", queued, err)
	}
	// Advance this message beyond the undo window before running delivery.
	if err := env.messageRepo.SetCancellableUntil(ctx, result.Message.ID, time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	sent := captureExplicitDeliveryEmails(t, env)
	if err := env.service.fireEmail(ctx, result.Conversation.ID, queued); err != nil {
		t.Fatalf("deliver first email: %v", err)
	}
	if len(*sent) != 1 || !strings.Contains((*sent)[0].TextBody, body) {
		t.Fatalf("first email delivery=%+v", *sent)
	}
}

func TestSupportInboxServiceCreateConversationWithMessageCleansUpFailure(t *testing.T) {
	for _, scenario := range []string{"translation failure", "message write failure", "invalid tag", "request cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			env, existing, provider := translationFixture(t)
			db := env.messageRepo.DB()
			svc := env.service.supportInboxService.SetSupportTagRepo(repository.NewSupportTagRepository(db))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			mustExec(t, db, `INSERT INTO support_tags (id, workspace_id, name, color, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
				"outbound-tag", existing.WorkspaceID, "Outbound", "#2563eb", time.Now(), time.Now())
			tags := []string{"outbound-tag"}
			switch scenario {
			case "translation failure":
				provider.fail = true
			case "message write failure":
				mustExec(t, db, `CREATE TRIGGER reject_outbound_reply BEFORE INSERT ON support_messages WHEN NEW.sender_type = 'user' AND NEW.message_type = 'reply' BEGIN SELECT RAISE(FAIL, 'message write failed'); END`)
			case "invalid tag":
				tags = append(tags, "missing-tag")
			case "request cancelled":
				provider.onCall = cancel
			}
			result, err := svc.CreateConversationWithMessage(ctx, model.CreateConversationWithMessageRequest{
				WorkspaceID: existing.WorkspaceID, Subject: "Unsuccessful outbound", Content: "Hello",
				CustomerEmail: strPtr("recipient@example.com"), Channels: []string{"email"}, TagIDs: tags,
			}, "22222222-2222-2222-2222-222222222222")
			if err == nil || result != nil {
				t.Fatalf("expected failed send, got result=%+v err=%v", result, err)
			}
			var conversations, messages, tagLinks int64
			if err := db.Model(&model.SupportConversation{}).Where("workspace_id = ? AND id <> ?", existing.WorkspaceID, existing.ID).Count(&conversations).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&model.SupportMessage{}).Where("workspace_id = ? AND conversation_id <> ?", existing.WorkspaceID, existing.ID).Count(&messages).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Model(&model.SupportConversationTag{}).Where("tag_id = ?", "outbound-tag").Count(&tagLinks).Error; err != nil {
				t.Fatal(err)
			}
			if conversations != 0 || messages != 0 || tagLinks != 0 {
				t.Fatalf("failed send left conversations=%d messages=%d tag links=%d", conversations, messages, tagLinks)
			}
		})
	}
}
