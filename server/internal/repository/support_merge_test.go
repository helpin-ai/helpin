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

func setupSupportMergeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:merge_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`CREATE TABLE support_events (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, event_type TEXT, actor_type TEXT, channel TEXT, metadata TEXT, occurred_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE support_conversations (id TEXT PRIMARY KEY, workspace_id TEXT, display_id INTEGER, subject TEXT, status TEXT, merged_into_id TEXT, merge_revision INTEGER DEFAULT 0, customer_email TEXT, email_cc TEXT, email_unsubscribed BOOLEAN DEFAULT false, primary_recipient_state TEXT, contact_last_seen_at DATETIME, ai_resolved_at DATETIME, ai_resolution_type TEXT, crm_contact_id TEXT, anonymous_id TEXT, anonymized_at DATETIME, mailbox_id TEXT, assigned_user_id TEXT, assigned_agent_id TEXT, customer_requested_human_at DATETIME, customer_awaiting_response BOOLEAN DEFAULT false, needs_human_reply BOOLEAN DEFAULT false, human_takeover BOOLEAN, ai_paused_at DATETIME, ai_paused_by_user_id TEXT, ai_active_run_id TEXT, ai_control_version INTEGER DEFAULT 0, ai_state TEXT, flow_state TEXT, resolved_at DATETIME, closed_at DATETIME, support_state_version INTEGER DEFAULT 0, updated_at DATETIME, created_at DATETIME)`,
		`CREATE TABLE support_messages (id TEXT PRIMARY KEY, workspace_id TEXT, conversation_id TEXT, origin_conversation_id TEXT, merge_revision INTEGER DEFAULT 0, sender_type TEXT, message_type TEXT, system_event_type TEXT, sender_user_id TEXT, sender_agent_id TEXT, sender_display_name TEXT, sender_avatar_url TEXT, content TEXT, is_internal BOOLEAN DEFAULT 0, metadata TEXT, via_channel TEXT, email_notified_at DATETIME, email_read_at DATETIME, cancellable_until DATETIME, deleted_at DATETIME, created_at DATETIME, updated_at DATETIME)`,
		`INSERT INTO support_conversations (id, workspace_id, subject, status, customer_email) VALUES ('a','ws','First','open','sam@example.test'), ('b','ws','Second','open','sam@example.test'), ('c','ws','Third','open','sam@example.test'), ('foreign','other','Private','open','sam@example.test')`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.AutoMigrate(&model.SupportConversationUserState{}, &model.SupportConversationMerge{}, &model.SupportMergeMessage{}, &model.SupportMergeJob{}, &model.SupportPendingSend{}, &model.SupportAIFollowUp{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMergePreservesOriginalsAndUnmergeHistory(t *testing.T) {
	ctx := context.Background()
	db := setupSupportMergeTestDB(t)
	repo := NewSupportMergeRepository(db)
	before := model.SupportMessage{ID: "before", WorkspaceID: "ws", ConversationID: "a", SenderType: "customer", MessageType: "reply", Content: "Original", Metadata: "{}"}
	if err := NewSupportMessageRepository(db).Create(ctx, &before); err != nil {
		t.Fatal(err)
	}
	merges, err := repo.Merge(ctx, "ws", "b", []string{"a"}, "actor", "operation", false)
	if err != nil || len(merges) != 1 {
		t.Fatalf("merge: %v, %v", merges, err)
	}
	during := model.SupportMessage{ID: "during", WorkspaceID: "ws", ConversationID: "a", SenderType: "customer", MessageType: "reply", Content: "Reply via original", Metadata: "{}"}
	if err := NewSupportMessageRepository(db).Create(ctx, &during); err != nil {
		t.Fatal(err)
	}
	if during.ConversationID != "b" || during.OriginConversationID == nil || *during.OriginConversationID != "a" {
		t.Fatalf("origin/routing lost: %+v", during)
	}
	if err := repo.Unmerge(ctx, "ws", merges[0].ID, "actor", "undo"); err != nil {
		t.Fatal(err)
	}
	after := model.SupportMessage{ID: "after", WorkspaceID: "ws", ConversationID: "b", SenderType: "customer", MessageType: "reply", Content: "After separation", Metadata: "{}"}
	if err := NewSupportMessageRepository(db).Create(ctx, &after); err != nil {
		t.Fatal(err)
	}
	var original model.SupportMessage
	if err := db.First(&original, "id = ?", "before").Error; err != nil || original.ConversationID != "a" {
		t.Fatalf("original moved: %v %+v", err, original)
	}
	var grants []model.SupportMergeMessage
	if err := db.Where("conversation_id = ?", "a").Find(&grants).Error; err != nil {
		t.Fatal(err)
	}
	if len(grants) != 1 || grants[0].MessageID != "during" {
		t.Fatalf("shared history boundary: %+v", grants)
	}
	var source model.SupportConversation
	if err := db.First(&source, "id = ?", "a").Error; err != nil {
		t.Fatal(err)
	}
	if source.MergedIntoID != nil || source.AIPausedAt == nil {
		t.Fatalf("source not restored with AI paused: %+v", source)
	}
}

func TestMergeRejectsForeignWorkspaceAndRollsBack(t *testing.T) {
	db := setupSupportMergeTestDB(t)
	repo := NewSupportMergeRepository(db)
	if _, err := repo.Merge(context.Background(), "ws", "b", []string{"a", "foreign"}, "actor", "op", false); err == nil {
		t.Fatal("accepted another workspace")
	}
	var count int64
	db.Model(&model.SupportConversationMerge{}).Count(&count)
	if count != 0 {
		t.Fatal("partial merge persisted")
	}
}

func TestMergeGraphIdempotencyAndSubtreeRestoration(t *testing.T) {
	ctx := context.Background()
	db := setupSupportMergeTestDB(t)
	repo := NewSupportMergeRepository(db)
	first, err := repo.Merge(ctx, "ws", "b", []string{"a"}, "actor", "op1", false)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := repo.Merge(ctx, "ws", "b", []string{"a"}, "actor", "op1", false)
	if err != nil || len(retry) != 1 || retry[0].ID != first[0].ID {
		t.Fatalf("retry: %v %v", retry, err)
	}
	if _, err := repo.Merge(ctx, "ws", "c", []string{"a"}, "actor", "op1", false); err == nil {
		t.Fatal("idempotency key reused for another operation")
	}
	second, err := repo.Merge(ctx, "ws", "c", []string{"b"}, "actor", "op2", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Merge(ctx, "ws", "a", []string{"c"}, "actor", "cycle", false); err == nil {
		t.Fatal("accepted cycle")
	}
	if err := repo.Unmerge(ctx, "ws", second[0].ID, "actor", "undo"); err != nil {
		t.Fatal(err)
	}
	root, err := repo.Root(ctx, "ws", "a")
	if err != nil || root != "b" {
		t.Fatalf("restored subtree root %q %v", root, err)
	}
}

func TestMergeSourceRejectsStaffWrites(t *testing.T) {
	ctx := context.Background()
	db := setupSupportMergeTestDB(t)
	if _, err := NewSupportMergeRepository(db).Merge(ctx, "ws", "b", []string{"a"}, "actor", "op", false); err != nil {
		t.Fatal(err)
	}
	msg := model.SupportMessage{ID: "stale", WorkspaceID: "ws", ConversationID: "a", SenderType: "user", MessageType: "reply", Content: "Stale draft", Metadata: "{}"}
	if err := NewSupportMessageRepository(db).Create(ctx, &msg); err == nil {
		t.Fatal("silently redirected staff draft")
	}
}

func TestMergeAIContextExcludesPrivateCrossCustomerHistory(t *testing.T) {
	ctx := context.Background()
	db := setupSupportMergeTestDB(t)
	repo := NewSupportMergeRepository(db)
	messages := NewSupportMessageRepository(db)
	for _, id := range []string{"a", "b"} {
		m := model.SupportMessage{ID: "private-" + id, WorkspaceID: "ws", ConversationID: id, SenderType: "customer", MessageType: "reply", Content: "Private history", Metadata: "{}"}
		if err := messages.Create(ctx, &m); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec("UPDATE support_conversations SET customer_email='other@example.test' WHERE id='b'").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Merge(ctx, "ws", "b", []string{"a"}, "actor", "op", true); err != nil {
		t.Fatal(err)
	}
	shared := model.SupportMessage{ID: "shared", WorkspaceID: "ws", ConversationID: "a", SenderType: "customer", MessageType: "reply", Content: "Shared discussion", Metadata: "{}"}
	if err := messages.Create(ctx, &shared); err != nil {
		t.Fatal(err)
	}
	history, err := messages.ListForSupportAI(ctx, "ws", "b", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].ID != "shared" {
		t.Fatalf("AI received private history: %+v", history)
	}
}

func TestMergeRejectsStalePrimaryDraft(t *testing.T) {
	ctx := context.Background()
	db := setupSupportMergeTestDB(t)
	if _, err := NewSupportMergeRepository(db).Merge(ctx, "ws", "b", []string{"a"}, "actor", "op", false); err != nil {
		t.Fatal(err)
	}
	revision := int64(0)
	msg := model.SupportMessage{ID: "draft", WorkspaceID: "ws", ConversationID: "b", SenderType: "user", MessageType: "reply", Content: "Previously composed", Metadata: "{}", ExpectedMergeRevision: &revision}
	if err := NewSupportMessageRepository(db).Create(ctx, &msg); err == nil {
		t.Fatal("stale draft acquired new recipients")
	}
	revision = 1
	if err := NewSupportMessageRepository(db).Create(ctx, &msg); err != nil {
		t.Fatalf("reviewed draft rejected: %v", err)
	}
}

func TestMergeQueuesDurableRunCancellation(t *testing.T) {
	db := setupSupportMergeTestDB(t)
	if err := db.Exec("UPDATE support_conversations SET ai_active_run_id='old-run' WHERE id='a'").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := NewSupportMergeRepository(db).Merge(context.Background(), "ws", "b", []string{"a"}, "actor", "op", false); err != nil {
		t.Fatal(err)
	}
	var jobs []model.SupportMergeJob
	if err := db.Where("run_id = ?", "old-run").Find(&jobs).Error; err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].ConversationID != "a" || jobs[0].CompletedAt != nil {
		t.Fatalf("lost durable cancellation: %+v", jobs)
	}
}

func TestMergeFrozenAudienceHistoryAndReadReceipts(t *testing.T) {
	db := setupSupportMergeTestDB(t)
	ctx := context.Background()
	repo := NewSupportMergeRepository(db)
	messages := NewSupportMessageRepository(db)
	if err := db.Exec(`UPDATE support_conversations SET email_cc='{cc@example.test}', anonymous_id='visitor-a' WHERE id='a'`).Error; err != nil {
		t.Fatal(err)
	}
	edges, err := repo.Merge(ctx, "ws", "b", []string{"a"}, "actor", "op", false)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 105; i++ {
		msg := model.SupportMessage{ID: fmt.Sprintf("message-%03d", i), WorkspaceID: "ws", ConversationID: "a", SenderType: "customer", MessageType: "reply", Content: "New shared reply", Metadata: "{}"}
		if err := messages.Create(ctx, &msg); err != nil {
			t.Fatal(err)
		}
	}
	shared, more, err := repo.SharedMessages(ctx, "ws", "a", "", true, nil, 0)
	if err != nil || more || len(shared) != 105 {
		t.Fatalf("full widget history: %d %v %v", len(shared), more, err)
	}
	loaded, err := messages.GetByID(ctx, "message-104")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.WidgetConversationIDs) != 1 || loaded.WidgetConversationIDs[0] != "a" {
		t.Fatalf("reload lost widget audience: %+v", loaded.WidgetConversationIDs)
	}
	if !strings.Contains(loaded.Metadata, "cc@example.test") {
		t.Fatalf("explicit CC missing: %s", loaded.Metadata)
	}
	revision := int64(1)
	outgoing := model.SupportMessage{ID: "staff-reply", CreatedAt: time.Now().Add(-time.Second), WorkspaceID: "ws", ConversationID: "b", SenderType: "user", MessageType: "reply", Content: "Reply", Metadata: `{"delivery_mode":"chat_only"}`, ExpectedMergeRevision: &revision}
	if err := messages.Create(ctx, &outgoing); err != nil {
		t.Fatal(err)
	}
	if err := NewSupportConversationRepository(db).MarkContactRead(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	read, err := messages.GetByID(ctx, outgoing.ID)
	if err != nil || read.WidgetReadAt == nil {
		t.Fatalf("shared widget receipt missing: %+v %v", read, err)
	}
	if err := repo.Unmerge(ctx, "ws", edges[0].ID, "actor", "undo"); err != nil {
		t.Fatal(err)
	}
	after := model.SupportMessage{ID: "after", WorkspaceID: "ws", ConversationID: "b", SenderType: "customer", MessageType: "reply", Content: "Private again", Metadata: "{}"}
	if err := messages.Create(ctx, &after); err != nil {
		t.Fatal(err)
	}
	retained, _, err := repo.SharedMessages(ctx, "ws", "a", edges[0].ID, true, nil, 0)
	if err != nil || len(retained) != 106 {
		t.Fatalf("ended interval widened: %d %v", len(retained), err)
	}
}

func TestMergeRetainsHumanRequestAndDetachesErasedSources(t *testing.T) {
	db := setupSupportMergeTestDB(t)
	ctx := context.Background()
	repo := NewSupportMergeRepository(db)
	now := time.Now().UTC()
	if err := db.Exec("UPDATE support_conversations SET customer_requested_human_at=?, human_takeover=true WHERE id='a'", now).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Merge(ctx, "ws", "b", []string{"a"}, "actor", "op", false); err != nil {
		t.Fatal(err)
	}
	var primary model.SupportConversation
	if err := db.Where("id='b'").First(&primary).Error; err != nil {
		t.Fatal(err)
	}
	if primary.CustomerRequestedHumanAt == nil || primary.HumanTakeover == nil || !*primary.HumanTakeover {
		t.Fatal("merge lost the human request")
	}
	if err := repo.Transaction(ctx, "ws", func(tx *SupportMergeRepository) error { return tx.DetachErasedConversations(ctx, "ws", []string{"a"}) }); err != nil {
		t.Fatal(err)
	}
	edges, err := repo.Active(ctx, "ws")
	if err != nil || len(edges) != 0 {
		t.Fatalf("erased source remains connected: %v %v", edges, err)
	}
}

func TestMergePreservesPersonalUnreadUntilPrimaryRead(t *testing.T) {
	ctx := context.Background()
	db := setupSupportMergeTestDB(t)
	state := model.SupportConversationUserState{WorkspaceID: "ws", ConversationID: "a", UserID: "reader", UnreadCustomerMessageCount: 2}
	if err := db.Create(&state).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewSupportMergeRepository(db)
	edges, err := repo.Merge(ctx, "ws", "b", []string{"a"}, "actor", "unread-merge", false)
	if err != nil {
		t.Fatal(err)
	}
	var root model.SupportConversationUserState
	if err := db.First(&root, "conversation_id = ? AND user_id = ?", "b", "reader").Error; err != nil || !root.ManuallyUnread {
		t.Fatalf("primary unread lost: %+v %v", root, err)
	}
	if err := repo.Unmerge(ctx, "ws", edges[0].ID, "actor", "unread-restore"); err != nil {
		t.Fatal(err)
	}
	var original model.SupportConversationUserState
	if err := db.First(&original, "conversation_id = ? AND user_id = ?", "a", "reader").Error; err != nil || original.UnreadCustomerMessageCount != 2 {
		t.Fatalf("original read cursor changed: %+v %v", original, err)
	}
}
