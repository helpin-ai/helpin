package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/llm"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/storage"
)

func TestSupportAgentFindsPreviousResolvedConversations(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	seedProductToolConversation(t, f.db)
	mustExec(t, f.db, `UPDATE support_conversations SET workspace_id=?, status='resolved' WHERE id='conv-1'`, f.workspaceID)
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetSupportOperationalServices(f.supportSvc, nil)
	output, err := svc.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: f.workspaceID, AgentID: "agent", AgentScopeResolved: true, TargetType: "workspace", TargetID: f.workspaceID}, "support.list_conversations", json.RawMessage(`{"query":"Widget"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "conv-1") {
		t.Fatalf("earlier resolved conversation missing: %s", output)
	}
}

func TestSupportAgentListsInboxesWithoutHumanIdentity(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetSupportOperationalServices(f.supportSvc, nil)
	output, err := svc.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: f.workspaceID, AgentID: "agent", AgentScopeResolved: true, TargetType: "workspace", TargetID: f.workspaceID}, "support.list_inboxes", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "shared") {
		t.Fatalf("shared inbox missing: %s", output)
	}
}

func TestSupportMessageToolSignsPrivateAttachments(t *testing.T) {
	db := newTestDB(t)
	seedProductToolConversation(t, db)
	createSupportAttachmentTestTable(t, db)
	mustExec(t, db, `INSERT INTO support_attachments(id,workspace_id,conversation_id,message_id,file_name,file_size,content_type,storage_key,public_url,is_uploaded,uploaded_by_type) VALUES ('image','ws-1','conv-1','msg-1','error.png',20000,'image/png','private/error.png','',true,'customer')`)
	repo := repository.NewSupportAttachmentRepository(db)
	attachments := NewSupportAttachmentService(repo, storage.NewS3Client("test", "test-secret", "bucket", "us-east-1", "https://storage.test", ""))
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetSupportDependencies(repository.NewSupportMessageRepository(db), repository.NewSupportConversationRepository(db), nil)
	svc.SetSupportAttachmentRepository(repo)
	svc.SetSupportOperationalServices(&SupportInboxService{attachmentService: attachments}, nil)
	output, err := svc.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws-1", TargetType: "workspace", TargetID: "ws-1"}, "support.list_conversation_messages", json.RawMessage(`{"conversation_id":"conv-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "X-Amz-Signature") {
		t.Fatalf("private image has no signed URL: %s", output)
	}
}

func TestSupportMessageToolReadsScreenshot(t *testing.T) {
	db := newTestDB(t)
	seedProductToolConversation(t, db)
	createSupportAttachmentTestTable(t, db)
	mustExec(t, db, `INSERT INTO support_attachments(id,workspace_id,conversation_id,message_id,file_name,file_size,content_type,storage_key,public_url,is_uploaded,uploaded_by_type) VALUES ('image','ws-1','conv-1','msg-1','error.png',20000,'image/png','private/error.png','',true,'customer')`)
	repo := repository.NewSupportAttachmentRepository(db)
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetSupportDependencies(repository.NewSupportMessageRepository(db), repository.NewSupportConversationRepository(db), nil)
	svc.SetSupportOperationalServices(&SupportInboxService{attachmentService: NewSupportAttachmentService(repo, storage.NewS3Client("test", "test-secret", "bucket", "us-east-1", "https://storage.test", ""))}, nil)
	calls := 0
	svc.SetSupportMediaAnalyzer(dockMediaCompletionFunc(func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
		calls++
		if len(req.Messages) != 1 || len(req.Messages[0].ContentParts) < 2 {
			t.Fatalf("media not delivered: %#v", req.Messages)
		}
		found := false
		for _, part := range req.Messages[0].ContentParts {
			if part.ImageURL != nil && strings.Contains(part.ImageURL.URL, "X-Amz-Signature") {
				found = true
			}
		}
		if !found {
			t.Fatal("no signed image in vision request")
		}
		if req.Model != "google/gemini-3.8-flash" {
			t.Fatalf("not vision route: %s", req.Model)
		}
		return &llm.ChatResponse{Content: "image: LinkedIn rejected the upload with HTTP 403."}, nil
	}))
	output, err := svc.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws-1", TargetType: "support_conversation", TargetID: "conv-1"}, "support.list_conversation_messages", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !strings.Contains(string(output), "LinkedIn rejected") {
		t.Fatalf("no image understanding: calls=%d output=%s", calls, output)
	}
}

func TestSupportAgentDiscoveryRespectsWorkspaceAndMailboxTeams(t *testing.T) {
	f := newSupportTriageTestFixture(t, nil, nil)
	a := f.createMailbox(t, "Allowed", "allowed", false)
	b := f.createMailbox(t, "Other", "other", false)
	mustExec(t, f.db, `UPDATE support_mailboxes SET linked_team_id='team-a' WHERE id=?`, a.ID)
	mustExec(t, f.db, `UPDATE support_mailboxes SET linked_team_id='team-b' WHERE id=?`, b.ID)
	shared := f.createConversation(t, "shared history", "customer@example.com", nil)
	allowed := f.createConversation(t, "allowed history", "customer@example.com", &a.ID)
	denied := f.createConversation(t, "private history", "customer@example.com", &b.ID)
	foreign := f.createConversation(t, "foreign history", "customer@example.com", nil)
	mustExec(t, f.db, `UPDATE support_conversations SET workspace_id='another-workspace' WHERE id=?`, foreign.ID)
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetSupportOperationalServices(f.supportSvc, nil)
	svc.SetSupportDependencies(f.messageRepo, f.conversationRepo, nil)
	meta := model.InternalCommandContext{WorkspaceID: f.workspaceID, AgentID: "agent", AgentScopeResolved: true, AgentTeamIDs: []string{"team-a"}, TargetType: "workspace", TargetID: f.workspaceID}
	output, err := svc.Execute(context.Background(), meta, "support.list_conversations", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{shared.ID, allowed.ID} {
		if !strings.Contains(string(output), id) {
			t.Errorf("allowed history %s missing: %s", id, output)
		}
	}
	for _, id := range []string{denied.ID, foreign.ID} {
		if strings.Contains(string(output), id) {
			t.Errorf("inaccessible history %s leaked", id)
		}
	}
	for _, id := range []string{denied.ID, foreign.ID} {
		input, _ := json.Marshal(map[string]string{"conversation_id": id})
		if _, err := svc.Execute(context.Background(), meta, "support.list_conversation_messages", input); err == nil {
			t.Errorf("read inaccessible history %s", id)
		}
	}
	output, err = svc.Execute(context.Background(), meta, "support.list_inboxes", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), a.ID) || strings.Contains(string(output), b.ID) {
		t.Fatalf("wrong mailbox scope: %s", output)
	}
	// An unresolved automatic identity must not receive workspace-wide access.
	meta.AgentScopeResolved = false
	output, err = svc.Execute(context.Background(), meta, "support.list_conversations", json.RawMessage(`{}`))
	if err == nil && (strings.Contains(string(output), allowed.ID) || strings.Contains(string(output), shared.ID)) {
		t.Fatalf("unresolved identity received access: %s", output)
	}
}

func TestSupportAgentScopeDoesNotReplaceHumanPermissions(t *testing.T) {
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	actor := &authorization.Actor{WorkspaceID: "ws", UserID: "user", Role: model.RoleViewer, WorkspaceMemberID: "member"}
	ctx := authorization.WithActor(context.Background(), actor)
	got := svc.supportOperationalContext(ctx, model.InternalCommandContext{WorkspaceID: "ws", AgentID: "agent", AgentScopeResolved: true})
	if authorization.GetActor(got) != actor || authorization.GetSupportAgentScope(got) != nil {
		t.Fatal("agent scope replaced human permissions")
	}
}

type supportMediaCompletionCapture struct{ request AICompletionRequest }

func (p *supportMediaCompletionCapture) ChatCompletion(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return nil, fmt.Errorf("must use metered completion")
}
func (p *supportMediaCompletionCapture) Complete(_ context.Context, req AICompletionRequest) (*llm.ChatResponse, error) {
	p.request = req
	return &llm.ChatResponse{Content: "Screenshot observation"}, nil
}

func TestSupportMediaUsesSupportUsagePolicy(t *testing.T) {
	provider := &supportMediaCompletionCapture{}
	_, err := analyzeAttachmentMedia(context.Background(), provider, "ws", BillingFeatureSupportAIReply, aipolicy.ActionSupportMediaEnrichment, "explain", []mediaAttachment{{ID: "image", FileName: "screenshot.png", FileType: "image/png", URL: "https://storage.test/signed"}})
	if err != nil {
		t.Fatal(err)
	}
	r := provider.request
	if r.FeatureKey != BillingFeatureSupportAIReply || r.ActionKey != aipolicy.ActionSupportMediaEnrichment || r.OperationKey != AIUsageOperationMediaEnrichment || r.IdempotencyKey == "" || r.WorkspaceID != "ws" {
		t.Fatalf("incorrect usage attribution: %+v", r)
	}
	action, ok := aipolicy.DefaultRegistry().Lookup(r.ActionKey)
	if !ok || !action.Chargeable || action.FeatureKey != BillingFeatureSupportAIReply {
		t.Fatalf("missing metered support media policy: %+v", action)
	}
}

func TestSupportMessageToolPreservesHistoryWhenMediaUnavailable(t *testing.T) {
	db := newTestDB(t)
	seedProductToolConversation(t, db)
	createSupportAttachmentTestTable(t, db)
	mustExec(t, db, `INSERT INTO support_attachments(id,workspace_id,conversation_id,message_id,file_name,file_size,content_type,storage_key,public_url,is_uploaded,uploaded_by_type) VALUES ('image','ws-1','conv-1','msg-1','error.png',20000,'image/png','','https://storage.test/image.png',true,'customer')`)
	svc := NewInternalCommandService(nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetSupportDependencies(repository.NewSupportMessageRepository(db), repository.NewSupportConversationRepository(db), nil)
	svc.SetSupportAttachmentRepository(repository.NewSupportAttachmentRepository(db))
	svc.SetSupportMediaAnalyzer(dockMediaCompletionFunc(func(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
		return nil, fmt.Errorf("media unavailable")
	}))
	output, err := svc.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws-1", TargetType: "support_conversation", TargetID: "conv-1"}, "support.list_conversation_messages", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("image failure hid readable history: %v", err)
	}
	if !strings.Contains(string(output), "The widget will not load.") || !strings.Contains(string(output), "attachment_analysis_error") {
		t.Fatalf("history or explicit media limitation missing: %s", output)
	}
}
