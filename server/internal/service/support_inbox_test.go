package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

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
			Status:      "closed",
			Priority:    "high",
		}
		err := repo.Create(ctx, conversation2)
		if err != nil {
			t.Fatalf("create second conversation: %v", err)
		}

		// List all
		conversations, total, err := repo.List(ctx, workspaceID, "", "", model.PMPagination{}, "", model.RoleOwner, nil)
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
		openConvs, totalOpen, err := repo.List(ctx, workspaceID, "open", "", model.PMPagination{}, "", model.RoleOwner, nil)
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
		_, totalHigh, err := repo.List(ctx, workspaceID, "", "high", model.PMPagination{}, "", model.RoleOwner, nil)
		if err != nil {
			t.Fatalf("list high priority: %v", err)
		}
		if totalHigh != 1 {
			t.Errorf("expected 1 high priority, got %d", totalHigh)
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
		page1, total, err := pRepo.List(ctx, paginationWS, "", "", model.PMPagination{Page: 1, PerPage: 2}, "", model.RoleOwner, nil)
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
		page3, _, err := pRepo.List(ctx, paginationWS, "", "", model.PMPagination{Page: 3, PerPage: 2}, "", model.RoleOwner, nil)
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

		teamConversations, total, err := repo.List(ctx, linkedWorkspaceID, "", "", model.PMPagination{}, "wm-linked-team", model.RoleMember, nil)
		if err != nil {
			t.Fatalf("list conversations for linked team member: %v", err)
		}
		if total != 1 || len(teamConversations) != 1 || teamConversations[0].ID != privateConversation.ID {
			t.Fatalf("expected linked team member to see private conversation, got total=%d conversations=%#v", total, teamConversations)
		}

		outsiderConversations, outsiderTotal, err := repo.List(ctx, linkedWorkspaceID, "", "", model.PMPagination{}, "wm-linked-outsider", model.RoleMember, nil)
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

		humanConv := &model.SupportConversation{
			WorkspaceID:     workspaceID,
			Subject:         "Human owned conversation",
			Status:          "open",
			OpenedByUserID:  strPtr("user-123"),
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

		aiPending := "pending"
		aiConv := &model.SupportConversation{
			WorkspaceID:    workspaceID,
			Subject:        "AI owned conversation",
			Status:         "open",
			OpenedByUserID: strPtr("user-123"),
			AIState:        &aiPending,
			TeamLastSeenAt: &now,
		}
		if err := repo.Create(ctx, aiConv); err != nil {
			t.Fatalf("create AI conversation: %v", err)
		}
		if err := db.Exec(
			`UPDATE support_conversations SET team_last_seen_at = ?, updated_at = ?, ai_state = ? WHERE id = ?`,
			now.Add(-2*time.Minute), now, aiPending, aiConv.ID,
		).Error; err != nil {
			t.Fatalf("seed AI state: %v", err)
		}
		if err := db.Exec(
			`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content, message_type, is_internal, created_at, updated_at) VALUES (?, ?, ?, 'customer', ?, 'reply', 0, ?, ?)`,
			"msg-ai-unread", workspaceID, aiConv.ID, "AI can take this", customerMessageAt, customerMessageAt,
		).Error; err != nil {
			t.Fatalf("seed AI unread message: %v", err)
		}

		aiEscalated := "escalated"
		escalatedConv := &model.SupportConversation{
			WorkspaceID:    workspaceID,
			Subject:        "Escalated AI conversation",
			Status:         "open",
			AIState:        &aiEscalated,
			TeamLastSeenAt: &now,
		}
		if err := repo.Create(ctx, escalatedConv); err != nil {
			t.Fatalf("create escalated conversation: %v", err)
		}
		if err := db.Exec(
			`UPDATE support_conversations SET team_last_seen_at = ?, updated_at = ?, ai_state = ?, assigned_agent_id = NULL, opened_by_user_id = NULL WHERE id = ?`,
			now.Add(-2*time.Minute), now, aiEscalated, escalatedConv.ID,
		).Error; err != nil {
			t.Fatalf("seed escalated state: %v", err)
		}
		if err := db.Exec(
			`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content, message_type, is_internal, created_at, updated_at) VALUES (?, ?, ?, 'customer', ?, 'reply', 0, ?, ?)`,
			"msg-escalated-unread", workspaceID, escalatedConv.ID, "Need a human now", customerMessageAt, customerMessageAt,
		).Error; err != nil {
			t.Fatalf("seed escalated unread message: %v", err)
		}

		stats, err := repo.GetUnreadStats(ctx, workspaceID, "user-123", "", model.RoleOwner, nil)
		if err != nil {
			t.Fatalf("get unread stats: %v", err)
		}

		if stats.Total != 2 {
			t.Fatalf("expected total unread human inbox count 2, got %d", stats.Total)
		}
		if stats.MyInbox != 1 {
			t.Fatalf("expected my inbox count 1, got %d", stats.MyInbox)
		}
		if stats.Unassigned != 1 {
			t.Fatalf("expected unassigned count 1, got %d", stats.Unassigned)
		}
		if stats.AIAll != 2 {
			t.Fatalf("expected all AI unread count 2, got %d", stats.AIAll)
		}
		if stats.AIPending != 0 {
			t.Fatalf("expected AI pending unread count 0 in sqlite-backed unread stats test, got %d", stats.AIPending)
		}
	})

	t.Run("Mailbox unread counts exclude AI-managed conversations that stay in AI views", func(t *testing.T) {
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
		}
		if err := repo.Create(ctx, aiPendingConv); err != nil {
			t.Fatalf("create AI pending billing conversation: %v", err)
		}
		if err := db.Exec(
			`UPDATE support_conversations SET team_last_seen_at = ?, updated_at = ?, ai_state = ? WHERE id = ?`,
			now.Add(-2*time.Minute), now, aiPending, aiPendingConv.ID,
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
		}
		if err := repo.Create(ctx, aiEscalatedConv); err != nil {
			t.Fatalf("create escalated billing conversation: %v", err)
		}
		if err := db.Exec(
			`UPDATE support_conversations SET team_last_seen_at = ?, updated_at = ?, ai_state = ? WHERE id = ?`,
			now.Add(-2*time.Minute), now, aiEscalated, aiEscalatedConv.ID,
		).Error; err != nil {
			t.Fatalf("seed escalated billing state: %v", err)
		}
		if err := db.Exec(
			`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, content, message_type, is_internal, created_at, updated_at) VALUES (?, ?, ?, 'customer', ?, 'reply', 0, ?, ?)`,
			"msg-billing-ai-escalated-unread", workspaceID, aiEscalatedConv.ID, "Need a human for billing", customerMessageAt, customerMessageAt,
		).Error; err != nil {
			t.Fatalf("seed escalated billing unread message: %v", err)
		}

		count, err := mailboxRepo.CountUnread(ctx, workspaceID, &billingMailbox.ID)
		if err != nil {
			t.Fatalf("count billing mailbox unread: %v", err)
		}
		if count != 2 {
			t.Fatalf("expected billing mailbox unread count 2, got %d", count)
		}
	})
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
		repository.NewDocsCollectionRepository(db),
		repository.NewDocsHelpcenterRepository(db),
	)
	svc.SetNotificationService(notificationService, repository.NewWorkspaceRepository(db))

	customerName := "Customer"
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
		repository.NewDocsCollectionRepository(db),
		repository.NewDocsHelpcenterRepository(db),
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
			ToObjectType:   model.CRMObjectCompany,
			ToObjectID:     "company-1",
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

	resp, err := svc.CreateTaskFromConversation(ctx, env.wsID, conversation.ID, env.userID, model.CreateTaskFromConversationRequest{})
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

	resp, err := svc.CreateTaskFromConversation(ctx, env.wsID, conversation.ID, env.userID, model.CreateTaskFromConversationRequest{})
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

	resp, err := svc.CreateTaskFromConversation(ctx, env.wsID, conversation.ID, env.userID, model.CreateTaskFromConversationRequest{})
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

	resp, err := svc.CreateTaskFromConversation(ctx, env.wsID, conversation.ID, env.userID, model.CreateTaskFromConversationRequest{})
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
			Title:       "Greeting",
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
	})

	t.Run("Search canned responses", func(t *testing.T) {
		ctx := context.Background()

		// Create multiple responses
		responses := []struct {
			shortCode string
			title     string
			content   string
		}{
			{"greetshort", "Greetshort", "Hello there! How can we help?"},
			{"thankshort", "Thankshort", "Thank you for reaching out!"},
			{"closingshort", "Closingshort", "Is there anything else?"},
		}

		for _, r := range responses {
			err := repo.Create(ctx, &model.SupportCannedResponse{
				WorkspaceID: workspaceID,
				ShortCode:   r.shortCode,
				Title:       r.title,
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

		// Search by title
		results, err = repo.Search(ctx, workspaceID, "Thankshort")
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 result for 'Thankshort', got %d", len(results))
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

	t.Run("Update canned response", func(t *testing.T) {
		ctx := context.Background()

		response := &model.SupportCannedResponse{
			WorkspaceID: workspaceID,
			ShortCode:   "test",
			Title:       "Test",
			Content:     "Original content",
		}
		err := repo.Create(ctx, response)
		if err != nil {
			t.Fatalf("create: %v", err)
		}

		// Update
		response.Title = "Updated Title"
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
		if fetched.Title != "Updated Title" {
			t.Errorf("expected title 'Updated Title', got %q", fetched.Title)
		}
	})

	t.Run("Delete canned response", func(t *testing.T) {
		ctx := context.Background()

		response := &model.SupportCannedResponse{
			WorkspaceID: workspaceID,
			ShortCode:   "delete-me",
			Title:       "Delete Me",
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
			Title:       "Isolated",
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
	valid := []string{"open", "in_progress", "waiting", "resolved", "closed"}
	for _, s := range valid {
		if !validConversationStatuses[s] {
			t.Errorf("expected %q to be valid", s)
		}
	}

	invalid := []string{"", "pending", "spam", "snoozed", "OPEN", "Closed", "deleted"}
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
