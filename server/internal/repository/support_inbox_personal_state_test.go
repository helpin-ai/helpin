package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestConversationListProjectionSelectUsesMaterializedState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	repo := NewSupportConversationRepository(db)
	selectSQL := repo.conversationListProjectionSelect()
	for _, expected := range []string{
		"list_last_message_preview",
		"last_public_sender_type",
		"customer_awaiting_response",
		"unread_customer_message_count",
		"visitor_country_code",
	} {
		if !strings.Contains(selectSQL, expected) {
			t.Errorf("projection select missing %q: %s", expected, selectSQL)
		}
	}
	if !strings.Contains(selectSQL, "support_state_version = 0") {
		t.Fatalf("projection select must retain a guarded legacy fallback during backfill: %s", selectSQL)
	}
	if strings.Count(selectSQL, "support_state_version = 0") < 4 {
		t.Fatalf("projection fallbacks are not consistently guarded: %s", selectSQL)
	}
}

func setupSupportPersonalStateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:support_personal_state_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE support_conversations (
			id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, mailbox_id TEXT,
			status TEXT NOT NULL DEFAULT 'open', flow_state TEXT, ai_state TEXT,
			human_takeover BOOLEAN NOT NULL DEFAULT 0, customer_requested_human_at DATETIME,
			assigned_user_id TEXT, opened_by_user_id TEXT, assigned_agent_id TEXT,
			last_public_sender_type TEXT, customer_awaiting_response BOOLEAN NOT NULL DEFAULT 0,
			team_last_seen_at DATETIME, needs_human_reply BOOLEAN NOT NULL DEFAULT 0,
			support_state_version INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE support_messages (
			id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, conversation_id TEXT NOT NULL,
			sender_type TEXT NOT NULL, message_type TEXT NOT NULL, system_event_type TEXT,
			is_internal BOOLEAN NOT NULL DEFAULT 0, created_at DATETIME NOT NULL, deleted_at DATETIME
		)`,
		`CREATE TABLE support_conversation_user_states (
			workspace_id TEXT NOT NULL, conversation_id TEXT NOT NULL, user_id TEXT NOT NULL,
			last_read_customer_message_id TEXT, last_read_customer_message_at DATETIME,
			unread_customer_message_count INTEGER NOT NULL DEFAULT 0,
			manually_unread BOOLEAN NOT NULL DEFAULT 0, mentioned_at DATETIME,
			relevance_mask INTEGER NOT NULL DEFAULT 0, version INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (conversation_id, user_id)
		)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}

func TestMarkPersonalReadOnlyChangesRequestingUserThroughRenderedMessage(t *testing.T) {
	db := setupSupportPersonalStateTestDB(t)
	repo := NewSupportConversationRepository(db)
	ctx := context.Background()
	t0 := time.Date(2026, 9, 2, 9, 0, 0, 0, time.UTC)
	if err := db.Exec(`INSERT INTO support_conversations (id, workspace_id) VALUES ('c1', 'w1')`).Error; err != nil {
		t.Fatal(err)
	}
	for id, createdAt := range map[string]time.Time{"m1": t0, "m2": t0.Add(time.Minute)} {
		if err := db.Exec(`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, message_type, is_internal, created_at) VALUES (?, 'w1', 'c1', 'customer', 'reply', 0, ?)`, id, createdAt).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, userID := range []string{"u1", "u2"} {
		if err := db.Exec(`INSERT INTO support_conversation_user_states (workspace_id, conversation_id, user_id, unread_customer_message_count, relevance_mask) VALUES ('w1', 'c1', ?, 2, ?)`, userID, model.SupportRelevanceAssignee).Error; err != nil {
			t.Fatal(err)
		}
	}

	state, err := repo.MarkPersonalRead(ctx, "w1", "c1", "u1", "m1")
	if err != nil {
		t.Fatalf("mark personal read: %v", err)
	}
	if state.UnreadCustomerMessageCount != 1 || state.LastReadCustomerMessageID == nil || *state.LastReadCustomerMessageID != "m1" {
		t.Fatalf("requesting state = %#v, want cursor m1 with one newer reply", state)
	}
	var other model.SupportConversationUserState
	if err := db.Where("conversation_id = ? AND user_id = ?", "c1", "u2").First(&other).Error; err != nil {
		t.Fatal(err)
	}
	if other.UnreadCustomerMessageCount != 2 || other.LastReadCustomerMessageID != nil {
		t.Fatalf("other user's state changed: %#v", other)
	}
}

func TestMarkPersonalReadNeverRegressesCursor(t *testing.T) {
	db := setupSupportPersonalStateTestDB(t)
	repo := NewSupportConversationRepository(db)
	ctx := context.Background()
	t0 := time.Date(2026, 9, 2, 9, 0, 0, 0, time.UTC)
	if err := db.Exec(`INSERT INTO support_conversations (id, workspace_id) VALUES ('c1', 'w1')`).Error; err != nil {
		t.Fatal(err)
	}
	for id, createdAt := range map[string]time.Time{"m1": t0, "m2": t0.Add(time.Minute)} {
		if err := db.Exec(`INSERT INTO support_messages (id, workspace_id, conversation_id, sender_type, message_type, is_internal, created_at) VALUES (?, 'w1', 'c1', 'customer', 'reply', 0, ?)`, id, createdAt).Error; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.MarkPersonalRead(ctx, "w1", "c1", "u1", "m2"); err != nil {
		t.Fatal(err)
	}
	state, err := repo.MarkPersonalRead(ctx, "w1", "c1", "u1", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if state.LastReadCustomerMessageID == nil || *state.LastReadCustomerMessageID != "m2" {
		t.Fatalf("cursor regressed: %#v", state.LastReadCustomerMessageID)
	}
}

func TestMarkPersonalUnreadCreatesManualReminderWithoutRelevance(t *testing.T) {
	db := setupSupportPersonalStateTestDB(t)
	repo := NewSupportConversationRepository(db)
	if err := db.Exec(`INSERT INTO support_conversations (id, workspace_id) VALUES ('c1', 'w1')`).Error; err != nil {
		t.Fatal(err)
	}

	state, err := repo.MarkPersonalUnread(context.Background(), "w1", "c1", "u1")
	if err != nil {
		t.Fatalf("mark personal unread: %v", err)
	}
	if !state.ManuallyUnread || state.RelevanceMask != 0 || state.EffectiveUnreadCount() != 1 {
		t.Fatalf("manual state = %#v", state)
	}
	duplicate, err := repo.MarkPersonalUnread(context.Background(), "w1", "c1", "u1")
	if err != nil {
		t.Fatalf("duplicate mark personal unread: %v", err)
	}
	if duplicate.Version != state.Version {
		t.Fatalf("duplicate request advanced version from %d to %d", state.Version, duplicate.Version)
	}
}

func TestGetUnreadStatsUsesRequestingUsersMaterializedState(t *testing.T) {
	db := setupSupportPersonalStateTestDB(t)
	repo := NewSupportConversationRepository(db)
	if err := db.Exec(`INSERT INTO support_conversations (id, workspace_id, status, assigned_user_id) VALUES ('c1', 'w1', 'open', 'u1')`).Error; err != nil {
		t.Fatal(err)
	}
	for _, userID := range []string{"u1", "u2"} {
		if err := db.Exec(`INSERT INTO support_conversation_user_states (workspace_id, conversation_id, user_id, unread_customer_message_count, relevance_mask) VALUES ('w1', 'c1', ?, 1, ?)`, userID, model.SupportRelevanceAssignee).Error; err != nil {
			t.Fatal(err)
		}
	}

	stats, err := repo.GetUnreadStats(context.Background(), "w1", "u1", "", model.RoleOwner, nil)
	if err != nil {
		t.Fatalf("get u1 stats: %v", err)
	}
	if stats.Inbox != 1 || stats.Total != 1 {
		t.Fatalf("u1 stats = %#v, want one personal unread", stats)
	}

	stats, err = repo.GetUnreadStats(context.Background(), "w1", "u3", "", model.RoleOwner, nil)
	if err != nil {
		t.Fatalf("get u3 stats: %v", err)
	}
	if stats.Inbox != 0 || stats.Total != 0 {
		t.Fatalf("u3 stats = %#v, want zero personal unread", stats)
	}
}

func TestGetUnreadStatsTotalIncludesAIHandlingUnread(t *testing.T) {
	db := setupSupportPersonalStateTestDB(t)
	repo := NewSupportConversationRepository(db)
	for _, statement := range []string{
		`INSERT INTO support_conversations (id, workspace_id, status, flow_state) VALUES ('ai-active', 'w1', 'open', 'ai_handling')`,
		`INSERT INTO support_conversations (id, workspace_id, status, flow_state) VALUES ('ai-resolved', 'w1', 'open', 'resolved_by_ai')`,
		`INSERT INTO support_conversation_user_states (workspace_id, conversation_id, user_id, unread_customer_message_count) VALUES ('w1', 'ai-active', 'u1', 1)`,
		`INSERT INTO support_conversation_user_states (workspace_id, conversation_id, user_id, unread_customer_message_count) VALUES ('w1', 'ai-resolved', 'u1', 1)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}

	stats, err := repo.GetUnreadStats(context.Background(), "w1", "u1", "", model.RoleOwner, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 1 || stats.AIActive != 1 || stats.Inbox != 0 {
		t.Fatalf("stats = %#v, want one total AI-handling unread and no human-inbox unread", stats)
	}
}

func TestCountByParamsReturnsPersonalUnreadForSavedViews(t *testing.T) {
	db := setupSupportPersonalStateTestDB(t)
	repo := NewSupportConversationRepository(db)
	if err := db.Exec(`INSERT INTO support_conversations (id, workspace_id, status) VALUES ('c1', 'w1', 'open')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO support_conversation_user_states (workspace_id, conversation_id, user_id, unread_customer_message_count, relevance_mask) VALUES ('w1', 'c1', 'u1', 1, ?)`, model.SupportRelevanceMention).Error; err != nil {
		t.Fatal(err)
	}

	params := ConversationRepositoryListParams{
		ConversationListParams: ConversationListParams{WorkspaceID: "w1", UserID: "u1"},
		Role:                   model.RoleOwner,
	}
	total, unread, err := repo.CountByParams(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || unread != 1 {
		t.Fatalf("counts = total:%d unread:%d, want 1/1", total, unread)
	}

	params.UserID = "u2"
	_, unread, err = repo.CountByParams(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if unread != 0 {
		t.Fatalf("u2 unread = %d, want 0", unread)
	}
}

func TestClearPersonalReadWithoutCustomerMessagesClearsManualReminder(t *testing.T) {
	db := setupSupportPersonalStateTestDB(t)
	repo := NewSupportConversationRepository(db)
	now := time.Now().UTC()
	if err := db.Exec(`INSERT INTO support_conversations (id, workspace_id, status, team_last_seen_at) VALUES ('c1', 'w1', 'open', ?)`, now).Error; err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	if _, err := repo.MarkPersonalUnread(context.Background(), "w1", "c1", "u1"); err != nil {
		t.Fatalf("mark unread: %v", err)
	}
	state, err := repo.ClearPersonalRead(context.Background(), "w1", "c1", "u1")
	if err != nil {
		t.Fatalf("clear read: %v", err)
	}
	if state.EffectiveUnreadCount() != 0 || state.ManuallyUnread {
		t.Fatalf("state = %#v, want cleared", state)
	}
}

func TestCountScopesAggregatesAllMailboxCountersInOneResult(t *testing.T) {
	db := setupSupportPersonalStateTestDB(t)
	repo := NewSupportMailboxRepository(db)
	for _, statement := range []string{
		`INSERT INTO support_conversations (id, workspace_id, mailbox_id, status, assigned_user_id, needs_human_reply) VALUES ('shared-conv', 'w1', NULL, 'open', 'u1', 1)`,
		`INSERT INTO support_conversations (id, workspace_id, mailbox_id, status, assigned_user_id, needs_human_reply) VALUES ('mailbox-conv', 'w1', 'm1', 'open', 'u1', 0)`,
		`INSERT INTO support_conversation_user_states (workspace_id, conversation_id, user_id, unread_customer_message_count, relevance_mask) VALUES ('w1', 'shared-conv', 'u1', 1, 1)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed scope counts: %v", err)
		}
	}
	counts, err := repo.CountScopes(context.Background(), "w1", "u1", []string{"m1"})
	if err != nil {
		t.Fatalf("count scopes: %v", err)
	}
	if counts["shared"].TotalCount != 1 || counts["shared"].UnreadCount != 1 || counts["shared"].NeedsHumanReplyCount != 1 {
		t.Fatalf("shared counts = %#v", counts["shared"])
	}
	if counts["m1"].TotalCount != 1 || counts["m1"].UnreadCount != 0 {
		t.Fatalf("mailbox counts = %#v", counts["m1"])
	}
}
