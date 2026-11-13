//go:build integration

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

// privacyFixture extends the outbox fixture with CRM linking and a webhook
// secret, so updates go through the real accept-and-process path.
func newChannelPrivacyFixture(t *testing.T) (*outboxFixture, string) {
	t.Helper()
	f := newOutboxFixture(t)
	f.inbox.contactRepo = repository.NewCRMContactRepository(f.db)
	f.inbox.assocRepo = repository.NewCRMAssociationRepository(f.db)
	sum := sha256.Sum256([]byte("hook-secret"))
	mustExec(t, f.db, `UPDATE support_channel_accounts SET webhook_secret_hash = ? WHERE id = ?`, hex.EncodeToString(sum[:]), f.account.ID)
	contact := uuid.NewString()
	mustExec(t, f.db, `INSERT INTO crm_contacts (id, workspace_id, display_id, first_name, email) VALUES (?, ?, 1, 'Ada', 'ada@example.com')`, contact, f.workspace)
	return f, contact
}

func (f *outboxFixture) identityOf(t *testing.T, conversationID string) *model.SupportChannelIdentity {
	t.Helper()
	var identity model.SupportChannelIdentity
	f.db.Raw(`SELECT i.* FROM support_channel_identities i JOIN support_conversations c ON c.channel_identity_id = i.id WHERE c.id = ?`, conversationID).Scan(&identity)
	if identity.ID == "" {
		return nil
	}
	return &identity
}

// acceptAndProcess delivers a Telegram update as the webhook would and runs
// the worker's job steps on it. processEventJob also starts background work,
// which cannot share this test's single-connection transaction.
func (f *outboxFixture) acceptAndProcess(t *testing.T, body []byte) {
	t.Helper()
	ctx := context.Background()
	if err := f.svc.AcceptWebhook(ctx, "telegram", f.account.ID, secretHeader("hook-secret"), body); err != nil {
		t.Fatal(err)
	}
	for {
		job, err := f.svc.jobs.Claim(ctx, SupportInboundKindChannelEvent, f.svc.now())
		if err != nil {
			t.Fatal(err)
		}
		if job == nil {
			return
		}
		var payload channelEventPayload
		if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
			t.Fatal(err)
		}
		receipt, err := f.inbox.receiveChannelMessage(ctx, f.account, payload.Event, 20<<20)
		if err != nil {
			t.Fatal(err)
		}
		if !receipt.duplicate {
			if err := f.svc.jobs.SetConversation(ctx, job, receipt.message.ConversationID, receipt.message.ID); err != nil {
				t.Fatal(err)
			}
		}
		if err := f.svc.jobs.Finish(ctx, job, nil, f.svc.now()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestChannelIdentityLinkingPostgres(t *testing.T) {
	f, contact := newChannelPrivacyFixture(t)
	ctx := context.Background()
	if _, err := f.inbox.UpdateConversationCRMContact(ctx, f.workspace, f.conv, &contact, f.userID); err != nil {
		t.Fatal(err)
	}
	if identity := f.identityOf(t, f.conv); identity == nil || derefString(identity.CRMContactID) != contact {
		t.Fatalf("identity after linking = %+v", identity)
	}

	t.Run("a later conversation starts linked and shows on the contact", func(t *testing.T) {
		mustExec(t, f.db, `UPDATE support_conversations SET status = 'resolved', resolved_at = now() - interval '6 days' WHERE id = ?`, f.conv)
		receipt, err := f.inbox.receiveChannelMessage(ctx, f.account, textEvent("7001", "7001:50", "Back again"), 20<<20)
		if err != nil || !receipt.conversationCreated {
			t.Fatalf("receipt = %+v %v", receipt, err)
		}
		var linked *string
		f.db.Raw(`SELECT crm_contact_id FROM support_conversations WHERE id = ?`, receipt.message.ConversationID).Scan(&linked)
		if derefString(linked) != contact {
			t.Errorf("new conversation contact = %v", linked)
		}
		var associations int64
		f.db.Model(&model.CRMAssociation{}).Where("from_object_id = ? AND to_object_id = ?", receipt.message.ConversationID, contact).Count(&associations)
		if associations != 1 {
			t.Errorf("associations = %d", associations)
		}
	})

	t.Run("unlinking clears the identity", func(t *testing.T) {
		if _, err := f.inbox.UpdateConversationCRMContact(ctx, f.workspace, f.conv, nil, f.userID); err != nil {
			t.Fatal(err)
		}
		if identity := f.identityOf(t, f.conv); identity == nil || identity.CRMContactID != nil {
			t.Fatalf("identity after unlinking = %+v", identity)
		}
	})
}

func TestChannelContactDeletionForgetsTheTelegramPersonPostgres(t *testing.T) {
	f, contact := newChannelPrivacyFixture(t)
	ctx := context.Background()
	update := telegramUpdate(901, "private", "my address is 1 Main St")
	f.acceptAndProcess(t, update)
	var conv string
	f.db.Raw(`SELECT c.id FROM support_conversations c JOIN support_channel_identities i ON i.id = c.channel_identity_id WHERE i.external_user_id = '9001'`).Scan(&conv)
	if conv == "" {
		var errs []string
		f.db.Raw(`SELECT status || ' ' || last_error FROM support_inbound_jobs WHERE workspace_id = ?`, f.workspace).Scan(&errs)
		t.Fatalf("no conversation for the update; jobs %v", errs)
	}
	if _, err := f.inbox.UpdateConversationCRMContact(ctx, f.workspace, conv, &contact, f.userID); err != nil {
		t.Fatal(err)
	}
	identity := f.identityOf(t, conv)
	// A link made before identities carried it: only the conversation names
	// the contact. The same person's other, unlinked conversation goes too.
	mustExec(t, f.db, `UPDATE support_channel_identities SET crm_contact_id = NULL WHERE id = ?`, identity.ID)
	mustExec(t, f.db, `UPDATE support_conversations SET status = 'resolved', resolved_at = now() - interval '6 days' WHERE id = ?`, conv)
	other, err := f.inbox.receiveChannelMessage(ctx, f.account, textEvent("9001", "9001:77", "A new issue"), 20<<20)
	if err != nil || !other.conversationCreated {
		t.Fatalf("other = %+v %v", other, err)
	}

	// Work in flight: a queued reply and an accepted, unprocessed update.
	reply := f.reply(t, conv, "user", "We'll ship today")
	pendingUpdate := telegramUpdate(902, "private", "and my phone is 555")
	if err := f.svc.AcceptWebhook(ctx, "telegram", f.account.ID, secretHeader("hook-secret"), pendingUpdate); err != nil {
		t.Fatal(err)
	}

	if _, err := repository.NewCRMContactRepository(f.db).DeleteAnonymizingSupport(ctx, f.workspace, contact); err != nil {
		t.Fatal(err)
	}

	var identities int64
	f.db.Model(&model.SupportChannelIdentity{}).Where("id = ?", identity.ID).Count(&identities)
	if identities != 0 {
		t.Error("the identity survived")
	}
	if f.row(t, reply.ID) != nil {
		t.Error("the queued reply survived")
	}
	var jobs []model.SupportInboundJob
	f.db.Where("workspace_id = ? AND kind = ?", f.workspace, SupportInboundKindChannelEvent).Find(&jobs)
	for _, job := range jobs {
		if job.Payload != "" || job.Status != "completed" {
			t.Errorf("job %s kept status %s payload %q", job.ID, job.Status, job.Payload)
		}
	}
	var keys []string
	f.db.Raw(`SELECT COALESCE(external_message_key, '') FROM support_messages WHERE conversation_id IN ?`, []string{conv, other.message.ConversationID}).Scan(&keys)
	for _, key := range keys {
		if strings.Contains(key, "9001") {
			t.Errorf("message key still holds the chat id: %s", key)
		}
	}
	var anonymized struct {
		AnonymizedAt      *string
		ChannelIdentityID *string
	}
	f.db.Raw(`SELECT anonymized_at::text, channel_identity_id::text FROM support_conversations WHERE id = ?`, conv).Scan(&anonymized)
	if anonymized.AnonymizedAt == nil || anonymized.ChannelIdentityID != nil {
		t.Errorf("conversation = %+v", anonymized)
	}
	var otherAnonymized *string
	f.db.Raw(`SELECT anonymized_at::text FROM support_conversations WHERE id = ?`, other.message.ConversationID).Scan(&otherAnonymized)
	if otherAnonymized == nil {
		t.Error("the person's other conversation was not erased")
	}

	f.due(t)
	if _, err := f.svc.ProcessOutbox(ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.tg.sent) != 0 {
		t.Errorf("sent after deletion: %v", f.tg.sent)
	}

	t.Run("a replayed update creates nothing", func(t *testing.T) {
		var before int64
		f.db.Model(&model.SupportConversation{}).Where("workspace_id = ?", f.workspace).Count(&before)
		f.acceptAndProcess(t, update)
		f.acceptAndProcess(t, pendingUpdate)
		var after, identities int64
		f.db.Model(&model.SupportConversation{}).Where("workspace_id = ?", f.workspace).Count(&after)
		f.db.Model(&model.SupportChannelIdentity{}).Where("external_user_id = '9001'").Count(&identities)
		if after != before || identities != 0 {
			t.Errorf("conversations %d -> %d, identities %d", before, after, identities)
		}
	})
}

func TestChannelConversationDeletionPostgres(t *testing.T) {
	f, contact := newChannelPrivacyFixture(t)
	ctx := context.Background()
	repo := repository.NewSupportConversationRepository(f.db)

	t.Run("an unlinked person known only here is forgotten", func(t *testing.T) {
		identity := f.identityOf(t, f.conv)
		reply := f.reply(t, f.conv, "user", "Queued")
		if err := repo.Delete(ctx, f.workspace, f.conv); err != nil {
			t.Fatal(err)
		}
		var count int64
		f.db.Model(&model.SupportChannelIdentity{}).Where("id = ?", identity.ID).Count(&count)
		if count != 0 {
			t.Error("the identity survived")
		}
		if f.row(t, reply.ID) != nil {
			t.Error("the queued reply survived")
		}
	})

	t.Run("a linked person stays with their contact", func(t *testing.T) {
		receipt, err := f.inbox.receiveChannelMessage(ctx, f.account, textEvent("8100", "8100:1", "Hello"), 20<<20)
		if err != nil {
			t.Fatal(err)
		}
		conv := receipt.message.ConversationID
		if _, err := f.inbox.UpdateConversationCRMContact(ctx, f.workspace, conv, &contact, f.userID); err != nil {
			t.Fatal(err)
		}
		identity := f.identityOf(t, conv)
		if err := repo.Delete(ctx, f.workspace, conv); err != nil {
			t.Fatal(err)
		}
		var count int64
		f.db.Model(&model.SupportChannelIdentity{}).Where("id = ?", identity.ID).Count(&count)
		if count != 1 {
			t.Error("a linked identity was deleted with one conversation")
		}
	})

	t.Run("a person with another conversation stays", func(t *testing.T) {
		first, err := f.inbox.receiveChannelMessage(ctx, f.account, textEvent("8200", "8200:1", "One"), 20<<20)
		if err != nil {
			t.Fatal(err)
		}
		mustExec(t, f.db, `UPDATE support_conversations SET status = 'resolved', resolved_at = now() - interval '6 days' WHERE id = ?`, first.message.ConversationID)
		if _, err := f.inbox.receiveChannelMessage(ctx, f.account, textEvent("8200", "8200:2", "Two"), 20<<20); err != nil {
			t.Fatal(err)
		}
		identity := f.identityOf(t, first.message.ConversationID)
		if err := repo.Delete(ctx, f.workspace, first.message.ConversationID); err != nil {
			t.Fatal(err)
		}
		var count int64
		f.db.Model(&model.SupportChannelIdentity{}).Where("id = ?", identity.ID).Count(&count)
		if count != 1 {
			t.Error("the identity was deleted while another conversation uses it")
		}
	})
}
