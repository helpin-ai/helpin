package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func newCSATTestService(t *testing.T, csatEnabled bool) (*SupportInboxService, *gorm.DB, string) {
	t.Helper()
	db := newTestDB(t)
	workspaceID := uuid.NewString()
	seedWorkspace(t, db, workspaceID, "CSAT", "csat-"+workspaceID[:8], "owner")
	settings, err := json.Marshal(map[string]any{"csat_enabled": csatEnabled})
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO support_widget_installations (id, workspace_id, widget_key, secret_key, settings, active) VALUES (?, ?, ?, 'secret', ?, true)`,
		uuid.NewString(), workspaceID, "key-"+workspaceID[:8], string(settings))
	svc := NewSupportInboxService(
		repository.NewSupportConversationRepository(db), repository.NewSupportMailboxRepository(db),
		repository.NewSupportMessageRepository(db), nil, nil, repository.NewSupportInboxInstallationRepository(db),
		repository.NewSupportInboxSessionRepository(db), nil, nil, nil, nil, nil, nil, nil, nil,
	)
	return svc, db, workspaceID
}

func seedCSATConversation(t *testing.T, db *gorm.DB, workspaceID, anonymousID, status string) *model.SupportConversation {
	t.Helper()
	assigned := uuid.NewString()
	flow := model.SupportConversationFlowStateResolvedByHuman
	conversation := &model.SupportConversation{
		ID: uuid.NewString(), WorkspaceID: workspaceID, Subject: "Billing question", Status: status,
		FlowState: &flow, AnonymousID: &anonymousID, AssignedUserID: &assigned, CustomerName: strPtr("Ada"),
	}
	if err := repository.NewSupportConversationRepository(db).Create(context.Background(), conversation); err != nil {
		t.Fatal(err)
	}
	return conversation
}

func seedCSATSession(t *testing.T, db *gorm.DB, workspaceID, anonymousID string) string {
	t.Helper()
	token := "token-" + uuid.NewString()
	session := &model.SupportWidgetSession{
		WorkspaceID: workspaceID, SessionToken: token, AnonymousID: anonymousID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := repository.NewSupportInboxSessionRepository(db).Create(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	return token
}

func csatTimelineEvents(t *testing.T, db *gorm.DB, conversationID string) []model.SupportMessage {
	t.Helper()
	var messages []model.SupportMessage
	if err := db.Where("conversation_id = ? AND system_event_type = ?", conversationID, model.SystemEventCSATRated).
		Order("created_at").Find(&messages).Error; err != nil {
		t.Fatal(err)
	}
	return messages
}

func TestSubmitWidgetCSATStoresRatingAndTimeline(t *testing.T) {
	svc, db, workspaceID := newCSATTestService(t, true)
	ctx := context.Background()
	conversation := seedCSATConversation(t, db, workspaceID, "anon-1", model.SupportConversationStatusResolved)
	token := seedCSATSession(t, db, workspaceID, "anon-1")

	if _, err := svc.SubmitWidgetCSAT(ctx, token, conversation.ID, 6, ""); !errors.Is(err, ErrSupportCSATInvalid) {
		t.Fatalf("out-of-range rating: %v", err)
	}
	summary, err := svc.SubmitWidgetCSAT(ctx, token, conversation.ID, 4, "  Quick and clear  ")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Rating != 4 || summary.Comment == nil || *summary.Comment != "Quick and clear" {
		t.Fatalf("summary: %+v", summary)
	}
	var stored model.SupportCSATResponse
	if err := db.Where("conversation_id = ?", conversation.ID).Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Channel != model.SupportCSATChannelWidget || stored.ResolvedBy != model.SupportCSATResolvedByHuman ||
		stored.AssignedUserID == nil || *stored.AssignedUserID != *conversation.AssignedUserID {
		t.Fatalf("stored: %+v", stored)
	}
	events := csatTimelineEvents(t, db, conversation.ID)
	if len(events) != 1 || !events[0].IsInternal || events[0].Content != "Customer rated 4/5 (Satisfied)" {
		t.Fatalf("timeline: %+v", events)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(events[0].Metadata), &metadata); err != nil || metadata["rating"] != float64(4) || metadata["comment"] != "Quick and clear" {
		t.Fatalf("metadata %v %v", metadata, err)
	}

	// Resubmitting the same rating changes nothing.
	if _, err := svc.SubmitWidgetCSAT(ctx, token, conversation.ID, 4, "Quick and clear"); err != nil {
		t.Fatal(err)
	}
	if got := len(csatTimelineEvents(t, db, conversation.ID)); got != 1 {
		t.Fatalf("unchanged rating added a timeline entry: %d", got)
	}

	// Changing the rating updates the row and records the change.
	if _, err := svc.SubmitWidgetCSAT(ctx, token, conversation.ID, 2, ""); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.SupportCSATResponse{}).Where("conversation_id = ?", conversation.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("rows: %d %v", count, err)
	}
	events = csatTimelineEvents(t, db, conversation.ID)
	if len(events) != 2 || events[1].Content != "Customer changed their rating to 2/5 (Unsatisfied)" {
		t.Fatalf("change timeline: %+v", events)
	}

	conversations, err := svc.GetVisitorConversations(ctx, workspaceID, "anon-1")
	if err != nil {
		t.Fatal(err)
	}
	public := model.PublicWidgetConversations(conversations)
	if len(public) != 1 || public[0].CSATRating == nil || *public[0].CSATRating != 2 {
		t.Fatalf("widget conversation: %+v", public)
	}
}

func TestSubmitWidgetCSATRejectsUnavailableConversations(t *testing.T) {
	ctx := context.Background()
	t.Run("another visitor", func(t *testing.T) {
		svc, db, workspaceID := newCSATTestService(t, true)
		conversation := seedCSATConversation(t, db, workspaceID, "anon-1", model.SupportConversationStatusResolved)
		token := seedCSATSession(t, db, workspaceID, "anon-2")
		if _, err := svc.SubmitWidgetCSAT(ctx, token, conversation.ID, 5, ""); !errors.Is(err, ErrSupportCSATUnavailable) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("open conversation", func(t *testing.T) {
		svc, db, workspaceID := newCSATTestService(t, true)
		conversation := seedCSATConversation(t, db, workspaceID, "anon-1", model.SupportConversationStatusOpen)
		token := seedCSATSession(t, db, workspaceID, "anon-1")
		if _, err := svc.SubmitWidgetCSAT(ctx, token, conversation.ID, 5, ""); !errors.Is(err, ErrSupportCSATUnavailable) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("csat disabled", func(t *testing.T) {
		svc, db, workspaceID := newCSATTestService(t, false)
		conversation := seedCSATConversation(t, db, workspaceID, "anon-1", model.SupportConversationStatusResolved)
		token := seedCSATSession(t, db, workspaceID, "anon-1")
		if _, err := svc.SubmitWidgetCSAT(ctx, token, conversation.ID, 5, ""); !errors.Is(err, ErrSupportCSATUnavailable) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("expired session", func(t *testing.T) {
		svc, db, workspaceID := newCSATTestService(t, true)
		conversation := seedCSATConversation(t, db, workspaceID, "anon-1", model.SupportConversationStatusResolved)
		if _, err := svc.SubmitWidgetCSAT(ctx, "missing-token", conversation.ID, 5, ""); !errors.Is(err, ErrSupportWidgetSessionInvalid) {
			t.Fatalf("got %v", err)
		}
	})
}

func TestSubmitConversationCSATAppliesToMergeTarget(t *testing.T) {
	svc, db, workspaceID := newCSATTestService(t, true)
	ctx := context.Background()
	target := seedCSATConversation(t, db, workspaceID, "anon-1", model.SupportConversationStatusResolved)
	source := seedCSATConversation(t, db, workspaceID, "anon-1", model.SupportConversationStatusResolved)
	mustExec(t, db, `UPDATE support_conversations SET merged_into_id = ? WHERE id = ?`, target.ID, source.ID)
	if err := db.Create(&model.SupportConversationMerge{
		ID: uuid.NewString(), WorkspaceID: workspaceID, OperationID: uuid.NewString(),
		SourceID: source.ID, TargetID: target.ID, ActorID: uuid.NewString(), CreatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	token := seedCSATSession(t, db, workspaceID, "anon-1")
	if _, err := svc.SubmitWidgetCSAT(ctx, token, source.ID, 5, ""); err != nil {
		t.Fatal(err)
	}
	var stored model.SupportCSATResponse
	if err := db.Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.ConversationID != target.ID {
		t.Fatalf("rating stored on %s, want merge target %s", stored.ConversationID, target.ID)
	}
	conversations := []model.SupportConversation{*source}
	conversations[0].MergedIntoID = &target.ID
	if err := svc.hydrateConversationCSAT(ctx, workspaceID, conversations); err != nil {
		t.Fatal(err)
	}
	if conversations[0].CSAT == nil || conversations[0].CSAT.Rating != 5 {
		t.Fatalf("merged conversation shows %+v", conversations[0].CSAT)
	}
}

func TestCustomerPortalRateRequest(t *testing.T) {
	svc, db, _, ws := setupCustomerPortalService(t)
	ctx := context.Background()
	setPortalContact(t, db, ws.ID, "customer@example.com", portalAccess(model.PortalAccessAllowed))
	ws = setPortalSettings(t, svc, db, ws, `{"portal_enabled":true,"portal_intake_enabled":true,"csat_enabled":true}`)
	access := signInPortal(t, svc, ws, issuePortalLink(t, db, ws.ID, "customer@example.com", time.Now().Add(-time.Hour)))
	request, err := svc.CreateRequest(ctx, access, "Export broken", "Please help", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := svc.RequestDetail(ctx, access, request.Reference)
	if err != nil {
		t.Fatal(err)
	}
	if detail.CanRate || detail.CSAT != nil {
		t.Fatalf("open request offered rating: %+v", detail)
	}
	if _, err := svc.RateRequest(ctx, access, request.Reference, 5, ""); !errors.Is(err, ErrSupportCSATUnavailable) {
		t.Fatalf("rating an open request: %v", err)
	}
	mustExec(t, db, `UPDATE support_conversations SET status = 'resolved' WHERE workspace_id = ?`, ws.ID)
	detail, err = svc.RateRequest(ctx, access, request.Reference, 5, "Fixed fast")
	if err != nil {
		t.Fatal(err)
	}
	if !detail.CanRate || detail.CSAT == nil || detail.CSAT.Rating != 5 || detail.CSAT.Comment == nil || *detail.CSAT.Comment != "Fixed fast" {
		t.Fatalf("rated detail: %+v", detail)
	}
	var stored model.SupportCSATResponse
	if err := db.Take(&stored).Error; err != nil || stored.Channel != model.SupportCSATChannelPortal {
		t.Fatalf("stored %+v %v", stored, err)
	}
	if _, err := svc.RateRequest(ctx, access, "missing", 5, ""); !errors.Is(err, ErrPortalRequestNotFound) {
		t.Fatalf("unknown reference: %v", err)
	}
}

func TestSubmitConversationCSATRejectsCrossCustomerMerge(t *testing.T) {
	svc, db, workspaceID := newCSATTestService(t, true)
	ctx := context.Background()
	target := seedCSATConversation(t, db, workspaceID, "anon-other", model.SupportConversationStatusResolved)
	source := seedCSATConversation(t, db, workspaceID, "anon-1", model.SupportConversationStatusResolved)
	mustExec(t, db, `UPDATE support_conversations SET merged_into_id = ?, customer_name = 'Someone else' WHERE id = ?`, target.ID, source.ID)
	if err := db.Create(&model.SupportConversationMerge{
		ID: uuid.NewString(), WorkspaceID: workspaceID, OperationID: uuid.NewString(),
		SourceID: source.ID, TargetID: target.ID, ActorID: uuid.NewString(), CreatedAt: time.Now(), CrossCustomer: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	// The other customer rated the merged conversation.
	if _, err := svc.SubmitConversationCSAT(ctx, model.SupportCSATSubmission{
		WorkspaceID: workspaceID, ConversationID: target.ID, Channel: model.SupportCSATChannelPortal, Rating: 1, Comment: "private",
	}, model.SupportInboxSettings{CSATEnabled: true}); err != nil {
		t.Fatal(err)
	}
	token := seedCSATSession(t, db, workspaceID, "anon-1")
	if _, err := svc.SubmitWidgetCSAT(ctx, token, source.ID, 5, ""); !errors.Is(err, ErrSupportCSATUnavailable) {
		t.Fatalf("cross-customer rating: %v", err)
	}
	conversations := []model.SupportConversation{*source}
	conversations[0].MergedIntoID = &target.ID
	if err := svc.hydrateConversationCSAT(ctx, workspaceID, conversations); err != nil {
		t.Fatal(err)
	}
	if conversations[0].CSAT != nil {
		t.Fatalf("another customer's rating leaked: %+v", conversations[0].CSAT)
	}
	accepts, err := svc.conversationAcceptsCSAT(ctx, workspaceID, &conversations[0])
	if err != nil || accepts {
		t.Fatalf("cross-customer source accepts rating: %v %v", accepts, err)
	}
}
