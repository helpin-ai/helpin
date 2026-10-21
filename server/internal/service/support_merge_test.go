package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestMergeRequiresExplicitWorkspaceActor(t *testing.T) {
	for _, actor := range []*authorization.Actor{nil, {WorkspaceID: "other", UserID: "user", Role: "owner"}, {WorkspaceID: "ws", UserID: "user", Role: "viewer"}} {
		ctx := context.Background()
		if actor != nil {
			ctx = authorization.WithActor(ctx, actor)
		}
		if _, err := requireSupportMergeActor(ctx, "ws"); err == nil {
			t.Fatalf("accepted actor %+v", actor)
		}
	}
}

func TestMergePreviewFingerprintTracksAudienceAndOwnership(t *testing.T) {
	p := model.SupportMergePreview{Target: model.SupportConversation{ID: "target", MergeRevision: 2}, Sources: []model.SupportConversation{{ID: "source"}}, Participants: []string{"sam@example.test"}}
	first := supportMergeFingerprint("ws", "actor", p)
	p.Participants = append(p.Participants, "alex@example.test")
	if supportMergeFingerprint("ws", "actor", p) == first {
		t.Fatal("changed recipients accepted stale preview")
	}
	p.Participants = p.Participants[:1]
	p.Target.MergeRevision++
	if supportMergeFingerprint("ws", "actor", p) == first {
		t.Fatal("changed merge graph accepted stale preview")
	}
	if supportMergeFingerprint("ws", "another-actor", p) == supportMergeFingerprint("ws", "actor", p) {
		t.Fatal("preview not bound to actor")
	}
}

func TestMergeServiceReviewCommitAndRestoration(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&model.SupportPendingSend{}, &model.SupportAIFollowUp{}); err != nil {
		t.Fatal(err)
	}
	ws := "11111111-1111-1111-1111-111111111111"
	actorID := "22222222-2222-2222-2222-222222222222"
	actor := &authorization.Actor{WorkspaceID: ws, UserID: actorID, WorkspaceMemberID: "member", Role: "owner"}
	ctx := authorization.WithActor(context.Background(), actor)
	sourceID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	targetID := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	email := "sam@example.test"
	for _, id := range []string{sourceID, targetID} {
		if err := db.Create(&model.SupportConversation{ID: id, WorkspaceID: ws, Subject: "Test", Status: "open", CustomerEmail: &email}).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := &SupportInboxService{conversationRepo: repository.NewSupportConversationRepository(db), installationRepo: repository.NewSupportInboxInstallationRepository(db), messageRepo: repository.NewSupportMessageRepository(db)}
	request := model.SupportMergeRequest{SourceIDs: []string{sourceID}, OperationID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc"}
	preview, err := svc.PreviewConversationMerge(ctx, ws, targetID, request)
	if err != nil {
		t.Fatal(err)
	}
	request.PreviewToken = preview.PreviewToken
	if err := db.Model(&model.SupportConversation{}).Where("id = ?", targetID).Update("customer_email", "changed@example.test").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.MergeConversations(ctx, ws, targetID, request); err == nil {
		t.Fatal("stale audience committed")
	}
	if err := db.Model(&model.SupportConversation{}).Where("id = ?", targetID).Update("customer_email", email).Error; err != nil {
		t.Fatal(err)
	}
	result, err := svc.MergeConversations(ctx, ws, targetID, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Merges) != 1 {
		t.Fatalf("merge result: %+v", result)
	}
	undo, err := svc.PreviewConversationUnmerge(ctx, ws, result.Merges[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := svc.UnmergeConversation(ctx, ws, result.Merges[0].ID, model.SupportMergeRequest{OperationID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", PreviewToken: undo.PreviewToken})
	if err != nil {
		t.Fatal(err)
	}
	if restored.ConversationID != sourceID {
		t.Fatalf("restored wrong conversation: %+v", restored)
	}
}

func TestMergeServicePolicyAccessAndMissingRestoreInbox(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&model.SupportPendingSend{}, &model.SupportAIFollowUp{}); err != nil {
		t.Fatal(err)
	}
	ws := "11111111-1111-1111-1111-111111111111"
	actor := &authorization.Actor{WorkspaceID: ws, UserID: "actor", WorkspaceMemberID: "member", Role: model.RoleOwner}
	ctx := authorization.WithActor(context.Background(), actor)
	a, b := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	for _, row := range []struct{ id, email string }{{a, "a@example.test"}, {b, "b@example.test"}} {
		if err := db.Create(&model.SupportConversation{ID: row.id, WorkspaceID: ws, Subject: "Test", Status: "open", CustomerEmail: &row.email}).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := &SupportInboxService{conversationRepo: repository.NewSupportConversationRepository(db), installationRepo: repository.NewSupportInboxInstallationRepository(db), messageRepo: repository.NewSupportMessageRepository(db)}
	req := model.SupportMergeRequest{SourceIDs: []string{a}, OperationID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc"}
	if _, err := svc.PreviewConversationMerge(ctx, ws, b, req); err == nil {
		t.Fatal("cross-customer accepted without opt-in")
	}
	settings := model.DefaultSupportInboxSettings()
	settings.AllowCrossCustomerMerge = true
	data, _ := json.Marshal(settings)
	if err := svc.installationRepo.Create(ctx, &model.SupportWidgetInstallation{WorkspaceID: ws, WidgetKey: "key", SecretKey: "secret", Settings: string(data), Active: true}); err != nil {
		t.Fatal(err)
	}
	preview, err := svc.PreviewConversationMerge(ctx, ws, b, req)
	if err != nil || !preview.CrossCustomer {
		t.Fatalf("opt-in preview: %+v %v", preview, err)
	}
	req.PreviewToken = preview.PreviewToken
	private := "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	if err := db.Model(&model.SupportConversation{}).Where("id = ?", a).Update("mailbox_id", private).Error; err != nil {
		t.Fatal(err)
	}
	member := *actor
	member.Role = model.RoleMember
	if _, err := svc.PreviewConversationMerge(authorization.WithActor(context.Background(), &member), ws, b, req); !errors.Is(err, repository.ErrSupportMergeNotFound) {
		t.Fatalf("private source exposed: %v", err)
	}
	if _, err := svc.MergeConversations(ctx, ws, b, req); !errors.Is(err, repository.ErrSupportMergeChanged) {
		t.Fatalf("ownership change did not invalidate preview: %v", err)
	}
	preview, err = svc.PreviewConversationMerge(ctx, ws, b, req)
	if err != nil {
		t.Fatal(err)
	}
	req.PreviewToken = preview.PreviewToken
	result, err := svc.MergeConversations(ctx, ws, b, req)
	if err != nil {
		t.Fatal(err)
	}
	undo, err := svc.PreviewConversationUnmerge(ctx, ws, result.Merges[0].ID)
	if err != nil || !undo.RestoreInboxRequired || undo.PreviewToken != "" {
		t.Fatalf("missing inbox was accepted: %+v %v", undo, err)
	}
	shared := "shared"
	undo, err = svc.PreviewConversationUnmerge(ctx, ws, result.Merges[0].ID, &shared)
	if err != nil || undo.RestoreInboxRequired || undo.PreviewToken == "" {
		t.Fatalf("shared restore preview: %+v %v", undo, err)
	}
	if _, err := svc.UnmergeConversation(ctx, ws, result.Merges[0].ID, model.SupportMergeRequest{OperationID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", PreviewToken: undo.PreviewToken, RestoreMailboxID: &shared}); err != nil {
		t.Fatal(err)
	}
}
