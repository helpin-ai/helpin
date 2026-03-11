package service

import (
	"context"
	"testing"
	"time"

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
		fetched, err := repo.GetByID(ctx, workspaceID, conversation.ID)
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
		conversations, total, err := repo.List(ctx, workspaceID, "", "", model.PMPagination{})
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
		openConvs, totalOpen, err := repo.List(ctx, workspaceID, "open", "", model.PMPagination{})
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
		_, totalHigh, err := repo.List(ctx, workspaceID, "", "high", model.PMPagination{})
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
		fetched, err := repo.GetByID(ctx, workspaceID, conversation.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if fetched.Status != "resolved" {
			t.Errorf("expected status 'resolved', got %q", fetched.Status)
		}
	})

	t.Run("GetByID not found returns nil", func(t *testing.T) {
		ctx := context.Background()

		fetched, err := repo.GetByID(ctx, workspaceID, "non-existent-id")
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
		fetched, err := repo.GetByID(ctx, workspaceID, otherConv.ID)
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
		page1, total, err := pRepo.List(ctx, paginationWS, "", "", model.PMPagination{Page: 1, PerPage: 2})
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
		page3, _, err := pRepo.List(ctx, paginationWS, "", "", model.PMPagination{Page: 3, PerPage: 2})
		if err != nil {
			t.Fatalf("page 3: %v", err)
		}
		if len(page3) != 1 {
			t.Errorf("expected 1 result on page 3, got %d", len(page3))
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

		messages, err := msgRepo.ListByTicket(ctx, workspaceID, conv.ID, true)
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

	t.Run("List messages with legacy ticket_id", func(t *testing.T) {
		// Simulate a legacy message that only has ticket_id set (not conversation_id).
		legacyTicketID := conv.ID
		mustExec(t, db, `INSERT INTO support_messages (id, workspace_id, conversation_id, ticket_id, sender_type, content, message_type, is_internal) VALUES (?, ?, '', ?, 'user', 'Legacy message', 'reply', 0)`,
			"legacy-msg-001", workspaceID, legacyTicketID)

		messages, err := msgRepo.ListByTicket(ctx, workspaceID, conv.ID, true)
		if err != nil {
			t.Fatalf("list messages with legacy: %v", err)
		}
		// Should include both the new message (conversation_id) and legacy (ticket_id)
		if len(messages) < 2 {
			t.Errorf("expected at least 2 messages (new + legacy), got %d", len(messages))
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
		allMsgs, err := msgRepo.ListByTicket(ctx, workspaceID, conv.ID, true)
		if err != nil {
			t.Fatalf("list all: %v", err)
		}

		// Without internal
		publicMsgs, err := msgRepo.ListByTicket(ctx, workspaceID, conv.ID, false)
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

		messages, err := msgRepo.ListByTicket(ctx, orderWS, oConv.ID, true)
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

// ---------------------------------------------------------------------------
// Widget Installation Repository Tests
// ---------------------------------------------------------------------------

func TestWidgetInstallationRepository(t *testing.T) {
	db := newTestDB(t)

	workspaceID := "ws-widget-test"
	seedWorkspace(t, db, workspaceID, "Widget Test WS", "widget-test-ws", "user-123")

	repo := repository.NewWidgetInstallationRepository(db)
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

	repo := repository.NewWidgetSessionRepository(db)
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
		session.TicketID = &convID
		if err := repo.Update(ctx, session); err != nil {
			t.Fatalf("update: %v", err)
		}

		fetched, _ := repo.GetByToken(ctx, "token_update_test")
		if fetched.ConversationID == nil || *fetched.ConversationID != convID {
			t.Error("expected conversation_id to be set after update")
		}
		if fetched.TicketID == nil || *fetched.TicketID != convID {
			t.Error("expected ticket_id to be set after update")
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
