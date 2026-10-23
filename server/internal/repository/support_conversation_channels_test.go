package repository

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestConversationChannelsUsesActualHistory(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		sender                  string
		internal, sent          bool
		legacy                  bool
		workspace, conversation string
		want                    []string
	}{
		{name: "inbound email", sender: "customer", want: []string{"widget", "email"}},
		{name: "legacy chat emailed as fallback", sender: "agent", sent: true, legacy: true, want: []string{"widget", "email"}},
		{name: "sent email", sender: "user", sent: true, want: []string{"widget", "email"}},
		{name: "queued or failed email", sender: "user", want: []string{"widget"}},
		{name: "private note", sender: "customer", internal: true, want: []string{"widget"}},
		{name: "other workspace", sender: "customer", workspace: "other", want: []string{"widget"}},
		{name: "other conversation", sender: "customer", conversation: "other", want: []string{"widget"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupSupportMessageTestDB(t)
			ws, conv := "ws", "conv"
			if tc.workspace != "" {
				ws = tc.workspace
			}
			if tc.conversation != "" {
				conv = tc.conversation
			}
			channel := "email"
			msg := model.SupportMessage{ID: "old-email", WorkspaceID: ws, ConversationID: conv, SenderType: tc.sender, MessageType: "reply", IsInternal: tc.internal, ViaChannel: &channel, Content: "old reply", CreatedAt: time.Now().AddDate(-1, 0, 0)}
			if tc.legacy {
				msg.ViaChannel = nil
			}
			if tc.sent {
				now := time.Now()
				msg.EmailNotifiedAt = &now
			}
			if err := db.Exec(`INSERT INTO support_messages (id,workspace_id,conversation_id,sender_type,message_type,is_internal,via_channel,email_notified_at,content,created_at) VALUES (?,?,?,?,?,?,?,?,?,?)`, msg.ID, ws, conv, msg.SenderType, msg.MessageType, msg.IsInternal, msg.ViaChannel, msg.EmailNotifiedAt, msg.Content, msg.CreatedAt).Error; err != nil {
				t.Fatal(err)
			}
			got, err := NewSupportConversationRepository(db).Channels(context.Background(), "ws", "conv", "widget")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("channels=%v, want %v", got, tc.want)
			}
		})
	}
}
