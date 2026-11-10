package service

import (
	"context"
	"testing"
	"time"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestPortalReplyExpectedBy(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	soon, later, past := now.Add(20*time.Minute), now.Add(3*time.Hour), now.Add(-time.Minute)
	for _, tc := range []struct {
		name   string
		record model.SupportConversationSLARecord
		want   *time.Time
	}{
		{"first response running", model.SupportConversationSLARecord{FirstResponseState: model.SupportSLAStatePending, FirstResponseDueAt: &soon, ResolutionState: model.SupportSLAStatePending, ResolutionDueAt: &later}, &soon},
		{"next response running", model.SupportConversationSLARecord{FirstResponseState: model.SupportSLAStateAchieved, FirstResponseDueAt: &past, NextResponseState: model.SupportSLAStatePending, NextResponseDueAt: &later}, &later},
		{"only resolution running stays internal", model.SupportConversationSLARecord{ResolutionState: model.SupportSLAStatePending, ResolutionDueAt: &later}, nil},
		{"paused while the AI answers", model.SupportConversationSLARecord{FirstResponseState: model.SupportSLAStatePaused}, nil},
		{"overdue is never shown", model.SupportConversationSLARecord{FirstResponseState: model.SupportSLAStatePending, FirstResponseDueAt: &past}, nil},
		{"no policy applies any more", model.SupportConversationSLARecord{FirstResponseState: model.SupportSLAStatePending, FirstResponseDueAt: &soon, DetachedAt: &past}, nil},
	} {
		got := portalReplyExpectedBy(tc.record, now)
		if (got == nil) != (tc.want == nil) || (got != nil && !got.Equal(*tc.want)) {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestCustomerPortalShowsReplyExpectedByWhenEnabled(t *testing.T) {
	svc, db, _, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	svc.inbox.SetSLAService(NewSupportSLAService(repository.NewSupportSLARepository(db), svc.inbox))
	setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	access := signInPortal(t, svc, ws, issuePortalLink(t, db, ws.ID, "customer@example.com", time.Now().Add(-time.Hour)))
	request, err := svc.CreateRequest(ctx, access, "Export broken", "It fails.", nil, nil, PortalRequestFields{})
	if err != nil {
		t.Fatal(err)
	}
	var conversationID string
	if err := db.Table("support_portal_request_references").Where("reference = ?", request.Reference).Pluck("conversation_id", &conversationID).Error; err != nil {
		t.Fatal(err)
	}
	due := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	if err := db.Create(&model.SupportConversationSLARecord{ConversationID: conversationID, WorkspaceID: ws.ID,
		FirstResponseState: model.SupportSLAStatePending, FirstResponseDueAt: &due, NextDueAt: &due}).Error; err != nil {
		t.Fatal(err)
	}

	detail, err := svc.RequestDetail(ctx, access, request.Reference)
	if err != nil || detail.ReplyExpectedBy != nil {
		t.Fatalf("shown while the setting is off: %+v %v", detail.ReplyExpectedBy, err)
	}
	access.Workspace.Settings.PortalShowReplyBy = true
	detail, err = svc.RequestDetail(ctx, access, request.Reference)
	if err != nil || detail.ReplyExpectedBy == nil || !detail.ReplyExpectedBy.Equal(due) {
		t.Fatalf("detail reply-by: %+v %v", detail.ReplyExpectedBy, err)
	}
	requests, err := svc.Requests(ctx, access, "")
	if err != nil || len(requests) != 1 || requests[0].ReplyExpectedBy == nil || !requests[0].ReplyExpectedBy.Equal(due) {
		t.Fatalf("list reply-by: %+v %v", requests, err)
	}
}
