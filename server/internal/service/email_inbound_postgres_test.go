//go:build integration

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/storage"
)

func TestInboundAttachmentCommitBoundaryPostgres(t *testing.T) {
	db := privacyServiceDB(t)
	ctx := context.Background()
	ws := uuid.NewString()
	seedContactPrivacyWorkspace(t, db, ws)
	conv := uuid.NewString()
	privacyExec(t, db, `INSERT INTO support_conversations(id,workspace_id,display_id,subject,customer_email,status,channel) VALUES (?,?,1,'File','customer@example.com','open','email')`, conv, ws)
	svc := privacyEmail(t, db, ws)
	var uploads atomic.Int32
	store := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { uploads.Add(1); w.WriteHeader(http.StatusOK) }))
	defer store.Close()
	svc.SetAttachmentService(NewSupportAttachmentService(repository.NewSupportAttachmentRepository(db), storage.NewS3Client("test", "test", "test-bucket", "us-east-1", store.URL, "")))
	p := model.PostmarkInboundPayload{MessageID: uuid.NewString(), FromFull: model.PostmarkAddress{Email: "customer@example.com"}, To: "conv-" + conv + "@replies.example.com", TextBody: "Here is the file", Attachments: []model.PostmarkInboundAttachment{{Name: "file.pdf", ContentType: "application/pdf", Content: "YWJj", ContentLength: 3}}}
	if err := svc.AcceptInboundEmail(ctx, p, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.processNextInboundJob(ctx, "email"); err != nil {
		t.Fatal(err)
	}
	privacyCheck(t, db, `SELECT count(*)=1 FROM support_messages WHERE conversation_id=? AND sender_type='customer'`, conv)
	privacyCheck(t, db, `SELECT count(*)=1 FROM support_attachments WHERE conversation_id=? AND processing_status='processing' AND is_uploaded=false`, conv)
	if uploads.Load() != 0 {
		t.Fatal("attachment uploaded in message transaction")
	}
	if err := svc.processNextInboundJob(ctx, "attachment"); err != nil {
		t.Fatal(err)
	}
	privacyCheck(t, db, `SELECT count(*)=1 FROM support_attachments WHERE conversation_id=? AND is_uploaded=true`, conv)
	if uploads.Load() != 1 {
		t.Fatalf("uploads %d", uploads.Load())
	}
	// Replay after completion cannot create more messages, files, or uploads.
	if err := svc.AcceptInboundEmail(ctx, p, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.processNextInboundJob(ctx, "email"); err != nil {
		t.Fatal(err)
	}
	privacyCheck(t, db, `SELECT count(*)=1 FROM support_messages WHERE conversation_id=? AND sender_type='customer'`, conv)

	// Reproduce the former lock dependency using the real production trigger.
	tx := db.Begin()
	defer tx.Rollback()
	if err := tx.Exec(`SELECT id FROM support_conversations WHERE id=? FOR UPDATE`, conv).Error; err != nil {
		t.Fatal(err)
	}
	blockedCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	err := db.WithContext(blockedCtx).Create(&model.SupportAttachment{ID: uuid.NewString(), WorkspaceID: ws, ConversationID: &conv, FileName: "old.pdf", ContentType: "application/pdf", UploadedByType: "customer"}).Error
	if err == nil {
		t.Fatal("expected separate attachment connection to block on conversation lock")
	}
}

func TestInboundPendingJobsRespectPrivacyPostgres(t *testing.T) {
	db := privacyServiceDB(t)
	ctx := context.Background()
	ws, contact, conv := uuid.NewString(), uuid.NewString(), uuid.NewString()
	seedContactPrivacyWorkspace(t, db, ws)
	privacyExec(t, db, `INSERT INTO crm_contacts(id,workspace_id,display_id,first_name,email) VALUES (?,?,1,'Alice','alice@example.com')`, contact, ws)
	privacyExec(t, db, `INSERT INTO support_conversations(id,workspace_id,display_id,subject,customer_email,crm_contact_id,status,channel) VALUES (?,?,1,'Files','alice@example.com',?,'open','email')`, conv, ws, contact)
	svc := privacyEmail(t, db, ws)
	p := model.PostmarkInboundPayload{MessageID: uuid.NewString(), FromFull: model.PostmarkAddress{Email: "alice@example.com"}, To: "conv-" + conv + "@replies.example.com", TextBody: "My file", Attachments: []model.PostmarkInboundAttachment{{Name: "file.pdf", ContentType: "application/pdf", Content: "YWJj", ContentLength: 3}}}
	if err := svc.AcceptInboundEmail(ctx, p, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.processNextInboundJob(ctx, "email"); err != nil {
		t.Fatal(err)
	}
	privacyCheck(t, db, `SELECT count(*)=1 FROM support_inbound_jobs WHERE conversation_id=? AND payload<>''`, conv)
	if _, err := repository.NewCRMContactRepository(db).DeleteAnonymizingSupport(ctx, ws, contact); err != nil {
		t.Fatal(err)
	}
	privacyCheck(t, db, `SELECT count(*)=0 FROM support_inbound_jobs WHERE conversation_id=? AND payload<>''`, conv)
	// A late receipt for an anonymized timeline is an empty tombstone.
	p.MessageID = uuid.NewString()
	if err := svc.AcceptInboundEmail(ctx, p, ""); err != nil {
		t.Fatal(err)
	}
	privacyCheck(t, db, `SELECT count(*)=0 FROM support_inbound_jobs WHERE conversation_id=? AND payload<>''`, conv)
	// Explicit deletion also clears an accepted receipt before it can be processed.
	other := uuid.NewString()
	privacyExec(t, db, `INSERT INTO support_conversations(id,workspace_id,display_id,subject,customer_email,status,channel) VALUES (?,?,2,'Delete','other@example.com','open','email')`, other, ws)
	p.MessageID = uuid.NewString()
	p.To = "conv-" + other + "@replies.example.com"
	p.FromFull.Email = "other@example.com"
	if err := svc.AcceptInboundEmail(ctx, p, ""); err != nil {
		t.Fatal(err)
	}
	privacyExec(t, db, `DELETE FROM support_conversations WHERE id=?`, other)
	privacyCheck(t, db, `SELECT count(*)=0 FROM support_inbound_jobs WHERE conversation_id=? AND payload<>''`, other)
}
