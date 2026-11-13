package repository

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestGetUnreadStatsExcludesMergedConversations(t *testing.T) {
	for _, mode := range []string{"personal", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			for _, scope := range []string{"all", "shared", "mailbox"} {
				t.Run(scope, func(t *testing.T) {
					db := setupSupportPersonalStateTestDB(t)
					if err := db.Exec(`ALTER TABLE support_messages ADD COLUMN metadata TEXT`).Error; err != nil {
						t.Fatal(err)
					}
					var mailboxID *string
					if scope != "all" {
						value := ""
						if scope == "mailbox" {
							value = "mailbox-1"
						}
						mailboxID = &value
					}
					var storedMailboxID *string
					if scope == "mailbox" {
						storedMailboxID = mailboxID
					}
					userID := "u1"
					for _, prefix := range []string{"target-", "merged-"} {
						for _, conversation := range []struct {
							id, status, flowState string
							assignedUserID        *string
						}{
							{id: "open", status: "open", flowState: "waiting_for_human", assignedUserID: &userID},
							{id: "waiting", status: "waiting_on_customer", flowState: "waiting_for_human", assignedUserID: &userID},
							{id: "unassigned", status: "open", flowState: "waiting_for_human"},
							{id: "ai", status: "open", flowState: "ai_handling"},
						} {
							id := prefix + conversation.id
							if err := db.Exec(`INSERT INTO support_conversations
								(id, workspace_id, mailbox_id, status, flow_state, assigned_user_id, needs_human_reply)
								VALUES (?, 'w1', ?, ?, ?, ?, true)`, id, storedMailboxID, conversation.status,
								conversation.flowState, conversation.assignedUserID).Error; err != nil {
								t.Fatal(err)
							}
							relevance := 0
							if conversation.assignedUserID != nil {
								relevance = model.SupportRelevanceAssignee
							}
							if err := db.Exec(`INSERT INTO support_conversation_user_states
								(workspace_id, conversation_id, user_id, unread_customer_message_count, relevance_mask)
								VALUES ('w1', ?, 'u1', 1, ?)`, id, relevance).Error; err != nil {
								t.Fatal(err)
							}
							if err := db.Exec(`INSERT INTO support_messages
								(id, workspace_id, conversation_id, sender_type, message_type, created_at)
								VALUES (?, 'w1', ?, 'customer', 'reply', ?)`, "message-"+id, id, time.Now()).Error; err != nil {
								t.Fatal(err)
							}
						}
					}
					repo := NewSupportConversationRepository(db)
					readStats := repo.GetUnreadStats
					if mode == "legacy" {
						readStats = repo.getLegacyUnreadStats
					}
					before, err := readStats(context.Background(), "w1", "u1", "", model.RoleOwner, mailboxID)
					if err != nil {
						t.Fatal(err)
					}
					if before.InboxTotal != 6 || before.MineTotal != 4 || before.WaitingTotal != 2 || before.AIActiveTotal != 2 {
						t.Fatalf("unexpected counts before merge: %+v", before)
					}
					if err := db.Exec(`UPDATE support_conversations
						SET merged_into_id = 'target-' || substr(id, 8) WHERE id LIKE 'merged-%'`).Error; err != nil {
						t.Fatal(err)
					}
					got, err := readStats(context.Background(), "w1", "u1", "", model.RoleOwner, mailboxID)
					if err != nil {
						t.Fatal(err)
					}
					want := model.UnreadStats{
						Inbox: 3, Mine: 2, Waiting: 1, AIActive: 1, Total: 4, MyInbox: 2, Unassigned: 1,
						InboxTotal: 3, MineTotal: 2, WaitingTotal: 1, AIActiveTotal: 1,
						InboxNeedsHumanReply: 3, MineNeedsHumanReply: 2, WaitingNeedsHumanReply: 1, AIActiveNeedsHumanReply: 1,
					}
					if got != want {
						t.Fatalf("counts after merge = %+v, want %+v", got, want)
					}
				})
			}
		})
	}
}
