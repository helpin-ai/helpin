//go:build integration

package service

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/intercom"
	"github.com/helpin-ai/helpin/server/internal/intercom/intercomtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func strp(value string) *string { return &value }

// longIntercomTag is longer than the tag names an import creates.
var longIntercomTag = "Customer escalations " + strings.Repeat("x", 70)

func TestIntercomConversationImportPostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	ctx := context.Background()
	workspaceID, userID := seedSupportOpsWorkspace(t, db)
	mustExec(t, db, `DELETE FROM support_import_jobs`)
	mustExec(t, db, `DELETE FROM webhook_events`)
	mustExec(t, db, `INSERT INTO workspace_members (workspace_id, user_id, email, display_name, role, status) VALUES (?, ?, 'grace@acme.test', 'Grace', 'admin', 'active')`, workspaceID, userID)
	inbox := NewSupportInboxService(repository.NewSupportConversationRepository(db), repository.NewSupportMailboxRepository(db),
		repository.NewSupportMessageRepository(db), nil, nil, repository.NewSupportInboxInstallationRepository(db), nil, nil, nil, nil, nil,
		repository.NewUserRepository(db), nil, nil, nil)
	inbox.workspaceRepo = repository.NewWorkspaceRepository(db)
	inbox.tagRepo = repository.NewSupportTagRepository(db)
	// A webhook and an SLA policy are in place: imported history must
	// trigger neither.
	webhooks := NewWebhookService(repository.NewWebhookRepository(db), inbox, WebhookConfig{EncryptionKey: []byte(strings.Repeat("k", 32)), AllowPrivateNetworks: true})
	if _, err := webhooks.CreateEndpoint(ctx, workspaceID, userID, model.WebhookEndpointRequest{URL: "https://hooks.example.com/x"}); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO support_sla_policies (workspace_id, name, enabled, first_response_minutes, active_since, pause_while_waiting, created_at, updated_at)
		VALUES (?, 'All', true, 5, now() - interval '10 years', true, now(), now())`, workspaceID)

	fake := intercomtest.New()
	defer fake.Close()
	now := time.Now().UTC()
	at := func(minutesAgo int) int64 { return now.Add(-time.Duration(minutesAgo) * time.Minute).Unix() }
	closedAt := at(100)
	closed := intercom.Conversation{ID: "901", State: "closed", Priority: "priority", CreatedAt: at(300), UpdatedAt: at(100),
		Title:  strp("Refund for March"),
		Source: intercom.ConversationSource{Type: "email", Subject: "Refund", Body: "Hi, I was charged twice.", Author: intercom.Author{Type: "user", ID: "u1", Name: "Ada Customer", Email: "ada@customer.test"}}}
	closed.Contacts.Contacts = append(closed.Contacts.Contacts, struct {
		ID string `json:"id"`
	}{ID: "u1"})
	closed.Tags.Tags = append(closed.Tags.Tags, struct {
		Name string `json:"name"`
	}{Name: "Billing"}, struct {
		Name string `json:"name"`
	}{Name: longIntercomTag})
	closed.Statistics = &struct {
		LastCloseAt *int64 `json:"last_close_at"`
	}{LastCloseAt: &closedAt}
	closed.Parts.Parts = []intercom.ConversationPart{
		{ID: "p1", PartType: "note", Body: strp("Checked Stripe, duplicate charge."), CreatedAt: at(250), Author: intercom.Author{Type: "admin", Name: "Grace", Email: "grace@acme.test"}},
		{ID: "p2", PartType: "comment", Body: strp("Refunded, sorry about that!"), CreatedAt: at(200), Author: intercom.Author{Type: "admin", Name: "Grace", Email: "GRACE@acme.test"}},
		{ID: "p3", PartType: "close", Body: nil, CreatedAt: at(100), Author: intercom.Author{Type: "admin", Name: "Grace"}},
	}
	open := intercom.Conversation{ID: "902", State: "open", CreatedAt: at(50), UpdatedAt: at(10),
		Source: intercom.ConversationSource{Type: "conversation", Body: "Can I export to CSV?", Author: intercom.Author{Type: "lead", ID: "l1", Name: "Lee"}}}
	open.Parts.Parts = []intercom.ConversationPart{
		{ID: "p4", PartType: "comment", Body: strp("Yes, from Reports."), CreatedAt: at(40), Author: intercom.Author{Type: "bot", Name: "Fin"}},
		{ID: "p5", PartType: "comment", Body: strp("Thanks, and PDF?"), CreatedAt: at(5), Author: intercom.Author{Type: "lead", Name: "Lee"}},
		{ID: "p6", PartType: "comment", CreatedAt: at(4), Author: intercom.Author{Type: "lead", Name: "Lee"}, Attachments: []intercom.Attachment{{Name: "invoice.pdf"}}},
	}
	open.Tags.Tags = append(open.Tags.Tags, struct {
		Name string `json:"name"`
	}{Name: strings.ToUpper(longIntercomTag)})
	empty := intercom.Conversation{ID: "903", State: "closed", CreatedAt: at(30), UpdatedAt: at(30)}
	old := intercom.Conversation{ID: "900", State: "closed", CreatedAt: now.AddDate(-2, 0, 0).Unix(), Source: intercom.ConversationSource{Body: "Old"}}
	fake.Conversations = []intercom.Conversation{old, closed, open, empty}
	fake.Contacts["u1"] = intercom.Contact{ID: "u1", Email: strp("ada@customer.test"), Name: strp("Ada Customer")}
	fake.RateLimitNext = 1

	imports := NewSupportImportService(repository.NewSupportImportRepository(db), inbox, []byte(strings.Repeat("i", 32)))
	imports.SetIntercomBaseURL(fake.URL)
	since := now.AddDate(-1, 0, 0).Format("2006-01-02")
	preview, err := imports.Preview(ctx, model.IntercomConversationImportRequest{AccessToken: intercomtest.Token, Since: since})
	if err != nil || preview.Conversations != 3 || preview.WorkspaceName != "Acme" {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	if _, err := imports.Start(ctx, workspaceID, userID, model.IntercomConversationImportRequest{AccessToken: "bad", Since: since}); err == nil {
		t.Fatal("a rejected token started an import")
	}
	job, err := imports.Start(ctx, workspaceID, userID, model.IntercomConversationImportRequest{AccessToken: intercomtest.Token, Since: since})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(job.PayloadEncrypted, intercomtest.Token) {
		t.Fatal("the access token is stored in plain text")
	}
	if _, err := imports.Start(ctx, workspaceID, userID, model.IntercomConversationImportRequest{AccessToken: intercomtest.Token, Since: since}); err == nil {
		t.Fatal("a second concurrent import was accepted")
	}
	for i := 0; i < 5; i++ {
		if worked, err := imports.ProcessNext(ctx); err != nil {
			t.Fatal(err)
		} else if !worked {
			break
		}
	}
	finished, err := imports.repo.GetJob(ctx, workspaceID, job.ID)
	if err != nil || finished.Status != model.SupportImportStatusDone || finished.Imported != 2 || finished.Failed != 1 || finished.Total != 3 {
		t.Fatalf("job: %+v %v", finished, err)
	}

	var conversations []model.SupportConversation
	db.Where("workspace_id = ? AND source = 'intercom'", workspaceID).Order("created_at").Find(&conversations)
	if len(conversations) != 2 {
		t.Fatalf("imported %d conversations", len(conversations))
	}
	refund, chat := conversations[0], conversations[1]
	if refund.Subject != "Refund for March" || refund.Status != model.SupportConversationStatusResolved || refund.Priority != "high" ||
		refund.Channel != "email" || refund.CustomerEmail == nil || *refund.CustomerEmail != "ada@customer.test" || refund.ResolvedAt == nil ||
		refund.ResolvedAt.Unix() != closedAt || refund.DisplayID == chat.DisplayID || refund.CustomerAwaitingResponse {
		t.Fatalf("refund conversation: %+v", refund)
	}
	if chat.Status != model.SupportConversationStatusOpen || chat.Channel != "widget" || !chat.CustomerAwaitingResponse ||
		chat.LastPublicSenderType == nil || *chat.LastPublicSenderType != "customer" || chat.TeamLastSeenAt == nil {
		t.Fatalf("open conversation: %+v", chat)
	}
	// The two customer messages after the bot reply are unanswered, counted
	// once although message triggers also project them.
	if chat.UnansweredCustomerMessageCount != 2 || !chat.NeedsHumanReply {
		t.Fatalf("open conversation state: unanswered=%d needs_reply=%v", chat.UnansweredCustomerMessageCount, chat.NeedsHumanReply)
	}
	var attachmentOnly model.SupportMessage
	db.Where("conversation_id = ? AND created_at = ?", chat.ID, time.Unix(at(4), 0)).Take(&attachmentOnly)
	if attachmentOnly.Content != "(Attachment not imported: invoice.pdf)" {
		t.Fatalf("attachment-only message: %q", attachmentOnly.Content)
	}
	// Imported history queues no AI tagging or translation work.
	var tagJobs, liveJobs int64
	db.Table("support_tag_jobs").Where("conversation_id IN ?", []string{refund.ID, chat.ID}).Count(&tagJobs)
	db.Table("support_live_messages").Where("conversation_id IN ?", []string{refund.ID, chat.ID}).Count(&liveJobs)
	if tagJobs != 0 || liveJobs != 0 {
		t.Fatalf("imported history queued %d tag jobs and %d translations", tagJobs, liveJobs)
	}
	if finished.PayloadEncrypted != "" {
		t.Fatal("a finished import kept its access token")
	}
	var messages []model.SupportMessage
	db.Where("conversation_id = ?", refund.ID).Order("created_at").Find(&messages)
	if len(messages) != 3 || messages[1].MessageType != "note" || !messages[1].IsInternal ||
		messages[2].SenderType != "user" || messages[2].SenderUserID == nil || *messages[2].SenderUserID != userID || messages[0].SenderType != "customer" {
		t.Fatalf("refund messages: %+v", messages)
	}
	var bot model.SupportMessage
	db.Where("conversation_id = ? AND sender_type = 'ai'", chat.ID).Take(&bot)
	if bot.Content != "Yes, from Reports." {
		t.Fatalf("bot reply: %+v", bot)
	}
	var tags int64
	db.Model(&model.SupportConversationTag{}).Where("conversation_id = ?", refund.ID).Count(&tags)
	if tags != 2 {
		t.Fatalf("tags: %d", tags)
	}
	var longTags []model.SupportTag
	db.Where("workspace_id = ? AND lower(name) LIKE 'customer escalations%'", workspaceID).Find(&longTags)
	if len(longTags) != 1 || longTags[0].Name != truncateWebhookText(longIntercomTag, supportImportTagNameLimit) {
		t.Fatalf("long tag: %+v", longTags)
	}
	// A later page finds the shortened tag instead of creating it again.
	later := newIntercomConversationImporter(imports, nil, *job)
	if id, err := later.tagID(ctx, longIntercomTag); err != nil || id != longTags[0].ID {
		t.Fatalf("long tag on a later page: %q %v", id, err)
	}
	var events int64
	db.Model(&model.WebhookEvent{}).Where("workspace_id = ?", workspaceID).Count(&events)
	if events != 0 {
		t.Fatalf("imported history produced %d webhook events", events)
	}
	sla := NewSupportSLAService(repository.NewSupportSLARepository(db), inbox)
	if _, err := sla.ProcessQueue(ctx, 50); err != nil {
		t.Fatal(err)
	}
	var records int64
	db.Model(&model.SupportConversationSLARecord{}).Where("conversation_id IN ?", []string{refund.ID, chat.ID}).Count(&records)
	if records != 0 {
		t.Fatalf("imported conversations got SLA records: %d", records)
	}

	// A job that crashed after storing conversations but before saving its
	// progress counts them as its own when it resumes.
	mustExec(t, db, `UPDATE support_import_jobs SET status = 'running', cursor = '', imported = 0, skipped = 0, failed = 0, failures = '[]', locked_until = NULL,
		payload_encrypted = ? WHERE id = ?`, job.PayloadEncrypted, job.ID)
	if worked, err := imports.ProcessNext(ctx); err != nil || !worked {
		t.Fatalf("resume: %v %v", worked, err)
	}
	resumed, _ := imports.repo.GetJob(ctx, workspaceID, job.ID)
	if resumed.Imported != 2 || resumed.Skipped != 0 {
		t.Fatalf("resumed job: %+v", resumed)
	}

	// Importing again skips everything already imported.
	again, err := imports.Start(ctx, workspaceID, userID, model.IntercomConversationImportRequest{AccessToken: intercomtest.Token, Since: since})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if worked, _ := imports.ProcessNext(ctx); !worked {
			break
		}
	}
	rerun, _ := imports.repo.GetJob(ctx, workspaceID, again.ID)
	var total int64
	db.Model(&model.SupportConversation{}).Where("workspace_id = ? AND source = 'intercom'", workspaceID).Count(&total)
	if rerun.Status != model.SupportImportStatusDone || rerun.Skipped != 2 || rerun.Imported != 0 || total != 2 {
		t.Fatalf("rerun: %+v conversations=%d", rerun, total)
	}

	// A cancelled import stops.
	cancelled, err := imports.Start(ctx, workspaceID, userID, model.IntercomConversationImportRequest{AccessToken: intercomtest.Token, Since: since})
	if err != nil {
		t.Fatal(err)
	}
	if err := imports.Cancel(ctx, workspaceID, cancelled.ID); err != nil {
		t.Fatal(err)
	}
	if worked, _ := imports.ProcessNext(ctx); worked {
		t.Fatal("a cancelled import was processed")
	}
	if err := imports.Cancel(ctx, workspaceID, uuid.NewString()); err == nil {
		t.Fatal("cancelling an unknown import succeeded")
	}
}

func TestIntercomConversationImportRecoveryPostgres(t *testing.T) {
	db := openSupportOpsPostgres(t)
	ctx := context.Background()
	workspaceID, userID := seedSupportOpsWorkspace(t, db)
	mustExec(t, db, `DELETE FROM support_import_jobs`)
	inbox := NewSupportInboxService(repository.NewSupportConversationRepository(db), repository.NewSupportMailboxRepository(db),
		repository.NewSupportMessageRepository(db), nil, nil, repository.NewSupportInboxInstallationRepository(db), nil, nil, nil, nil, nil,
		repository.NewUserRepository(db), nil, nil, nil)
	fake := intercomtest.New()
	defer fake.Close()
	repo := repository.NewSupportImportRepository(db)
	imports := NewSupportImportService(repo, inbox, []byte(strings.Repeat("i", 32)))
	imports.SetIntercomBaseURL(fake.URL)
	request := model.IntercomConversationImportRequest{AccessToken: intercomtest.Token}
	get := func(id string) *model.SupportImportJob {
		t.Helper()
		job, err := repo.GetJob(ctx, workspaceID, id)
		if err != nil {
			t.Fatal(err)
		}
		return job
	}

	// Intercom refusing the request stops the import at once.
	refused, err := imports.Start(ctx, workspaceID, userID, request)
	if err != nil {
		t.Fatal(err)
	}
	fake.SearchStatus, fake.SearchFailures = http.StatusBadRequest, 1
	if worked, err := imports.ProcessNext(ctx); !worked || err != nil {
		t.Fatalf("refused page: %v %v", worked, err)
	}
	if job := get(refused.ID); job.Status != model.SupportImportStatusFailed || !strings.Contains(job.Error, "Search failed") || job.PayloadEncrypted != "" {
		t.Fatalf("refused import: %+v", job)
	}

	// An unavailable API is retried later, then gives up.
	flaky, err := imports.Start(ctx, workspaceID, userID, request)
	if err != nil {
		t.Fatal(err)
	}
	fake.SearchStatus, fake.SearchFailures = http.StatusServiceUnavailable, 6
	if worked, err := imports.ProcessNext(ctx); !worked || err != nil {
		t.Fatalf("flaky page: %v %v", worked, err)
	}
	job := get(flaky.ID)
	if job.Status != model.SupportImportStatusRunning || job.Attempts != 1 || job.LockedUntil == nil || time.Until(*job.LockedUntil) < 50*time.Second {
		t.Fatalf("retried import: %+v", job)
	}
	mustExec(t, db, `UPDATE support_import_jobs SET attempts = ?, locked_until = NULL WHERE id = ?`, supportImportMaxAttempts-1, flaky.ID)
	fake.SearchFailures = 6
	if worked, err := imports.ProcessNext(ctx); !worked || err != nil {
		t.Fatalf("last try: %v %v", worked, err)
	}
	if job := get(flaky.ID); job.Status != model.SupportImportStatusFailed || job.PayloadEncrypted != "" {
		t.Fatalf("given-up import: %+v", job)
	}

	// Only the worker holding the lease writes, and a cancel wins over the
	// worker finishing.
	leased, err := imports.Start(ctx, workspaceID, userID, request)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	claimed, err := repo.ClaimJob(ctx, now, now.Add(time.Minute), "lease-a")
	if err != nil || claimed == nil || claimed.ID != leased.ID {
		t.Fatalf("claim: %+v %v", claimed, err)
	}
	if held, err := repo.SaveLeased(ctx, leased.ID, "lease-b", map[string]any{"imported": 99}); err != nil || held {
		t.Fatalf("a stale worker wrote progress: %v %v", held, err)
	}
	if err := imports.Cancel(ctx, workspaceID, leased.ID); err != nil {
		t.Fatal(err)
	}
	if held, err := repo.SaveLeased(ctx, leased.ID, "lease-a", map[string]any{"imported": 3}); err != nil || !held {
		t.Fatalf("the cancelled page's counts were not saved: %v %v", held, err)
	}
	if finished, err := repo.FinishJob(ctx, leased.ID, "lease-a", model.SupportImportStatusDone, "", now); err != nil || finished {
		t.Fatalf("a cancelled import was finished: %v %v", finished, err)
	}
	if job := get(leased.ID); job.Status != model.SupportImportStatusCancelled || job.Imported != 3 || job.PayloadEncrypted != "" {
		t.Fatalf("cancelled import: %+v", job)
	}
	if held, err := repo.ExtendLease(ctx, leased.ID, "lease-a", now.Add(time.Hour)); err != nil || held {
		t.Fatalf("a cancelled import kept its lease: %v %v", held, err)
	}

	// Jobs take turns: the one worked on least recently goes first.
	first, second := uuid.NewString(), uuid.NewString()
	for _, id := range []string{first, second} {
		if err := repo.CreateJob(ctx, &model.SupportImportJob{ID: id, WorkspaceID: workspaceID, Source: model.SupportImportSourceIntercom,
			Status: model.SupportImportStatusRunning, Since: now, Failures: []byte("[]"), PayloadEncrypted: "x", StartedBy: userID}); err != nil {
			t.Fatal(err)
		}
	}
	mustExec(t, db, `UPDATE support_import_jobs SET updated_at = ? WHERE id = ?`, now.Add(-time.Minute), first)
	mustExec(t, db, `UPDATE support_import_jobs SET updated_at = ?, created_at = ? WHERE id = ?`, now.Add(-time.Hour), now.Add(time.Minute), second)
	if next, err := repo.ClaimJob(ctx, now, now.Add(time.Minute), "lease-c"); err != nil || next == nil || next.ID != second {
		t.Fatalf("claimed %+v, want the job that waited longest", next)
	}
}
