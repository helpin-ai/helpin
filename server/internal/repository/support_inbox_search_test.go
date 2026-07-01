package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/gorm"
)

func setupSupportConversationSearchTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupSupportConversationMessageTestDB(t)
	if err := db.Exec(`ALTER TABLE support_mailboxes ADD COLUMN active BOOLEAN NOT NULL DEFAULT 1`).Error; err != nil {
		t.Fatalf("add support_mailboxes active: %v", err)
	}
	if err := db.Exec(`ALTER TABLE support_mailboxes ADD COLUMN linked_team_id TEXT`).Error; err != nil {
		t.Fatalf("add support_mailboxes linked_team_id: %v", err)
	}
	if err := db.Exec(`CREATE TABLE support_mailbox_memberships (
		id TEXT PRIMARY KEY,
		mailbox_id TEXT NOT NULL,
		workspace_member_id TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error; err != nil {
		t.Fatalf("create support_mailbox_memberships: %v", err)
	}
	if err := db.Exec(`CREATE TABLE workspace_members (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		role TEXT NOT NULL DEFAULT 'member',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error; err != nil {
		t.Fatalf("create workspace_members: %v", err)
	}
	if err := db.Exec(`CREATE TABLE team_workspace_memberships (
		id TEXT PRIMARY KEY,
		team_id TEXT NOT NULL,
		workspace_member_id TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error; err != nil {
		t.Fatalf("create team_workspace_memberships: %v", err)
	}
	if err := db.Exec(`CREATE TABLE support_conversation_tags (
		conversation_id TEXT NOT NULL,
		tag_id TEXT NOT NULL
	)`).Error; err != nil {
		t.Fatalf("create support_conversation_tags: %v", err)
	}
	return db
}

func insertSearchConversation(t *testing.T, db *gorm.DB, c model.SupportConversation) {
	t.Helper()
	if c.Status == "" {
		c.Status = model.SupportConversationStatusOpen
	}
	if c.Priority == "" {
		c.Priority = "medium"
	}
	if c.Channel == "" {
		c.Channel = "widget"
	}
	if c.Source == "" {
		c.Source = "widget"
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = c.CreatedAt
	}
	if err := db.Exec(`INSERT INTO support_conversations
		(id, workspace_id, mailbox_id, display_id, subject, status, priority,
		 channel, customer_name, customer_email, opened_by_user_id, assigned_user_id,
		 assigned_agent_id, source, ai_state, flow_state, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.WorkspaceID, c.MailboxID, c.DisplayID, c.Subject, c.Status, c.Priority,
		c.Channel, c.CustomerName, c.CustomerEmail, c.OpenedByUserID, c.AssignedUserID,
		c.AssignedAgentID, c.Source, c.AIState, c.FlowState, c.CreatedAt, c.UpdatedAt,
	).Error; err != nil {
		t.Fatalf("insert search conversation %s: %v", c.ID, err)
	}
}

func TestSupportConversationRepositorySearchMatchesCoreFieldsAndMessages(t *testing.T) {
	db := setupSupportConversationSearchTestDB(t)
	repo := NewSupportConversationRepository(db)
	ctx := context.Background()
	wsID := "workspace-search"
	base := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	email := "beth@example.com"
	customer := "Beth Example"

	insertSearchConversation(t, db, model.SupportConversation{
		ID:            "subject-match",
		WorkspaceID:   wsID,
		DisplayID:     31,
		Subject:       "Adding teammates to project",
		CustomerName:  &customer,
		CustomerEmail: &email,
		CreatedAt:     base,
		UpdatedAt:     base.Add(5 * time.Minute),
	})
	insertMessage(t, db, model.SupportMessage{
		ID:             "subject-message",
		WorkspaceID:    wsID,
		ConversationID: "subject-match",
		SenderType:     "customer",
		Content:        "The add teammate button reloads on the project page.",
		MessageType:    "reply",
		CreatedAt:      base.Add(time.Minute),
	})
	insertSearchConversation(t, db, model.SupportConversation{
		ID:          "message-match",
		WorkspaceID: wsID,
		DisplayID:   32,
		Subject:     "Billing issue",
		CreatedAt:   base,
		UpdatedAt:   base.Add(2 * time.Minute),
	})
	insertMessage(t, db, model.SupportMessage{
		ID:             "message-body",
		WorkspaceID:    wsID,
		ConversationID: "message-match",
		SenderType:     "customer",
		Content:        "Can your project export include audit history?",
		MessageType:    "reply",
		CreatedAt:      base.Add(time.Minute),
	})

	resp, err := repo.Search(ctx, ConversationRepositorySearchParams{
		SupportConversationSearchParams: model.SupportConversationSearchParams{
			WorkspaceID: wsID,
			Query:       "project",
			Pagination:  model.PMPagination{Page: 1, PerPage: 50},
		},
		Role: model.RoleOwner,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if resp.Total != 2 || len(resp.Data) != 2 {
		t.Fatalf("total=%d len=%d, want 2", resp.Total, len(resp.Data))
	}
	if resp.Data[0].Conversation.ID != "subject-match" {
		t.Fatalf("first result = %s, want subject-match", resp.Data[0].Conversation.ID)
	}
	if !searchContainsString(resp.Data[0].MatchedFields, "title") {
		t.Fatalf("matched fields = %v, want title", resp.Data[0].MatchedFields)
	}
	if resp.Data[0].Snippet == "" || len(resp.Data[0].Highlights) == 0 {
		t.Fatalf("expected snippet and highlights, got snippet=%q highlights=%+v", resp.Data[0].Snippet, resp.Data[0].Highlights)
	}

	resp, err = repo.Search(ctx, ConversationRepositorySearchParams{
		SupportConversationSearchParams: model.SupportConversationSearchParams{
			WorkspaceID: wsID,
			Query:       "beth@example.com",
			Pagination:  model.PMPagination{Page: 1, PerPage: 50},
		},
		Role: model.RoleOwner,
	})
	if err != nil {
		t.Fatalf("Search email: %v", err)
	}
	if resp.Total != 1 || resp.Data[0].Conversation.ID != "subject-match" || !searchContainsString(resp.Data[0].MatchedFields, "customer_email") {
		t.Fatalf("email search got total=%d data=%+v", resp.Total, resp.Data)
	}

	resp, err = repo.Search(ctx, ConversationRepositorySearchParams{
		SupportConversationSearchParams: model.SupportConversationSearchParams{
			WorkspaceID: wsID,
			Query:       "#31",
			Pagination:  model.PMPagination{Page: 1, PerPage: 50},
		},
		Role: model.RoleOwner,
	})
	if err != nil {
		t.Fatalf("Search display id: %v", err)
	}
	if resp.Total != 1 || resp.Data[0].Conversation.ID != "subject-match" || !searchContainsString(resp.Data[0].MatchedFields, "display_id") {
		t.Fatalf("display search got total=%d data=%+v", resp.Total, resp.Data)
	}
}

func TestSupportConversationRepositorySearchFiltersOnlyAndMailboxAccess(t *testing.T) {
	db := setupSupportConversationSearchTestDB(t)
	repo := NewSupportConversationRepository(db)
	ctx := context.Background()
	wsID := "workspace-search"
	memberID := "member-1"
	base := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	assigneeID := "user-1"
	openStatus := model.SupportConversationStatusOpen

	if err := db.Exec(`INSERT INTO workspace_members (id, workspace_id, user_id, status, role) VALUES (?, ?, ?, 'active', 'member')`, memberID, wsID, assigneeID).Error; err != nil {
		t.Fatalf("insert workspace member: %v", err)
	}
	if err := db.Exec(`INSERT INTO support_mailboxes (id, workspace_id, name, handle, active) VALUES ('allowed-box', ?, 'Allowed', 'allowed', 1), ('blocked-box', ?, 'Blocked', 'blocked', 1)`, wsID, wsID).Error; err != nil {
		t.Fatalf("insert mailboxes: %v", err)
	}
	if err := db.Exec(`INSERT INTO support_mailbox_memberships (id, mailbox_id, workspace_member_id) VALUES ('membership-1', 'allowed-box', ?)`, memberID).Error; err != nil {
		t.Fatalf("insert mailbox membership: %v", err)
	}

	insertSearchConversation(t, db, model.SupportConversation{
		ID:             "shared-open",
		WorkspaceID:    wsID,
		DisplayID:      41,
		Subject:        "Shared open",
		Status:         openStatus,
		AssignedUserID: &assigneeID,
		CreatedAt:      base,
		UpdatedAt:      base.Add(time.Minute),
	})
	allowedMailbox := "allowed-box"
	insertSearchConversation(t, db, model.SupportConversation{
		ID:          "allowed-open",
		WorkspaceID: wsID,
		MailboxID:   &allowedMailbox,
		DisplayID:   42,
		Subject:     "Allowed open",
		Status:      openStatus,
		CreatedAt:   base,
		UpdatedAt:   base.Add(2 * time.Minute),
	})
	blockedMailbox := "blocked-box"
	insertSearchConversation(t, db, model.SupportConversation{
		ID:          "blocked-open",
		WorkspaceID: wsID,
		MailboxID:   &blockedMailbox,
		DisplayID:   43,
		Subject:     "Blocked open",
		Status:      openStatus,
		CreatedAt:   base,
		UpdatedAt:   base.Add(3 * time.Minute),
	})
	insertSearchConversation(t, db, model.SupportConversation{
		ID:          "resolved",
		WorkspaceID: wsID,
		DisplayID:   44,
		Subject:     "Resolved",
		Status:      model.SupportConversationStatusResolved,
		CreatedAt:   base,
		UpdatedAt:   base.Add(4 * time.Minute),
	})

	resp, err := repo.Search(ctx, ConversationRepositorySearchParams{
		SupportConversationSearchParams: model.SupportConversationSearchParams{
			WorkspaceID: wsID,
			Statuses:    []string{model.SupportConversationStatusOpen},
			Pagination:  model.PMPagination{Page: 1, PerPage: 50},
		},
		WorkspaceMemberID: memberID,
		Role:              model.RoleMember,
	})
	if err != nil {
		t.Fatalf("Search filters-only: %v", err)
	}
	gotIDs := searchResultIDs(resp.Data)
	wantIDs := []string{"allowed-open", "shared-open"}
	if !stringSlicesEqual(gotIDs, wantIDs) {
		t.Fatalf("filters-only ids = %v, want %v", gotIDs, wantIDs)
	}

	resp, err = repo.Search(ctx, ConversationRepositorySearchParams{
		SupportConversationSearchParams: model.SupportConversationSearchParams{
			WorkspaceID: wsID,
			AssignedTo:  []string{"me"},
			UserID:      assigneeID,
			Pagination:  model.PMPagination{Page: 1, PerPage: 50},
		},
		WorkspaceMemberID: memberID,
		Role:              model.RoleMember,
	})
	if err != nil {
		t.Fatalf("Search assigned me: %v", err)
	}
	gotIDs = searchResultIDs(resp.Data)
	if !stringSlicesEqual(gotIDs, []string{"shared-open"}) {
		t.Fatalf("assigned ids = %v, want shared-open", gotIDs)
	}
}

func TestSupportConversationRepositorySearchCapsTotals(t *testing.T) {
	db := setupSupportConversationSearchTestDB(t)
	repo := NewSupportConversationRepository(db)
	ctx := context.Background()
	wsID := "workspace-search"
	base := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)

	for i := 1; i <= 1001; i++ {
		insertSearchConversation(t, db, model.SupportConversation{
			ID:          fmt.Sprintf("cap-%04d", i),
			WorkspaceID: wsID,
			DisplayID:   i,
			Subject:     "cap match",
			CreatedAt:   base,
			UpdatedAt:   base.Add(time.Duration(i) * time.Second),
		})
	}

	resp, err := repo.Search(ctx, ConversationRepositorySearchParams{
		SupportConversationSearchParams: model.SupportConversationSearchParams{
			WorkspaceID: wsID,
			Query:       "cap",
			Pagination:  model.PMPagination{Page: 1, PerPage: 10},
		},
		Role: model.RoleOwner,
	})
	if err != nil {
		t.Fatalf("Search cap: %v", err)
	}
	if resp.Total != 1000 || !resp.Meta.TotalCapped || resp.Meta.TotalCap != 1000 {
		t.Fatalf("cap meta total=%d capped=%v cap=%d", resp.Total, resp.Meta.TotalCapped, resp.Meta.TotalCap)
	}
}

func searchContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func searchResultIDs(results []model.SupportConversationSearchResult) []string {
	ids := make([]string, 0, len(results))
	for _, result := range results {
		ids = append(ids, result.Conversation.ID)
	}
	return ids
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
