package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupSupportMessageTestDB creates an in-memory SQLite DB with the
// support_messages schema needed for the message-actions repository tests.
// It mirrors the production schema closely enough that GORM's reflection
// over model.SupportMessage works against it (column names + nullability).
func setupSupportMessageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := fmt.Sprintf("file:support_message_actions_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE support_messages (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		conversation_id TEXT,
		sender_type TEXT NOT NULL,
		message_type TEXT NOT NULL DEFAULT 'reply',
		system_event_type TEXT,
		sender_user_id TEXT,
		sender_agent_id TEXT,
		sender_display_name TEXT,
		sender_avatar_url TEXT,
		content TEXT NOT NULL,
		is_internal BOOLEAN NOT NULL DEFAULT 0,
		metadata TEXT NOT NULL DEFAULT '{}',
		via_channel TEXT,
		email_notified_at DATETIME,
		email_read_at DATETIME,
		cancellable_until DATETIME,
		deleted_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error; err != nil {
		t.Fatalf("create support_messages: %v", err)
	}
	return db
}

func setupSupportConversationMessageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupSupportMessageTestDB(t)
	if err := db.Exec(`CREATE TABLE support_conversations (
            anonymized_at DATETIME,
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		mailbox_id TEXT,
		display_id INTEGER NOT NULL DEFAULT 1,
		subject TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'open',
		flow_state TEXT,
		priority TEXT NOT NULL DEFAULT 'medium',
		channel TEXT NOT NULL DEFAULT 'widget',
		customer_name TEXT,
		customer_email TEXT,
		customer_phone TEXT,
		opened_by_user_id TEXT,
		assigned_user_id TEXT,
		assigned_agent_id TEXT,
		linked_task_id TEXT,
		source TEXT NOT NULL DEFAULT 'widget',
		anonymous_id TEXT,
		crm_contact_id TEXT,
		resolved_at DATETIME,
		closed_at DATETIME,
		team_last_seen_at DATETIME,
		contact_last_seen_at DATETIME,
		email_unsubscribed BOOLEAN NOT NULL DEFAULT 0,
		list_last_message_id TEXT,
		list_last_message_at DATETIME,
		list_last_message_preview TEXT,
		list_last_message_is_internal BOOLEAN NOT NULL DEFAULT 0,
		last_public_message_id TEXT,
		last_public_message_at DATETIME,
		last_public_sender_type TEXT,
		last_public_sender_display_name TEXT,
		last_customer_message_id TEXT,
		last_customer_message_at DATETIME,
		unanswered_customer_message_count INTEGER NOT NULL DEFAULT 0,
		customer_awaiting_response BOOLEAN NOT NULL DEFAULT 0,
		needs_human_reply BOOLEAN NOT NULL DEFAULT 0,
		support_state_version INTEGER NOT NULL DEFAULT 0,
		visitor_country_code TEXT,
		visitor_country_name TEXT,
		view_search_document TEXT,
		ai_state TEXT,
		ai_resolved_at DATETIME,
		ai_escalated_at DATETIME,
 delayed_team_reply_sent_for DATETIME,
		ai_resolution_type TEXT,
		ai_turn_count INTEGER NOT NULL DEFAULT 0,
		customer_requested_human_at DATETIME,
		ai_active_run_id TEXT,
		human_takeover BOOLEAN DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error; err != nil {
		t.Fatalf("create support_conversations: %v", err)
	}
	if err := db.Exec(`CREATE TABLE support_conversation_user_states (
		workspace_id TEXT NOT NULL,
		conversation_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		last_read_customer_message_id TEXT,
		last_read_customer_message_at DATETIME,
		unread_customer_message_count INTEGER NOT NULL DEFAULT 0,
		manually_unread BOOLEAN NOT NULL DEFAULT 0,
		mentioned_at DATETIME,
		relevance_mask INTEGER NOT NULL DEFAULT 0,
		version INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME,
		updated_at DATETIME,
		PRIMARY KEY (conversation_id, user_id)
	)`).Error; err != nil {
		t.Fatalf("create support_conversation_user_states: %v", err)
	}
	if err := db.Exec(`CREATE TABLE support_mailboxes (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		name TEXT,
		handle TEXT,
		icon TEXT
	)`).Error; err != nil {
		t.Fatalf("create support_mailboxes: %v", err)
	}
	if err := db.Exec(`CREATE TABLE support_widget_sessions (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		conversation_id TEXT,
		anonymous_id TEXT NOT NULL,
		country_code TEXT,
		country_name TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error; err != nil {
		t.Fatalf("create support_widget_sessions: %v", err)
	}
	return db
}

func TestSupportConversationRepositoryMailboxHelpersRespectAlias(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("open dry-run sqlite db: %v", err)
	}
	repo := NewSupportConversationRepository(db)
	shared := ""

	scopeQuery := repo.applyMailboxScope(
		db.Table("support_conversations AS sc"),
		"sc",
		&shared,
	).Find(&[]model.SupportConversation{})
	scopeSQL := scopeQuery.Statement.SQL.String()
	if !strings.Contains(scopeSQL, "sc.mailbox_id IS NULL") || strings.Contains(scopeSQL, "support_conversations.mailbox_id") {
		t.Fatalf("mailbox scope SQL should use alias sc, got %s", scopeSQL)
	}

	accessQuery := repo.applyMailboxAccess(
		db.Table("support_conversations AS sc"),
		"sc",
		"workspace-member-1",
		model.RoleMember,
	).Find(&[]model.SupportConversation{})
	accessSQL := accessQuery.Statement.SQL.String()
	if !strings.Contains(accessSQL, "sc.mailbox_id IS NULL") || !strings.Contains(accessSQL, "OR sc.mailbox_id IN") {
		t.Fatalf("mailbox access SQL should use alias sc, got %s", accessSQL)
	}
}

func TestSupportConversationRepositoryListActiveByCustomerEmailFiltersStatusAndWindow(t *testing.T) {
	db := setupSupportConversationMessageTestDB(t)
	repo := NewSupportConversationRepository(db)
	ctx := context.Background()
	wsID := "workspace-1"
	email := "buyer@example.com"
	now := time.Now().UTC()

	seed := func(id, status string, age time.Duration, customerEmail string) {
		t.Helper()
		if err := db.Exec(`INSERT INTO support_conversations
			(id, workspace_id, status, customer_email, updated_at, created_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			id, wsID, status, customerEmail, now.Add(-age), now.Add(-age),
		).Error; err != nil {
			t.Fatalf("seed conversation %s: %v", id, err)
		}
	}

	seed("open-recent", model.SupportConversationStatusOpen, time.Hour, email)
	seed("resolved-recent", model.SupportConversationStatusResolved, time.Hour, email)
	seed("open-old", model.SupportConversationStatusOpen, 40*24*time.Hour, email)
	seed("open-other-email", model.SupportConversationStatusOpen, time.Hour, "other@example.com")

	since := now.Add(-30 * 24 * time.Hour)
	got, err := repo.ListActiveByCustomerEmail(ctx, wsID, "BUYER@example.com", nil, since, 2)
	if err != nil {
		t.Fatalf("ListActiveByCustomerEmail: %v", err)
	}
	if len(got) != 1 || got[0].ID != "open-recent" {
		t.Fatalf("got %d matches, want open-recent; got=%+v", len(got), got)
	}
}

// insertMessage is a small helper for the tests below — it inserts a row
// directly via SQL so we can vary the columns the repo cares about
// (sender_type, sender_user_id, message_type, is_internal) without
// dragging in every other field on the model.
func insertMessage(t *testing.T, db *gorm.DB, m model.SupportMessage) {
	t.Helper()
	if m.MessageType == "" {
		m.MessageType = "reply"
	}
	if m.Metadata == "" {
		m.Metadata = "{}"
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	var deletedAt any
	if m.DeletedAt.Valid {
		deletedAt = m.DeletedAt.Time
	}
	if err := db.Exec(`INSERT INTO support_messages
		(id, workspace_id, conversation_id, sender_type, message_type, sender_user_id,
		 system_event_type, sender_display_name, content, is_internal, metadata, cancellable_until, deleted_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.WorkspaceID, m.ConversationID, m.SenderType, m.MessageType,
		m.SenderUserID, m.SystemEventType, m.SenderDisplayName, m.Content, m.IsInternal, m.Metadata, m.CancellableUntil,
		deletedAt, m.CreatedAt,
	).Error; err != nil {
		t.Fatalf("insert message %s: %v", m.ID, err)
	}
	var conversation model.SupportConversation
	if err := db.Where("id = ?", m.ConversationID).First(&conversation).Error; err == nil {
		conversation.ApplyMessageProjection(m)
		if err := db.Model(&model.SupportConversation{}).Where("id = ?", m.ConversationID).Updates(map[string]any{
			"list_last_message_id":              conversation.ListLastMessageID,
			"list_last_message_at":              conversation.ListLastMessageAt,
			"list_last_message_preview":         conversation.ListLastMessagePreview,
			"list_last_message_is_internal":     conversation.ListLastMessageIsInternal,
			"last_public_message_id":            conversation.LastPublicMessageID,
			"last_public_message_at":            conversation.LastPublicMessageAt,
			"last_public_sender_type":           conversation.LastPublicSenderType,
			"last_public_sender_display_name":   conversation.LastPublicSenderDisplayName,
			"last_customer_message_id":          conversation.LastCustomerMessageID,
			"last_customer_message_at":          conversation.LastCustomerMessageAt,
			"unanswered_customer_message_count": conversation.UnansweredCustomerMessageCount,
			"customer_awaiting_response":        conversation.CustomerAwaitingResponse,
			"needs_human_reply":                 conversation.NeedsHumanReply,
			"support_state_version":             conversation.SupportStateVersion,
		}).Error; err != nil {
			t.Fatalf("project message %s: %v", m.ID, err)
		}
	}
}

func insertConversation(t *testing.T, db *gorm.DB, c model.SupportConversation) {
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
		(id, workspace_id, anonymous_id, display_id, subject, status, priority,
		 channel, source, contact_last_seen_at, support_state_version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.WorkspaceID, c.AnonymousID, c.DisplayID, c.Subject, c.Status,
		c.Priority, c.Channel, c.Source, c.ContactLastSeenAt, c.SupportStateVersion, c.CreatedAt, c.UpdatedAt,
	).Error; err != nil {
		t.Fatalf("insert conversation %s: %v", c.ID, err)
	}
}

func TestSupportMessageRepository_GetMessageForActor(t *testing.T) {
	db := setupSupportMessageTestDB(t)
	repo := NewSupportMessageRepository(db)
	ctx := context.Background()

	tests := []struct {
		name    string
		seed    model.SupportMessage
		queryAs string // user ID we look up as
		wantHit bool
	}{
		{
			name: "owned reply is returned",
			seed: model.SupportMessage{
				ID: "m1", WorkspaceID: "w", ConversationID: "c",
				SenderType: "user", SenderUserID: strPtr("u1"),
				Content: "hi", MessageType: "reply",
			},
			queryAs: "u1",
			wantHit: true,
		},
		{
			name: "not owned (different user)",
			seed: model.SupportMessage{
				ID: "m2", WorkspaceID: "w", ConversationID: "c",
				SenderType: "user", SenderUserID: strPtr("u1"),
				Content: "hi", MessageType: "reply",
			},
			queryAs: "u2",
			wantHit: false,
		},
		{
			name: "AI message is never owned by a human user",
			seed: model.SupportMessage{
				ID: "m3", WorkspaceID: "w", ConversationID: "c",
				SenderType: "ai", SenderUserID: nil,
				Content: "hi", MessageType: "reply",
			},
			queryAs: "u1",
			wantHit: false,
		},
		{
			name: "internal note excluded",
			seed: model.SupportMessage{
				ID: "m4", WorkspaceID: "w", ConversationID: "c",
				SenderType: "user", SenderUserID: strPtr("u1"),
				Content: "note", MessageType: "reply", IsInternal: true,
			},
			queryAs: "u1",
			wantHit: false,
		},
		{
			name: "system message excluded",
			seed: model.SupportMessage{
				ID: "m5", WorkspaceID: "w", ConversationID: "c",
				SenderType: "user", SenderUserID: strPtr("u1"),
				Content: "sys", MessageType: "system",
			},
			queryAs: "u1",
			wantHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			insertMessage(t, db, tt.seed)
			got, err := repo.GetMessageForActor(ctx, tt.seed.ID, tt.queryAs)
			if err != nil {
				t.Fatalf("GetMessageForActor: %v", err)
			}
			if tt.wantHit && got == nil {
				t.Fatalf("expected hit, got nil")
			}
			if !tt.wantHit && got != nil {
				t.Fatalf("expected miss, got %+v", got)
			}
		})
	}
}

func TestSupportMessageRepository_SoftDeleteMessage(t *testing.T) {
	db := setupSupportMessageTestDB(t)
	repo := NewSupportMessageRepository(db)
	ctx := context.Background()

	insertMessage(t, db, model.SupportMessage{
		ID: "m1", WorkspaceID: "w", ConversationID: "c",
		SenderType: "user", SenderUserID: strPtr("u1"),
		Content: "hi", MessageType: "reply",
	})

	// owned soft-delete writes deleted_at
	if err := repo.SoftDeleteMessage(ctx, "m1", "u1"); err != nil {
		t.Fatalf("SoftDeleteMessage owned: %v", err)
	}
	var deletedAt sql.NullTime
	if err := db.Raw(`SELECT deleted_at FROM support_messages WHERE id = ?`, "m1").Scan(&deletedAt).Error; err != nil {
		t.Fatalf("read deleted_at m1: %v", err)
	}
	if !deletedAt.Valid {
		t.Fatalf("expected deleted_at to be set on m1")
	}

	// once soft-deleted, the actor lookup misses (filtered by deleted_at)
	got, err := repo.GetMessageForActor(ctx, "m1", "u1")
	if err != nil {
		t.Fatalf("GetMessageForActor after delete: %v", err)
	}
	if got != nil {
		t.Fatalf("expected soft-deleted row to be filtered, got %+v", got)
	}

	// not-owned soft-delete is a no-op (no row matches predicate)
	insertMessage(t, db, model.SupportMessage{
		ID: "m2", WorkspaceID: "w", ConversationID: "c",
		SenderType: "user", SenderUserID: strPtr("u1"),
		Content: "hi", MessageType: "reply",
	})
	if err := repo.SoftDeleteMessage(ctx, "m2", "stranger"); err != nil {
		t.Fatalf("SoftDeleteMessage not-owned: %v", err)
	}
	deletedAt = sql.NullTime{}
	if err := db.Raw(`SELECT deleted_at FROM support_messages WHERE id = ?`, "m2").Scan(&deletedAt).Error; err != nil {
		t.Fatalf("read deleted_at m2: %v", err)
	}
	if deletedAt.Valid {
		t.Fatalf("expected non-owned soft-delete to be a no-op (deleted_at = %v)", deletedAt.Time)
	}
}

func TestSupportMessageRepository_SetCancellableUntil(t *testing.T) {
	db := setupSupportMessageTestDB(t)
	repo := NewSupportMessageRepository(db)
	ctx := context.Background()

	insertMessage(t, db, model.SupportMessage{
		ID: "m1", WorkspaceID: "w", ConversationID: "c",
		SenderType: "user", SenderUserID: strPtr("u1"),
		Content: "hi", MessageType: "reply",
	})

	want := time.Now().UTC().Add(2 * time.Minute).Truncate(time.Second)
	if err := repo.SetCancellableUntil(ctx, "m1", want); err != nil {
		t.Fatalf("SetCancellableUntil: %v", err)
	}
	var got *time.Time
	if err := db.Raw(`SELECT cancellable_until FROM support_messages WHERE id = ?`, "m1").Scan(&got).Error; err != nil {
		t.Fatalf("read cancellable_until: %v", err)
	}
	if got == nil || !got.UTC().Equal(want) {
		t.Fatalf("cancellable_until: got %v want %v", got, want)
	}
}

func TestSupportConversationRepository_ListByAnonymousIDIgnoresSoftDeletedMessages(t *testing.T) {
	db := setupSupportConversationMessageTestDB(t)
	repo := NewSupportConversationRepository(db)
	ctx := context.Background()
	base := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)

	insertConversation(t, db, model.SupportConversation{
		ID:                "c1",
		WorkspaceID:       "w",
		AnonymousID:       strPtr("anon-1"),
		DisplayID:         1,
		Subject:           "conversation",
		ContactLastSeenAt: supportMessageTimePtr(base),
		CreatedAt:         base,
		UpdatedAt:         base.Add(3 * time.Minute),
	})
	insertMessage(t, db, model.SupportMessage{
		ID:             "visible",
		WorkspaceID:    "w",
		ConversationID: "c1",
		SenderType:     "user",
		SenderUserID:   strPtr("u1"),
		Content:        "visible reply",
		MessageType:    "reply",
		CreatedAt:      base.Add(time.Minute),
	})
	insertMessage(t, db, model.SupportMessage{
		ID:             "deleted",
		WorkspaceID:    "w",
		ConversationID: "c1",
		SenderType:     "user",
		SenderUserID:   strPtr("u1"),
		Content:        "deleted reply",
		MessageType:    "reply",
		CreatedAt:      base.Add(2 * time.Minute),
		DeletedAt:      gorm.DeletedAt{Time: base.Add(3 * time.Minute), Valid: true},
	})

	conversations, err := repo.ListByAnonymousID(ctx, "w", "anon-1")
	if err != nil {
		t.Fatalf("ListByAnonymousID: %v", err)
	}
	if len(conversations) != 1 {
		t.Fatalf("conversations = %d, want 1", len(conversations))
	}
	if conversations[0].LastMessage == nil || *conversations[0].LastMessage != "visible reply" {
		t.Fatalf("last_message = %v, want visible reply", conversations[0].LastMessage)
	}
	if conversations[0].UnreadCount != 1 {
		t.Fatalf("unread_count = %d, want 1", conversations[0].UnreadCount)
	}
}

func TestSupportConversationRepository_ListIncludesLatestPublicMessageSenderMetadata(t *testing.T) {
	db := setupSupportConversationMessageTestDB(t)
	repo := NewSupportConversationRepository(db)
	ctx := context.Background()
	base := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	displayName := "Rosa Marin"

	insertConversation(t, db, model.SupportConversation{
		ID:          "c1",
		WorkspaceID: "w",
		DisplayID:   1,
		Subject:     "conversation",
		CreatedAt:   base,
		UpdatedAt:   base.Add(3 * time.Minute),
	})
	insertMessage(t, db, model.SupportMessage{
		ID:             "customer",
		WorkspaceID:    "w",
		ConversationID: "c1",
		SenderType:     "customer",
		Content:        "customer question",
		MessageType:    "reply",
		CreatedAt:      base.Add(time.Minute),
	})
	insertMessage(t, db, model.SupportMessage{
		ID:                "agent",
		WorkspaceID:       "w",
		ConversationID:    "c1",
		SenderType:        "user",
		SenderUserID:      strPtr("u1"),
		SenderDisplayName: &displayName,
		Content:           "visible agent reply",
		MessageType:       "reply",
		CreatedAt:         base.Add(2 * time.Minute),
	})

	conversations, total, err := repo.List(ctx, ConversationRepositoryListParams{
		ConversationListParams: ConversationListParams{WorkspaceID: "w"},
		Role:                   model.RoleOwner,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 || len(conversations) != 1 {
		t.Fatalf("total=%d len=%d, want 1", total, len(conversations))
	}
	got := conversations[0]
	if got.LastMessage == nil || *got.LastMessage != "visible agent reply" {
		t.Fatalf("last_message = %v, want visible agent reply", got.LastMessage)
	}
	if got.LastMessageSenderType == nil || *got.LastMessageSenderType != "user" {
		t.Fatalf("last_message_sender_type = %v, want user", got.LastMessageSenderType)
	}
	if got.LastMessageSenderDisplayName == nil || *got.LastMessageSenderDisplayName != displayName {
		t.Fatalf("last_message_sender_display_name = %v, want %q", got.LastMessageSenderDisplayName, displayName)
	}
}

func TestSupportConversationRepository_GetByIDForUserReturnsPersonalUnread(t *testing.T) {
	db := setupSupportConversationMessageTestDB(t)
	repo := NewSupportConversationRepository(db)
	insertConversation(t, db, model.SupportConversation{ID: "personal-detail", WorkspaceID: "w", DisplayID: 1, Subject: "detail", SupportStateVersion: 1})
	if err := db.Exec(`INSERT INTO support_conversation_user_states
		(workspace_id, conversation_id, user_id, unread_customer_message_count, manually_unread, relevance_mask, version)
		VALUES ('w', 'personal-detail', 'u1', 2, 0, ?, 7)`, model.SupportRelevanceAssignee).Error; err != nil {
		t.Fatal(err)
	}

	conversation, err := repo.GetByIDForUser(context.Background(), "w", "personal-detail", "u1", "", model.RoleOwner)
	if err != nil {
		t.Fatal(err)
	}
	if conversation.UnreadCount != 2 || conversation.PersonalStateVersion != 7 {
		t.Fatalf("personal detail = unread:%d version:%d", conversation.UnreadCount, conversation.PersonalStateVersion)
	}

	conversation, err = repo.GetByIDForUser(context.Background(), "w", "personal-detail", "u2", "", model.RoleOwner)
	if err != nil {
		t.Fatal(err)
	}
	if conversation.UnreadCount != 0 {
		t.Fatalf("other user unread = %d, want 0", conversation.UnreadCount)
	}
}

func TestSupportConversationRepository_ListPreviewIgnoresSystemEventsButKeepsInternalNotes(t *testing.T) {
	db := setupSupportConversationMessageTestDB(t)
	repo := NewSupportConversationRepository(db)
	ctx := context.Background()
	base := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	resolvedEvent := "resolved"

	insertConversation(t, db, model.SupportConversation{
		ID:          "status-event",
		WorkspaceID: "w",
		DisplayID:   1,
		Subject:     "status event",
		CreatedAt:   base,
		UpdatedAt:   base.Add(3 * time.Minute),
	})
	insertMessage(t, db, model.SupportMessage{
		ID:             "customer-reply",
		WorkspaceID:    "w",
		ConversationID: "status-event",
		SenderType:     "customer",
		Content:        "real customer reply",
		MessageType:    "reply",
		CreatedAt:      base.Add(time.Minute),
	})
	insertMessage(t, db, model.SupportMessage{
		ID:              "resolved-event",
		WorkspaceID:     "w",
		ConversationID:  "status-event",
		SenderType:      "system",
		Content:         "Resolved conversation",
		MessageType:     "system",
		SystemEventType: &resolvedEvent,
		IsInternal:      true,
		CreatedAt:       base.Add(2 * time.Minute),
	})

	insertConversation(t, db, model.SupportConversation{
		ID:          "internal-note",
		WorkspaceID: "w",
		DisplayID:   2,
		Subject:     "internal note",
		CreatedAt:   base,
		UpdatedAt:   base.Add(4 * time.Minute),
	})
	insertMessage(t, db, model.SupportMessage{
		ID:             "older-public",
		WorkspaceID:    "w",
		ConversationID: "internal-note",
		SenderType:     "customer",
		Content:        "public question",
		MessageType:    "reply",
		CreatedAt:      base.Add(time.Minute),
	})
	insertMessage(t, db, model.SupportMessage{
		ID:             "real-note",
		WorkspaceID:    "w",
		ConversationID: "internal-note",
		SenderType:     "user",
		SenderUserID:   strPtr("u1"),
		Content:        "check billing context",
		MessageType:    "reply",
		IsInternal:     true,
		CreatedAt:      base.Add(3 * time.Minute),
	})

	insertConversation(t, db, model.SupportConversation{
		ID:          "empty-internal-handoff",
		WorkspaceID: "w",
		DisplayID:   3,
		Subject:     "empty internal handoff",
		CreatedAt:   base,
		UpdatedAt:   base.Add(5 * time.Minute),
	})
	insertMessage(t, db, model.SupportMessage{
		ID:                "ai-handoff-reply",
		WorkspaceID:       "w",
		ConversationID:    "empty-internal-handoff",
		SenderType:        "ai",
		SenderDisplayName: strPtr("Helpin AI"),
		Content:           "Let me connect you with a team member who can help further.",
		MessageType:       "reply",
		CreatedAt:         base.Add(time.Minute),
	})
	insertMessage(t, db, model.SupportMessage{
		ID:             "empty-handoff-note",
		WorkspaceID:    "w",
		ConversationID: "empty-internal-handoff",
		SenderType:     "agent",
		Content:        "",
		MessageType:    "reply",
		IsInternal:     true,
		CreatedAt:      base.Add(4 * time.Minute),
	})

	conversations, total, err := repo.List(ctx, ConversationRepositoryListParams{
		ConversationListParams: ConversationListParams{WorkspaceID: "w"},
		Role:                   model.RoleOwner,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 3 || len(conversations) != 3 {
		t.Fatalf("total=%d len=%d, want 3", total, len(conversations))
	}

	byID := map[string]model.SupportConversation{}
	for _, conversation := range conversations {
		byID[conversation.ID] = conversation
	}
	if got := byID["status-event"].LastMessage; got == nil || *got != "real customer reply" {
		t.Fatalf("status event last_message = %v, want real customer reply", got)
	}
	if got := byID["internal-note"].ListLastActivityAt; got == nil || !got.Equal(base.Add(3*time.Minute)) {
		t.Fatalf("internal note activity timestamp = %v, want note time", got)
	}
	if got := byID["status-event"].ListLastActivityAt; got == nil || !got.Equal(base.Add(2*time.Minute)) {
		t.Fatalf("system event activity timestamp = %v, want status time", got)
	}
	if got := byID["internal-note"].LastMessage; got == nil || *got != "Note: check billing context" {
		t.Fatalf("internal note last_message = %v, want prefixed internal note", got)
	}
	if got := byID["empty-internal-handoff"].LastMessage; got == nil || *got != "Let me connect you with a team member who can help further." {
		t.Fatalf("empty internal handoff last_message = %v, want public handoff reply", got)
	}
}

func TestSupportConversationRepository_MarkContactReadIgnoresSoftDeletedMessages(t *testing.T) {
	db := setupSupportConversationMessageTestDB(t)
	repo := NewSupportConversationRepository(db)
	ctx := context.Background()
	base := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)

	insertConversation(t, db, model.SupportConversation{
		ID:                "c1",
		WorkspaceID:       "w",
		AnonymousID:       strPtr("anon-1"),
		DisplayID:         1,
		Subject:           "conversation",
		ContactLastSeenAt: supportMessageTimePtr(base),
		CreatedAt:         base,
		UpdatedAt:         base,
	})
	insertMessage(t, db, model.SupportMessage{
		ID:             "deleted",
		WorkspaceID:    "w",
		ConversationID: "c1",
		SenderType:     "user",
		SenderUserID:   strPtr("u1"),
		Content:        "deleted reply",
		MessageType:    "reply",
		CreatedAt:      base.Add(time.Minute),
		DeletedAt:      gorm.DeletedAt{Time: base.Add(2 * time.Minute), Valid: true},
	})

	if err := repo.MarkContactRead(ctx, "c1"); err != nil {
		t.Fatalf("MarkContactRead: %v", err)
	}
	var got time.Time
	if err := db.Raw(`SELECT contact_last_seen_at FROM support_conversations WHERE id = ?`, "c1").Scan(&got).Error; err != nil {
		t.Fatalf("read contact_last_seen_at: %v", err)
	}
	if !got.UTC().Equal(base) {
		t.Fatalf("contact_last_seen_at changed to %v, want %v", got, base)
	}
}

func supportMessageTimePtr(t time.Time) *time.Time {
	return &t
}

func TestSupportConversationRepository_ListOrdersByTimelineActivity(t *testing.T) {
	db := setupSupportConversationMessageTestDB(t)
	repo := NewSupportConversationRepository(db)
	base := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	for i, id := range []string{"older", "empty", "newer"} {
		insertConversation(t, db, model.SupportConversation{ID: id, WorkspaceID: "w", DisplayID: i + 1, CreatedAt: base.Add(time.Duration(i) * time.Hour), UpdatedAt: base.Add(time.Duration(12-i) * time.Hour)})
		if id != "empty" {
			insertMessage(t, db, model.SupportMessage{ID: id + "-reply", WorkspaceID: "w", ConversationID: id, SenderType: "customer", Content: "question", CreatedAt: base})
			if id == "newer" {
				insertMessage(t, db, model.SupportMessage{ID: id + "-status", WorkspaceID: "w", ConversationID: id, SenderType: "system", MessageType: "system", SystemEventType: strPtr("resolved"), CreatedAt: base.Add(2 * time.Hour)})
			}
		}
	}
	// A deleted timeline entry must not make an old conversation look recent.
	insertMessage(t, db, model.SupportMessage{ID: "deleted-status", WorkspaceID: "w", ConversationID: "older", SenderType: "system", MessageType: "system", SystemEventType: strPtr("resolved"), CreatedAt: base.Add(20 * time.Hour), DeletedAt: gorm.DeletedAt{Time: base.Add(21 * time.Hour), Valid: true}})
	detail, err := repo.GetByIDForUser(context.Background(), "w", "newer", "user", "", model.RoleOwner)
	if err != nil {
		t.Fatal(err)
	}
	if detail.ListLastActivityAt == nil || !detail.ListLastActivityAt.Equal(base.Add(2*time.Hour)) {
		t.Fatalf("detail activity = %v", detail.ListLastActivityAt)
	}
	legacyDetail, err := repo.GetByID(context.Background(), "w", "newer", "", model.RoleOwner)
	if err != nil {
		t.Fatal(err)
	}
	if legacyDetail.ListLastActivityAt == nil || !legacyDetail.ListLastActivityAt.Equal(base.Add(2*time.Hour)) {
		t.Fatalf("legacy detail activity = %v", legacyDetail.ListLastActivityAt)
	}
	for _, sortOrder := range []string{"newest", "oldest"} {
		t.Run(sortOrder, func(t *testing.T) {
			got, total, err := repo.List(context.Background(), ConversationRepositoryListParams{ConversationListParams: ConversationListParams{WorkspaceID: "w", Sort: sortOrder}, Role: model.RoleOwner})
			if err != nil {
				t.Fatal(err)
			}
			if total != 3 || len(got) != 3 {
				t.Fatalf("got %d rows, total %d", len(got), total)
			}
			want := []string{"newer", "empty", "older"}
			if sortOrder == "oldest" {
				want = []string{"older", "empty", "newer"}
			}
			for _, conversation := range got {
				if conversation.ID == "newer" {
					if conversation.ListLastActivityAt == nil || !conversation.ListLastActivityAt.Equal(base.Add(2*time.Hour)) {
						t.Errorf("activity timestamp = %v, want status event time", conversation.ListLastActivityAt)
					}
					if conversation.ListLastMessageAt == nil || !conversation.ListLastMessageAt.Equal(base) {
						t.Errorf("message timestamp changed: %v", conversation.ListLastMessageAt)
					}
				}
			}
			for i := range want {
				if got[i].ID != want[i] {
					t.Errorf("row %d = %s, want %s", i, got[i].ID, want[i])
				}
			}
		})
	}
}
