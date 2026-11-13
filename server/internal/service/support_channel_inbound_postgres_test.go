//go:build integration

package service

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/helpin-ai/helpin/server/internal/channels"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func newChannelInbox(db *gorm.DB) *SupportInboxService {
	return NewSupportInboxService(repository.NewSupportConversationRepository(db), repository.NewSupportMailboxRepository(db),
		repository.NewSupportMessageRepository(db), nil, nil, repository.NewSupportInboxInstallationRepository(db),
		nil, nil, nil, nil, nil, repository.NewUserRepository(db), nil, nil, nil)
}

func seedChannelAccount(t *testing.T, db *gorm.DB, workspaceID string) *model.SupportChannelAccount {
	t.Helper()
	account := &model.SupportChannelAccount{ID: uuid.NewString(), WorkspaceID: workspaceID, Provider: "telegram",
		ExternalAccountID: "bot-" + uuid.NewString()[:8], Handle: "acme_bot", TokenEncrypted: "enc", WebhookSecretHash: "hash", Status: "active"}
	if err := db.Create(account).Error; err != nil {
		t.Fatal(err)
	}
	return account
}

func textEvent(user, key, text string) channels.Event {
	return channels.Event{Kind: channels.EventMessage, ExternalUserID: user, ExternalChatID: user, ExternalMessageKey: key,
		DisplayName: "Ada Lovelace", Username: "ada_l", Text: text}
}

func TestReceiveChannelMessagePostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	ctx := context.Background()
	workspaceID, _ := seedSupportOpsWorkspace(t, db)
	account := seedChannelAccount(t, db, workspaceID)
	inbox := newChannelInbox(db)

	first, err := inbox.receiveChannelMessage(ctx, account, textEvent("7001", "7001:1", "My **order** is late"), 20<<20)
	if err != nil {
		t.Fatal(err)
	}
	if !first.conversationCreated || first.duplicate {
		t.Fatalf("first receipt = %+v", first)
	}
	var conv model.SupportConversation
	if err := db.First(&conv, "id = ?", first.message.ConversationID).Error; err != nil {
		t.Fatal(err)
	}
	if conv.Channel != "telegram" || conv.Source != "telegram" || derefString(conv.CustomerName) != "Ada Lovelace" ||
		derefString(conv.ChannelAccountID) != account.ID || conv.ChannelIdentityID == nil || conv.Subject != "My **order** is late" {
		t.Fatalf("conversation = %+v", conv)
	}
	var stored model.SupportMessage
	db.First(&stored, "id = ?", first.message.ID)
	if stored.Content != "My **order** is late" || derefString(stored.ViaChannel) != "telegram" || derefString(stored.ExternalMessageKey) != "7001:1" || stored.SenderType != "customer" {
		t.Fatalf("message = %+v", stored)
	}

	t.Run("a provider retry stores nothing new", func(t *testing.T) {
		again, err := inbox.receiveChannelMessage(ctx, account, textEvent("7001", "7001:1", "My **order** is late"), 20<<20)
		if err != nil || !again.duplicate {
			t.Fatalf("retry = %+v %v", again, err)
		}
		var count int64
		db.Model(&model.SupportMessage{}).Where("conversation_id = ?", conv.ID).Count(&count)
		if count != 1 {
			t.Fatalf("messages = %d", count)
		}
	})

	t.Run("the next message continues the conversation and refreshes the profile", func(t *testing.T) {
		event := textEvent("7001", "7001:2", "Any news?")
		event.DisplayName = "Ada K. Lovelace"
		next, err := inbox.receiveChannelMessage(ctx, account, event, 20<<20)
		if err != nil || next.conversationCreated || next.message.ConversationID != conv.ID {
			t.Fatalf("next = %+v %v", next, err)
		}
		var identity model.SupportChannelIdentity
		db.First(&identity, "id = ?", *conv.ChannelIdentityID)
		if identity.DisplayName != "Ada K. Lovelace" || identity.LastInboundAt == nil {
			t.Fatalf("identity = %+v", identity)
		}
	})

	t.Run("another person gets their own conversation", func(t *testing.T) {
		other, err := inbox.receiveChannelMessage(ctx, account, textEvent("8002", "8002:1", "Hello"), 20<<20)
		if err != nil || !other.conversationCreated || other.message.ConversationID == conv.ID {
			t.Fatalf("other = %+v %v", other, err)
		}
	})

	t.Run("a recently resolved conversation reopens", func(t *testing.T) {
		mustExec(t, db, `UPDATE support_conversations SET status = 'resolved', resolved_at = now() - interval '3 days' WHERE id = ?`, conv.ID)
		reopened, err := inbox.receiveChannelMessage(ctx, account, textEvent("7001", "7001:3", "Still broken"), 20<<20)
		if err != nil || reopened.message.ConversationID != conv.ID || !reopened.conversationChanged || reopened.reopened == nil {
			t.Fatalf("reopen = %+v %v", reopened, err)
		}
		var status string
		db.Raw(`SELECT status FROM support_conversations WHERE id = ?`, conv.ID).Scan(&status)
		if status != "open" {
			t.Fatalf("status = %s", status)
		}
	})

	t.Run("an old resolved conversation starts a new one", func(t *testing.T) {
		mustExec(t, db, `UPDATE support_conversations SET status = 'resolved', resolved_at = now() - interval '6 days' WHERE id = ?`, conv.ID)
		fresh, err := inbox.receiveChannelMessage(ctx, account, textEvent("7001", "7001:4", "New problem"), 20<<20)
		if err != nil || !fresh.conversationCreated || fresh.message.ConversationID == conv.ID {
			t.Fatalf("fresh = %+v %v", fresh, err)
		}
	})

	t.Run("files become placeholders with download jobs; oversized ones are refused", func(t *testing.T) {
		event := textEvent("9003", "9003:1", "")
		event.Files = []channels.InboundFile{
			{FileID: "photo-1", FileName: "photo.jpg", ContentType: "image/jpeg", Size: 2048},
			{FileID: "big-1", FileName: "video.mp4", ContentType: "video/mp4", Size: 30 << 20},
		}
		receipt, err := inbox.receiveChannelMessage(ctx, account, event, 20<<20)
		if err != nil {
			t.Fatal(err)
		}
		var subject string
		db.Raw(`SELECT subject FROM support_conversations WHERE id = ?`, receipt.message.ConversationID).Scan(&subject)
		if subject != "Attachment" {
			t.Errorf("subject = %q", subject)
		}
		var states []string
		db.Raw(`SELECT processing_status FROM support_attachments WHERE message_id = ? ORDER BY file_name`, receipt.message.ID).Scan(&states)
		if strings.Join(states, ",") != "processing,rejected" {
			t.Fatalf("attachment states = %v", states)
		}
		var jobs int64
		db.Model(&model.SupportInboundJob{}).Where("message_id = ? AND kind = ?", receipt.message.ID, SupportInboundKindChannelAttachment).Count(&jobs)
		if jobs != 1 {
			t.Fatalf("download jobs = %d", jobs)
		}
	})

	t.Run("an unsupported message is a visible note", func(t *testing.T) {
		event := channels.Event{Kind: channels.EventUnsupported, UnsupportedType: "sticker", ExternalUserID: "8002", ExternalChatID: "8002", ExternalMessageKey: "8002:2", DisplayName: "Grace"}
		receipt, err := inbox.receiveChannelMessage(ctx, account, event, 20<<20)
		if err != nil || receipt.message.Content != "_Sent a sticker on Telegram, which Helpin can't display._" {
			t.Fatalf("unsupported = %+v %v", receipt, err)
		}
	})

	t.Run("the bot's AI switch disables AI on its conversations", func(t *testing.T) {
		got, err := repository.NewSupportConversationRepository(db).GetByID(ctx, workspaceID, conv.ID, "", model.RoleOwner)
		if err != nil || got.MailboxAIRepliesDisabled {
			t.Fatalf("before = %v %v", got.MailboxAIRepliesDisabled, err)
		}
		mustExec(t, db, `UPDATE support_channel_accounts SET ai_replies_disabled = true WHERE id = ?`, account.ID)
		got, err = repository.NewSupportConversationRepository(db).GetByID(ctx, workspaceID, conv.ID, "", model.RoleOwner)
		if err != nil || !got.MailboxAIRepliesDisabled {
			t.Fatalf("after = %v %v", got.MailboxAIRepliesDisabled, err)
		}
		settings := model.DefaultSupportInboxSettings()
		msg := &model.SupportMessage{SenderType: "customer", Content: "hi"}
		if model.SupportAIReplyAllowed(settings, got, msg) {
			t.Fatal("AI allowed although the bot's AI replies are off")
		}
	})
}

// Concurrent first messages from one person must open one conversation; this
// needs separate connections, so it runs outside a wrapping transaction.
func TestReceiveChannelMessageConcurrentFirstMessagesPostgres(t *testing.T) {
	dsn := os.Getenv("SUPPORT_OPS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SUPPORT_OPS_TEST_DATABASE_URL to a migrated disposable PostgreSQL database")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	workspaceID, _ := seedSupportOpsWorkspace(t, db)
	account := seedChannelAccount(t, db, workspaceID)
	inbox := newChannelInbox(db)
	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := inbox.receiveChannelMessage(context.Background(), account, textEvent("4242", "4242:"+uuid.NewString(), "burst"), 20<<20)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var conversations, messages int64
	db.Model(&model.SupportConversation{}).Where("workspace_id = ? AND channel_account_id = ?", workspaceID, account.ID).Count(&conversations)
	db.Model(&model.SupportMessage{}).Where("workspace_id = ? AND channel_account_id = ?", workspaceID, account.ID).Count(&messages)
	if conversations != 1 || messages != 5 {
		t.Fatalf("conversations = %d, messages = %d", conversations, messages)
	}
}
