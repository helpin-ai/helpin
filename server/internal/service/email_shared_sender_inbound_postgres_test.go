//go:build integration

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// Mail to the shared sender must reach the right conversation on PostgreSQL:
// replies across workspaces, one copy per email, a new conversation in the
// only workspace that emailed the sender, and an immediate admin-visible
// failure otherwise.
func TestSharedSenderInboundPostgres(t *testing.T) {
	db := privacyServiceDB(t)
	ctx := context.Background()
	first, second := uuid.NewString(), uuid.NewString()
	seedContactPrivacyWorkspace(t, db, first)
	owner := uuid.NewString()
	privacyExec(t, db, `INSERT INTO users (id,email,full_name,password_hash) VALUES (?,?,'Second owner','test-only')`, owner, owner+"@example.invalid")
	privacyExec(t, db, `INSERT INTO workspaces (id,name,slug,workspace_key,owner_id) VALUES (?,'Second workspace',?,'SCND',?)`, second, second, owner)
	svc := privacyEmail(t, db, first)
	svc.SetSupportInboxService(privacyInbox(db))

	thread := func(workspaceID string, displayID int, customer string) (string, string) {
		conv, logID := uuid.NewString(), uuid.NewString()
		privacyExec(t, db, `INSERT INTO support_conversations(id,workspace_id,display_id,subject,customer_email,status,channel) VALUES (?,?,?,'Pricing',?,'open','email')`, conv, workspaceID, displayID, customer)
		messageID := "<helpin-" + logID + "@replies.example.com>"
		privacyExec(t, db, `INSERT INTO support_email_logs(id,workspace_id,conversation_id,direction,from_email,to_email,subject,rfc_message_id) VALUES (?,?,?,'outbound',?,?,'Re: Pricing (#1)',?)`,
			logID, workspaceID, conv, "support+conv-"+conv+"@example.com", customer, messageID)
		return conv, messageID
	}
	payload := func(from, subject, body string, headers ...model.PostmarkHeader) model.PostmarkInboundPayload {
		return model.PostmarkInboundPayload{
			MessageID: uuid.NewString(), From: from, FromFull: model.PostmarkAddress{Email: from},
			To: "support@example.com", ToFull: []model.PostmarkAddress{{Email: "support@example.com"}}, OriginalRecipient: "support@example.com",
			Subject: subject, TextBody: body, StrippedTextReply: body, Headers: headers,
		}
	}

	// A reply joins its conversation in the second workspace, once.
	conv, sentID := thread(second, 1, "reply@customer.test")
	reply := payload("reply@customer.test", "Re: Pricing (#1)", "Sounds good.",
		model.PostmarkHeader{Name: "Message-ID", Value: "<reply@customer.test>"},
		model.PostmarkHeader{Name: "In-Reply-To", Value: sentID})
	for i := 0; i < 2; i++ {
		reply.MessageID = uuid.NewString()
		if err := svc.ProcessInboundEmail(ctx, reply, ""); err != nil {
			t.Fatal(err)
		}
	}
	privacyCheck(t, db, `SELECT count(*)=1 FROM support_messages WHERE conversation_id=? AND sender_type='customer'`, conv)

	// A new email from someone only the first workspace emailed opens a
	// conversation there without claiming a route.
	thread(first, 2, "new_person@customer.test")
	if err := svc.ProcessInboundEmail(ctx, payload("New_Person@customer.test", "Another question", "Do you invoice?"), ""); err != nil {
		t.Fatal(err)
	}
	privacyCheck(t, db, `SELECT count(*)=1 FROM support_conversations WHERE workspace_id=? AND subject='Another question'`, first)
	privacyCheck(t, db, `SELECT count(*)=1 FROM support_email_logs l JOIN support_conversations c ON c.id=l.conversation_id WHERE c.subject='Another question' AND l.direction='inbound' AND l.email_route_id IS NULL`)

	// Someone both workspaces emailed: the job fails once, with its reason.
	thread(first, 3, "both@customer.test")
	thread(second, 4, "both@customer.test")
	ambiguous := payload("both@customer.test", "Hello", "Which team is this?")
	if err := svc.AcceptInboundEmail(ctx, ambiguous, ""); err != nil {
		t.Fatal(err)
	}
	privacyExec(t, db, `UPDATE support_inbound_jobs SET available_at = ? WHERE kind = 'email'`, time.Now().UTC().Add(-time.Second))
	if err := svc.processNextInboundJob(ctx, "email"); err != nil {
		t.Fatal(err)
	}
	var job model.SupportInboundJob
	if err := db.Where("kind = 'email'").First(&job).Error; err != nil {
		t.Fatal(err)
	}
	if job.Status != "failed" || job.Attempts != 1 || job.Payload == "" || !strings.Contains(job.LastError, "2 workspaces have emailed this sender") {
		t.Fatalf("expected one failed attempt with a reason, got %q %d %q", job.Status, job.Attempts, job.LastError)
	}
}
